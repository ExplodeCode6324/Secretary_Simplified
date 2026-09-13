package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"secretarysimplified/contract"
	"sort"
	"time"
)

type RevisionConflict struct {
	CurrentRevision int `json:"current_revision"`
}

func (e *RevisionConflict) Error() string {
	return fmt.Sprintf("REVISION_CONFLICT: current=%d", e.CurrentRevision)
}
func (s *Store) PutItem(ctx context.Context, v contract.Item, expectedRevision int) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return PutItemTx(ctx, tx, v, expectedRevision) })
}
func PutItemTx(ctx context.Context, tx *sql.Tx, v contract.Item, expectedRevision int) error {
	if v.Revision != expectedRevision+1 {
		var current int
		tx.QueryRowContext(ctx, "SELECT revision FROM item WHERE id=?", v.ID).Scan(&current)
		return &RevisionConflict{CurrentRevision: current}
	}
	if e := contract.Validate("Item", v); e != nil {
		return e
	}
	if _, e := contract.NormalizeTimestamp(v.CreatedAt); e != nil {
		return e
	}
	v.CreatedAt, _ = contract.NormalizeTimestamp(v.CreatedAt)
	var timestampErr error
	v.UpdatedAt, timestampErr = contract.NormalizeTimestamp(v.UpdatedAt)
	if timestampErr != nil {
		return fmt.Errorf("INVALID_UPDATED_AT: %w", timestampErr)
	}
	if v.TimeState != "CONFIRMED" && v.DueAt != nil {
		return fmt.Errorf("INVALID_TIME_STATE")
	}
	if v.DueAt != nil {
		t, e := contract.NormalizeTimestamp(*v.DueAt)
		if e != nil {
			return e
		}
		v.DueAt = &t
	}

	if _, e := time.LoadLocation(v.Timezone); e != nil {
		return fmt.Errorf("INVALID_TIMEZONE: %w", e)
	}
	if v.ParentID != nil {
		if *v.ParentID == v.ID {
			return fmt.Errorf("INVALID_PARENT")
		}
		var n int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM item WHERE id=?", *v.ParentID).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return fmt.Errorf("REFERENCE_NOT_FOUND: parent")
		}
	}
	for _, ev := range v.Evidence {
		var h, dc string
		if e := tx.QueryRowContext(ctx, "SELECT sha256,data_class FROM object_ref WHERE id=?", ev.ObjectID).Scan(&h, &dc); e != nil {
			return fmt.Errorf("REFERENCE_NOT_FOUND: evidence: %w", e)
		}
		if h != ev.SHA256 || dc != ev.DataClass {
			return fmt.Errorf("EVIDENCE_MISMATCH")
		}
	}
	var before any
	b, _ := json.Marshal(v)
	if expectedRevision == 0 {
		_, e := tx.ExecContext(ctx, "INSERT INTO item(id,revision,domain,status,due_at,payload_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", v.ID, v.Revision, v.Domain, v.Status, v.DueAt, string(b), v.CreatedAt, v.UpdatedAt)
		if e != nil {
			return e
		}
	} else {
		old, e := getItem(ctx, tx, v.ID)
		if e != nil {
			return e
		}
		if v.CreatedAt != old.CreatedAt {
			return fmt.Errorf("IMMUTABLE_CREATED_AT")
		}
		before = old
		r, e := tx.ExecContext(ctx, "UPDATE item SET revision=?,domain=?,status=?,due_at=?,payload_json=?,updated_at=? WHERE id=? AND revision=?", v.Revision, v.Domain, v.Status, v.DueAt, string(b), v.UpdatedAt, v.ID, expectedRevision)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			var current int
			tx.QueryRowContext(ctx, "SELECT revision FROM item WHERE id=?", v.ID).Scan(&current)
			return &RevisionConflict{CurrentRevision: current}
		}
	}
	if _, e := tx.ExecContext(ctx, "DELETE FROM item_dependency WHERE item_id=?", v.ID); e != nil {
		return e
	}
	for _, dep := range v.DependencyIDs {
		var cycle int
		e := tx.QueryRowContext(ctx, `WITH RECURSIVE reachable(id) AS (SELECT ? UNION SELECT d.depends_on_id FROM item_dependency d JOIN reachable r ON d.item_id=r.id) SELECT COUNT(*) FROM reachable WHERE id=?`, dep, v.ID).Scan(&cycle)
		if e != nil {
			return e
		}
		if cycle > 0 {
			return fmt.Errorf("DEPENDENCY_CYCLE")
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO item_dependency VALUES(?,?)", v.ID, dep); e != nil {
			return e
		}
	}
	kind := "item.updated"
	if expectedRevision == 0 {
		kind = "item.created"
	}
	return AppendEvent(ctx, tx, &contract.ChangeEvent{SchemaVersion: 1, ID: contract.NewID(), RootID: v.ID, EntityType: "Item", EntityID: v.ID, EntityRevision: v.Revision, EventType: kind, Origin: "ItemService", CreatedAt: v.UpdatedAt, Change: map[string]any{"before": before, "after": v, "evidence": v.Evidence}, Extensions: map[string]any{}})
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func getItem(ctx context.Context, q queryRower, id string) (contract.Item, error) {
	var v contract.Item
	var b []byte
	var rid, domain, status, created, updated string
	var rev int
	var due *string
	e := q.QueryRowContext(ctx, "SELECT id,revision,domain,status,due_at,payload_json,created_at,updated_at FROM item WHERE id=?", id).Scan(&rid, &rev, &domain, &status, &due, &b, &created, &updated)
	if e != nil {
		return v, e
	}
	if e = contract.Decode("Item", b, &v); e != nil {
		return v, e
	}
	if v.ID != rid || v.Revision != rev || v.Domain != domain || v.Status != status || v.CreatedAt != created || v.UpdatedAt != updated || (due == nil) != (v.DueAt == nil) || (due != nil && *due != *v.DueAt) {
		return v, fmt.Errorf("STORAGE_CORRUPTION: item")
	}
	rows, e := q.QueryContext(ctx, "SELECT depends_on_id FROM item_dependency WHERE item_id=? ORDER BY depends_on_id", id)
	if e != nil {
		return v, e
	}
	deps := []string{}
	for rows.Next() {
		var d string
		if e = rows.Scan(&d); e != nil {
			rows.Close()
			return v, e
		}
		deps = append(deps, d)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	expected := append([]string{}, v.DependencyIDs...)
	sort.Strings(expected)
	if len(deps) != len(expected) {
		return v, fmt.Errorf("STORAGE_CORRUPTION: item dependencies")
	}
	for i, d := range deps {
		if expected[i] != d {
			return v, fmt.Errorf("STORAGE_CORRUPTION: item dependencies")
		}
	}

	return v, nil
}
func (s *Store) GetItem(ctx context.Context, id string) (contract.Item, error) {
	return getItem(ctx, s.DB, id)
}
func (s *Store) ListItems(ctx context.Context) ([]contract.Item, error) {
	rows, e := s.DB.QueryContext(ctx, "SELECT id FROM item ORDER BY created_at,id")
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []contract.Item{}
	for _, id := range ids {
		v, e := s.GetItem(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
