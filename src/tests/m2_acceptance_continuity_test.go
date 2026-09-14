package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"testing"
	"time"
)

func continuityItem() contract.Item {
	return contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "continuity target", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
}
func TestAcceptanceA08IntradayAuthorityOverridesEarlierSnapshot(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	it := continuityItem()
	if e := s.PutItem(ctx, it, 0); e != nil {
		t.Fatal(e)
	}
	epoch := time.Now().UTC()
	mem := memory.Service{Store: s, Model: p, Config: c, Epoch: epoch}
	first, e := mem.RefreshSlot(ctx, 0, epoch)
	if e != nil {
		t.Fatal(e)
	}
	it.Revision++
	it.Status = "DONE"
	it.UpdatedAt = contract.Now()
	if e = s.PutItem(ctx, it, 1); e != nil {
		t.Fatal(e)
	}
	in := input(contract.NewID())
	in.Text = "show exact current state of " + it.ID
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	builder := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, _, e := builder.Build(ctx, turn, epoch.Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	wire := req.Input.(map[string]any)
	live := wire["live"].(contract.LiveWorldStateInput)
	if len(live.Items) != 1 || live.Items[0].ID != it.ID || live.Items[0].Revision != 2 || live.Items[0].Status != "DONE" {
		t.Fatal("intraday authority stale", live.Items)
	}
	if wire["consciousness"] != nil {
		t.Fatal("snapshot with old Item reference was not removed")
	}
	same, e := s.ConsciousnessAt(ctx, 0)
	if e != nil || same.ID != first.ID {
		t.Fatal("intraday update regenerated slot", e)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM consciousness_snapshot").Scan(&n)
	if n != 1 {
		t.Fatal("extra slot", n)
	}
}
func TestAcceptanceA09PersistedQuestionsReferencesAndSummaryGap(t *testing.T) {
	s, c, _ := setup(t)
	ctx := context.Background()
	it := continuityItem()
	if e := s.PutItem(ctx, it, 0); e != nil {
		t.Fatal(e)
	}
	in := input(contract.NewID())
	in.Text = "continuity agreement: use the blue folder for item " + it.ID + "; when should it be delivered?"
	if _, e := s.AcceptInput(ctx, in, 100); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(ctx, in.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	old := snap.Conversation
	v := old
	v.Revision++
	v.ThroughSequence = 1
	v.SummaryFromSequence = 1
	v.Summary = "Use the blue folder; delivery question unresolved."
	v.FocusEntityIDs = []string{it.ID}
	v.CommitmentItemIDs = []string{it.ID}
	question := contract.NewID()
	v.PendingQuestions = []map[string]any{{"id": question, "text": "When should it be delivered?", "item_id": it.ID, "created_sequence": 1, "resolved": false}}
	v.Extensions, e = contract.ClassifyExtensions(v.Extensions, "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SaveConversation(ctx, v, old.Revision, 0); e != nil {
		t.Fatal(e)
	}
	s.Close()
	reopened, e := store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	loaded, e := reopened.Snapshot(ctx, in.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(v)
	b, _ := json.Marshal(loaded.Conversation)
	if string(a) != string(b) {
		t.Fatal("summary/questions/Item references changed across reopen")
	}
	page, e := reopened.SearchMemoryPage(ctx, "continuity agreement", []string{it.ID}, nil)
	if e != nil || len(page.Events) != 1 || page.Events[0].Text != in.Text {
		t.Fatal("cross-session original retrieval lost", page, e)
	}
	bad := loaded.Conversation
	bad.Revision++
	bad.ThroughSequence = 3
	if e = reopened.SaveConversation(ctx, bad, loaded.Conversation.Revision, 1); e == nil {
		t.Fatal("missing original coverage accepted")
	}
	bad = loaded.Conversation
	bad.Revision++
	if e = reopened.SaveConversation(ctx, bad, loaded.Conversation.Revision-1, 1); e == nil {
		t.Fatal("stale summary CAS accepted")
	}
	again, e := reopened.Snapshot(ctx, in.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = json.Marshal(again.Conversation)
	if string(a) != string(b) {
		t.Fatal("rejected summary altered persistent state")
	}
}

type worldSummaryFixture struct {
	model.Client
	Fact contract.WorldFact
}

func (c *worldSummaryFixture) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	draft := contract.ConsciousnessDraft{SchemaVersion: 1, FocalGoals: []contract.FocusEntry{{Entity: contract.ReadRef{EntityType: "WorldFact", ID: c.Fact.ID, Revision: c.Fact.Revision}, Reason: "historical preference", Evidence: []contract.EvidenceRef{}}}, PriorityItems: []contract.FocusEntry{}, OpenLoops: []contract.FocusEntry{}, ImportantChanges: []contract.FocusEntry{}, Uncertainties: []string{}, BriefSummary: "The historical preferred drink is tea.", Extensions: map[string]any{}}
	b, e := json.Marshal(draft)
	return model.Result{Output: b}, e
}
func TestAcceptanceA07RetractionInvalidatesDerivedWorldSnapshot(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	obj, e := s.PutObject(ctx, []byte("synthetic explicit tea preference, later withdrawn"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	value := map[string]any{"key": "drink", "value": "tea"}
	proposal := contract.WorldUpdateProposal{SchemaVersion: 1, ID: contract.NewID(), RequestID: contract.NewID(), EntityID: contract.NewID(), Predicate: "master.preference", Operation: "ASSERT", FactID: contract.NewID(), Value: &value, Evidence: []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, Locator: "full", OriginID: obj.ID, DataClass: "SYNTHETIC"}}, Basis: "MASTER_EXPLICIT", PolicyRevision: 1, Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
	commitFixture := func() contract.WorldFact {
		t.Helper()
		if e = s.Write(ctx, func(tx *sql.Tx) error { return s.PutProposalTx(ctx, tx, proposal) }); e != nil {
			t.Fatal(e)
		}
		v, e := s.CommitWorld(ctx, proposal, func(*sql.Tx) error { return nil })
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	original := commitFixture()
	epoch := time.Now()
	mem := memory.Service{Store: s, Model: &worldSummaryFixture{Client: p, Fact: original}, Config: c, Epoch: epoch}
	old, e := mem.RefreshSlot(ctx, 0, epoch)
	if e != nil {
		t.Fatal(e)
	}
	proposal.ID = contract.NewID()
	proposal.RequestID = contract.NewID()
	proposal.Operation = "RETRACT"
	proposal.ExpectedRevision = 1
	proposal.Value = nil
	current := commitFixture()
	if current.Status != "RETRACTED" || current.Revision != 2 {
		t.Fatal(current)
	}
	in := input(contract.NewID())
	in.Text = "show latest world fact " + original.ID
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, _, e := b.Build(ctx, turn, epoch.Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	wire := req.Input.(map[string]any)
	facts := wire["world"].(contract.WorldModelInput).Facts
	if len(facts) != 1 || facts[0].ID != original.ID || facts[0].Status != "RETRACTED" || wire["consciousness"] != nil {
		t.Fatal("derived snapshot revived old fact", facts)
	}
	kept, e := s.ConsciousnessAt(ctx, 0)
	if e != nil || kept.ID != old.ID {
		t.Fatal("history was erased", e)
	}
	var history int
	s.DB.QueryRow("SELECT count(*) FROM world_fact_version WHERE fact_id=?", original.ID).Scan(&history)
	if history != 2 {
		t.Fatal("world history lost")
	}
}
