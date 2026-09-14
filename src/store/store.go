package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/platform"
	"strings"
	"sync"
)

//go:embed 001_baseline.sql
var baseline string

type Store struct {
	DB         *sql.DB
	ObjectsDir string
	mu         sync.Mutex
}

func connect(path, objects string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(objects, 0700); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	db, e := sql.Open("sqlite", u.String()+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)&_txlock=immediate")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(4)
	s := &Store{DB: db, ObjectsDir: objects}
	if e = db.Ping(); e != nil {
		db.Close()
		return nil, e
	}
	os.Chmod(path, 0600)
	return s, nil
}
func Init(path, objects string) (*Store, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	l, e := platform.AcquireLock(path + ".migration.lock")
	if e != nil {
		return nil, e
	}
	defer l.Close()
	if _, e = os.Stat(path); e == nil {
		return Open(path, objects)
	}
	s, e := connect(path, objects)
	if e != nil {
		return nil, e
	}
	conn, e := s.DB.Conn(context.Background())
	if e != nil {
		s.Close()
		return nil, e
	}
	defer conn.Close()
	ddl := strings.TrimSuffix(strings.TrimSpace(baseline), "COMMIT;")
	if _, e = conn.ExecContext(context.Background(), ddl); e == nil {
		_, e = conn.ExecContext(context.Background(), "INSERT INTO schema_migration(version,checksum,applied_at) VALUES(1,?,?); COMMIT;", contract.Hash([]byte(baseline)), contract.Now())
	}
	if e != nil {
		conn.ExecContext(context.Background(), "ROLLBACK")
		conn.Close()
		s.Close()
		return nil, e
	}
	if e = installAuthority(context.Background(), s, ""); e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}
func Open(path, objects string) (*Store, error) {
	if _, e := os.Stat(path); e != nil {
		return nil, fmt.Errorf("database missing; run init: %w", e)
	}
	s, e := connect(path, objects)
	if e != nil {
		return nil, e
	}
	var migrationExists int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migration'").Scan(&migrationExists); e != nil {
		s.Close()
		return nil, e
	}
	if migrationExists == 0 {
		s.Close()
		return nil, fmt.Errorf("INITIALIZATION_INCOMPLETE: no schema_migration; preserve this file and initialize a new empty data directory")
	}
	if e = validateMigrations(context.Background(), s.DB, true); e != nil {
		s.Close()
		return nil, e
	}

	return s, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit()
}
func AppendEvent(ctx context.Context, tx *sql.Tx, e *contract.ChangeEvent) error {
	var seq int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(seq),0)+1 FROM change_event").Scan(&seq); err != nil {
		return err
	}
	e.Seq = seq
	if err := contract.ValidateEvent(*e); err != nil {
		return err
	}
	b, _ := json.Marshal(e)
	_, err := tx.ExecContext(ctx, "INSERT INTO change_event(seq,id,root_id,entity_type,entity_id,entity_revision,event_type,causation_id,created_at,payload_json) VALUES(?,?,?,?,?,?,?,?,?,?)", e.Seq, e.ID, e.RootID, e.EntityType, e.EntityID, e.EntityRevision, e.EventType, e.CausationID, e.CreatedAt, string(b))
	return err
}
func (s *Store) Events(ctx context.Context, after int) ([]contract.ChangeEvent, error) {
	rows, e := s.DB.QueryContext(ctx, "SELECT payload_json FROM change_event WHERE seq>? ORDER BY seq LIMIT 1000", after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []contract.ChangeEvent{}
	for rows.Next() {
		var b []byte
		var v contract.ChangeEvent
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = contract.Decode("ChangeEvent", b, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
