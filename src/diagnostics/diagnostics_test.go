package diagnostics

import (
	"context"
	"path/filepath"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestDoctorDoesNotInventHeartbeats(t *testing.T) {
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := Doctor(context.Background(), s, d)
	if e != nil {
		t.Fatal(e)
	}
	if r.Database.Integrity != "ok" || r.Database.SchemaVersion != 1 {
		t.Fatal(r)
	}
	for _, p := range []string{"core", "runner"} {
		if r.Heartbeats[p].(map[string]any)["state"] != "UNKNOWN" {
			t.Fatal("invented heartbeat")
		}
	}
}
func TestDoctorEpochAndBackupHistory(t *testing.T) {
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.db"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Backup(context.Background(), filepath.Join(t.TempDir(), "backup")); e != nil {
		t.Fatal(e)
	}
	r, e := DoctorWithEpoch(context.Background(), s, d, time.Now().Add(-25*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if r.Database.Consciousness["overdue"] != true {
		t.Fatal("overdue not calculated", r.Database.Consciousness)
	}
	if r.LastBackup.(map[string]any)["state"] != "COMPLETE" {
		t.Fatal("backup time unavailable after actual backup")
	}
	if r.Database.BudgetExhaustedReason == "" {
		t.Fatal("unknown budget lacks reason")
	}
}
