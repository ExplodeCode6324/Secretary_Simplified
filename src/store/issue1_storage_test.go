package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"secretarysimplified/contract"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestObjectPublishProcessHelper(t *testing.T) {
	dir := os.Getenv("SECRETARY_TEST_OBJECT_DIR")
	if dir == "" {
		return
	}
	s, e := Open(filepath.Join(dir, "state.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	b := bytes.Repeat([]byte("synthetic atomic publication\n"), 8192)
	ctx := context.Background()
	stage := os.Getenv("SECRETARY_TEST_OBJECT_STAGE")
	if stage != "" {
		ctx = context.WithValue(ctx, objectPublishHookKey{}, func(point, path string) error {
			if point == stage {
				if point == "temp_created" {
					if e := os.WriteFile(path, b[:len(b)/2], 0600); e != nil {
						t.Fatal(e)
					}
				}
				os.Exit(41)
			}
			return nil
		})
	}
	if os.Getenv("SECRETARY_TEST_OBJECT_BARRIER") == "1" {
		ready := filepath.Join(dir, "ready-"+os.Getenv("SECRETARY_TEST_OBJECT_SUFFIX"))
		if e = os.WriteFile(ready, nil, 0600); e != nil {
			t.Fatal(e)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, e = os.Stat(filepath.Join(dir, "start-publish")); e == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("barrier timeout")
			}
			time.Sleep(time.Millisecond)
		}
		ctx = context.WithValue(ctx, objectPublishHookKey{}, func(stage, path string) error {
			if stage == "temp_created" {
				time.Sleep(30 * time.Millisecond)
			}
			return nil
		})
	}
	v, e := s.PutObject(ctx, b, "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(v)
	if e = os.WriteFile(filepath.Join(dir, "result-"+os.Getenv("SECRETARY_TEST_OBJECT_SUFFIX")+".json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func objectChild(dir, stage, suffix string) *exec.Cmd {
	c := exec.Command(os.Args[0], "-test.run=^TestObjectPublishProcessHelper$", "-test.count=1")
	c.Env = append(os.Environ(), "SECRETARY_TEST_OBJECT_DIR="+dir, "SECRETARY_TEST_OBJECT_STAGE="+stage, "SECRETARY_TEST_OBJECT_SUFFIX="+suffix)
	return c
}
func TestObjectPublicationCrashStagesRecover(t *testing.T) {
	for _, stage := range []string{"temp_created", "before_publish", "after_publish", "before_commit"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			s, e := Init(filepath.Join(dir, "state.db"), filepath.Join(dir, "objects"))
			if e != nil {
				t.Fatal(e)
			}
			s.Close()
			output, e := objectChild(dir, stage, "crash").CombinedOutput()
			var ex *exec.ExitError
			if !errors.As(e, &ex) || ex.ExitCode() != 41 {
				t.Fatalf("stage not injected: %v %s", e, output)
			}
			s, e = Open(filepath.Join(dir, "state.db"), filepath.Join(dir, "objects"))
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			b := bytes.Repeat([]byte("synthetic atomic publication\n"), 8192)
			v, e := s.PutObject(context.Background(), b, "text/plain", "SYNTHETIC")
			if e != nil {
				t.Fatal(e)
			}
			got, e := s.ReadObject(context.Background(), v.ID)
			if e != nil || !bytes.Equal(got, b) {
				t.Fatal(e)
			}
			var n int
			s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&n)
			if n != 1 {
				t.Fatal(n)
			}
			if _, e = s.RecoverObjectStaging(context.Background(), 100); e != nil {
				t.Fatal(e)
			}
			temps, _ := filepath.Glob(filepath.Join(dir, "objects", ".object-tmp-*"))
			if len(temps) != 0 {
				t.Fatal(temps)
			}
		})
	}
}
func TestObjectPublicationIndependentProcesses(t *testing.T) {
	dir := t.TempDir()
	s, e := Init(filepath.Join(dir, "state.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a, b := objectChild(dir, "", "a"), objectChild(dir, "", "b")
	a.Env = append(a.Env, "SECRETARY_TEST_OBJECT_BARRIER=1")
	b.Env = append(b.Env, "SECRETARY_TEST_OBJECT_BARRIER=1")
	if e = a.Start(); e != nil {
		t.Fatal(e)
	}
	if e = b.Start(); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, ea := os.Stat(filepath.Join(dir, "ready-a"))
		_, eb := os.Stat(filepath.Join(dir, "ready-b"))
		if ea == nil && eb == nil {
			break
		}
		if time.Now().After(deadline) {
			a.Process.Kill()
			b.Process.Kill()
			t.Fatal("independent publishers not ready")
		}
		time.Sleep(time.Millisecond)
	}
	if e = os.WriteFile(filepath.Join(dir, "start-publish"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 2)
	go func() { done <- a.Wait() }()
	go func() { done <- b.Wait() }()
	expected := bytes.Repeat([]byte("synthetic atomic publication\n"), 8192)
	finished := 0
	for finished < 2 {
		select {
		case e := <-done:
			if e != nil {
				t.Fatal(e)
			}
			finished++
		case <-time.After(time.Millisecond):
			files, _ := filepath.Glob(filepath.Join(dir, "objects", "*.blob"))
			for _, f := range files {
				raw, e := os.ReadFile(f)
				if e != nil || !bytes.Equal(raw, expected) {
					t.Fatalf("observed partial final file: %v len=%d", e, len(raw))
				}
			}
		}
	}
	var va, vb contract.ObjectRef
	raw, _ := os.ReadFile(filepath.Join(dir, "result-a.json"))
	json.Unmarshal(raw, &va)
	raw, _ = os.ReadFile(filepath.Join(dir, "result-b.json"))
	json.Unmarshal(raw, &vb)
	aa, _ := json.Marshal(va)
	bb, _ := json.Marshal(vb)
	if va.ID == "" || !bytes.Equal(aa, bb) {
		t.Fatal("non-idempotent result", string(aa), string(bb))
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
}
func TestObjectOrphanRecoveryDoesNotOverwriteCommittedDamage(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	b := []byte("synthetic orphan repair")
	v, e := s.PutObject(ctx, b, "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("DELETE FROM object_ref WHERE id=?", v.ID); e != nil {
		t.Fatal(e)
	}
	bad := []byte("half")
	os.WriteFile(filepath.Join(s.ObjectsDir, v.RelativePath), bad, 0600)
	if _, e = s.PutObject(ctx, b, "text/plain", "SYNTHETIC"); e != nil {
		t.Fatal(e)
	}
	got, e := s.ReadObject(ctx, v.ID)
	if e != nil || !bytes.Equal(got, b) {
		t.Fatal(e)
	}
	quarantine, _ := filepath.Glob(filepath.Join(s.ObjectsDir, v.RelativePath+".orphan-*"))
	if len(quarantine) != 1 {
		t.Fatal("missing isolated corrupt evidence", quarantine)
	}
	original, _ := os.ReadFile(quarantine[0])
	if !bytes.Equal(original, bad) {
		t.Fatal("quarantine evidence changed")
	}
	os.WriteFile(filepath.Join(s.ObjectsDir, v.RelativePath), bad, 0600)
	if _, e = s.ReadObject(ctx, v.ID); e == nil {
		t.Fatal("read accepted committed damage")
	}
	if _, e = s.PutObject(ctx, b, "text/plain", "SYNTHETIC"); e == nil {
		t.Fatal("write healed committed damage")
	}
	raw, _ := os.ReadFile(filepath.Join(s.ObjectsDir, v.RelativePath))
	if !bytes.Equal(raw, bad) {
		t.Fatal("damaged evidence replaced")
	}
}
func TestAdmissionRejectionDoesNotArchiveObjects(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	accepted := questionInput()
	if _, e := s.AcceptInput(ctx, accepted, 1); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 12; i++ {
		in := questionInput()
		in.Text = contract.NewID()
		if _, e := s.AcceptInput(ctx, in, 1); e == nil || e.Error() != "BACKPRESSURE" {
			t.Fatal(e)
		}
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&count)
	files, _ := filepath.Glob(filepath.Join(s.ObjectsDir, "*.blob"))
	if count != 1 || len(files) != 1 {
		t.Fatal("full queue leaked", count, len(files))
	}
	for i := 0; i < 12; i++ {
		in := questionInput()
		in.Text = contract.NewID()
		_, e := s.AcceptTyped(ctx, in, 100, map[string]any{"text": "fixed synthetic receipt", "evidence": []any{}}, nil, func(*sql.Tx, contract.InputTurn) error { return errors.New("synthetic rejection") })
		if e == nil {
			t.Fatal("typed reject accepted")
		}
	}
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&count)
	files, _ = filepath.Glob(filepath.Join(s.ObjectsDir, "*.blob"))
	if count != 1 || len(files) != 1 {
		t.Fatal("typed rollback leaked", count, len(files))
	}
	v, e := s.AcceptInput(ctx, accepted, 1)
	if e != nil || v.ID == "" {
		t.Fatal("valid replay damaged", e)
	}
}
func TestAdmissionLastSlotConcurrentStores(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	var path string
	s.DB.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path)
	other, e := Open(path, s.ObjectsDir)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, db := range []*Store{s, other} {
		wg.Add(1)
		go func(db *Store) {
			defer wg.Done()
			in := questionInput()
			in.Text = contract.NewID()
			_, e := db.AcceptInput(ctx, in, 1)
			errs <- e
		}(db)
	}
	wg.Wait()
	close(errs)
	success, reject := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if strings.Contains(e.Error(), "BACKPRESSURE") {
			reject++
		} else {
			t.Fatal(e)
		}
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&n)
	files, _ := filepath.Glob(filepath.Join(s.ObjectsDir, "*.blob"))
	if success != 1 || reject != 1 || n != 1 || len(files) != 1 {
		t.Fatal(success, reject, n, len(files))
	}
}

func TestObjectCancelledRollbackAndLockWait(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	v, e := s.PutObject(ctx, []byte("preserved synthetic evidence"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	e = s.WriteObjects(cancelled, func(_ *sql.Tx, w *ObjectWriter) error {
		if _, e := w.Put(cancelled, []byte("cancelled synthetic admission"), "text/plain", "SYNTHETIC"); e != nil {
			return e
		}
		cancel()
		return context.Canceled
	})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&n)
	files, _ := filepath.Glob(filepath.Join(s.ObjectsDir, "*.blob"))
	if n != 1 || len(files) != 1 {
		t.Fatal("cancel left admission bytes", n, len(files))
	}
	if _, e = s.ReadObject(ctx, v.ID); e != nil {
		t.Fatal("existing ref damaged", e)
	}
	lock, e := s.lockObjects(ctx)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := os.Stat(filepath.Join(s.ObjectsDir, ".publish.lock"))
	start := time.Now()
	wait, cancelWait := context.WithTimeout(ctx, 40*time.Millisecond)
	defer cancelWait()
	_, e = s.PutObject(wait, []byte("will not publish"), "text/plain", "SYNTHETIC")
	unlockObjects(lock)
	if !errors.Is(e, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatal("flock wait did not cancel", e, time.Since(start))
	}
	after, _ := os.Stat(filepath.Join(s.ObjectsDir, ".publish.lock"))
	if !os.SameFile(before, after) {
		t.Fatal("publication lock inode replaced")
	}
}
func TestObjectRecoveryIsBoundedAndProtectsReferencePaths(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	v, e := s.PutObject(ctx, []byte("kept synthetic object"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	referencedTemp := ".object-tmp-" + contract.NewID()
	if e = os.Rename(filepath.Join(s.ObjectsDir, v.RelativePath), filepath.Join(s.ObjectsDir, referencedTemp)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE object_ref SET relative_path=? WHERE id=?", referencedTemp, v.ID); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = os.WriteFile(filepath.Join(s.ObjectsDir, ".object-tmp-"+contract.NewID()), []byte("uncommitted synthetic stage"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	unknown := filepath.Join(s.ObjectsDir, ".object-tmp-not-a-uuid")
	os.WriteFile(unknown, []byte("not our namespace"), 0600)
	removed, e := s.RecoverObjectStaging(ctx, 1)
	if e != nil || removed != 1 {
		t.Fatal(removed, e)
	}
	if _, e = s.ReadObject(ctx, v.ID); e != nil {
		t.Fatal("cleanup removed referenced temp-shaped path", e)
	}
	if _, e = os.Stat(unknown); e != nil {
		t.Fatal("cleanup removed unknown file")
	}
	removed, e = s.RecoverObjectStaging(ctx, 10)
	if e != nil || removed != 1 {
		t.Fatal(removed, e)
	}
}

func TestObjectUncertainCommitResultPreservesCommittedReference(t *testing.T) {
	s := foundationDB(t)
	b := []byte("committed synthetic object before caller result lost")
	ctx := context.WithValue(context.Background(), objectPublishHookKey{}, func(stage, path string) error {
		if stage == "after_commit" {
			return errors.New("injected lost commit acknowledgement")
		}
		return nil
	})
	original, e := s.PutObject(ctx, b, "text/plain", "SYNTHETIC")
	if e == nil {
		t.Fatal("missing result-loss injection")
	}
	got, e := s.ReadObject(context.Background(), original.ID)
	if e != nil || !bytes.Equal(got, b) {
		t.Fatal("committed file removed after uncertain acknowledgement", e)
	}
	replay, e := s.PutObject(context.Background(), b, "text/plain", "SYNTHETIC")
	if e != nil || replay.ID != original.ID || replay.CreatedAt != original.CreatedAt {
		t.Fatal("committed retry changed identity", e)
	}
}

func TestObjectValidOrphanSyncPrecedesMetadata(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	b := []byte("synthetic fully written but uncommitted orphan")
	v := objectValue(b, "text/plain", "SYNTHETIC")
	if e := os.WriteFile(filepath.Join(s.ObjectsDir, v.RelativePath), b, 0600); e != nil {
		t.Fatal(e)
	}
	syncObserved := false
	injected := context.WithValue(ctx, objectPublishHookKey{}, func(stage, path string) error {
		if stage == "adopt_after_sync" {
			syncObserved = true
			var n int
			if e := s.DB.QueryRow("SELECT count(*) FROM object_ref WHERE id=?", v.ID).Scan(&n); e != nil || n != 0 {
				t.Fatal("metadata preceded adoption durability", e, n)
			}
			return errors.New("injected before orphan metadata")
		}
		return nil
	})
	if _, e := s.PutObject(injected, b, "text/plain", "SYNTHETIC"); e == nil || !syncObserved {
		t.Fatal("missing durability checkpoint", e)
	}
	if _, e := s.PutObject(ctx, b, "text/plain", "SYNTHETIC"); e != nil {
		t.Fatal(e)
	}
	if got, e := s.ReadObject(ctx, v.ID); e != nil || !bytes.Equal(got, b) {
		t.Fatal(e)
	}
}

// Full row snapshots include SQLite sequences: a rejected Typed transaction must
// restore request, conversation, receipt, event and business state, not just counts.
func TestAdmissionTypedBusinessWriteRollbackFullBaseline(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	original := questionInput()
	if _, err := s.AcceptInput(ctx, original, 100); err != nil {
		t.Fatal(err)
	}
	existing := fixtureItem()
	if err := s.PutItem(ctx, existing, 0); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		rows, err := s.DB.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for rows.Next() {
			var name string
			if err = rows.Scan(&name); err != nil {
				t.Fatal(err)
			}
			names = append(names, name)
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		all := map[string][]string{}
		for _, name := range names {
			rs, err := s.DB.Query(`SELECT * FROM "` + strings.ReplaceAll(name, `"`, `""`) + `"`)
			if err != nil {
				t.Fatal(err)
			}
			cols, err := rs.Columns()
			if err != nil {
				t.Fatal(err)
			}
			encoded := []string{}
			for rs.Next() {
				values := make([]any, len(cols))
				dest := make([]any, len(cols))
				for i := range values {
					dest[i] = &values[i]
				}
				if err = rs.Scan(dest...); err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(values)
				if err != nil {
					t.Fatal(err)
				}
				encoded = append(encoded, string(raw))
			}
			if err = rs.Err(); err != nil {
				t.Fatal(err)
			}
			rs.Close()
			sort.Strings(encoded)
			all[name] = encoded
		}
		raw, err := json.Marshal(all)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	blobs := func() string {
		t.Helper()
		paths, err := filepath.Glob(filepath.Join(s.ObjectsDir, "*.blob"))
		if err != nil {
			t.Fatal(err)
		}
		objects := map[string][]byte{}
		for _, path := range paths {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			objects[filepath.Base(path)] = b
		}
		raw, err := json.Marshal(objects)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	before, beforeObjects := snapshot(), blobs()
	in := questionInput()
	in.Text = "synthetic Typed writes an item then aborts"
	item := fixtureItem()
	injected := errors.New("INJECTED_AFTER_TYPED_BUSINESS_WRITE")
	called := false
	_, err := s.AcceptTyped(ctx, in, 100, map[string]any{"text": "synthetic reply", "evidence": []any{}}, []string{}, func(tx *sql.Tx, turn contract.InputTurn) error {
		if err := PutItemTx(ctx, tx, item, 0); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM item WHERE id=?", item.ID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("business write did not happen: %d %v", n, err)
		}
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM change_event WHERE entity_id=?", item.ID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("event write did not happen: %d %v", n, err)
		}
		if turn.ID == "" {
			t.Fatal("input not accepted inside callback")
		}
		called = true
		return injected
	})
	if !called || !errors.Is(err, injected) {
		t.Fatal("callback failure was not exercised", called, err)
	}
	if snapshot() != before {
		t.Fatal("Typed rollback changed persisted table content")
	}
	if blobs() != beforeObjects {
		t.Fatal("Typed rollback changed formal object names or bytes")
	}
	// The exact rejected request remains admissible; successful replay cannot run
	// its business callback twice.
	calls := 0
	apply := func(tx *sql.Tx, _ contract.InputTurn) error { calls++; return PutItemTx(ctx, tx, item, 0) }
	for i := 0; i < 2; i++ {
		if _, err = s.AcceptTyped(ctx, in, 100, map[string]any{"text": "synthetic reply", "evidence": []any{}}, []string{}, apply); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("duplicate Typed business execution", calls)
	}
}
