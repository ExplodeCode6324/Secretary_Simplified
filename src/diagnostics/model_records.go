package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"strings"
)

// RecordingModel adds local evidence around a model.Client. Provider code remains
// independent of business storage; only synthetic request/response bodies archive.
type RecordingModel struct {
	Inner  model.Client
	Store  *store.Store
	Config config.Config
	Dir    string
}

func (r *RecordingModel) Encode(req model.Request) ([]byte, error) { return r.Inner.Encode(req) }
func (r *RecordingModel) Generate(ctx context.Context, req model.Request) (result model.Result, err error) {
	if e := model.ValidateInputPolicy(r.Config, req); e != nil {
		return result, e
	}
	wire, e := r.Inner.Encode(req)
	if e != nil {
		return result, e
	}
	root, id := req.RootID, req.ContextID
	if root == "" {
		root = contract.NewID()
	}
	if id == "" {
		id = contract.NewID()
	}
	rec := contract.ModelCallRecord{SchemaVersion: 1, CallID: contract.NewID(), RootID: root, ContextID: id, ProviderProfile: r.Config.Model.Profile, ModelID: r.Config.Model.Model, RequestHash: contract.Hash(wire), StartedAt: contract.Now(), InputTokens: len(wire), CountMode: "CONSERVATIVE_ESTIMATE", Status: "RUNNING", Extensions: map[string]any{}}
	if e = r.save(rec); e != nil {
		return result, e
	}
	if req.DataClass == "SYNTHETIC" {
		ref, e := r.Store.PutObject(ctx, wire, "application/json", "SYNTHETIC")
		if e != nil {
			return result, e
		}
		b, _ := json.MarshalIndent(ref, "", "  ")
		if e = atomicDiagnostic(filepath.Join(r.Dir, rec.CallID+".request.json"), b); e != nil {
			return result, e
		}
	}
	result, err = r.Inner.Generate(ctx, req)
	result.CallID = rec.CallID
	finished := contract.Now()
	rec.FinishedAt = &finished
	rec.Status = "SUCCEEDED"
	if err != nil {
		rec.Status = "FAILED"
		code := "MODEL_CALL_FAILED"
		if strings.HasPrefix(err.Error(), "MODEL_") && !strings.ContainsAny(err.Error(), " \n\r") {
			code = err.Error()
		}
		if errors.Is(err, context.Canceled) {
			rec.Status = "CANCELLED"
			code = "CANCELLED"
		}
		rec.ErrorCode = &code
	}
	if result.InputTokens > 0 || result.OutputTokens > 0 {
		rec.CountMode = "PROVIDER"
		rec.InputTokens = result.InputTokens
		rec.OutputTokens = result.OutputTokens
	} else {
		rec.OutputTokens = len(result.Output)
	}
	archivedOutput := result.Output
	if len(archivedOutput) == 0 && len(result.RawResponse) > 0 {
		archivedOutput = result.RawResponse
	}
	if len(archivedOutput) > 0 && req.DataClass == "SYNTHETIC" {
		ref, e := r.Store.PutObject(context.WithoutCancel(ctx), archivedOutput, "application/json", "SYNTHETIC")
		if e != nil {
			return result, e
		}
		rec.OutputRef = &ref
	}
	if e = r.save(rec); e != nil {
		return result, e
	}
	return result, err
}
func (r *RecordingModel) save(v contract.ModelCallRecord) error {
	if e := contract.Validate("ModelCallRecord", v); e != nil {
		return e
	}
	if e := os.MkdirAll(r.Dir, 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return atomicDiagnostic(filepath.Join(r.Dir, v.CallID+".json"), b)
}
func atomicDiagnostic(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".record-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}

func Recorded(inner model.Client, s *store.Store, c config.Config) model.Client {
	if _, ok := inner.(*RecordingModel); ok {
		return inner
	}
	dir := c.DataDir
	if dir == "" {
		dir = filepath.Dir(s.ObjectsDir)
	}
	return &RecordingModel{Inner: inner, Store: s, Config: c, Dir: filepath.Join(dir, "reports", "model_calls")}
}

// RecordDecisionOutcome is separate from provider status: a valid JSON response
// can still fail program semantics or transaction admission.
func RecordDecisionOutcome(c config.Config, s *store.Store, callID, contextID, intentID string, attempt int, status, reason string) error {
	dir := c.DataDir
	if dir == "" {
		dir = filepath.Dir(s.ObjectsDir)
	}
	dir = filepath.Join(dir, "reports", "decision_attempts")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(map[string]any{"schema_version": 1, "call_id": callID, "context_id": contextID, "intent_id": intentID, "attempt": attempt, "status": status, "reason": reason, "recorded_at": contract.Now()}, "", "  ")
	if e != nil {
		return e
	}
	id := callID
	if id == "" {
		id = contract.NewID()
	}
	return atomicDiagnostic(filepath.Join(dir, id+".json"), b)
}
