package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/model"
	"testing"
	"time"
)

type retrievalClient struct {
	Base        model.Client
	Calls       int
	IDs         []string
	SawEvidence bool
	AlwaysRead  bool
}

func (c *retrievalClient) Encode(r model.Request) ([]byte, error) { return c.Base.Encode(r) }
func (c *retrievalClient) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	c.Calls++
	c.IDs = append(c.IDs, r.ContextID)
	controls := []contract.Control{}
	if c.Calls == 1 || c.AlwaysRead {
		controls = append(controls, contract.Control{Kind: "READ_MEMORY", Payload: map[string]any{"query": "needle", "entity_ids": []string{}, "cursor": nil}})
	} else {
		m := r.Input.(map[string]any)
		refs := m["retrieved_evidence"].([]contract.EvidenceRef)
		c.SawEvidence = len(refs) > 0
	}
	v := contract.DecisionEnvelope{SchemaVersion: 1, ContextID: r.ContextID, Reply: map[string]any{"text": "retrieval complete", "evidence": []any{}}, Actions: []contract.ActionProposal{}, Controls: controls, Extensions: map[string]any{}}
	b, e := json.Marshal(v)
	return model.Result{Output: b}, e
}
func TestReadMemoryRebuildsContextAndPersistsAttempt(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	old := input(contract.NewID())
	old.Text = "historical needle evidence"
	if _, e := s.AcceptInput(ctx, old, 100); e != nil {
		t.Fatal(e)
	}
	current := input(contract.NewID())
	current.Text = "retrieve needle"
	turn, e := s.AcceptInput(ctx, current, 100)
	if e != nil {
		t.Fatal(e)
	}
	fake := &retrievalClient{Base: p}
	svc := core.Service{Store: s, Model: fake, Config: c}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	if fake.Calls != 2 || !fake.SawEvidence || fake.IDs[0] == fake.IDs[1] {
		t.Fatal(fake)
	}
	files, e := os.ReadDir(filepath.Join(c.DataDir, "reports", "decision_attempts"))
	if e != nil || len(files) != 2 {
		t.Fatal("missing semantic attempt evidence", len(files), e)
	}
}
func TestReadMemoryRejectsHistoricalPersonalAndCapsCalls(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	old := input(contract.NewID())
	old.Text = "personal needle"
	old.DataClass = "PERSONAL"
	if _, e := s.AcceptInput(ctx, old, 100); e != nil {
		t.Fatal(e)
	}
	turn, e := s.AcceptInput(ctx, input(contract.NewID()), 100)
	if e != nil {
		t.Fatal(e)
	}
	fake := &retrievalClient{Base: p}
	svc := core.Service{Store: s, Model: fake, Config: c}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	if fake.Calls != 1 {
		t.Fatal("retrieved personal content reached model", fake.Calls)
	}
	s2, c2, p2 := setup(t)
	turn, e = s2.AcceptInput(ctx, input(contract.NewID()), 100)
	if e != nil {
		t.Fatal(e)
	}
	repeated := &retrievalClient{Base: p2, AlwaysRead: true}
	svc = core.Service{Store: s2, Model: repeated, Config: c2}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	if repeated.Calls != 4 {
		t.Fatal("retrieval root budget not bounded", repeated.Calls)
	}
}
func TestProviderRejectsArbitraryMapAndNestedClassEscalation(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	if _, e := p.Encode(model.Request{OutputType: "DecisionEnvelope", DataClass: "SYNTHETIC", Input: map[string]any{"secret_marker": "DO_NOT_SEND"}}); e == nil {
		t.Fatal("untyped map reached wire")
	}
	turn, e := s.AcceptInput(ctx, input(contract.NewID()), 100)
	if e != nil {
		t.Fatal(e)
	}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, _, e := b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	m := req.Input.(map[string]any)
	env := m["current_input"].(contract.InputEnvelope)
	env.DataClass = "SECRET"
	m["current_input"] = env
	if _, e = p.Encode(req); e == nil {
		t.Fatal("nested secret mislabeled synthetic reached wire")
	}
}
