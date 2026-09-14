package tests

import (
	"bytes"
	"context"
	"encoding/json"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"strings"
	"testing"
	"time"
)

type classifiedOutputFixture struct {
	model.Client
	seen string
	item bool
}

func (m *classifiedOutputFixture) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	m.seen = r.DataClass
	raw, e := model.Fixture(r)
	if e != nil {
		return model.Result{}, e
	}
	var d contract.DecisionEnvelope
	json.Unmarshal(raw, &d)
	if m.item {
		a := createAction()
		a.Payload["title"] = "SYNTHETIC_PERSONAL_CANARY_864"
		d.Actions = []contract.ActionProposal{a}
	} else {
		d.Reply["questions"] = []any{map[string]any{"text": "OUTPUT_QUERY_864 confirm SYNTHETIC_PERSONAL_CANARY_864?", "item_id": nil}}
	}
	raw, e = json.Marshal(d)
	return model.Result{Output: raw}, e
}
func TestOutputClassPropagationQuestionAndItem(t *testing.T) {
	for _, itemMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "question", true: "item"}[itemMode], func(t *testing.T) {
			s, c, _ := setup(t)
			ctx := context.Background()
			c.Policy.AllowedClasses = []string{"SYNTHETIC", "PERSONAL"}
			m := &classifiedOutputFixture{Client: model.New(c), item: itemMode}
			session := contract.NewID()
			prior := input(session)
			prior.DataClass = "PERSONAL"
			prior.Text = "SYNTHETIC_PERSONAL_CANARY_864"
			pt, e := s.AcceptInput(ctx, prior, 100)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.FinishTurn(ctx, pt.ID, map[string]any{"text": "recorded", "evidence": []any{}}, nil, nil); e != nil {
				t.Fatal(e)
			}
			ask := input(session)
			ask.Text = "Produce the requested output from the previous value"
			at, e := s.AcceptInput(ctx, ask, 100)
			if e != nil {
				t.Fatal(e)
			}
			svc := core.Service{Store: s, Config: c, Model: m, GrantID: runtimeGrant(t, s)}
			if e = svc.Process(ctx, at); e != nil {
				t.Fatal(e)
			}
			if m.seen != "PERSONAL" {
				t.Fatal("wrong effective output", m.seen)
			}
			next := input(contract.NewID())
			nt, e := s.AcceptInput(ctx, next, 100)
			if e != nil {
				t.Fatal(e)
			}
			c.Policy.AllowedClasses = []string{"SYNTHETIC"}
			b := ctxbuild.Builder{Store: s, Config: c, Model: model.New(c)}
			if !itemMode {
				page, e := s.SearchMemoryPage(ctx, "OUTPUT_QUERY_864", nil, nil)
				if e != nil || len(page.Events) != 1 {
					t.Fatal(e, page)
				}
				if len(page.DataClasses) != 1 || page.DataClasses[0] != "PERSONAL" {
					t.Fatal(page.DataClasses)
				}
				b.RetrievalServed = true
				b.RetrievedEvents = page.Events
				b.RetrievedClasses = page.DataClasses
			} else {
				items, e := s.ListItems(ctx)
				if e != nil || len(items) != 1 {
					t.Fatal(e, items)
				}
				if class, e := contract.ReadClassification(items[0].Extensions); e != nil || class != "PERSONAL" {
					t.Fatal(class, e)
				}
				for _, ref := range items[0].Evidence {
					if ref.DataClass != "SYNTHETIC" {
						t.Fatal("original evidence rewritten")
					}
				}
				oldEvidence, _ := json.Marshal(items[0].Evidence)
				updated := items[0]
				updated.Revision++
				updated.Title = "updated"
				updated.Extensions, _ = contract.ClassifyExtensions(nil, "SYNTHETIC")
				if e = s.PutItem(ctx, updated, 1); e != nil {
					t.Fatal(e)
				}
				updated, e = s.GetItem(ctx, updated.ID)
				if e != nil {
					t.Fatal(e)
				}
				newEvidence, _ := json.Marshal(updated.Evidence)
				class, _ := contract.ReadClassification(updated.Extensions)
				if class != "PERSONAL" || !bytes.Equal(oldEvidence, newEvidence) {
					t.Fatal("update downgraded/changed evidence")
				}
				snap, e := s.Snapshot(ctx, "00000000-0000-4000-8000-000000000000")
				if e != nil {
					t.Fatal(e)
				}
				if e = ctxbuild.CheckDisclosure(c, snap, contract.InputEnvelope{}); e == nil {
					t.Fatal("background leaked derived Item")
				}
			}
			if _, _, e = b.Build(ctx, nt, time.Now()); e == nil || !strings.Contains(e.Error(), "DISCLOSURE_DENIED") {
				t.Fatal("SYN-only not blocked", e)
			}
			c.Policy.AllowedClasses = []string{"SYNTHETIC", "PERSONAL"}
			b.Config = c
			b.Model = model.New(c)
			req, _, e := b.Build(ctx, nt, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			if req.DataClass != "PERSONAL" {
				t.Fatal(req.DataClass)
			}
			wire, e := b.Model.Encode(req)
			if e != nil {
				t.Fatal(e)
			}
			if !itemMode && !bytes.Contains(wire, []byte("SYNTHETIC_PERSONAL_CANARY_864")) {
				t.Fatal("authorized positive control omitted canary")
			}
		})
	}
}
func TestOutputClassLegacyUnknownAndStrictHelper(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	in := input(contract.NewID())
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishTurn(ctx, turn.ID, map[string]any{"text": "legacy query", "evidence": []any{}}, nil, nil); e != nil {
		t.Fatal(e)
	}
	// Deliberately construct historical unmarked bytes in this disposable fixture only.
	_, e = s.DB.Exec("UPDATE conversation_event SET payload_json=json_remove(payload_json,'$.extensions.\"security.classification\"') WHERE role='ASSISTANT'")
	if e != nil {
		t.Fatal(e)
	}
	var before string
	s.DB.QueryRow("SELECT payload_json FROM conversation_event WHERE role='ASSISTANT'").Scan(&before)
	if _, e = s.SearchMemoryPage(ctx, "legacy query", nil, nil); e == nil || e.Error() != "OUTPUT_CLASS_UNKNOWN" {
		t.Fatal(e)
	}
	if _, e = s.Snapshot(ctx, in.SessionID); e == nil || e.Error() != "OUTPUT_CLASS_UNKNOWN" {
		t.Fatal(e)
	}
	var after string
	s.DB.QueryRow("SELECT payload_json FROM conversation_event WHERE role='ASSISTANT'").Scan(&after)
	if before != after {
		t.Fatal("legacy mutated")
	}
	if _, e = contract.ReadClassification(map[string]any{}); e == nil {
		t.Fatal("missing defaulted")
	}
	if _, e = contract.ClassifyExtensions(map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC", "extra": true}}, "PERSONAL"); e == nil {
		t.Fatal("illegal old label")
	}
	if e = contract.CheckNoClassification(map[string]any{"reply": map[string]any{"nested": map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}}); e == nil {
		t.Fatal("nested injection")
	}
}

func TestOutputClassMemoryAndLegacySnapshots(t *testing.T) {
	s, c, _ := setup(t)
	ctx := context.Background()
	c.Policy.AllowedClasses = []string{"SYNTHETIC", "PERSONAL"}
	p := model.New(c)
	session := contract.NewID()
	in := input(session)
	in.DataClass = "PERSONAL"
	in.Text = "SYNTHETIC_PERSONAL_MEMORY_CANARY"
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishTurn(ctx, turn.ID, map[string]any{"text": "recorded", "evidence": []any{}}, nil, nil); e != nil {
		t.Fatal(e)
	}
	svc := memory.Service{Store: s, Config: c, Model: p, Epoch: time.Now().UTC()}
	state, e := svc.Summarize(ctx, session)
	if e != nil {
		t.Fatal(e)
	}
	if class, e := contract.ReadClassification(state.Extensions); e != nil || class != "PERSONAL" {
		t.Fatal(class, e)
	}
	// Explicitly classified synthetic fixture bytes represent a PERSONAL input-derived Item.
	ext, _ := contract.ClassifyExtensions(nil, "PERSONAL")
	it := contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "synthetic memory fixture", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: ext}
	if e = s.PutItem(ctx, it, 0); e != nil {
		t.Fatal(e)
	}
	conscious, e := svc.RefreshSlot(ctx, 0, svc.Epoch)
	if e != nil {
		t.Fatal(e)
	}
	if class, e := contract.ReadClassification(conscious.Extensions); e != nil || class != "PERSONAL" {
		t.Fatal(class, e)
	}
	c.Policy.AllowedClasses = []string{"SYNTHETIC"}
	snap, e := s.Snapshot(ctx, session)
	if e != nil {
		t.Fatal(e)
	}
	if e = ctxbuild.CheckDisclosure(c, snap, contract.InputEnvelope{}); e == nil {
		t.Fatal("marked memory failed policy")
	}
	// Preserve deliberately constructed legacy bytes; never classify by remaining inputs.
	_, e = s.DB.Exec("UPDATE consciousness_snapshot SET payload_json=json_remove(payload_json,'$.extensions.\"security.classification\"')")
	if e != nil {
		t.Fatal(e)
	}
	var old string
	s.DB.QueryRow("SELECT payload_json FROM consciousness_snapshot").Scan(&old)
	if _, e = svc.RefreshSlot(ctx, 1, svc.Epoch.Add(24*time.Hour)); e == nil || e.Error() != "OUTPUT_CLASS_UNKNOWN" {
		t.Fatal("legacy consciousness repushed", e)
	}
	var after string
	s.DB.QueryRow("SELECT payload_json FROM consciousness_snapshot").Scan(&after)
	if old != after {
		t.Fatal("legacy rewritten")
	}
	// Remove only this probe's consciousness row to isolate legacy summary admission.
	if _, e = s.DB.Exec("DELETE FROM consciousness_snapshot"); e != nil {
		t.Fatal(e)
	}
	_, e = s.DB.Exec("UPDATE conversation_session SET payload_json=json_remove(payload_json,'$.extensions.\"security.classification\"') WHERE id=?", session)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Summarize(ctx, session); e == nil || e.Error() != "OUTPUT_CLASS_UNKNOWN" {
		t.Fatal("legacy summary repushed", e)
	}
}
