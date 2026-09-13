package store

import (
	"context"
	"encoding/json"
	"secretarysimplified/contract"
)

func (s *Store) GetManifest(ctx context.Context, id string) (contract.ContextManifest, error) {
	var v contract.ContextManifest
	var b []byte
	e := s.DB.QueryRowContext(ctx, "SELECT payload_json FROM context_manifest WHERE id=?", id).Scan(&b)
	if e == nil {
		e = contract.Decode("ContextManifest", b, &v)
	}
	return v, e
}
func (s *Store) EventPage(ctx context.Context, after, limit int) (map[string]any, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT seq,payload_json FROM change_event WHERE seq>? ORDER BY seq LIMIT ?", after, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []contract.ChangeEvent{}
	last := after
	for rows.Next() {
		var seq int
		var b []byte
		var v contract.ChangeEvent
		if e = rows.Scan(&seq, &b); e != nil {
			return nil, e
		}
		if e = contract.Decode("ChangeEvent", b, &v); e != nil {
			return nil, e
		}
		last = seq
		out = append(out, v)
	}
	return map[string]any{"items": out, "next_after_seq": last}, rows.Err()
}
func (s *Store) ModelDiagnostics(ctx context.Context) any {
	var count int
	s.DB.QueryRowContext(ctx, "SELECT count(*) FROM context_manifest").Scan(&count)
	return map[string]any{"context_count": count}
}

var _ = json.Marshal
