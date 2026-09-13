package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"secretarysimplified/contract"
)

func rvDB(t *testing.T) *Store {
	t.Helper()
	d := t.TempDir()
	s, e := Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func rvItem() contract.Item {
	return contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "review", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: map[string]any{}}
}

func TestReviewA03ConflictCurrentRevision(t *testing.T) {
	d := t.TempDir()
	dbp := filepath.Join(d, "state.db")
	objp := filepath.Join(d, "objects")
	s1, e := Init(dbp, objp)
	if e != nil {
		t.Fatal(e)
	}
	defer s1.Close()
	s2, e := Open(dbp, objp)
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	c := context.Background()
	it := rvItem()
	if e := s1.PutItem(c, it, 0); e != nil {
		t.Fatal(e)
	}
	up := it
	up.Revision = 2
	up.Title = "A"
	e1 := s1.PutItem(c, up, 1)
	e2 := s2.PutItem(c, up, 1)
	wins := 0
	var loser error
	if e1 == nil {
		wins++
	} else {
		loser = e1
	}
	if e2 == nil {
		wins++
	} else {
		loser = e2
	}
	if wins != 1 {
		t.Fatalf("expected exactly one winner: e1=%v e2=%v", e1, e2)
	}
	got, ge := s1.GetItem(c, it.ID)
	if ge != nil {
		t.Fatal(ge)
	}
	if got.Revision != 2 {
		t.Fatalf("revision=%d", got.Revision)
	}
	t.Logf("A03 loser error = %q; current revision = %d", loser.Error(), got.Revision)
	if !strings.Contains(loser.Error(), "2") {
		t.Logf("REVIEW-FINDING A03: conflict error does not carry current revision (got %q, current=2)", loser.Error())
	}
}

func TestReviewA04NoPartialDependencyWriteOnCycle(t *testing.T) {
	s := rvDB(t)
	c := context.Background()
	a, b := rvItem(), rvItem()
	if e := s.PutItem(c, a, 0); e != nil {
		t.Fatal(e)
	}
	if e := s.PutItem(c, b, 0); e != nil {
		t.Fatal(e)
	}
	cc := rvItem()
	cc.DependencyIDs = []string{b.ID}
	if e := s.PutItem(c, cc, 0); e != nil {
		t.Fatal(e)
	}
	ev0, _ := s.Events(c, 0)
	b2 := b
	b2.Revision = 2
	b2.DependencyIDs = []string{a.ID, cc.ID}
	if e := s.PutItem(c, b2, 1); e == nil {
		t.Fatal("cycle accepted")
	} else {
		t.Logf("cycle rejected: %v", e)
	}
	got, e := s.GetItem(c, b.ID)
	if e != nil {
		t.Fatal(e)
	}
	if got.Revision != 1 || len(got.DependencyIDs) != 0 {
		t.Fatalf("partial write after rollback: rev=%d deps=%v", got.Revision, got.DependencyIDs)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM item_dependency WHERE item_id=?", b.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("partial dependency rows persisted: %d", n)
	}
	ev1, _ := s.Events(c, 0)
	if len(ev0) != len(ev1) {
		t.Fatalf("events leaked on rollback: %d -> %d", len(ev0), len(ev1))
	}
}

func TestReviewA04DanglingEvidenceAndDependency(t *testing.T) {
	s := rvDB(t)
	c := context.Background()
	ev0, _ := s.Events(c, 0)
	x := rvItem()
	x.DependencyIDs = []string{contract.NewID()}
	if e := s.PutItem(c, x, 0); e == nil {
		t.Fatal("dangling dependency accepted")
	} else {
		t.Logf("dangling dependency rejected: %v", e)
	}
	y := rvItem()
	y.Evidence = []contract.EvidenceRef{{ObjectID: contract.NewID(), SHA256: strings.Repeat("a", 64), Locator: "x", OriginID: "o", DataClass: "SYNTHETIC"}}
	if e := s.PutItem(c, y, 0); e == nil {
		t.Fatal("dangling evidence accepted")
	} else {
		t.Logf("dangling evidence rejected: %v", e)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM item").Scan(&n)
	if n != 0 {
		t.Fatalf("partial item rows: %d", n)
	}
	ev1, _ := s.Events(c, 0)
	if len(ev0) != len(ev1) {
		t.Fatalf("events leaked: %d -> %d", len(ev0), len(ev1))
	}
}

func TestReviewA19JsonSqlConsistencyChecks(t *testing.T) {
	s := rvDB(t)
	c := context.Background()
	it := rvItem()
	if e := s.PutItem(c, it, 0); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE item SET status='DONE' WHERE id=?", it.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.GetItem(c, it.ID); e == nil {
		t.Fatal("status divergence not detected")
	} else {
		t.Logf("sql status tamper -> %v", e)
	}
	if _, e := s.DB.Exec("UPDATE item SET status='OPEN' WHERE id=?", it.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE item SET payload_json=json_set(payload_json,'$.revision',9) WHERE id=?", it.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.GetItem(c, it.ID); e == nil {
		t.Fatal("payload revision divergence not detected")
	} else {
		t.Logf("payload revision tamper -> %v", e)
	}
}

func TestReviewA19MigrationRejection(t *testing.T) {
	{
		d := t.TempDir()
		dbp := filepath.Join(d, "state.db")
		objp := filepath.Join(d, "objects")
		s, e := Init(dbp, objp)
		if e != nil {
			t.Fatal(e)
		}
		s.DB.Exec("UPDATE schema_migration SET version=2")
		s.Close()
		if _, e := Open(dbp, objp); e == nil {
			t.Fatal("future version accepted")
		} else {
			t.Logf("version=2 -> %v", e)
		}
	}
	{
		d := t.TempDir()
		dbp := filepath.Join(d, "state.db")
		objp := filepath.Join(d, "objects")
		s, e := Init(dbp, objp)
		if e != nil {
			t.Fatal(e)
		}
		s.DB.Exec("UPDATE schema_migration SET checksum='deadbeef'")
		s.Close()
		if _, e := Open(dbp, objp); e == nil {
			t.Fatal("checksum mismatch accepted")
		} else {
			t.Logf("checksum mismatch -> %v", e)
		}
	}
	{
		d := t.TempDir()
		dbp := filepath.Join(d, "state.db")
		objp := filepath.Join(d, "objects")
		if e := os.WriteFile(dbp, []byte{}, 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := Init(dbp, objp); e == nil {
			t.Fatal("half-initialized db accepted")
		} else {
			t.Logf("half-init (crash window) -> %v", e)
		}
	}
	if _, e := Open(filepath.Join(t.TempDir(), "missing.db"), filepath.Join(t.TempDir(), "objects")); e == nil {
		t.Fatal("missing db accepted")
	} else {
		t.Logf("missing db -> %v", e)
	}
}

func TestReviewA04SourceSyncedDivergenceProbe(t *testing.T) {
	s := rvDB(t)
	c := context.Background()
	sid := contract.NewID()
	if e := s.EnsureSource(c, contract.SourceState{SchemaVersion: 1, ID: sid, Kind: "FIXTURE", Revision: 1, StaleAfterSeconds: 86400, Enabled: true, DataClass: "SYNTHETIC", Extensions: map[string]any{}}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.Exec("UPDATE source_state SET revision=5 WHERE id=?", sid); e != nil {
		t.Fatal(e)
	}
	ev0, e := s.Events(c, 0)
	if e != nil {
		t.Fatal(e)
	}
	err := s.SourceSynced(c, sid, 1)
	ev1, _ := s.Events(c, 0)
	var revSQL int
	s.DB.QueryRow("SELECT revision FROM source_state WHERE id=?", sid).Scan(&revSQL)
	t.Logf("SourceSynced err=%v sql_rev=%d events=%d->%d", err, revSQL, len(ev0), len(ev1))
	if err == nil && revSQL == 5 && len(ev1) == len(ev0)+1 {
		t.Logf("REVIEW-FINDING: SourceSynced CAS update matched 0 rows, yet method returned success and appended a source.synced event (no RowsAffected check)")
	}
}

func TestReviewA04PragmasFourConnsAndWAL(t *testing.T) {
	s := rvDB(t)
	var mode string
	if e := s.DB.QueryRow("PRAGMA journal_mode").Scan(&mode); e != nil {
		t.Fatal(e)
	}
	if mode != "wal" {
		t.Errorf("journal_mode=%q want wal", mode)
	}
	for i := 0; i < 4; i++ {
		conn, e := s.DB.Conn(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		var fk, bt, sy int
		conn.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fk)
		conn.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&bt)
		conn.QueryRowContext(context.Background(), "PRAGMA synchronous").Scan(&sy)
		t.Logf("conn %d: foreign_keys=%d busy_timeout=%d synchronous=%d", i, fk, bt, sy)
		if fk != 1 || bt != 5000 || sy != 2 {
			t.Errorf("conn %d: fk=%d bt=%d sync=%d", i, fk, bt, sy)
		}
		conn.Close()
	}
}

func TestReviewTimestampLeniencyProbe(t *testing.T) {
	s := rvDB(t)
	c := context.Background()
	for _, bad := range []string{"garbage", "2026-09-14T03:00:00z", "2026-09-14T03:00:00+08:00"} {
		it := rvItem()
		it.UpdatedAt = bad
		e := s.PutItem(c, it, 0)
		t.Logf("updated_at=%q -> err=%v", bad, e)
		if e == nil {
			got, ge := s.GetItem(c, it.ID)
			t.Logf("  persisted updated_at=%q get_err=%v", got.UpdatedAt, ge)
		}
	}
}

func TestReviewA19MigrationFailureRollback(t *testing.T) {
	d := t.TempDir()
	dbp := filepath.Join(d, "state.db")
	objp := filepath.Join(d, "objects")
	s0, e := connect(dbp, objp)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := s0.DB.Exec("CREATE TABLE item(x INTEGER)"); e != nil {
		t.Fatal(e)
	}
	s0.Close()
	if _, e := Init(dbp, objp); e == nil {
		t.Fatal("conflicting schema accepted")
	} else {
		t.Logf("init with pre-existing conflicting table -> %v", e)
	}
	s1, e := connect(dbp, objp)
	if e != nil {
		t.Fatal(e)
	}
	defer s1.Close()
	var name string
	err := s1.DB.QueryRow("SELECT name FROM sqlite_master WHERE name='object_ref'").Scan(&name)
	t.Logf("after failed migration, object_ref present: %v (err=%v)", name != "", err)
	if err == nil {
		t.Logf("REVIEW-FINDING: failed migration left partial DDL committed (object_ref exists) — rollback not effective")
	}
	var mig string
	err2 := s1.DB.QueryRow("SELECT name FROM sqlite_master WHERE name='schema_migration'").Scan(&mig)
	t.Logf("schema_migration present after failed init: %v (err=%v)", err2 == nil, err2)
}
