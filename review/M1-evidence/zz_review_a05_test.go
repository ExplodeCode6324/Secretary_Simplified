package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"secretarysimplified/contract"
	"secretarysimplified/store"
)

func rvSvc(t *testing.T) (*Service, *store.Store, string) {
	t.Helper()
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	q := filepath.Join(d, "quarantine")
	return &Service{Store: s, QuarantineDir: q}, s, q
}

func rvRec(ext, ver, title string) FixtureRecord {
	return FixtureRecord{ExternalID: ext, Version: ver, Value: json.RawMessage(`{"title":"` + title + `","domain":"work","due_at":null,"status":"OPEN"}`)}
}

func TestReviewA05RepeatPageObjects(t *testing.T) {
	svc, s, _ := rvSvc(t)
	c := context.Background()
	sid := contract.NewID()
	batch := []FixtureRecord{rvRec("a", "v1", "one"), rvRec("b", "v1", "two")}
	out1, e := svc.Sync(c, sid, batch)
	if e != nil {
		t.Fatal(e)
	}
	var obj1, rec1 int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&obj1)
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record").Scan(&rec1)
	out2, e := svc.Sync(c, sid, batch)
	if e != nil {
		t.Fatal(e)
	}
	var obj2, rec2 int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&obj2)
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record").Scan(&rec2)
	t.Logf("sync1=%+v objects=%d records=%d; sync2=%+v objects=%d records=%d", out1, obj1, rec1, out2, obj2, rec2)
	if rec2 != 2 {
		t.Errorf("dedupe broken: records=%d", rec2)
	}
	if out2.Inserted != 0 || out2.Duplicates != 2 {
		t.Errorf("repeat counts wrong: %+v", out2)
	}
	if obj2 > obj1 {
		t.Logf("REVIEW-FINDING A05: repeated page added %d new object_ref rows/blobs (objects %d->%d); '重复页无重复对象' 口径需与设计对齐", obj2-obj1, obj1, obj2)
	}
}

func TestReviewA05EmptyPageAndFailureBehavior(t *testing.T) {
	svc, s, _ := rvSvc(t)
	c := context.Background()
	empty := contract.NewID()
	out, e := svc.Sync(c, empty, nil)
	if e != nil {
		t.Fatal(e)
	}
	var rev int
	var ls sql.NullString
	s.DB.QueryRow("SELECT revision,last_success_at FROM source_state WHERE id=?", empty).Scan(&rev, &ls)
	t.Logf("empty page: out=%+v rev=%d last_success_valid=%v", out, rev, ls.Valid)
	if !ls.Valid {
		t.Error("empty success page did not update freshness")
	}

	src := contract.NewID()
	if _, e := svc.Sync(c, src, []FixtureRecord{rvRec("x", "v1", "keep")}); e != nil {
		t.Fatal(e)
	}
	var revBefore int
	var lsBefore sql.NullString
	s.DB.QueryRow("SELECT revision,last_success_at FROM source_state WHERE id=?", src).Scan(&revBefore, &lsBefore)
	bad := FixtureRecord{ExternalID: "", Version: "v1", Value: json.RawMessage(`{}`)}
	svc.QuarantineDir = ""
	_, e = svc.Sync(c, src, []FixtureRecord{rvRec("y", "v1", "new"), bad})
	t.Logf("failure sync err=%v", e)
	if e == nil {
		t.Fatal("expected failure")
	}
	var revAfter int
	var lsAfter sql.NullString
	s.DB.QueryRow("SELECT revision,last_success_at FROM source_state WHERE id=?", src).Scan(&revAfter, &lsAfter)
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record WHERE source_id=?", src).Scan(&count)
	t.Logf("failure sync: rev %d->%d last_success %v->%v records=%d", revBefore, revAfter, lsBefore.Valid, lsAfter.Valid, count)
	if count < 2 {
		t.Errorf("records lost despite failure: %d", count)
	}
	if revAfter != revBefore || lsAfter.Valid != lsBefore.Valid {
		t.Errorf("freshness advanced despite failure: rev %d->%d", revBefore, revAfter)
	}
}

func TestReviewA05ConflictQuarantineAccumulation(t *testing.T) {
	svc, s, q := rvSvc(t)
	c := context.Background()
	sid := contract.NewID()
	r := rvRec("c", "v1", "first")
	if _, e := svc.Sync(c, sid, []FixtureRecord{r}); e != nil {
		t.Fatal(e)
	}
	r.Value = rvRec("c", "v1", "changed").Value
	out1, e := svc.Sync(c, sid, []FixtureRecord{r})
	if e != nil {
		t.Fatal(e)
	}
	f1, _ := os.ReadDir(q)
	out2, e := svc.Sync(c, sid, []FixtureRecord{r})
	if e != nil {
		t.Fatal(e)
	}
	f2, _ := os.ReadDir(q)
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record WHERE source_id=?", sid).Scan(&n)
	t.Logf("conflict: out1=%+v files=%d | out2=%+v files=%d records=%d", out1, len(f1), out2, len(f2), n)
	if n != 1 {
		t.Errorf("conflicting version persisted: %d", n)
	}
	if len(f2) > len(f1) {
		t.Logf("REVIEW-NOTE: repeated conflicting sync grows quarantine dir %d->%d (new file per attempt, ID-named)", len(f1), len(f2))
	}
}

func TestReviewDeletedTombstone(t *testing.T) {
	svc, s, _ := rvSvc(t)
	c := context.Background()
	sid := contract.NewID()
	out, e := svc.Sync(c, sid, []FixtureRecord{{ExternalID: "d1", Version: "v1", Deleted: true}})
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record WHERE source_id=?", sid).Scan(&n)
	t.Logf("tombstone alone: out=%+v err=%v records=%d", out, e, n)
	if e != nil {
		t.Logf("REVIEW-FINDING A05: deleted tombstone rejected (Sync error: %v); SourceRecord schema VALID requires normalized object, tombstone has none", e)
	} else {
		var b []byte
		if e := s.DB.QueryRow("SELECT payload_json FROM source_record WHERE source_id=?", sid).Scan(&b); e != nil {
			t.Fatal(e)
		}
		var rec contract.SourceRecord
		json.Unmarshal(b, &rec)
		t.Logf("tombstone accepted: deleted=%v status=%s normalized_nil=%v", rec.Deleted, rec.ValidationStatus, rec.Normalized == nil)
	}
	sid2 := contract.NewID()
	out2, e2 := svc.Sync(c, sid2, []FixtureRecord{rvRec("g", "v1", "good"), {ExternalID: "d2", Version: "v1", Deleted: true}})
	var n2 int
	s.DB.QueryRow("SELECT COUNT(*) FROM source_record WHERE source_id=?", sid2).Scan(&n2)
	t.Logf("mixed batch (good,tombstone): out=%+v err=%v records=%d", out2, e2, n2)
}

func TestReviewA05OrphanObjectsOnRejectedRecords(t *testing.T) {
	svc, s, _ := rvSvc(t)
	c := context.Background()
	sid := contract.NewID()
	r := rvRec("o1", "v1", "first")
	if _, e := svc.Sync(c, sid, []FixtureRecord{r}); e != nil {
		t.Fatal(e)
	}
	var objA int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&objA)
	r.Value = rvRec("o1", "v1", "changed").Value
	if _, e := svc.Sync(c, sid, []FixtureRecord{r}); e != nil {
		t.Fatal(e)
	}
	var objB int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref").Scan(&objB)
	var orphans int
	s.DB.QueryRow("SELECT COUNT(*) FROM object_ref o WHERE NOT EXISTS (SELECT 1 FROM source_record sr WHERE sr.raw_ref=o.id)").Scan(&orphans)
	t.Logf("object_ref: after 1st sync=%d, after conflict sync=%d, orphans=%d", objA, objB, orphans)
	if objB > objA {
		t.Logf("REVIEW-FINDING: rejected/conflicting records still create object_ref rows + blobs before validation (orphans=%d)", orphans)
	}
}
