package tests

import (
	"context"
	"fmt"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/model"
	"strings"
	"testing"
	"time"
)

func TestRetrievalBudgetPreservesEvidenceAndRequiredAuthority(t *testing.T) {
	s, c, _ := setup(t)
	ctx := context.Background()
	old := input(contract.NewID())
	old.Text = "synthetic retrieval anchor password GREEN842"
	if _, e := s.AcceptInput(ctx, old, 100); e != nil {
		t.Fatal(e)
	}
	page, e := s.SearchMemoryPage(ctx, "GREEN842", nil, nil)
	if e != nil || len(page.Events) != 1 {
		t.Fatal(page, e)
	}
	now := contract.Now()
	ids := []string{}
	for i := 0; i < 3; i++ {
		id := contract.NewID()
		ids = append(ids, id)
		it := contract.Item{SchemaVersion: 1, ID: id, Revision: 1, Domain: "work", Kind: "TASK", Title: strings.Repeat("unrelated payload ", 25), Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: now, UpdatedAt: now, Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
		if i > 0 {
			it.DependencyIDs = []string{ids[i-1]}
		}
		if e = s.PutItem(ctx, it, 0); e != nil {
			t.Fatal(e)
		}
	}
	in := input(contract.NewID())
	in.Text = "retrieve GREEN842 from prior conversation"
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	page, e = s.SearchMemoryPage(ctx, "GREEN842", nil, nil)
	if e != nil || len(page.Events) != 2 {
		t.Fatal(page, e)
	}
	c.Limits.InputTokens = 32000
	b := ctxbuild.Builder{Store: s, Model: model.New(c), Config: c, RetrievalServed: true, RetrievedEvents: page.Events, RetrievedClasses: page.DataClasses}
	req, manifest, e := b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	m := req.Input.(map[string]any)
	ext := m["extensions"].(map[string]any)["context.retrieval"].(map[string]any)
	if len(ext["events"].([]contract.ConversationEvent)) != 1 {
		t.Fatal("retrieval was lost")
	}
	if ext["events"].([]contract.ConversationEvent)[0].TurnID == turn.ID {
		t.Fatal("current query cannot be historical evidence anchor")
	}
	for _, sec := range manifest.Sections {
		if sec["name"] == "items" && sec["selected_count"] != 0 {
			t.Fatal("unrelated fallback retained", sec)
		}
	}
	c.Limits.InputTokens = manifest.InputBytes + 300
	b.Config = c
	b.Model = model.New(c)
	turn.Input.Text = "retrieve current password and preserve required item " + ids[2]
	_, _, e = b.Build(ctx, turn, time.Now())
	if e == nil || !strings.Contains(e.Error(), "CONTEXT_REQUIRED_OVERFLOW") {
		t.Fatal("required item/evidence silently omitted", e)
	}
}

type retrievalBoundClient struct {
	model.Client
	limit int
}

func (c *retrievalBoundClient) Encode(r model.Request) ([]byte, error) {
	wire, e := c.Client.Encode(r)
	if e != nil {
		return nil, e
	}
	m := r.Input.(map[string]any)
	ext := m["extensions"].(map[string]any)
	if raw, ok := ext["context.retrieval"]; ok {
		events := raw.(map[string]any)["events"].([]contract.ConversationEvent)
		limit := c.limit
		if limit == 0 {
			limit = 1
		}
		if len(events) > limit {
			return nil, fmt.Errorf("CONTEXT_REQUIRED_OVERFLOW")
		}
	}
	return wire, nil
}
func TestServedRetrievalZeroAndGroupedEviction(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	in := input(contract.NewID())
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	fixtureItem := contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "unrelated fallback", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
	if e = s.PutItem(ctx, fixtureItem, 0); e != nil {
		t.Fatal(e)
	}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c, RetrievalServed: true}
	r, m, e := b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ext := r.Input.(map[string]any)["extensions"].(map[string]any)["context.retrieval"].(map[string]any)
	if len(ext["events"].([]contract.ConversationEvent)) != 0 {
		t.Fatal("zero hit not represented")
	}
	for _, sec := range m.Sections {
		if sec["name"] == "items" && sec["selected_count"] != 0 {
			t.Fatal("zero-hit restored fallback")
		}
	}
	makeEvent := func(text string, age int, evidence bool) contract.ConversationEvent {
		ev := contract.ConversationEvent{SchemaVersion: 1, ID: contract.NewID(), SessionID: in.SessionID, Sequence: age + 1, TurnID: contract.NewID(), Role: "MASTER", Text: text, CreatedAt: contract.Timestamp(time.Now().Add(-time.Duration(age) * time.Minute)), DeliveryState: "RECORDED", Evidence: []contract.EvidenceRef{}, Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
		if evidence {
			obj, e := s.PutObject(ctx, []byte(text), "text/plain", "SYNTHETIC")
			if e != nil {
				t.Fatal(e)
			}
			ev.Evidence = []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, Locator: "full", OriginID: obj.ID, DataClass: "SYNTHETIC"}}
		}
		return ev
	}
	newestNoEvidence := makeEvent("newest without evidence", 0, false)
	oldEvidence := makeEvent("old evidence", 3, true)
	olderEvidence := makeEvent("older evidence", 4, true)
	newEvidence := makeEvent("new evidence", 2, true)
	self := makeEvent("self must not count", 0, true)
	self.TurnID = turn.ID
	// The approved D09 dual-anchor revision replaces the historical one-event E/N expectation.
	b.Model = &retrievalBoundClient{Client: p, limit: 2}
	b.RetrievedEvents = []contract.ConversationEvent{newestNoEvidence, oldEvidence, olderEvidence, newEvidence, self}
	r, m, e = b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ext = r.Input.(map[string]any)["extensions"].(map[string]any)["context.retrieval"].(map[string]any)
	got := ext["events"].([]contract.ConversationEvent)
	if len(got) != 2 || got[0].ID != newEvidence.ID || got[1].ID != newestNoEvidence.ID || ext["omitted_count"] != 2 {
		t.Fatal("wrong E/N ordering or self/net-ref count", ext)
	}
	for _, sec := range m.Sections {
		if sec["name"] == "retrieved_evidence" && (sec["selected_count"] != 1 || sec["omitted_count"] != 2) {
			t.Fatal(sec)
		}
	}
	// A budget that fits only one event cannot discard either required class.
	b.Model = &retrievalBoundClient{Client: p, limit: 1}
	if _, _, err := b.Build(ctx, turn, time.Now()); err == nil || !strings.Contains(err.Error(), "CONTEXT_REQUIRED_OVERFLOW") {
		t.Fatal("dual required anchors were dropped", err)
	}
	b.RetrievedEvents = []contract.ConversationEvent{olderEvidence, oldEvidence, newEvidence, self}
	er, _, err := b.Build(ctx, turn, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ee := er.Input.(map[string]any)["extensions"].(map[string]any)["context.retrieval"].(map[string]any)["events"].([]contract.ConversationEvent)
	if len(ee) != 1 || ee[0].ID != newEvidence.ID {
		t.Fatal("E-only did not retain exactly latest E", ee)
	}
	oldNoEvidence := makeEvent("older without evidence", 2, false)
	b.RetrievedEvents = []contract.ConversationEvent{oldNoEvidence, newestNoEvidence}
	r, _, e = b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ext = r.Input.(map[string]any)["extensions"].(map[string]any)["context.retrieval"].(map[string]any)
	got = ext["events"].([]contract.ConversationEvent)
	if len(got) != 1 || got[0].ID != newestNoEvidence.ID || ext["omitted_count"] != 0 {
		t.Fatal("no-evidence fallback anchor wrong", ext)
	}
}
func TestRetrievalPageOmissionCountsUniqueReferenceLoss(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	for i := 0; i < 11; i++ {
		in := input(contract.NewID())
		in.Text = "shared query evidence"
		if _, e := s.AcceptInput(ctx, in, 100); e != nil {
			t.Fatal(e)
		}
	}
	page, e := s.SearchMemoryPage(ctx, "shared query", nil, nil)
	if e != nil || len(page.Events) != 10 || page.Cursor == nil || page.OmittedCount != 0 {
		t.Fatal("duplicate citations counted as coverage loss", page, e)
	}
	in := input(contract.NewID())
	in.Text = "oversized-query " + strings.Repeat("x", 2050)
	if _, e = s.AcceptInput(ctx, in, 100); e != nil {
		t.Fatal(e)
	}
	page, e = s.SearchMemoryPage(ctx, "oversized-query", nil, nil)
	if e != nil || len(page.Events) != 0 || page.OmittedCount != 1 {
		t.Fatal("oversized missing reference uncounted", page, e)
	}
}
