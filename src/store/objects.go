package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"strings"
	"syscall"
	"time"
)

// ObjectWriter is valid only inside WriteObjects. Never retain it or commit its
// transaction. Every publisher acquires a distinct flock fd before SQLite.
type ObjectWriter struct {
	store   *Store
	tx      *sql.Tx
	active  bool
	created []string
}
type objectPublishHookKey struct{}

func objectHook(ctx context.Context, stage, path string) error {
	if f, ok := ctx.Value(objectPublishHookKey{}).(func(string, string) error); ok {
		return f(stage, path)
	}
	return nil
}

func (s *Store) lockObjects(ctx context.Context) (*os.File, error) {
	f, e := os.OpenFile(filepath.Join(s.ObjectsDir, ".publish.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	for {
		if e = ctx.Err(); e != nil {
			f.Close()
			return nil, e
		}
		e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == nil {
			return f, nil
		}
		if e != syscall.EWOULDBLOCK && e != syscall.EAGAIN {
			f.Close()
			return nil, e
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func unlockObjects(f *os.File) { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }
func (s *Store) syncObjectDir() error {
	d, e := os.Open(s.ObjectsDir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}

// WriteObjects binds durable file publication, object_ref and business changes.
// Lock order is always flock -> Store mutex -> BEGIN IMMEDIATE. Calling this
// from Store.Write (or recursively) is forbidden; use the provided writer.Put.
func (s *Store) WriteObjects(ctx context.Context, fn func(*sql.Tx, *ObjectWriter) error) error {
	lock, e := s.lockObjects(ctx)
	if e != nil {
		return e
	}
	defer unlockObjects(lock)
	s.mu.Lock()
	defer s.mu.Unlock()
	if e = ctx.Err(); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	w := &ObjectWriter{store: s, tx: tx, active: true}
	defer func() { w.active = false }()
	e = fn(tx, w)
	if e == nil {
		e = objectHook(ctx, "before_commit", "")
	}
	if e == nil {
		e = ctx.Err()
	}
	if e == nil {
		e = tx.Commit()
		if e == nil {
			e = objectHook(ctx, "after_commit", "")
		}
	}
	if e == nil {
		return nil
	}
	// ctx cancellation may already have auto-rolled back SQLite. flock remains
	// held: no other publisher can adopt a final path before cleanup decides.
	tx.Rollback()
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var cleanup error
	for _, path := range w.created {
		var id string
		qerr := s.DB.QueryRowContext(persist, "SELECT id FROM object_ref WHERE relative_path=?", path).Scan(&id)
		if qerr == nil {
			_, qerr = s.ReadObject(persist, id)
			if qerr != nil {
				cleanup = errors.Join(cleanup, qerr)
			}
			continue
		}
		if qerr != sql.ErrNoRows {
			cleanup = errors.Join(cleanup, qerr)
			continue
		} // uncertain: retain complete orphan
		root, err := os.OpenRoot(s.ObjectsDir)
		if err == nil {
			err = root.Remove(path)
			root.Close()
		}
		if err != nil && !os.IsNotExist(err) {
			cleanup = errors.Join(cleanup, err)
		}
	}
	if len(w.created) > 0 {
		cleanup = errors.Join(cleanup, s.syncObjectDir())
	}
	return errors.Join(e, cleanup)
}
func (s *Store) PutObject(ctx context.Context, b []byte, mediaType, dataClass string) (v contract.ObjectRef, err error) {
	err = s.WriteObjects(ctx, func(_ *sql.Tx, w *ObjectWriter) error {
		var e error
		v, e = w.Put(ctx, b, mediaType, dataClass)
		return e
	})
	return
}
func objectValue(b []byte, mediaType, dataClass string) contract.ObjectRef {
	sum := sha256.Sum256(append([]byte(dataClass+"\x00"+mediaType+"\x00"), b...))
	sum[6] = (sum[6] & 15) | 80
	sum[8] = (sum[8] & 63) | 128
	id := fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
	return contract.ObjectRef{SchemaVersion: 1, ID: id, RelativePath: id + ".blob", SHA256: contract.Hash(b), MediaType: mediaType, ByteSize: len(b), DataClass: dataClass, CreatedAt: contract.Now(), Extensions: map[string]any{}}
}
func (w *ObjectWriter) Put(ctx context.Context, b []byte, mediaType, dataClass string) (v contract.ObjectRef, err error) {
	if !w.active {
		return v, errors.New("OBJECT_TRANSACTION_CLOSED")
	}
	if err = ctx.Err(); err != nil {
		return
	}
	v = objectValue(b, mediaType, dataClass)
	if err = contract.Validate("ObjectRef", v); err != nil {
		return
	}
	old, e := getObjectRef(ctx, w.tx, v.ID)
	if e == nil {
		if old.SHA256 != v.SHA256 || old.MediaType != v.MediaType || old.DataClass != v.DataClass || old.ByteSize != v.ByteSize {
			return old, errors.New("STORAGE_CORRUPTION: committed object metadata")
		}
		_, e = w.store.readObjectRef(old)
		return old, e
	}
	if e != sql.ErrNoRows {
		return v, e
	}
	root, e := os.OpenRoot(w.store.ObjectsDir)
	if e != nil {
		return v, e
	}
	defer root.Close()
	needsPublish := true
	if stat, e := root.Lstat(v.RelativePath); e == nil {
		if !stat.Mode().IsRegular() {
			return v, errors.New("STORAGE_CORRUPTION: nonregular orphan object")
		}
		raw, e := root.ReadFile(v.RelativePath)
		if e != nil {
			return v, e
		}
		if len(raw) == v.ByteSize && contract.Hash(raw) == v.SHA256 {
			// An orphan's hash proves content, not durability: it may predate
			// file/dir fsync in an interrupted older publication protocol.
			f, e := root.Open(v.RelativePath)
			if e != nil {
				return v, e
			}
			e = f.Sync()
			ce := f.Close()
			if e == nil {
				e = ce
			}
			if e != nil {
				return v, e
			}
			if e = w.store.syncObjectDir(); e != nil {
				return v, e
			}
			if e = objectHook(ctx, "adopt_after_sync", filepath.Join(w.store.ObjectsDir, v.RelativePath)); e != nil {
				return v, e
			}
			needsPublish = false
		} else {
			quarantine := v.RelativePath + ".orphan-" + contract.NewID()
			if e = root.Rename(v.RelativePath, quarantine); e != nil {
				return v, e
			}
			if e = w.store.syncObjectDir(); e != nil {
				return v, e
			}
		}
	} else if !os.IsNotExist(e) {
		return v, e
	}
	if needsPublish {
		temp := ".object-tmp-" + contract.NewID()
		f, e := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return v, e
		}
		defer root.Remove(temp)
		if e = objectHook(ctx, "temp_created", filepath.Join(w.store.ObjectsDir, temp)); e != nil {
			f.Close()
			return v, e
		}
		n, e := f.Write(b)
		if e == nil && n != len(b) {
			e = io.ErrShortWrite
		}
		if e == nil {
			e = f.Sync()
		}
		ce := f.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			return v, e
		}
		if e = ctx.Err(); e != nil {
			return v, e
		}
		if e = objectHook(ctx, "before_publish", filepath.Join(w.store.ObjectsDir, temp)); e != nil {
			return v, e
		}
		// Hard-link creation is atomic and never replaces a committed path. No
		// copy/rename fallback on filesystems that reject this operation.
		if e = root.Link(temp, v.RelativePath); e != nil {
			return v, e
		}
		w.created = append(w.created, v.RelativePath)
		if e = root.Remove(temp); e != nil {
			return v, e
		}
		if e = w.store.syncObjectDir(); e != nil {
			return v, e
		}
		if e = objectHook(ctx, "after_publish", filepath.Join(w.store.ObjectsDir, v.RelativePath)); e != nil {
			return v, e
		}
	}
	_, e = w.tx.ExecContext(ctx, "INSERT INTO object_ref(id,relative_path,sha256,media_type,byte_size,data_class,created_at) VALUES(?,?,?,?,?,?,?)", v.ID, v.RelativePath, v.SHA256, v.MediaType, v.ByteSize, v.DataClass, v.CreatedAt)
	return v, e
}

// RecoverObjectStaging removes at most limit known temporary files under the
// same publication lock. It never removes final blobs, quarantined evidence or
// a path named by a committed object_ref. No age heuristic is required because
// all active temp writers hold this flock too. The lock inode is never deleted.
func (s *Store) RecoverObjectStaging(ctx context.Context, limit int) (removed int, err error) {
	if limit < 1 || limit > 1000 {
		return 0, errors.New("INVALID_RECOVERY_LIMIT")
	}
	err = s.WriteObjects(ctx, func(tx *sql.Tx, _ *ObjectWriter) error {
		entries, e := os.ReadDir(s.ObjectsDir)
		if e != nil {
			return e
		}
		root, e := os.OpenRoot(s.ObjectsDir)
		if e != nil {
			return e
		}
		defer root.Close()
		for _, entry := range entries {
			if removed >= limit {
				break
			}
			name := entry.Name()
			if !strings.HasPrefix(name, ".object-tmp-") {
				continue
			}
			id := strings.TrimPrefix(name, ".object-tmp-")
			if e = contract.Validate("VersionRef", map[string]any{"id": id, "revision": 1}); e != nil {
				continue
			}
			var n int
			if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM object_ref WHERE relative_path=?", name).Scan(&n); e != nil {
				return e
			}
			if n > 0 {
				continue
			}
			if e = root.Remove(name); e != nil {
				return e
			}
			removed++
		}
		if removed > 0 {
			return s.syncObjectDir()
		}
		return nil
	})
	return
}
func getObjectRef(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (v contract.ObjectRef, e error) {
	v = contract.ObjectRef{SchemaVersion: 1, Extensions: map[string]any{}}
	e = q.QueryRowContext(ctx, "SELECT id,relative_path,sha256,media_type,byte_size,data_class,created_at FROM object_ref WHERE id=?", id).Scan(&v.ID, &v.RelativePath, &v.SHA256, &v.MediaType, &v.ByteSize, &v.DataClass, &v.CreatedAt)
	return
}
func (s *Store) GetObjectRef(ctx context.Context, id string) (contract.ObjectRef, error) {
	return getObjectRef(ctx, s.DB, id)
}
func (s *Store) readObjectRef(v contract.ObjectRef) ([]byte, error) {
	if filepath.IsAbs(v.RelativePath) || strings.Contains(v.RelativePath, "..") {
		return nil, errors.New("UNSAFE_OBJECT_PATH")
	}
	root, e := os.OpenRoot(s.ObjectsDir)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	stat, e := root.Lstat(v.RelativePath)
	if e != nil {
		return nil, e
	}
	if !stat.Mode().IsRegular() {
		return nil, errors.New("STORAGE_CORRUPTION: nonregular committed object")
	}
	b, e := root.ReadFile(v.RelativePath)
	if e != nil {
		return nil, e
	}
	if len(b) != v.ByteSize || contract.Hash(b) != v.SHA256 {
		return nil, errors.New("STORAGE_CORRUPTION: object hash")
	}
	return b, nil
}
func (s *Store) ReadObject(ctx context.Context, id string) ([]byte, error) {
	v, e := s.GetObjectRef(ctx, id)
	if e != nil {
		return nil, e
	}
	return s.readObjectRef(v)
}
