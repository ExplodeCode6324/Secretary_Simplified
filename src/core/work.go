package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/diagnostics"
	"secretarysimplified/ingest"
	"secretarysimplified/memory"
	"secretarysimplified/policy"
	"secretarysimplified/store"
	"secretarysimplified/transport"
	"time"
)

func (s *Service) InternalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/v1/health", func(w http.ResponseWriter, r *http.Request) {
		transport.Reply(w, 200, "", map[string]any{"core": "ONLINE"}, nil)
	})
	mux.HandleFunc("GET /internal/v1/work/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.GetCoreWork(r.Context(), r.PathValue("id"))
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("POST /internal/v1/work", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Run    contract.JobRun          `json:"run"`
			Permit contract.ExecutionPermit `json:"permit"`
		}
		if e := transport.Read(r, &in); e != nil {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		if e := contract.Validate("JobRun", in.Run); e != nil {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		if e := contract.Validate("ExecutionPermit", in.Permit); e != nil || in.Permit.RunID != in.Run.ID || in.Permit.FencingToken != in.Run.FencingToken || in.Permit.Capability != in.Run.Command.Capability {
			transport.Reply(w, 403, "", nil, errors.New("PERMISSION_DENIED"))
			return
		}
		if e := s.Store.ValidateIncomingWork(r.Context(), in.Run, in.Permit); e != nil {
			transport.Reply(w, 403, "", nil, e)
			return
		}
		old, e := s.Store.BeginWork(r.Context(), in.Run)
		if e != nil {
			transport.Reply(w, 409, "", nil, e)
			return
		}
		if old != nil {
			transport.Reply(w, 200, "", old, nil)
			return
		}
		receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: in.Run.ID, AttemptNo: in.Run.AttemptNo, FencingToken: in.Run.FencingToken, ReceiptKey: fmt.Sprintf("attempt-%d-final", in.Run.AttemptNo), Status: "SUCCEEDED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Now(), Extensions: map[string]any{}}
		uncertainEffect := false
		switch in.Run.Command.Capability {
		case "world.update":
			id, _ := in.Run.Command.Arguments["proposal_id"].(string)
			p, err := s.Store.GetProposal(r.Context(), id)
			if err != nil {
				e = err
				break
			}
			hash, err := contract.ValueHash(p)
			if err != nil {
				e = err
				break
			}
			_, e = s.Store.CommitWorldAtomic(r.Context(), p, func(tx *sql.Tx) error {
				return policy.ConsumePermitTx(r.Context(), tx, in.Permit.ID, hash, p.EntityID, p.Predicate, p.Operation, time.Now())
			}, func(tx *sql.Tx, fact contract.WorldFact) error {
				receipt.EffectObserved = true
				return store.FinishWorkTx(r.Context(), tx, in.Run, receipt)
			})
			if e == nil {
				transport.Reply(w, 200, "", receipt, nil)
				return
			}
		case "memory.refresh":
			epoch, err := time.Parse(time.RFC3339Nano, s.Config.Epoch)
			if err != nil {
				e = err
				break
			}
			svc := memory.Service{Store: s.Store, Model: s.Model, Epoch: epoch, Config: s.Config}
			slot, ok := in.Run.Command.Arguments["slot"].(float64)
			if !ok || slot < 0 || slot != float64(int(slot)) {
				e = errors.New("INVALID_SLOT")
				break
			}
			_, e = svc.RefreshSlot(r.Context(), int(slot), time.Now())
		case "source.sync":
			id, _ := in.Run.Command.Arguments["source_id"].(string)
			path := ""
			for _, source := range s.Config.SourceConfigs {
				if source.ID == id {
					path = source.FixturePath
				}
			}
			if path == "" {
				e = errors.New("SOURCE_NOT_CONFIGURED")
				break
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(s.Config.DataDir, path)
			}
			b, err := readSourceFixture(path)
			if err != nil {
				e = err
				break
			}
			var records []ingest.FixtureRecord
			if _, e = contract.ParseJSON(b); e != nil {
				break
			}
			if e = json.Unmarshal(b, &records); e != nil {
				break
			}
			adapter := ingest.Service{Store: s.Store, QuarantineDir: filepath.Join(s.Config.DataDir, "reports", "quarantine")}
			uncertainEffect = true // Sync may commit independent source records before an error.
			_, e = adapter.Sync(r.Context(), id, records, store.SourceSyncProvenance{RunID: in.Run.ID, AttemptNo: in.Run.AttemptNo, FencingToken: in.Run.FencingToken})
		case "briefing.build":
			now := contract.Now()
			turn := contract.InputTurn{SchemaVersion: 1, ID: in.Run.ID, SessionID: in.Run.ID, IntentID: in.Run.ID, RequestID: in.Run.ID, Input: contract.InputEnvelope{SchemaVersion: 1, RequestID: in.Run.ID, SessionID: in.Run.ID, PrincipalID: "system", Origin: "SYSTEM", ReceivedAt: now, Text: "根据当前权威事项与事实生成简短中文晨报，明确未知和未完成事项。不要生成任何actions或controls，不宣称业务已完成。", AttachmentRefs: []contract.ObjectRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}}
			builder := ctxbuild.Builder{Store: s.Store, Model: s.Model, Config: s.Config, TaskLocal: true}
			req, manifest, err := builder.Build(r.Context(), turn, time.Now())
			if err != nil {
				e = err
				break
			}
			req.Background = true
			result, err := diagnostics.Recorded(s.Model, s.Store, s.Config).Generate(r.Context(), req)
			if err != nil {
				e = err
				break
			}
			var d contract.DecisionEnvelope
			if e = contract.Decode("DecisionEnvelope", result.Output, &d); e != nil {
				break
			}
			if d.ContextID != manifest.ID || len(d.Actions) != 0 || len(d.Controls) != 0 {
				e = errors.New("INVALID_BRIEFING")
				break
			}
			text, _ := d.Reply["text"].(string)
			if e = r.Context().Err(); e != nil {
				break
			}
			uncertainEffect = true
			e = s.Store.WriteObjects(r.Context(), func(tx *sql.Tx, objects *store.ObjectWriter) error {
				obj, err := objects.Put(r.Context(), []byte(text), "text/plain", req.DataClass)
				if err != nil {
					return err
				}
				receipt.Artifacts = []contract.ObjectRef{obj}
				receipt.EffectObserved = true
				return store.FinishWorkTx(r.Context(), tx, in.Run, receipt)
			})
			if e == nil {
				transport.Reply(w, 200, "", receipt, nil)
				return
			}
			// The object reference and receipt rolled back together. Do not publish
			// a reference to rollback data; persistence failure remains conservative unknown.
			receipt.Artifacts = []contract.ObjectRef{}
			receipt.EffectObserved = false

		case "memory.search":
			q, _ := in.Run.Command.Arguments["query"].(string)
			_, e = s.Store.SearchMemory(r.Context(), q, 10)
		default:
			e = errors.New("UNKNOWN_CAPABILITY")
		}
		// The independent, bounded context is for settlement only, never model/business work.
		settleCtx, settleCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer settleCancel()
		if e != nil {
			if in.Run.Command.Capability == "memory.refresh" {
				proof, proofErr := s.Store.ReconcileMemoryWork(settleCtx, in.Run)
				if proofErr == nil && proof != nil {
					transport.Reply(w, 200, "", proof, nil)
					return
				}
				if proofErr != nil {
					uncertainEffect = true
				}
			}
			receipt.Status = "FAILED"
			code := "CORE_WORK_FAILED"
			if r.Context().Err() != nil {
				code = "CORE_WORK_CANCELLED_NO_EFFECT"
			}
			if uncertainEffect {
				receipt.Status = "RESULT_UNKNOWN"
				code = "CORE_WORK_EFFECT_UNKNOWN"
			}
			if e.Error() == "INPUT_TOO_LARGE" || e.Error() == "FIXTURE_UNAVAILABLE" {
				code = e.Error()
			}
			receipt.ErrorCode = &code
		} else {
			receipt.EffectObserved = true
		}
		if e = s.Store.FinishWork(settleCtx, in.Run, receipt); e != nil {
			log.Printf("CORE_WORK_SETTLEMENT_FAILED run=%s attempt=%d fence=%d", in.Run.ID, in.Run.AttemptNo, in.Run.FencingToken)
			transport.Reply(w, 409, "", nil, e)
			return
		}
		transport.Reply(w, 200, "", receipt, nil)
	})
	return mux
}

var _ = json.Marshal
