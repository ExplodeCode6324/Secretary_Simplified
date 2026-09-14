package ingest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
)

func TestAcceptanceA05EmptyPageAndFailurePreserve(t *testing.T) {
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	svc := Service{Store: s, QuarantineDir: filepath.Join(d, "quarantine")}
	ctx := context.Background()
	id := contract.NewID()
	record := FixtureRecord{ExternalID: "one", Version: "v1", Value: json.RawMessage(`{"title":"one","domain":"work","due_at":null,"status":"OPEN"}`)}
	if _, e = svc.Sync(ctx, id, []FixtureRecord{record}); e != nil {
		t.Fatal(e)
	}
	var rev int
	var before string
	s.DB.QueryRow(`SELECT revision,payload_json FROM source_state WHERE id=?`, id).Scan(&rev, &before)
	out, e := svc.Sync(ctx, id, []FixtureRecord{})
	if e != nil || out.Inserted != 0 {
		t.Fatal(out, e)
	}
	var next int
	var fresh string
	if e = s.DB.QueryRow(`SELECT revision,payload_json FROM source_state WHERE id=?`, id).Scan(&next, &fresh); e != nil || next != rev+1 || fresh == before {
		t.Fatal("empty success not committed", next, e)
	}
	var at string
	if e = s.DB.QueryRow(`SELECT last_success_at FROM source_state WHERE id=?`, id).Scan(&at); e != nil || at == "" {
		t.Fatal("freshness missing", e)
	}
	// A failed page delivery must not reach SourceSynced or erase prior records.
	failed, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = svc.Sync(failed, id, nil); e == nil {
		t.Fatal("cancelled fetch accepted")
	}
	var after string
	s.DB.QueryRow(`SELECT payload_json FROM source_state WHERE id=?`, id).Scan(&after)
	if after != fresh {
		t.Fatal("failed page changed last successful source state")
	}
	for table := range map[string]bool{"source_record": true, "object_ref": true} {
		var n int
		s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
		if n != 1 {
			t.Fatal(table, n)
		}
	}
}
