package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"secretarysimplified/contract"
)

func (s *Store) EnsureSource(ctx context.Context, v contract.SourceState) error {
	if e := contract.Validate("SourceState", v); e != nil {
		return e
	}
	if v.Cursor != nil {
		return fmt.Errorf("initial source cursor must be null")
	}
	b, _ := json.Marshal(v)
	_, e := s.DB.ExecContext(ctx, "INSERT INTO source_state(id,kind,revision,cursor_json,last_success_at,stale_after_seconds,payload_json) VALUES(?,?,?,NULL,?,?,?) ON CONFLICT(id) DO NOTHING", v.ID, v.Kind, v.Revision, v.LastSuccessAt, v.StaleAfterSeconds, string(b))
	return e
}
func (s *Store) SaveSourceRecord(ctx context.Context, v contract.SourceRecord) (bool, error) {
	if e := contract.Validate("SourceRecord", v); e != nil {
		return false, e
	}
	duplicate := false
	e := s.Write(ctx, func(tx *sql.Tx) error {
		var hash string
		e := tx.QueryRowContext(ctx, "SELECT content_hash FROM source_record WHERE source_id=? AND external_id=? AND source_version=?", v.SourceID, v.ExternalID, v.SourceVersion).Scan(&hash)
		if e == nil {
			if hash != v.ContentHash {
				return fmt.Errorf("SOURCE_VERSION_CONFLICT")
			}
			duplicate = true
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		b, _ := json.Marshal(v)
		_, e = tx.ExecContext(ctx, "INSERT INTO source_record(id,source_id,external_id,source_version,content_hash,raw_ref,observed_at,payload_json) VALUES(?,?,?,?,?,?,?,?)", v.ID, v.SourceID, v.ExternalID, v.SourceVersion, v.ContentHash, v.RawRef.ID, v.ObservedAt, string(b))
		return e
	})
	return duplicate, e
}
func (s *Store) SourceSynced(ctx context.Context, id string, count int, provenance ...SourceSyncProvenance) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		extensions := map[string]any{}
		if len(provenance) > 1 {
			return fmt.Errorf("INVALID_SOURCE_PROVENANCE")
		}
		if len(provenance) == 1 {
			p := provenance[0]
			run, e := rtRead(ctx, tx, "job_run", p.RunID)
			if e != nil {
				return e
			}
			if run["state"] != "RUNNING" || rtInt(run["attempt_no"]) != p.AttemptNo || rtInt(run["fencing_token"]) != p.FencingToken || rtObj(run["command"])["capability"] != "source.sync" || rtObj(rtObj(run["command"])["arguments"])["source_id"] != id {
				return fmt.Errorf("INVALID_SOURCE_PROVENANCE")
			}
			extensions["runtime.source_sync"] = map[string]any{"run_id": p.RunID, "attempt_no": p.AttemptNo, "fencing_token": p.FencingToken, "records_processed": count}
		}
		var b []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM source_state WHERE id=?", id).Scan(&b); e != nil {
			return e
		}
		var v contract.SourceState
		if e := contract.Decode("SourceState", b, &v); e != nil {
			return e
		}
		old := v
		v.Revision++
		now := contract.Now()
		v.LastSuccessAt = &now
		v.LastError = nil
		c := map[string]any{"processed": count}
		v.Cursor = &c
		b, _ = json.Marshal(v)
		cb, _ := json.Marshal(c)
		result, e := tx.ExecContext(ctx, "UPDATE source_state SET revision=?,cursor_json=?,last_success_at=?,payload_json=? WHERE id=? AND revision=?", v.Revision, string(cb), now, string(b), id, old.Revision)
		if e != nil {
			return e
		}
		affected, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if affected != 1 {
			return fmt.Errorf("STORAGE_CORRUPTION: source revision mismatch")
		}
		return AppendEvent(ctx, tx, &contract.ChangeEvent{SchemaVersion: 1, ID: contract.NewID(), RootID: id, EntityType: "source", EntityID: id, EntityRevision: v.Revision, EventType: "source.synced", Origin: "IngestService", CreatedAt: now, Change: map[string]any{"before": old, "after": v, "evidence": []any{}}, Extensions: extensions})
	})
}
func (s *Store) GetSource(ctx context.Context, id string) (contract.SourceState, error) {
	var v contract.SourceState
	var b []byte
	var rid, kind string
	var rev, stale int
	var last *string
	var cursor *string
	e := s.DB.QueryRowContext(ctx, "SELECT id,kind,revision,cursor_json,last_success_at,stale_after_seconds,payload_json FROM source_state WHERE id=?", id).Scan(&rid, &kind, &rev, &cursor, &last, &stale, &b)
	if e != nil {
		return v, e
	}
	if e = contract.Decode("SourceState", b, &v); e != nil {
		return v, e
	}
	cb, _ := json.Marshal(v.Cursor)
	var stored any
	if cursor != nil {
		if e = json.Unmarshal([]byte(*cursor), &stored); e != nil {
			return v, e
		}
	}
	sb, _ := json.Marshal(stored)
	if v.ID != rid || v.Kind != kind || v.Revision != rev || v.StaleAfterSeconds != stale || string(cb) != string(sb) || (last == nil) != (v.LastSuccessAt == nil) || (last != nil && *last != *v.LastSuccessAt) {
		return v, fmt.Errorf("STORAGE_CORRUPTION: source")
	}
	return v, nil
}
func (s *Store) GetSourceRecord(ctx context.Context, id string) (contract.SourceRecord, error) {
	var v contract.SourceRecord
	var b []byte
	var rid, source, external, version, hash, raw, observed string
	e := s.DB.QueryRowContext(ctx, "SELECT id,source_id,external_id,source_version,content_hash,raw_ref,observed_at,payload_json FROM source_record WHERE id=?", id).Scan(&rid, &source, &external, &version, &hash, &raw, &observed, &b)
	if e != nil {
		return v, e
	}
	if e = contract.Decode("SourceRecord", b, &v); e != nil {
		return v, e
	}
	if v.ID != rid || v.SourceID != source || v.ExternalID != external || v.SourceVersion != version || v.ContentHash != hash || v.RawRef.ID != raw || v.ObservedAt != observed {
		return v, fmt.Errorf("STORAGE_CORRUPTION: source record")
	}
	return v, nil
}

func (s *Store) LookupSourceVersion(ctx context.Context, source, external, version string) (string, error) {
	var hash string
	e := s.DB.QueryRowContext(ctx, "SELECT content_hash FROM source_record WHERE source_id=? AND external_id=? AND source_version=?", source, external, version).Scan(&hash)
	if e == sql.ErrNoRows {
		return "", nil
	}
	return hash, e
}

// SourceSyncProvenance identifies the already dispatched Core work execution.
type SourceSyncProvenance struct {
	RunID                   string
	AttemptNo, FencingToken int
}
