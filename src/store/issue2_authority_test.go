package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/platform"
	"strings"
	"testing"
)

func issue2LegacyDB(t *testing.T) (*Store, string, string, contract.InputEnvelope, contract.InputTurn) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "secretary.sqlite")
	objects := filepath.Join(dir, "objects")
	s, e := connect(path, objects)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(baseline); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO schema_migration VALUES(1,?,?)", contract.Hash([]byte(baseline)), contract.Now()); e != nil {
		t.Fatal(e)
	}
	in := questionInput()
	in.PrincipalID = "master"
	state := emptyConversation(in.SessionID)
	raw, _ := json.Marshal(state)
	if _, e = s.DB.Exec("INSERT INTO conversation_session VALUES(?,1,0,?)", in.SessionID, string(raw)); e != nil {
		t.Fatal(e)
	}
	hash, _ := inputSemanticHash(in)
	turn := contract.InputTurn{SchemaVersion: 1, ID: contract.NewID(), SessionID: in.SessionID, PrincipalID: in.PrincipalID, RequestID: in.RequestID, IntentID: contract.NewID(), State: "COMMITTED", Input: in, CommittedOperationKeys: []string{}, UpdatedAt: in.ReceivedAt, Extensions: map[string]any{}}
	receipt, _ := json.Marshal(map[string]any{"turn_id": turn.ID, "state": "COMMITTED"})
	if _, e = s.DB.Exec("INSERT INTO request_receipt VALUES(?,?,?,?,?,?,?)", in.PrincipalID, in.RequestID, hash, turn.IntentID, "COMMITTED", string(receipt), in.ReceivedAt); e != nil {
		t.Fatal(e)
	}
	raw, _ = json.Marshal(turn)
	if _, e = s.DB.Exec("INSERT INTO input_turn VALUES(?,?,?,?,?,?,?,?)", turn.ID, in.SessionID, in.PrincipalID, in.RequestID, turn.IntentID, turn.State, string(raw), turn.UpdatedAt); e != nil {
		t.Fatal(e)
	}
	return s, path, objects, in, turn
}
func TestIssue2AUTHLegacyExplicitMappingAndStrictReplay(t *testing.T) {
	old, path, objects, in, turn := issue2LegacyDB(t)
	ctx := context.Background()
	var before string
	old.DB.QueryRow("SELECT payload_json FROM input_turn WHERE id=?", turn.ID).Scan(&before)
	internal := contract.NewID()
	state := emptyConversation(internal)
	raw, _ := json.Marshal(state)
	old.DB.Exec("INSERT INTO conversation_session VALUES(?,1,0,?)", internal, string(raw))
	// A second legacy MASTER branch has its own immutable accepted receipt.
	legacyInput := in
	legacyInput.SessionID = internal
	legacyInput.RequestID = contract.NewID()
	legacyInput.Text = "legacy readonly pending"
	legacyTurn := turn
	legacyTurn.ID = contract.NewID()
	legacyTurn.SessionID = internal
	legacyTurn.RequestID = legacyInput.RequestID
	legacyTurn.IntentID = contract.NewID()
	legacyTurn.Input = legacyInput
	legacyTurn.State = "PENDING"
	legacyHash, _ := inputSemanticHash(legacyInput)
	legacyReceipt, _ := json.Marshal(map[string]any{"turn_id": legacyTurn.ID, "state": "ACCEPTED"})
	if _, e := old.DB.Exec("INSERT INTO request_receipt VALUES(?,?,?,?,?,?,?)", legacyInput.PrincipalID, legacyInput.RequestID, legacyHash, legacyTurn.IntentID, "ACCEPTED", string(legacyReceipt), legacyInput.ReceivedAt); e != nil {
		t.Fatal(e)
	}
	legacyRaw, _ := json.Marshal(legacyTurn)
	if _, e := old.DB.Exec("INSERT INTO input_turn VALUES(?,?,?,?,?,?,?,?)", legacyTurn.ID, internal, legacyInput.PrincipalID, legacyInput.RequestID, legacyTurn.IntentID, "PENDING", string(legacyRaw), legacyTurn.UpdatedAt); e != nil {
		t.Fatal(e)
	}
	emptyInternal := contract.NewID()
	emptyState := emptyConversation(emptyInternal)
	emptyRaw, _ := json.Marshal(emptyState)
	if _, e := old.DB.Exec("INSERT INTO conversation_session VALUES(?,1,0,?)", emptyInternal, string(emptyRaw)); e != nil {
		t.Fatal(e)
	}
	old.Close()
	if s, e := Open(path, objects); e == nil {
		s.Close()
		t.Fatal("v1 silently opened")
	}
	if s, e := UpgradeAuthority(path, objects, ""); e == nil {
		s.Close()
		t.Fatal("ambiguous legacy selection accepted")
	}
	if s, e := UpgradeAuthority(path, objects, emptyInternal); e == nil {
		s.Close()
		t.Fatal("internal-only selected")
	}
	s, e := UpgradeAuthority(path, objects, in.SessionID)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var after string
	s.DB.QueryRow("SELECT payload_json FROM input_turn WHERE id=?", turn.ID).Scan(&after)
	if before != after {
		t.Fatal("legacy evidence rewritten")
	}
	a, e := s.AuthorityState(ctx, 100)
	if e != nil || a.SessionID != in.SessionID || len(a.LegacySessions) != 2 {
		t.Fatal(a, e)
	}
	legacyGot, e := s.AcceptInput(ctx, legacyInput, 1)
	if e != nil || legacyGot.ID != legacyTurn.ID {
		t.Fatal("readonly legacy exact replay lost", legacyGot, e)
	}
	pending, e := s.PendingTurns(ctx)
	if e != nil || len(pending) != 0 {
		t.Fatal("legacy branch consumed", pending, e)
	}
	activeNew := in
	activeNew.RequestID = contract.NewID()
	if _, e = s.AcceptInput(ctx, activeNew, 1); e != nil {
		t.Fatal("legacy pending occupied active queue", e)
	}
	got, e := s.AcceptInput(ctx, in, 100)
	if e != nil || got.ID != turn.ID {
		t.Fatal("exact replay", got, e)
	}
	explicit := in
	explicit.SessionID = internal
	if _, e = s.AcceptInput(ctx, explicit, 100); e == nil || e.Error() != "IDEMPOTENCY_CONFLICT" {
		t.Fatal("explicit foreign request rewritten", e)
	}
	bound, e := s.ResolveSession(ctx, in.PrincipalID, in.RequestID, "")
	if e != nil || bound != in.SessionID {
		t.Fatal("omitted replay binding", bound, e)
	}
	newRequest := in
	newRequest.RequestID = contract.NewID()
	newRequest.SessionID = internal
	if _, e = s.AcceptInput(ctx, newRequest, 100); e == nil || e.Error() != "AUTHORITY_SESSION_MISMATCH" {
		t.Fatal(e)
	}
	if e = validateMigrations(ctx, s.DB, true); e != nil {
		t.Fatal(e)
	}
	// The new unknown-version fixture is v3. Historical v2 fixture code is untouched.
	if _, e = s.DB.Exec("INSERT INTO schema_migration VALUES(3,'unknown',?)", contract.Now()); e != nil {
		t.Fatal(e)
	}
	if v, e := Open(path, objects); e == nil {
		v.Close()
		t.Fatal("unknown v3 opened")
	}
}
func TestIssue2AUTHMigrationRequiresBothRolesStopped(t *testing.T) {
	old, path, objects, in, _ := issue2LegacyDB(t)
	old.Close()
	for _, role := range []string{"core", "runner"} {
		// Upgrade creates run directory; a failed explicit selection performs no migration.
		_, _ = UpgradeAuthority(path, objects, "")
		lock, e := platform.AcquireLock(filepath.Join(filepath.Dir(objects), "run", role+".lock"))
		if e != nil {
			t.Fatal(e)
		}
		if s, e := UpgradeAuthority(path, objects, in.SessionID); e == nil {
			s.Close()
			lock.Close()
			t.Fatal("active role migration allowed", role)
		} else if !strings.Contains(e.Error(), "AUTHORITY_MIGRATION_BUSY") {
			t.Fatal(e)
		}
		lock.Close()
	}
}
func TestIssue2AUTHSummaryStopsBeforeQueuedPhysicalGap(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	session, e := s.AuthoritySession(ctx)
	if e != nil {
		t.Fatal(e)
	}
	a := questionInput()
	a.SessionID = session
	b := a
	b.RequestID = contract.NewID()
	b.Text = "FUTURE_SUMMARY_CANARY"
	at, e := s.AcceptInput(ctx, a, 100)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.AcceptInput(ctx, b, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishTurn(ctx, at.ID, map[string]any{"text": "prior reply", "evidence": []any{}}, []string{}, nil); e != nil {
		t.Fatal(e)
	}
	sc, e := s.SummaryContext(ctx, session)
	if e != nil {
		t.Fatal(e)
	}
	events, e := s.ConversationHistory(sc, session, 0, 40)
	if e != nil || len(events) != 1 || events[0].TurnID != at.ID {
		t.Fatal(events, e)
	}
	snap, e := s.Snapshot(sc, session)
	if e != nil {
		t.Fatal(e)
	}
	snap.Conversation.Summary = "synthetic gap claim"
	snap.Conversation.SummaryFromSequence = 1
	snap.Conversation.ThroughSequence = 3
	snap.Conversation.Revision++
	snap.Conversation.Extensions, _ = contract.ClassifyExtensions(snap.Conversation.Extensions, "SYNTHETIC")
	if e = s.SaveConversation(ctx, snap.Conversation, snap.Conversation.Revision-1, 0); e == nil || e.Error() != "AUTHORITY_SUMMARY_GAP" {
		t.Fatal("summary crossed queued gap", e)
	}
}
func TestIssue2AUTHFrozenPrefixPersistsAndFiltersRetrieval(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	session, e := s.AuthoritySession(ctx)
	if e != nil {
		t.Fatal(e)
	}
	in := questionInput()
	in.SessionID = session
	in.Text = "EARLY_PREFIX_CONTENT"
	turn, e := s.AcceptInput(ctx, in, 10)
	if e != nil {
		t.Fatal(e)
	}
	var frozen TurnPrefix
	if e = s.WithAuthorityConsumer(ctx, func(held context.Context) error { var e error; frozen, e = s.FreezeTurn(held, turn); return e }); e != nil {
		t.Fatal(e)
	}
	later := in
	later.RequestID = contract.NewID()
	later.Text = "FUTURE_PREFIX_CANARY"
	later.DataClass = "PERSONAL"
	if _, e = s.AcceptInput(ctx, later, 10); e != nil {
		t.Fatal(e)
	}
	var path string
	s.DB.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path)
	s.Close()
	reopened, e := Open(path, s.ObjectsDir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if e = reopened.WithAuthorityConsumer(ctx, func(held context.Context) error {
		again, e := reopened.FreezeTurn(held, turn)
		if e != nil {
			return e
		}
		if again.Sequence != frozen.Sequence || again.RowID != frozen.RowID || again.State.Revision != frozen.State.Revision {
			t.Fatal("frozen boundary moved")
		}
		bounded := WithTurnPrefix(held, again)
		snap, e := reopened.SnapshotForTurn(bounded, turn)
		if e != nil {
			return e
		}
		if len(snap.Recent) != 1 || snap.Recent[0].Text != in.Text {
			t.Fatal("future prompt event")
		}
		for _, class := range snap.DataClasses {
			if class != "SYNTHETIC" {
				t.Fatal("future classification", class)
			}
		}
		page, e := reopened.SearchMemoryPage(bounded, "FUTURE_PREFIX", nil, nil)
		if e != nil {
			return e
		}
		if len(page.Events) != 0 || len(page.DataClasses) != 0 {
			t.Fatal("future retrieval escaped", page)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
