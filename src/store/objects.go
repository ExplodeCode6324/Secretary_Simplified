package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"strings"
	"sync"
)

var objectMu sync.Mutex

func (s *Store) PutObject(ctx context.Context, b []byte, mediaType, dataClass string) (contract.ObjectRef, error) {
	objectMu.Lock()
	defer objectMu.Unlock()
	sum := sha256.Sum256(append([]byte(dataClass+"\x00"+mediaType+"\x00"), b...))
	sum[6] = (sum[6] & 15) | 80
	sum[8] = (sum[8] & 63) | 128
	id := fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
	if old, e := s.GetObjectRef(ctx, id); e == nil {
		if _, e = s.ReadObject(ctx, id); e != nil {
			return old, e
		}
		return old, nil
	} else if e != sql.ErrNoRows {
		return old, e
	}
	v := contract.ObjectRef{SchemaVersion: 1, ID: id, SHA256: contract.Hash(b), MediaType: mediaType, ByteSize: len(b), DataClass: dataClass, CreatedAt: contract.Now(), Extensions: map[string]any{}}
	v.RelativePath = v.ID + ".blob"
	if e := contract.Validate("ObjectRef", v); e != nil {
		return v, e
	}
	root, e := os.OpenRoot(s.ObjectsDir)
	if e != nil {
		return v, e
	}
	defer root.Close()
	f, e := root.OpenFile(v.RelativePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(e) {
		existing, err := root.ReadFile(v.RelativePath)
		if err != nil {
			return v, err
		}
		if contract.Hash(existing) != v.SHA256 {
			return v, fmt.Errorf("STORAGE_CORRUPTION: orphan object hash")
		}
		f = nil
		e = nil
	} else if e != nil {
		return v, e
	}
	if f != nil {
		_, e = f.Write(b)
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
	}
	dir, e := os.Open(s.ObjectsDir)
	if e != nil {
		return v, e
	}
	e = dir.Sync()
	dir.Close()
	if e != nil {
		return v, e
	}
	_, e = s.DB.ExecContext(ctx, "INSERT INTO object_ref(id,relative_path,sha256,media_type,byte_size,data_class,created_at) VALUES(?,?,?,?,?,?,?)", v.ID, v.RelativePath, v.SHA256, v.MediaType, v.ByteSize, v.DataClass, v.CreatedAt)
	return v, e
}
func (s *Store) GetObjectRef(ctx context.Context, id string) (contract.ObjectRef, error) {
	v := contract.ObjectRef{SchemaVersion: 1, Extensions: map[string]any{}}
	e := s.DB.QueryRowContext(ctx, "SELECT id,relative_path,sha256,media_type,byte_size,data_class,created_at FROM object_ref WHERE id=?", id).Scan(&v.ID, &v.RelativePath, &v.SHA256, &v.MediaType, &v.ByteSize, &v.DataClass, &v.CreatedAt)
	return v, e
}
func (s *Store) ReadObject(ctx context.Context, id string) ([]byte, error) {
	v, e := s.GetObjectRef(ctx, id)
	if e != nil {
		return nil, e
	}
	if filepath.IsAbs(v.RelativePath) || strings.Contains(v.RelativePath, "..") {
		return nil, fmt.Errorf("UNSAFE_OBJECT_PATH")
	}
	r, e := os.OpenRoot(s.ObjectsDir)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := r.ReadFile(v.RelativePath)
	if e != nil {
		return nil, e
	}
	if len(b) != v.ByteSize || contract.Hash(b) != v.SHA256 {
		return nil, fmt.Errorf("STORAGE_CORRUPTION: object hash")
	}
	return b, nil
}
