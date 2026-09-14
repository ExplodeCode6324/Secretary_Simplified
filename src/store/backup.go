package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"time"
)

type BackupManifest struct {
	SchemaVersion  int                  `json:"schema_version"`
	Status         string               `json:"status"`
	CreatedAt      string               `json:"created_at"`
	DatabaseSHA256 string               `json:"database_sha256"`
	Objects        []contract.ObjectRef `json:"objects"`
}

func newEmptyDir(dir string) error {
	e := os.MkdirAll(dir, 0700)
	if e != nil {
		return e
	}
	files, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	if len(files) != 0 {
		return fmt.Errorf("DESTINATION_NOT_EMPTY")
	}
	return nil
}
func syncWrite(path string, b []byte) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
func (s *Store) Backup(ctx context.Context, dest string) (BackupManifest, error) {
	m := BackupManifest{SchemaVersion: 1, Status: "INCOMPLETE", CreatedAt: contract.Now(), Objects: []contract.ObjectRef{}}
	if e := newEmptyDir(dest); e != nil {
		return m, e
	}
	if e := syncWrite(filepath.Join(dest, "INCOMPLETE"), []byte(m.CreatedAt)); e != nil {
		return m, e
	}
	if e := os.Mkdir(filepath.Join(dest, "objects"), 0700); e != nil {
		return m, e
	}
	dbPath := filepath.Join(dest, "secretary.sqlite")
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		return m, e
	}
	e = conn.Raw(func(raw any) error {
		b, e := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		}).NewBackup(dbPath)
		if e != nil {
			return e
		}
		finished := false
		defer func() {
			if !finished {
				b.Finish()
			}
		}()
		for {
			if e = ctx.Err(); e != nil {
				return e
			}
			more, e := b.Step(128)
			if e != nil {
				return e
			}
			if !more {
				break
			}
			time.Sleep(time.Millisecond)
		}
		finished = true
		return b.Finish()
	})
	conn.Close()
	if e != nil {
		return m, e
	}
	snap, e := Open(dbPath, filepath.Join(dest, "objects"))
	if e != nil {
		return m, e
	}
	rows, e := snap.DB.QueryContext(ctx, "SELECT id FROM object_ref ORDER BY id")
	if e != nil {
		snap.Close()
		return m, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			snap.Close()
			return m, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		snap.Close()
		return m, e
	}
	for _, id := range ids {
		ref, e := snap.GetObjectRef(ctx, id)
		if e != nil {
			snap.Close()
			return m, e
		}
		if !filepath.IsLocal(ref.RelativePath) || filepath.Base(ref.RelativePath) != ref.RelativePath {
			snap.Close()
			return m, fmt.Errorf("UNSAFE_OBJECT_PATH")
		}
		b, e := s.ReadObject(ctx, id)
		if e != nil {
			snap.Close()
			return m, e
		}
		if e = syncWrite(filepath.Join(dest, "objects", ref.RelativePath), b); e != nil {
			snap.Close()
			return m, e
		}
		m.Objects = append(m.Objects, ref)
	}
	if _, e = snap.DB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); e != nil {
		snap.Close()
		return m, e
	}
	objectDir, e := os.Open(filepath.Join(dest, "objects"))
	if e != nil {
		snap.Close()
		return m, e
	}
	e = objectDir.Sync()
	objectDir.Close()
	if e != nil {
		snap.Close()
		return m, e
	}
	if e = snap.Close(); e != nil {
		return m, e
	}
	b, e := os.ReadFile(dbPath)
	if e != nil {
		return m, e
	}
	m.DatabaseSHA256 = contract.Hash(b)
	m.Status = "COMPLETE"
	b, _ = json.MarshalIndent(m, "", "  ")
	if e = syncWrite(filepath.Join(dest, "manifest.json"), b); e != nil {
		return m, e
	}
	if e = os.Remove(filepath.Join(dest, "INCOMPLETE")); e != nil {
		return m, e
	}
	dir, e := os.Open(dest)
	if e == nil {
		e = dir.Sync()
		dir.Close()
	}
	if e == nil {
		_, e = VerifyBackup(ctx, dest)
	}
	if e != nil {
		_ = syncWrite(filepath.Join(dest, "INCOMPLETE"), []byte("final verification failed"))
	}
	if e == nil {
		history, _ := json.Marshal(map[string]any{"state": "COMPLETE", "at": m.CreatedAt, "database_sha256": m.DatabaseSHA256})
		path := filepath.Join(filepath.Dir(s.ObjectsDir), "backup.latest.json")
		tmp := path + ".tmp"
		writeErr := os.WriteFile(tmp, history, 0600)
		if writeErr == nil {
			writeErr = os.Rename(tmp, path)
		}
		if writeErr != nil {
			return m, writeErr
		}
	}
	return m, e
}
func VerifyBackup(ctx context.Context, dir string) (BackupManifest, error) {
	var m BackupManifest
	if _, e := os.Stat(filepath.Join(dir, "INCOMPLETE")); e == nil {
		return m, fmt.Errorf("INCOMPLETE_BACKUP")
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		return m, e
	}
	defer root.Close()
	b, e := root.ReadFile("manifest.json")
	if e != nil {
		return m, e
	}
	if _, e = contract.ParseJSON(b); e != nil {
		return m, e
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil {
		return m, e
	}
	for _, key := range []string{"schema_version", "status", "created_at", "database_sha256", "objects"} {
		if _, ok := fields[key]; !ok {
			return m, fmt.Errorf("INVALID_MANIFEST: missing %s", key)
		}
	}
	if len(fields) != 5 {
		return m, fmt.Errorf("INVALID_MANIFEST: unknown field")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&m); e != nil {
		return m, e
	}
	if m.Objects == nil {
		return m, fmt.Errorf("INVALID_MANIFEST: null objects")
	}
	if normalized, parseErr := contract.NormalizeTimestamp(m.CreatedAt); parseErr != nil || normalized != m.CreatedAt {
		return m, fmt.Errorf("INVALID_MANIFEST: created_at")
	}

	if m.SchemaVersion != 1 || m.Status != "COMPLETE" {
		return m, fmt.Errorf("INVALID_MANIFEST")
	}
	b, e = root.ReadFile("secretary.sqlite")
	if e != nil {
		return m, e
	}
	if contract.Hash(b) != m.DatabaseSHA256 {
		return m, fmt.Errorf("BACKUP_DATABASE_HASH_MISMATCH")
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(dir, "secretary.sqlite")}
	db, e := sql.Open("sqlite", u.String()+"?mode=ro&immutable=1")
	if e != nil {
		return m, e
	}
	defer db.Close()
	var integrity string
	if e = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); e != nil || integrity != "ok" {
		return m, fmt.Errorf("BACKUP_INTEGRITY_FAILED: %v", e)
	}
	rows, e := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		return m, e
	}
	bad := rows.Next()
	e = rows.Err()
	rows.Close()
	if e != nil || bad {
		return m, fmt.Errorf("BACKUP_FOREIGN_KEY_FAILED")
	}
	if e = validateMigrations(ctx, db, false); e != nil {
		return m, e
	}

	var count int
	if e = db.QueryRowContext(ctx, "SELECT count(*) FROM object_ref").Scan(&count); e != nil {
		return m, e
	}
	if count != len(m.Objects) {
		return m, fmt.Errorf("BACKUP_OBJECT_COUNT_MISMATCH")
	}
	seen := map[string]bool{}
	for _, ref := range m.Objects {
		if e = contract.Validate("ObjectRef", ref); e != nil {
			return m, e
		}
		if seen[ref.ID] || !filepath.IsLocal(ref.RelativePath) || filepath.Base(ref.RelativePath) != ref.RelativePath {
			return m, fmt.Errorf("INVALID_OBJECT_MANIFEST")
		}
		seen[ref.ID] = true
		var hash, path, media, class, created string
		var size int
		if e = db.QueryRowContext(ctx, "SELECT relative_path,sha256,byte_size,media_type,data_class,created_at FROM object_ref WHERE id=?", ref.ID).Scan(&path, &hash, &size, &media, &class, &created); e != nil || path != ref.RelativePath || hash != ref.SHA256 || size != ref.ByteSize || media != ref.MediaType || class != ref.DataClass || created != ref.CreatedAt || len(ref.Extensions) != 0 {
			return m, fmt.Errorf("BACKUP_OBJECT_REF_MISMATCH")
		}
		b, e = root.ReadFile("objects/" + ref.RelativePath)
		if e != nil {
			return m, e
		}
		if len(b) != ref.ByteSize || contract.Hash(b) != ref.SHA256 {
			return m, fmt.Errorf("BACKUP_OBJECT_HASH_MISMATCH")
		}
	}
	return m, nil
}
func Restore(ctx context.Context, backupDir, target string) (*Store, error) {
	m, e := VerifyBackup(ctx, backupDir)
	if e != nil {
		return nil, e
	}
	if e = newEmptyDir(target); e != nil {
		return nil, e
	}
	if e = syncWrite(filepath.Join(target, "execution_frozen"), []byte("Restored from backup; reconcile historical runs before explicitly enabling execution.\n")); e != nil {
		return nil, e
	}
	if e = os.Mkdir(filepath.Join(target, "state"), 0700); e != nil {
		return nil, e
	}
	if e = os.Mkdir(filepath.Join(target, "objects"), 0700); e != nil {
		return nil, e
	}
	r, e := os.OpenRoot(backupDir)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := r.ReadFile("secretary.sqlite")
	if e != nil {
		return nil, e
	}
	if contract.Hash(b) != m.DatabaseSHA256 {
		return nil, fmt.Errorf("BACKUP_CHANGED_DURING_RESTORE")
	}
	if e = syncWrite(filepath.Join(target, "state", "secretary.sqlite"), b); e != nil {
		return nil, e
	}
	for _, ref := range m.Objects {
		b, e = r.ReadFile("objects/" + ref.RelativePath)
		if e != nil {
			return nil, e
		}
		if contract.Hash(b) != ref.SHA256 {
			return nil, fmt.Errorf("BACKUP_CHANGED_DURING_RESTORE")
		}
		if e = syncWrite(filepath.Join(target, "objects", ref.RelativePath), b); e != nil {
			return nil, e
		}
	}
	return Open(filepath.Join(target, "state", "secretary.sqlite"), filepath.Join(target, "objects"))
}
