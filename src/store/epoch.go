package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"time"
)

// PinEpoch persists the immutable deployment epoch as a referenced configuration
// object, so backup/restore carries it without adding a parallel settings table.
func (s *Store) PinEpoch(ctx context.Context, epoch time.Time) error {
	if epoch.IsZero() {
		return errors.New("INVALID_EPOCH")
	}
	canonical := contract.Timestamp(epoch)
	b, _ := json.Marshal(map[string]any{"schema_version": 1, "epoch": canonical})
	id := contract.DeriveID("secretary.deployment.epoch.v1")
	return s.Write(ctx, func(tx *sql.Tx) error {
		var hash, path string
		e := tx.QueryRowContext(ctx, "SELECT sha256,relative_path FROM object_ref WHERE id=?", id).Scan(&hash, &path)
		if e == nil {
			if hash != contract.Hash(b) {
				return errors.New("EPOCH_MISMATCH: explicit migration required")
			}
			root, e := os.OpenRoot(s.ObjectsDir)
			if e != nil {
				return e
			}
			defer root.Close()
			existing, e := root.ReadFile(path)
			if e != nil {
				return e
			}
			if contract.Hash(existing) != hash {
				return errors.New("STORAGE_CORRUPTION: epoch object")
			}
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		path = "system-epoch.json"
		target := filepath.Join(s.ObjectsDir, path)
		if e = syncWrite(target, b); os.IsExist(e) {
			existing, readErr := os.ReadFile(target)
			if readErr != nil {
				return readErr
			}
			if contract.Hash(existing) != contract.Hash(b) {
				return errors.New("EPOCH_MISMATCH: incomplete configuration object")
			}
			e = nil
		}
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO object_ref(id,relative_path,sha256,media_type,byte_size,data_class,created_at) VALUES(?,?,?,?,?,'SYNTHETIC',?)", id, path, contract.Hash(b), "application/vnd.secretary.epoch+json", len(b), contract.Now())
		return e
	})
}
