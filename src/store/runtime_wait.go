package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
	"time"
)

func waitSatisfied(ctx context.Context, tx *sql.Tx, w runtimeObject) bool {
	var state string
	switch rtStr(w["event_type"]) {
	case "item.updated":
		return tx.QueryRowContext(ctx, `SELECT status FROM item WHERE id=?`, w["entity_id"]).Scan(&state) == nil && state == w["expected_state"]
	case "task.updated":
		return tx.QueryRowContext(ctx, `SELECT state FROM task WHERE id=?`, w["entity_id"]).Scan(&state) == nil && state == w["expected_state"]
	case "source.synced":
		var raw string
		if tx.QueryRowContext(ctx, `SELECT payload_json FROM source_state WHERE id=?`, w["entity_id"]).Scan(&raw) != nil {
			return false
		}
		var source runtimeObject
		_ = json.Unmarshal([]byte(raw), &source)
		return source["status"] == w["expected_state"]
	}
	return false
}
func (s *Store) Wait(ctx context.Context, w contract.WaitSubscription) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return s.WaitTx(ctx, tx, w) })
}
func (s *Store) WaitTx(ctx context.Context, tx *sql.Tx, w contract.WaitSubscription) error {
	if e := contract.Validate("WaitSubscription", w); e != nil {
		return e
	}
	switch w.EventType {
	case "item.updated", "task.updated", "source.synced":
	default:
		return errors.New("UNREGISTERED_WAIT_EVENT")
	}
	m := rtMap(w)
	return func() error {
		t, e := rtRead(ctx, tx, "task", w.TaskID)
		if e != nil {
			return e
		}
		if e = rtInherit(m, t); e != nil {
			return e
		}
		if e = rtInherit(t, t, m); e != nil {
			return e
		}
		switch t["state"] {
		case "SUCCEEDED", "FAILED", "CANCELLED":
			return errors.New("TERMINAL_TASK")
		}
		var generation, seq int
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation),0)+1 FROM wait_subscription WHERE task_id=?`, w.TaskID).Scan(&generation); e != nil {
			return e
		}
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM change_event`).Scan(&seq); e != nil {
			return e
		}
		m["generation"] = generation
		m["cursor_seq"] = seq
		m["state"] = "ARMED"
		t["state"] = "WAITING"
		if waitSatisfied(ctx, tx, m) {
			m["state"] = "WOKEN"
			t["state"] = "PENDING"
		}
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM wait_subscription WHERE task_id=? AND state='ARMED'`, w.TaskID)
		if e != nil {
			return e
		}
		var olds []runtimeObject
		for rows.Next() {
			var raw string
			rows.Scan(&raw)
			var old runtimeObject
			_ = json.Unmarshal([]byte(raw), &old)
			olds = append(olds, old)
		}
		rows.Close()
		for _, old := range olds {
			old["state"] = "CANCELLED"
			if _, e = tx.ExecContext(ctx, `UPDATE wait_subscription SET state='CANCELLED',payload_json=? WHERE id=?`, rtJSON(old), old["id"]); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO wait_subscription(id,task_id,generation,cursor_seq,deadline_at,state,payload_json)VALUES(?,?,?,?,?,?,?)`, m["id"], w.TaskID, generation, seq, m["deadline_at"], m["state"], rtJSON(m))
		if e != nil {
			return e
		}
		return rtSaveTask(ctx, tx, t)
	}()
}
func (s *Store) WakeWaits(ctx context.Context, now time.Time) (int, error) {
	n := 0
	e := s.Write(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM wait_subscription WHERE state='ARMED'`)
		if e != nil {
			return e
		}
		var waits []runtimeObject
		for rows.Next() {
			var raw string
			rows.Scan(&raw)
			var w runtimeObject
			_ = json.Unmarshal([]byte(raw), &w)
			waits = append(waits, w)
		}
		rows.Close()
		var seq int
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM change_event`).Scan(&seq); e != nil {
			return e
		}
		for _, w := range waits {
			t, e := rtRead(ctx, tx, "task", rtStr(w["task_id"]))
			if e != nil {
				return e
			}
			if t["state"] != "WAITING" {
				w["state"] = "CANCELLED"
			} else {
				deadline, _ := time.Parse(time.RFC3339Nano, rtStr(w["deadline_at"]))
				if waitSatisfied(ctx, tx, w) {
					w["state"] = "WOKEN"
				} else if !deadline.After(now) {
					w["state"] = "EXPIRED"
				}
				if w["state"] != "ARMED" {
					t["state"] = "PENDING"
					if e = rtSaveTask(ctx, tx, t); e != nil {
						return e
					}
					n++
				}
			}
			w["cursor_seq"] = seq
			_, e = tx.ExecContext(ctx, `UPDATE wait_subscription SET cursor_seq=?,state=?,payload_json=? WHERE id=? AND generation=?`, seq, w["state"], rtJSON(w), w["id"], w["generation"])
			if e != nil {
				return e
			}
		}
		return nil
	})
	return n, e
}
func (s *Store) Replan(ctx context.Context, taskID string) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return s.ReplanTx(ctx, tx, taskID) })
}
func (s *Store) ReplanTx(ctx context.Context, tx *sql.Tx, taskID string) error {
	return func() error {
		t, e := rtRead(ctx, tx, "task", taskID)
		if e != nil {
			return e
		}
		switch t["state"] {
		case "SUCCEEDED", "FAILED", "CANCELLED":
			return errors.New("TERMINAL_TASK")
		}
		if e = chargeBudgetTx(ctx, tx, rtStr(t["root_id"]), "replans", 1); e != nil {
			return e
		}
		t["state"] = "PENDING"
		return rtSaveTask(ctx, tx, t)
	}()
}
