package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/executor"
	"secretarysimplified/ingest"
	"secretarysimplified/model"
	"secretarysimplified/platform"
	"secretarysimplified/store"
	"secretarysimplified/transport"
	"testing"
	"time"
)

type d08Bridge struct{ handler http.Handler }

func (b d08Bridge) Execute(ctx context.Context, run contract.JobRun, p contract.ExecutionPermit) (contract.ExecutorReceipt, error) {
	raw, _ := json.Marshal(map[string]any{"run": run, "permit": p})
	w := httptest.NewRecorder()
	b.handler.ServeHTTP(w, httptest.NewRequest("POST", "/internal/v1/work", bytes.NewReader(raw)).WithContext(ctx))
	var out contract.ExecutorReceipt
	if w.Code != 200 {
		return out, fmt.Errorf("work status %d: %s", w.Code, w.Body.String())
	}
	var envelope transport.Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &envelope); e != nil {
		return out, e
	}
	raw, _ = json.Marshal(envelope.Result)
	return out, contract.Decode("ExecutorReceipt", raw, &out)
}
func d08Command(t *testing.T, s *store.Store, cap string) contract.Command {
	t.Helper()
	args := map[string]any{}
	switch cap {
	case "alarm.play":
		obj, e := s.PutObject(context.Background(), []byte("muted audio fixture"), "audio/wav", "SYNTHETIC")
		if e != nil {
			t.Fatal(e)
		}
		args = map[string]any{"device_id": "muted-fixture", "audio_ref": obj, "max_duration_seconds": 1}
	case "briefing.build":
		args = map[string]any{"local_date": "2026-09-14", "timezone": "UTC", "item_ids": []string{}}
	case "source.sync":
		args = map[string]any{"source_id": contract.NewID()}
	}
	return contract.Command{SchemaVersion: 1, OperationKey: "d08", Capability: cap, CapabilityVersion: 1, Arguments: args, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
}
func d08Typed(t *testing.T, svc *core.Service, cmd contract.Command) contract.JobRun {
	t.Helper()
	cmd.Extensions = map[string]any{} // Public typed boundary receives content, never program classification.
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	a := contract.ActionProposal{OperationKey: cmd.OperationKey, Kind: "SUBMIT_TASK", Payload: map[string]any{"goal": "D08 criterion", "item_id": nil, "item_operation_key": nil, "deadline_at": nil, "command": cmd, "criteria": criteria}}
	turn, e := svc.TypedClass(context.Background(), contract.NewID(), contract.NewID(), []contract.ActionProposal{a}, "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	var raw string
	if e = svc.Store.DB.QueryRow(`SELECT payload_json FROM job_run WHERE json_extract(payload_json,'$.root_id')=?`, turn.IntentID).Scan(&raw); e != nil { // root is carried by task
		e = svc.Store.DB.QueryRow(`SELECT r.payload_json FROM job_run r JOIN task t ON t.id=r.task_id WHERE t.root_id=?`, turn.IntentID).Scan(&raw)
	}
	if e != nil {
		t.Fatal(e)
	}
	var run contract.JobRun
	if e = contract.Decode("JobRun", []byte(raw), &run); e != nil {
		t.Fatal(e)
	}
	return run
}

func TestD08PublicAdmissionExecutionAndNegativeEvidence(t *testing.T) {
	for _, cap := range []string{"alarm.play", "briefing.build", "source.sync"} {
		t.Run(cap, func(t *testing.T) {
			s, c, p := setup(t)
			ctx := context.Background()
			g := probeGrant(t, s, []string{cap}, []string{})
			cmd := d08Command(t, s, cap)
			if cap == "source.sync" {
				var raw string
				s.DB.QueryRow(`SELECT payload_json FROM authorization_grant WHERE id=?`, g).Scan(&raw)
				var grant contract.AuthorizationGrant
				contract.Decode("AuthorizationGrant", []byte(raw), &grant)
				grant.Revision++
				grant.Scope["source_ids"] = []string{cmd.Arguments["source_id"].(string)}
				if e := s.PutGrant(ctx, grant); e != nil {
					t.Fatal(e)
				}
			}
			if cap == "source.sync" {
				path := filepath.Join(c.DataDir, "fixture.json")
				os.WriteFile(path, []byte(`[]`), 0600)
				c.SourceConfigs = []config.SourceConfig{{ID: cmd.Arguments["source_id"].(string), FixturePath: path}}
			}
			svc := &core.Service{Store: s, Config: c, Model: p, GrantID: g}
			run := d08Typed(t, svc, cmd)
			if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || ok {
				t.Fatal("missing evidence passed", ok, e)
			}
			runner := executor.New(s, &executor.Options{Core: d08Bridge{svc.InternalHandler()}})
			if work, e := runner.Step(ctx); e != nil || !work {
				t.Fatal("execute", work, e)
			}
			if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || !ok {
				t.Fatal("valid evidence failed", ok, e)
			}
			switch cap {
			case "alarm.play":
				var session string
				s.DB.QueryRow(`SELECT id FROM alarm_session WHERE run_id=?`, run.ID).Scan(&session)
				if e := s.AlarmControl(ctx, session, contract.NewID(), 1, time.Now()); e != nil {
					t.Fatal(e)
				}
				if _, e := s.ScheduleStep(ctx, time.Now().Add(2*time.Second)); e != nil {
					t.Fatal(e)
				}
				runner.Options.Clock = &platform.ManualClock{T: time.Now().Add(3 * time.Second)}
				var nextRaw string
				s.DB.QueryRow(`SELECT payload_json FROM job_run WHERE id<>?`, run.ID).Scan(&nextRaw)
				var next contract.JobRun
				contract.Decode("JobRun", []byte(nextRaw), &next)
				if ok, e := s.VerifyTask(ctx, next.TaskID, ""); e != nil || ok {
					t.Fatal("old session verified snooze", ok, e)
				}
				if work, e := runner.Step(ctx); e != nil || !work {
					t.Fatal(work, e)
				}
				if ok, e := s.VerifyTask(ctx, next.TaskID, ""); e != nil || !ok {
					t.Fatal("new session missing", ok, e)
				}
				if e := s.ExpireMutedAlarms(ctx, time.Now().Add(10*time.Second)); e != nil {
					t.Fatal(e)
				}
				if ok, e := s.VerifyTask(ctx, next.TaskID, ""); e != nil || !ok {
					t.Fatal("STOPPED session blocked", ok, e)
				}
			case "briefing.build":
				var raw string
				s.DB.QueryRow(`SELECT payload_json FROM executor_receipt WHERE run_id=?`, run.ID).Scan(&raw)
				var receipt contract.ExecutorReceipt
				contract.Decode("ExecutorReceipt", []byte(raw), &receipt)
				if len(receipt.Artifacts) == 0 {
					t.Fatal("empty fixture artifact")
				}
				path := filepath.Join(s.ObjectsDir, receipt.Artifacts[0].RelativePath)
				os.WriteFile(path, []byte("tampered"), 0600)
				if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || ok {
					t.Fatal("tamper passed", ok, e)
				}
			case "source.sync":
				source := cmd.Arguments["source_id"].(string)
				second := d08Typed(t, svc, cmd)
				if ok, e := s.VerifyTask(ctx, second.TaskID, ""); e != nil || ok {
					t.Fatal("another run sync passed", ok, e)
				}
				if _, e := (&ingest.Service{Store: s}).Sync(ctx, source, nil, store.SourceSyncProvenance{RunID: run.ID, AttemptNo: 99, FencingToken: 99}); e == nil {
					t.Fatal("forged provenance accepted")
				}
				if work, e := runner.Step(ctx); e != nil || !work {
					t.Fatal(work, e)
				}
				if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || !ok {
					t.Fatal("later sync erased proof", ok, e)
				}
				s.DB.Exec(`UPDATE source_state SET cursor_json=NULL WHERE id=?`, source)
				if ok, e := s.VerifyTask(ctx, second.TaskID, ""); e != nil || ok {
					t.Fatal("null cursor passed", ok, e)
				}
			}
		})
	}
}

func TestD08BriefingRejectsMissingEmptyAndStaleReceipts(t *testing.T) {
	for _, mode := range []string{"empty_artifact_list", "empty_object", "old_fence", "old_attempt", "failed_receipt"} {
		t.Run(mode, func(t *testing.T) {
			s, c, p := setup(t)
			ctx := context.Background()
			g := probeGrant(t, s, []string{"briefing.build"}, []string{})
			svc := &core.Service{Store: s, Config: c, Model: p, GrantID: g}
			run := d08Typed(t, svc, d08Command(t, s, "briefing.build"))
			runner := executor.New(s, &executor.Options{Core: d08Bridge{svc.InternalHandler()}})
			if _, e := runner.Step(ctx); e != nil {
				t.Fatal(e)
			}
			var raw string
			s.DB.QueryRow(`SELECT payload_json FROM executor_receipt WHERE run_id=?`, run.ID).Scan(&raw)
			var receipt contract.ExecutorReceipt
			contract.Decode("ExecutorReceipt", []byte(raw), &receipt)
			switch mode {
			case "empty_artifact_list":
				receipt.Artifacts = []contract.ObjectRef{}
			case "empty_object":
				obj, e := s.PutObject(ctx, []byte{}, "text/plain", "SYNTHETIC")
				if e != nil {
					t.Fatal(e)
				}
				receipt.Artifacts = []contract.ObjectRef{obj}
			case "failed_receipt":
				receipt.Status = "FAILED"
			case "old_fence", "old_attempt":
				current, e := s.GetRun(ctx, run.ID)
				if e != nil {
					t.Fatal(e)
				}
				if mode == "old_fence" {
					current.FencingToken++
				} else {
					current.AttemptNo++
				}
				b, _ := json.Marshal(current)
				if _, e = s.DB.Exec(`UPDATE job_run SET fencing_token=?,attempt_no=?,payload_json=? WHERE id=?`, current.FencingToken, current.AttemptNo, string(b), run.ID); e != nil {
					t.Fatal(e)
				}
			}
			b, _ := json.Marshal(receipt)
			hash, _ := contract.ValueHash(receipt)
			if _, e := s.DB.Exec(`UPDATE executor_receipt SET payload_json=?,payload_hash=? WHERE id=?`, string(b), hash, receipt.ID); e != nil {
				t.Fatal(e)
			}
			if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || ok {
				t.Fatal("invalid receipt passed", ok, e)
			}
		})
	}
}

func TestD08PublicRejectsChangedCriterionAndStrictExtension(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	g := probeGrant(t, s, []string{"alarm.play"}, []string{})
	svc := &core.Service{Store: s, Config: c, Model: p, GrantID: g}
	cmd := d08Command(t, s, "alarm.play")
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	criteria[0].Expected["device_id"] = "different"
	a := contract.ActionProposal{OperationKey: "changed", Kind: "SUBMIT_TASK", Payload: map[string]any{"goal": "reject altered criterion", "item_id": nil, "item_operation_key": nil, "deadline_at": nil, "command": cmd, "criteria": criteria}}
	if _, e = svc.TypedClass(ctx, contract.NewID(), contract.NewID(), []contract.ActionProposal{a}, "SYNTHETIC"); e == nil {
		t.Fatal("changed criterion accepted")
	}
	criteria, e = store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	criteria[0].Expected["extra"] = "forbidden"
	if e = contract.Validate("Criterion", criteria[0]); e == nil {
		t.Fatal("extra expected accepted")
	}
	obj, e := s.PutObject(ctx, []byte("extension fixture"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	obj.Extensions["runtime.source_sync"] = map[string]any{"run_id": contract.NewID(), "attempt_no": 1, "fencing_token": 1, "records_processed": 0, "extra": true}
	if e = contract.Validate("ObjectRef", obj); e == nil {
		t.Fatal("extra provenance accepted")
	}
}

type d08InvalidBriefing struct{ model.Client }

func (p d08InvalidBriefing) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	v, e := p.Client.Generate(ctx, r)
	if e != nil {
		return v, e
	}
	var d contract.DecisionEnvelope
	if e = contract.Decode("DecisionEnvelope", v.Output, &d); e != nil {
		return v, e
	}
	d.Actions = []contract.ActionProposal{createAction()}
	v.Output, e = json.Marshal(d)
	return v, e
}
func TestD08BriefingModelActionsNeverVerify(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	g := probeGrant(t, s, []string{"briefing.build"}, []string{})
	svc := &core.Service{Store: s, Config: c, Model: d08InvalidBriefing{p}, GrantID: g}
	run := d08Typed(t, svc, d08Command(t, s, "briefing.build"))
	runner := executor.New(s, &executor.Options{Core: d08Bridge{svc.InternalHandler()}})
	if _, e := runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || ok {
		t.Fatal("model self action passed", ok, e)
	}
	var raw string
	s.DB.QueryRow(`SELECT payload_json FROM executor_receipt WHERE run_id=?`, run.ID).Scan(&raw)
	var receipt contract.ExecutorReceipt
	contract.Decode("ExecutorReceipt", []byte(raw), &receipt)
	if receipt.Status != "FAILED" || len(receipt.Artifacts) != 0 {
		t.Fatal("invalid briefing produced artifact", receipt.Status)
	}
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM item`).Scan(&n)
	if n != 0 {
		t.Fatal("briefing action applied")
	}
}
