package store

import (
	"context"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"testing"
)

func foundationDB(t *testing.T) *Store {
	t.Helper()
	d := t.TempDir()
	s, e := Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func fixtureItem() contract.Item {
	return contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "fixture", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
}
func TestItemCASCycleAndAtomicEvent(t *testing.T) {
	s := foundationDB(t)
	c := context.Background()
	a, b := fixtureItem(), fixtureItem()
	if e := s.PutItem(c, a, 0); e != nil {
		t.Fatal(e)
	}
	b.DependencyIDs = []string{a.ID}
	if e := s.PutItem(c, b, 0); e != nil {
		t.Fatal(e)
	}
	a.Revision = 2
	a.DependencyIDs = []string{b.ID}
	if s.PutItem(c, a, 1) == nil {
		t.Fatal("cycle accepted")
	}
	a.DependencyIDs = []string{}
	if e := s.PutItem(c, a, 1); e != nil {
		t.Fatal(e)
	}
	if s.PutItem(c, a, 1) == nil {
		t.Fatal("stale CAS accepted")
	}
	events, e := s.Events(c, 0)
	if e != nil {
		t.Fatal(e)
	}
	if len(events) != 3 {
		t.Fatalf("rollback/event count %d", len(events))
	}
	got, e := s.GetItem(c, a.ID)
	if e != nil || got.Revision != 2 {
		t.Fatal(got, e)
	}
}
func TestObjectTamperAndConnectionPragmas(t *testing.T) {
	s := foundationDB(t)
	c := context.Background()
	v, e := s.PutObject(c, []byte("fixture"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReadObject(c, v.ID); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(s.ObjectsDir, v.RelativePath), []byte("tamper"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ReadObject(c, v.ID); e == nil {
		t.Fatal("tamper accepted")
	}
	for i := 0; i < 4; i++ {
		conn, e := s.DB.Conn(c)
		if e != nil {
			t.Fatal(e)
		}
		defer conn.Close()
		for p, want := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000, "synchronous": 2} {
			var got int
			if e = conn.QueryRowContext(c, "PRAGMA "+p).Scan(&got); e != nil || got != want {
				t.Fatalf("%s: %d %v", p, got, e)
			}
		}
	}
}
func TestConflictCarriesRevisionAndSourceCASRollback(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	v := fixtureItem()
	if e := s.PutItem(ctx, v, 0); e != nil {
		t.Fatal(e)
	}
	v.Revision = 2
	if e := s.PutItem(ctx, v, 1); e != nil {
		t.Fatal(e)
	}
	e := s.PutItem(ctx, v, 1)
	conflict, ok := e.(*RevisionConflict)
	if !ok || conflict.CurrentRevision != 2 {
		t.Fatal(e)
	}
	src := contract.SourceState{SchemaVersion: 1, ID: contract.NewID(), Kind: "FIXTURE", Revision: 1, StaleAfterSeconds: 60, Enabled: true, DataClass: "SYNTHETIC", Extensions: map[string]any{}}
	if e = s.EnsureSource(ctx, src); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE source_state SET revision=5 WHERE id=?", src.ID); e != nil {
		t.Fatal(e)
	}
	events, _ := s.Events(ctx, 0)
	if e = s.SourceSynced(ctx, src.ID, 0); e == nil {
		t.Fatal("corrupt source revision accepted")
	}
	after, _ := s.Events(ctx, 0)
	if len(after) != len(events) {
		t.Fatal("false source event")
	}
}
func TestInputHashMatchesStoredSemanticEnvelopeAndRejectsNoObject(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	in := contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: contract.NewID(), PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: "one", DataClass: "SYNTHETIC", AttachmentRefs: []contract.ObjectRef{}, Extensions: map[string]any{}}
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	hash, e := inputSemanticHash(turn.Input)
	if e != nil {
		t.Fatal(e)
	}
	var stored string
	s.DB.QueryRow("SELECT payload_hash FROM request_receipt WHERE principal_id=? AND request_id=?", in.PrincipalID, in.RequestID).Scan(&stored)
	if hash != stored {
		t.Fatal("stored semantic envelope does not match receipt")
	}
	in.Text = "two"
	if _, e = s.AcceptInput(ctx, in, 100); e == nil {
		t.Fatal("conflict accepted")
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&n)
	if n != 1 {
		t.Fatal("rejected conflict archived unreferenced object", n)
	}
}
