package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
)

type publicCursor struct {
	Query   string `json:"query"`
	Seq     int    `json:"seq"`
	Created string `json:"created"`
	ID      string `json:"id"`
}

// PublicPage binds filters and the event watermark. A changed snapshot must be
// restarted rather than silently mixing versions from two database states.
func (s *Store) PublicPage(ctx context.Context, kind, a, b, cursor string, limit int) (RuntimePage, error) {
	out := RuntimePage{Items: []json.RawMessage{}}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return out, errors.New("INVALID_LIMIT")
	}
	queryHash, _ := contract.ValueHash([]any{kind, a, b, limit})
	cur := publicCursor{Query: queryHash}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(seq),0) FROM change_event").Scan(&out.SnapshotSeq); e != nil {
		return out, e
	}
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return out, errors.New("INVALID_CURSOR")
		}
		if _, err = contract.ParseJSON(raw); err != nil {
			return out, errors.New("INVALID_CURSOR")
		}
		if err = json.Unmarshal(raw, &cur); err != nil || cur.Query != queryHash {
			return out, errors.New("INVALID_CURSOR")
		}
		if cur.Seq != out.SnapshotSeq {
			return out, errors.New("CONFLICT: cursor snapshot changed")
		}
	} else {
		cur.Seq = out.SnapshotSeq
	}
	var q string
	switch kind {
	case "items":
		q = `SELECT payload_json,created_at,id FROM item WHERE (?='' OR domain=?) AND (?='' OR status=?) AND (created_at>? OR (created_at=? AND id>?)) ORDER BY created_at,id LIMIT ?`
	case "world":
		q = `SELECT v.payload_json,v.created_at,v.fact_id FROM world_fact_version v JOIN world_fact_head h ON h.fact_id=v.fact_id AND h.revision=v.revision WHERE (?='' OR v.entity_id=?) AND (?='' OR v.predicate=?) AND (v.created_at>? OR (v.created_at=? AND v.fact_id>?)) ORDER BY v.created_at,v.fact_id LIMIT ?`
	default:
		return out, errors.New("INVALID_RESOURCE")
	}
	rows, e := tx.QueryContext(ctx, q, a, a, b, b, cur.Created, cur.Created, cur.ID, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var created, id string
		if e = rows.Scan(&raw, &created, &id); e != nil {
			return out, e
		}
		if len(out.Items) == limit {
			raw, _ := json.Marshal(cur)
			next := base64.RawURLEncoding.EncodeToString(raw)
			out.NextCursor = &next
			break
		}
		out.Items = append(out.Items, json.RawMessage(raw))
		cur.Created = created
		cur.ID = id
	}
	return out, rows.Err()
}

// ItemRevision returns the immutable response version for an idempotent mutation.
func (s *Store) ItemRevision(ctx context.Context, id string, revision int) (contract.Item, error) {
	var v contract.Item
	var raw []byte
	e := s.DB.QueryRowContext(ctx, `SELECT json_extract(payload_json,'$.change.after') FROM change_event WHERE entity_type='Item' AND entity_id=? AND entity_revision=? ORDER BY seq LIMIT 1`, id, revision).Scan(&raw)
	if e == nil {
		e = contract.Decode("Item", raw, &v)
	}
	return v, e
}
