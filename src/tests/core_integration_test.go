package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) (*store.Store, config.Config, *model.Provider) {
	t.Helper()
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state", "secretary.sqlite"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	c := config.Default(d)
	return s, c, model.New(c)
}
func input(session string) contract.InputEnvelope {
	return contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: session, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: "synthetic integration input", AttachmentRefs: []contract.ObjectRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}
}
func createAction() contract.ActionProposal {
	return contract.ActionProposal{OperationKey: "create_item", Kind: "CREATE_ITEM", Payload: map[string]any{"domain": "work", "kind": "TASK", "title": "integration item", "priority": 1, "due_at": nil, "timezone": "UTC"}}
}
func TestFixtureInputCommitAndSemanticRetry(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	in := input(contract.NewID())
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	in.ReceivedAt = contract.Timestamp(time.Now().Add(time.Minute))
	again, e := s.AcceptInput(ctx, in, 100)
	if e != nil || again.ID != turn.ID {
		t.Fatal("retry receive time changed identity", again, e)
	}
	svc := core.Service{Store: s, Model: p, Config: c}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	got, e := s.GetTurn(ctx, turn.ID)
	if e != nil {
		t.Fatal(e)
	}
	if got.State != "COMMITTED" || got.Reply == nil || !strings.Contains((*got.Reply)["text"].(string), "Fixture profile") {
		t.Fatalf("fixture did not reach actual valid commit: %#v", got)
	}
	if e = svc.Process(ctx, got); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(ctx, in.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Recent) != 2 {
		t.Fatal("duplicate assistant reply", len(snap.Recent))
	}
	in.Text = "different"
	if _, e = s.AcceptInput(ctx, in, 100); e == nil {
		t.Fatal("changed semantic request accepted")
	}
}
func TestTypedItemsAnd24HourMemory(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	svc := core.Service{Store: s, Model: p, Config: c}
	rid, session := contract.NewID(), contract.NewID()
	a := createAction()
	turn, e := svc.TypedClass(ctx, rid, session, []contract.ActionProposal{a}, "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	if turn.State != "COMMITTED" {
		t.Fatal(turn.State)
	}
	if _, e = svc.TypedClass(ctx, rid, session, []contract.ActionProposal{a}, "SYNTHETIC"); e != nil {
		t.Fatal(e)
	}
	items, e := s.ListItems(ctx)
	if e != nil || len(items) != 1 {
		t.Fatal(items, e)
	}
	epoch := time.Now().UTC()
	mem := memory.Service{Store: s, Model: p, Epoch: epoch, Config: c}
	first, e := mem.Refresh(ctx, epoch)
	if e != nil {
		t.Fatal(e)
	}
	same, e := mem.Refresh(ctx, epoch.Add(23*time.Hour))
	if e != nil || same.ID != first.ID {
		t.Fatal("same slot regenerated", e)
	}
	next, e := mem.Refresh(ctx, epoch.Add(72*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if next.Slot != 3 || next.MissedSlots != 2 || next.PreviousSnapshotID == nil || *next.PreviousSnapshotID != first.ID {
		t.Fatal(next)
	}
	back, e := mem.Refresh(ctx, epoch.Add(time.Hour))
	if e != nil || back.ID != first.ID {
		t.Fatal("clock rollback regenerated", e)
	}
	if len(next.PriorityItems) != 1 || next.PriorityItems[0].Entity.ID != items[0].ID {
		t.Fatal("memory didn't consume current authority", next.PriorityItems)
	}
}
func TestContextDisclosureAndRequiredBudget(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	in := input(contract.NewID())
	in.DataClass = "PERSONAL"
	personal, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	in2 := input(in.SessionID)
	current, e := s.AcceptInput(ctx, in2, 100)
	if e != nil {
		t.Fatal(e)
	}
	builder := ctxbuild.Builder{Store: s, Model: p, Config: c}
	if _, _, e = builder.Build(ctx, personal, time.Now()); e == nil || e.Error() != "DISCLOSURE_DENIED" {
		t.Fatal("personal disclosure accepted", e)
	}
	if _, _, e = builder.Build(ctx, current, time.Now()); e == nil || e.Error() != "DISCLOSURE_DENIED" {
		t.Fatal("historical personal disclosure accepted", e)
	}
	clean := input(contract.NewID())
	turn, e := s.AcceptInput(ctx, clean, 100)
	if e != nil {
		t.Fatal(e)
	}
	c.Limits.InputTokens = 1024
	builder = ctxbuild.Builder{Store: s, Model: model.New(c), Config: c}
	if _, _, e = builder.Build(ctx, turn, time.Now()); e == nil || e.Error() != "CONTEXT_REQUIRED_OVERFLOW" {
		t.Fatal("required schema was silently pruned", e)
	}
}
func TestMemoryDisclosureDeniesSecretEvidence(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	obj, e := s.PutObject(ctx, []byte("secret fixture marker"), "text/plain", "SECRET")
	if e != nil {
		t.Fatal(e)
	}
	now := contract.Now()
	it := contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "NOTE", Title: "classified fixture", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, Locator: "all", OriginID: obj.ID, DataClass: "SECRET"}}, CreatedAt: now, UpdatedAt: now, Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
	if e = s.PutItem(ctx, it, 0); e != nil {
		t.Fatal(e)
	}
	mem := memory.Service{Store: s, Model: p, Epoch: time.Now(), Config: c}
	if _, e = mem.Refresh(ctx, time.Now()); e == nil || e.Error() != "DISCLOSURE_DENIED" {
		t.Fatal("background leaked secret classification", e)
	}
}
func TestWireManifestHashAndSchemaBudget(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	turn, e := s.AcceptInput(ctx, input(contract.NewID()), 100)
	if e != nil {
		t.Fatal(e)
	}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, m, e := b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	wire, e := p.Encode(req)
	if e != nil {
		t.Fatal(e)
	}
	if m.RequestHash != contract.Hash(wire) || m.InputBytes != len(wire) || m.InputTokens != len(wire) {
		t.Fatal("manifest not based on actual wire")
	}
	var v map[string]any
	if e = json.Unmarshal(wire, &v); e != nil {
		t.Fatal(e)
	}
	system := v["input"].([]any)[0].(map[string]any)["content"].(string)
	if strings.Contains(system, "$defs") {
		t.Fatal("schema duplicated in system prompt")
	}
	if e = contract.Validate("Context", req.Input); e != nil {
		t.Fatal(e)
	}
	contextValue := req.Input.(map[string]any)
	defs := contextValue["output_contract"].(map[string]any)["$defs"].(map[string]any)
	if _, ok := defs["DecisionEnvelope"]; !ok {
		t.Fatal("missing output schema")
	}
	if _, ok := defs["ExecutionPermit"]; ok {
		t.Fatal("unrelated schema leaked")
	}

}
func TestTypedFailureCannotBecomeModelInput(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	svc := core.Service{Store: s, Model: p, Config: c}
	a := contract.ActionProposal{OperationKey: "missing", Kind: "UPDATE_ITEM", Payload: map[string]any{"item_id": contract.NewID(), "expected_revision": 1, "changes": map[string]any{"title": "impossible"}}}
	if _, e := svc.TypedClass(ctx, contract.NewID(), contract.NewID(), []contract.ActionProposal{a}, "SYNTHETIC"); e == nil {
		t.Fatal("missing item update succeeded")
	}
	pending, e := s.PendingTurns(ctx)
	if e != nil || len(pending) != 0 {
		t.Fatal("failed typed action leaked into model queue", pending, e)
	}
}
func TestOriginalInputEvidenceArchivedAndDeduplicated(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	in := input(contract.NewID())
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	in.ReceivedAt = contract.Timestamp(time.Now().Add(time.Minute))
	if _, e = s.AcceptInput(ctx, in, 100); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(ctx, in.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Recent) != 1 || len(snap.Recent[0].Evidence) != 1 {
		t.Fatal("missing original evidence", snap.Recent)
	}
	ref := snap.Recent[0].Evidence[0]
	b, e := s.ReadObject(ctx, ref.ObjectID)
	if e != nil || string(b) != in.Text {
		t.Fatal("wrong original archive", e)
	}
	if len(turn.Input.AttachmentRefs) != 1 {
		t.Fatal("model cannot see original ref")
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&n)
	if n != 1 {
		t.Fatal("retry duplicated object", n)
	}
}
func TestConversationSummaryCASAndWatermark(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	session := contract.NewID()
	svc := core.Service{Store: s, Model: p, Config: c}
	for i := 0; i < 2; i++ {
		turn, e := s.AcceptInput(ctx, input(session), 100)
		if e != nil {
			t.Fatal(e)
		}
		if e = svc.Process(ctx, turn); e != nil {
			t.Fatal(e)
		}
	}
	mem := memory.Service{Store: s, Model: p, Config: c, Epoch: time.Now()}
	v, e := mem.Summarize(ctx, session)
	if e != nil {
		t.Fatal(e)
	}
	if v.ThroughSequence != 4 || v.SummaryFromSequence != 1 || len(v.RecentEventIDs) != 4 {
		t.Fatal(v)
	}
	again, e := mem.Summarize(ctx, session)
	if e != nil || again.Revision != v.Revision {
		t.Fatal("regenerated covered summary", e)
	}
	v.Revision++
	if e = s.SaveConversation(ctx, v, v.Revision-2, 0); e == nil {
		t.Fatal("stale coverage CAS accepted")
	}
}

type failingClient struct {
	Provider *model.Provider
	Calls    int
}

func (f *failingClient) Encode(r model.Request) ([]byte, error) { return f.Provider.Encode(r) }
func (f *failingClient) Generate(context.Context, model.Request) (model.Result, error) {
	f.Calls++
	return model.Result{}, fmt.Errorf("synthetic unavailable")
}

func TestRelevantContextKeepsExplicitDependencyClosure(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	ids := []string{}
	for i := 0; i < 30; i++ {
		now := contract.Now()
		v := contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: fmt.Sprintf("synthetic item %d", i), Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: now, UpdatedAt: now, Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
		if i == 1 {
			v.DependencyIDs = []string{ids[0]}
		}
		if e := s.PutItem(ctx, v, 0); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, v.ID)
	}
	in := input(contract.NewID())
	in.Text = "Please inspect " + ids[1]
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, m, e := b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	wire, e := p.Encode(req)
	if e != nil || len(wire) > c.Limits.InputTokens {
		t.Fatal(len(wire), e)
	}
	seen := map[string]bool{}
	for _, ref := range m.ReadSet {
		if ref.EntityType == "Item" {
			seen[ref.ID] = true
		}
	}
	if !seen[ids[0]] || !seen[ids[1]] || len(seen) != 2 {
		t.Fatal("dependency closure wrong", seen)
	}
	found := false
	for _, section := range m.Sections {
		if section["name"] == "items" {
			found = true
			if section["omitted_count"] != 28 {
				t.Fatal(section)
			}
		}
	}
	if !found {
		t.Fatal("missing omission evidence")
	}
}
