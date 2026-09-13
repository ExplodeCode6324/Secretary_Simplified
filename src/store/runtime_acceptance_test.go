package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptanceA03IndependentConnectionsCAS(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "state.db")
	objects := filepath.Join(d, "objects")
	a, e := Init(path, objects)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := Open(path, objects)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	item := fixtureItem()
	ctx := context.Background()
	if e = a.PutItem(ctx, item, 0); e != nil {
		t.Fatal(e)
	}
	item.Revision = 2
	gate := make(chan struct{})
	results := make(chan error, 2)
	for _, s := range []*Store{a, b} {
		go func(s *Store) { <-gate; results <- s.PutItem(ctx, item, 1) }(s)
	}
	close(gate)
	success := 0
	for i := 0; i < 2; i++ {
		e := <-results
		if e == nil {
			success++
		} else if !strings.Contains(e.Error(), "REVISION_CONFLICT") {
			t.Fatalf("unexpected failure %v", e)
		}
	}
	if success != 1 {
		t.Fatal("success count", success)
	}
	got, e := a.GetItem(ctx, item.ID)
	if e != nil || got.Revision != 2 {
		t.Fatal(got, e)
	}
	events, e := a.Events(ctx, 0)
	if e != nil || len(events) != 2 {
		t.Fatal("events", len(events), e)
	}
}

func TestAcceptanceA19BaselineRollbackAndVersionRefusal(t *testing.T) {
	t.Run("new baseline and unsupported version", func(t *testing.T) {
		d := t.TempDir()
		path := filepath.Join(d, "state.db")
		objects := filepath.Join(d, "objects")
		s, e := Init(path, objects)
		if e != nil {
			t.Fatal(e)
		}
		var v int
		if e = s.DB.QueryRow(`SELECT max(version) FROM schema_migration`).Scan(&v); e != nil || v != 1 {
			t.Fatal(v, e)
		}
		if _, e = s.DB.Exec(`UPDATE schema_migration SET version=2`); e != nil {
			t.Fatal(e)
		}
		s.Close()
		if x, e := Open(path, objects); e == nil {
			x.Close()
			t.Fatal("unsupported version accepted")
		}
	})
	t.Run("baseline DDL failure rolls back", func(t *testing.T) {
		original := baseline
		defer func() { baseline = original }()
		baseline = strings.Replace(baseline, "COMMIT;", "THIS IS INVALID SQL; COMMIT;", 1)
		d := t.TempDir()
		path := filepath.Join(d, "bad.db")
		if s, e := Init(path, filepath.Join(d, "objects")); e == nil {
			s.Close()
			t.Fatal("injected DDL accepted")
		}
		db, e := sql.Open("sqlite", path)
		if e != nil {
			t.Fatal(e)
		}
		defer db.Close()
		var n int
		if e = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&n); e != nil || n != 0 {
			t.Fatal("partial schema", n, e)
		}
		baseline = original
		if s, e := Open(path, filepath.Join(d, "objects")); e == nil {
			s.Close()
			t.Fatal("incomplete init opened")
		}
	})
}
