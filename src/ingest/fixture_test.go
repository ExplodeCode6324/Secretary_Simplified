package ingest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
)

func TestFixtureDedupConflictAndQuarantine(t *testing.T) {
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	svc := Service{Store: s, QuarantineDir: filepath.Join(d, "quarantine")}
	id := contract.NewID()
	r := FixtureRecord{ExternalID: "one", Version: "v1", Value: json.RawMessage(`{"title":"one","domain":"work","due_at":null,"status":"OPEN"}`)}
	c := context.Background()
	out, e := svc.Sync(c, id, []FixtureRecord{r, r})
	if e != nil || out.Inserted != 1 || out.Duplicates != 1 {
		t.Fatal(out, e)
	}
	r.Value = json.RawMessage(`{"title":"changed","domain":"work","due_at":null,"status":"OPEN"}`)
	out, e = svc.Sync(c, id, []FixtureRecord{r})
	if e != nil || out.Quarantined != 1 {
		t.Fatal(out, e)
	}
	files, e := os.ReadDir(svc.QuarantineDir)
	if e != nil || len(files) != 1 {
		t.Fatal(files, e)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record").Scan(&n)
	if n != 1 {
		t.Fatal("conflicting version persisted", n)
	}
}
func TestTombstoneAndNoDuplicateObjects(t *testing.T) {
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	svc := Service{Store: s, QuarantineDir: filepath.Join(d, "quarantine")}
	ctx := context.Background()
	id := contract.NewID()
	row := FixtureRecord{ExternalID: "deleted", Version: "v2", Deleted: true}
	out, e := svc.Sync(ctx, id, []FixtureRecord{row, row})
	if e != nil || out.Inserted != 1 || out.Duplicates != 1 {
		t.Fatal(out, e)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&n)
	if n != 1 {
		t.Fatal("duplicate raw objects", n)
	}
	row.Deleted = false
	row.Value = json.RawMessage(`{"title":"conflict","domain":"work","due_at":null,"status":"OPEN"}`)
	out, e = svc.Sync(ctx, id, []FixtureRecord{row, row})
	if e != nil || out.Quarantined != 2 {
		t.Fatal(out, e)
	}
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&n)
	if n != 1 {
		t.Fatal("rejected record created object", n)
	}
	files, e := os.ReadDir(svc.QuarantineDir)
	if e != nil || len(files) != 1 {
		t.Fatal("duplicate quarantine", len(files), e)
	}
}
