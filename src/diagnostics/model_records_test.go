package diagnostics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestRecordingModelPersistsActualFixtureEvidence(t *testing.T) {
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	c := config.Default(d)
	dir := filepath.Join(d, "reports", "model_calls")
	r := RecordingModel{Inner: model.New(c), Store: s, Config: c, Dir: dir}
	turn, e := s.AcceptInput(context.Background(), contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: contract.NewID(), PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: "fixture evidence", DataClass: "SYNTHETIC", AttachmentRefs: []contract.ObjectRef{}, Extensions: map[string]any{}}, 100)
	if e != nil {
		t.Fatal(e)
	}
	builder := ctxbuild.Builder{Store: s, Model: r.Inner, Config: c}
	req, manifest, e := builder.Build(context.Background(), turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	id := manifest.ID
	result, e := r.Generate(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	files, e := os.ReadDir(dir)
	if e != nil || len(files) != 2 {
		t.Fatal(files, e)
	}
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" || len(f.Name()) != 41 {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(dir, f.Name()))
		var rec contract.ModelCallRecord
		if e = contract.Decode("ModelCallRecord", b, &rec); e != nil {
			t.Fatal(e)
		}
		if rec.Status != "SUCCEEDED" || rec.OutputRef == nil || rec.ContextID != id {
			t.Fatal(rec)
		}
		out, e := s.ReadObject(context.Background(), rec.OutputRef.ID)
		if e != nil || string(out) != string(result.Output) {
			t.Fatal("not actual model output", e)
		}
		var payload any
		if json.Unmarshal(out, &payload) != nil {
			t.Fatal("invalid output")
		}
	}
}
