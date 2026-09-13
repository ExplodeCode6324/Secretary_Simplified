package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestBackupConcurrentRestoreFrozenAndTamper(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	o, e := s.PutObject(ctx, []byte("backup fixture"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if e := s.PutItem(ctx, fixtureItem(), 0); e != nil {
				t.Error(e)
				return
			}
		}
	}()
	dest := filepath.Join(t.TempDir(), "backup")
	m, e := s.Backup(ctx, dest)
	wg.Wait()
	if e != nil {
		t.Fatal(e)
	}
	if m.Status != "COMPLETE" {
		t.Fatal(m)
	}
	if _, e = VerifyBackup(ctx, dest); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "restore")
	restored, e := Restore(ctx, dest, target)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	if _, e = os.Stat(filepath.Join(target, "execution_frozen")); e != nil {
		t.Fatal(e)
	}
	b, e := restored.ReadObject(ctx, o.ID)
	if e != nil || string(b) != "backup fixture" {
		t.Fatal(string(b), e)
	}
	if _, e = Restore(ctx, dest, target); e == nil {
		t.Fatal("nonempty restore accepted")
	}
	if e = os.WriteFile(filepath.Join(dest, "objects", o.RelativePath), []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = VerifyBackup(ctx, dest); e == nil {
		t.Fatal("tampered object accepted")
	}
}
func TestBackupCorruptDatabaseRejected(t *testing.T) {
	s := foundationDB(t)
	d := filepath.Join(t.TempDir(), "backup")
	if _, e := s.Backup(context.Background(), d); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(d, "secretary.sqlite"), []byte("not a database"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := VerifyBackup(context.Background(), d); e == nil {
		t.Fatal("corrupt db accepted")
	}
}
func TestManifestClassificationAndRequiredFields(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	if _, e := s.PutObject(ctx, []byte("classified backup"), "text/plain", "SYNTHETIC"); e != nil {
		t.Fatal(e)
	}
	d := filepath.Join(t.TempDir(), "backup")
	m, e := s.Backup(ctx, d)
	if e != nil {
		t.Fatal(e)
	}
	m.Objects[0].DataClass = "SECRET"
	raw, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(d, "manifest.json"), raw, 0600)
	if _, e = VerifyBackup(ctx, d); e == nil {
		t.Fatal("manifest classification downgrade accepted")
	}
	var v map[string]any
	json.Unmarshal(raw, &v)
	delete(v, "created_at")
	raw, _ = json.Marshal(v)
	os.WriteFile(filepath.Join(d, "manifest.json"), raw, 0600)
	if _, e = VerifyBackup(ctx, d); e == nil {
		t.Fatal("missing manifest time accepted")
	}
}
