package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
	"time"
)

func runtimeList[T any](ctx context.Context, s *Store, table string) ([]T, error) {
	rows, e := s.DB.QueryContext(ctx, "SELECT payload_json FROM "+table)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var v T
		if e = json.Unmarshal([]byte(raw), &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) RuntimeTasks(ctx context.Context) ([]contract.Task, error) {
	return runtimeList[contract.Task](ctx, s, "task")
}
func (s *Store) RuntimeJobs(ctx context.Context) ([]contract.ScheduledJob, error) {
	return runtimeList[contract.ScheduledJob](ctx, s, "scheduled_job")
}
func (s *Store) RuntimeAlarms(ctx context.Context) ([]contract.AlarmSession, error) {
	return runtimeList[contract.AlarmSession](ctx, s, "alarm_session")
}
func (s *Store) TriggerJob(ctx context.Context, id, request string, now time.Time, expectedRevision int) (contract.JobRun, error) {
	var out contract.JobRun
	if request == "" || expectedRevision < 1 {
		return out, errors.New("REQUEST_ID_REQUIRED")
	}
	e := s.Write(ctx, func(tx *sql.Tx) error {
		hash := rtHash(runtimeObject{"operation": "job_trigger", "job_id": id, "expected_revision": expectedRevision})
		done, receiptErr := controlReceipt(ctx, tx, request, hash)
		if receiptErr != nil {
			return receiptErr
		}
		if done {
			var raw string
			if receiptErr = tx.QueryRowContext(ctx, `SELECT payload_json FROM job_run WHERE occurrence_key=?`, id+":"+request).Scan(&raw); receiptErr != nil {
				return receiptErr
			}
			return contract.Decode("JobRun", []byte(raw), &out)
		}
		j, e := rtRead(ctx, tx, "scheduled_job", id)
		if e != nil {
			return e
		}
		if rtInt(j["revision"]) != expectedRevision {
			return errors.New("REVISION_CONFLICT")
		}
		key := id + ":" + request
		var raw string
		e = tx.QueryRowContext(ctx, `SELECT payload_json FROM job_run WHERE occurrence_key=?`, key).Scan(&raw)
		if e == nil {
			return json.Unmarshal([]byte(raw), &out)
		}
		if e != sql.ErrNoRows {
			return e
		}
		grant := rtStr(rtObj(rtObj(j["extensions"])["runtime.authorization"])["grant_id"])
		g, e := rtRead(ctx, tx, "authorization_grant", grant)
		if e != nil {
			return e
		}
		if e = grantCheck(g, rtStr(rtObj(j["command"])["capability"]), now); e != nil {
			return e
		}
		var active int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM job_run WHERE job_id=? AND state IN ('QUEUED','CLAIMED','RUNNING','RESULT_UNKNOWN')`, id).Scan(&active); e != nil {
			return e
		}
		state := "QUEUED"
		if active > 0 {
			state = "SKIPPED"
		}
		run, e := createRunTx(ctx, tx, contract.NewID(), j, key, now, rtObj(j["command"]), rtObj(j["task_template"]), grant, state)
		if e != nil {
			return e
		}
		out, e = rtTyped[contract.JobRun](run)
		if e != nil {
			return e
		}
		task, e := rtRead(ctx, tx, "task", rtStr(run["task_id"]))
		if e != nil {
			return e
		}
		return saveControlReceipt(ctx, tx, request, hash, rtStr(task["root_id"]))
	})
	return out, e
}

// AlarmControl bypasses the work queue so stop remains available during Core outage.
func (s *Store) AlarmControl(ctx context.Context, id, request string, delay int, now time.Time) error {
	if request == "" || delay < 0 {
		return errors.New("INVALID_CONTROL")
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		m, e := rtRead(ctx, tx, "alarm_session", id)
		if e != nil {
			return e
		}
		root := id
		hash := rtHash(runtimeObject{"id": id, "delay": delay})
		var old string
		e = tx.QueryRowContext(ctx, `SELECT payload_hash FROM request_receipt WHERE principal_id='master' AND request_id=?`, request).Scan(&old)
		if e == nil {
			if old != hash {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		original, e := rtRead(ctx, tx, "job_run", rtStr(m["run_id"]))
		if e != nil {
			return e
		}
		task, e := rtRead(ctx, tx, "task", rtStr(original["task_id"]))
		if e != nil {
			return e
		}
		root = rtStr(task["root_id"])
		m["state"] = "STOPPED"
		m["revision"] = rtInt(m["revision"]) + 1
		m["stopped_at"] = rtTime(now)
		m["stop_reason"] = "MASTER_CONTROL"
		_, e = tx.ExecContext(ctx, `UPDATE alarm_session SET state='STOPPED',revision=?,payload_json=? WHERE id=?`, m["revision"], rtJSON(m), id)
		if e != nil {
			return e
		}
		if delay > 0 {
			due := now.Add(time.Duration(delay) * time.Second)
			job := rtBase()
			for k, v := range (runtimeObject{"id": contract.NewID(), "revision": 1, "root_id": root, "enabled": true, "schedule": runtimeObject{"kind": "once", "at": rtTime(due), "anchor_at": nil, "every_seconds": nil, "local_time": nil, "timezone": "UTC", "weekdays": []any{}, "event_type": nil, "filter": nil, "after_seq": nil}, "command": original["command"], "task_template": runtimeObject{"goal": task["goal"], "criteria": task["criteria"], "item_id": task["item_id"], "item_operation_key": nil}, "next_due_at": rtTime(due), "misfire": "FIRE_ONCE_WITHIN_GRACE", "grace_seconds": 300, "overlap": "SKIP", "max_attempts": 3, "updated_at": rtTime(now)}) {
				job[k] = v
			}
			dto, e := rtTyped[contract.ScheduledJob](job)
			if e != nil {
				return e
			}
			grant := rtStr(rtObj(rtObj(original["extensions"])["runtime.authorization"])["grant_id"])
			if e = s.RegisterJobTx(ctx, tx, dto, grant); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO request_receipt(principal_id,request_id,payload_hash,intent_id,state,response_json,created_at)VALUES('master',?,?,?,'COMMITTED',?,?)`, request, hash, root, rtJSON(runtimeObject{"alarm_session_id": id, "state": "STOPPED", "delay_seconds": delay}), rtTime(now))
		return e
	})
}
func (s *Store) RuntimeTask(ctx context.Context, id string) (contract.Task, error) {
	var raw string
	var t contract.Task
	e := s.DB.QueryRowContext(ctx, `SELECT payload_json FROM task WHERE id=?`, id).Scan(&raw)
	if e == nil {
		e = contract.Decode("Task", []byte(raw), &t)
	}
	return t, e
}
func (s *Store) RuntimeJob(ctx context.Context, id string) (contract.ScheduledJob, error) {
	var raw string
	var t contract.ScheduledJob
	e := s.DB.QueryRowContext(ctx, `SELECT payload_json FROM scheduled_job WHERE id=?`, id).Scan(&raw)
	if e == nil {
		e = contract.Decode("ScheduledJob", []byte(raw), &t)
	}
	return t, e
}
func controlReceipt(ctx context.Context, tx *sql.Tx, request, hash string) (bool, error) {
	var old string
	e := tx.QueryRowContext(ctx, `SELECT payload_hash FROM request_receipt WHERE principal_id='master' AND request_id=?`, request).Scan(&old)
	if e == nil {
		if old != hash {
			return true, errors.New("IDEMPOTENCY_CONFLICT")
		}
		return true, nil
	}
	if e != sql.ErrNoRows {
		return false, e
	}
	return false, nil
}
func saveControlReceipt(ctx context.Context, tx *sql.Tx, request, hash, root string) error {
	_, e := tx.ExecContext(ctx, `INSERT INTO request_receipt(principal_id,request_id,payload_hash,intent_id,state,response_json,created_at)VALUES('master',?,?,?,'COMMITTED','{}',?)`, request, hash, root, contract.Now())
	return e
}
func (s *Store) CancelTaskRequest(ctx context.Context, id, request string, revision int) error {
	if request == "" || revision < 1 {
		return errors.New("REQUEST_ID_AND_REVISION_REQUIRED")
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		hash := rtHash(runtimeObject{"operation": "cancel", "task_id": id, "expected_revision": revision})
		done, e := controlReceipt(ctx, tx, request, hash)
		if e != nil || done {
			return e
		}
		t, e := rtRead(ctx, tx, "task", id)
		if e != nil {
			return e
		}
		if rtInt(t["revision"]) != revision {
			return errors.New("REVISION_CONFLICT")
		}
		if e = s.CancelTaskTx(ctx, tx, id); e != nil {
			return e
		}
		return saveControlReceipt(ctx, tx, request, hash, rtStr(t["root_id"]))
	})
}
func (s *Store) SetJobEnabled(ctx context.Context, id, request string, revision int, enabled bool) error {
	if request == "" || revision < 1 {
		return errors.New("REQUEST_ID_AND_REVISION_REQUIRED")
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		hash := rtHash(runtimeObject{"operation": "job_enabled", "job_id": id, "expected_revision": revision, "enabled": enabled})
		done, e := controlReceipt(ctx, tx, request, hash)
		if e != nil || done {
			return e
		}
		j, e := rtRead(ctx, tx, "scheduled_job", id)
		if e != nil {
			return e
		}
		if rtInt(j["revision"]) != revision {
			return errors.New("REVISION_CONFLICT")
		}
		old := rtMap(j)
		j["revision"] = revision + 1
		j["enabled"] = enabled
		j["updated_at"] = contract.Now()
		if enabled {
			schedule, e := rtTyped[contract.Schedule](rtObj(j["schedule"]))
			if e != nil {
				return e
			}
			next, e := NextOccurrence(schedule, time.Now())
			if e != nil {
				return e
			}
			if next == nil {
				j["next_due_at"] = nil
			} else {
				j["next_due_at"] = rtTime(*next)
			}
		}
		_, e = tx.ExecContext(ctx, `UPDATE scheduled_job SET revision=?,enabled=?,next_due_at=?,payload_json=?,updated_at=? WHERE id=? AND revision=?`, j["revision"], enabled, j["next_due_at"], rtJSON(j), j["updated_at"], id, revision)
		if e != nil {
			return e
		}
		rows, scanErr := tx.QueryContext(ctx, `SELECT DISTINCT task_id FROM job_run WHERE job_id=? AND state IN ('QUEUED','CLAIMED')`, id)
		if scanErr != nil {
			return scanErr
		}
		var cancelled []string
		for rows.Next() {
			var taskID string
			if scanErr = rows.Scan(&taskID); scanErr != nil {
				rows.Close()
				return scanErr
			}
			cancelled = append(cancelled, taskID)
		}
		rows.Close()
		for _, taskID := range cancelled {
			if scanErr = s.CancelTaskTx(ctx, tx, taskID); scanErr != nil {
				return scanErr
			}
		}
		if e = rtChange(ctx, tx, rtStr(j["root_id"]), "scheduled_job", "scheduled_job.updated", old, j); e != nil {
			return e
		}
		return saveControlReceipt(ctx, tx, request, hash, rtStr(j["root_id"]))
	})
}

type RuntimePage struct {
	Items       []json.RawMessage `json:"items"`
	NextCursor  *string           `json:"next_cursor"`
	SnapshotSeq int               `json:"snapshot_seq"`
}
type runtimeCursor struct {
	Table   string `json:"table"`
	Limit   int    `json:"limit"`
	Last    int    `json:"last"`
	Through int    `json:"through"`
	Seq     int    `json:"seq"`
}

func (s *Store) RuntimePage(ctx context.Context, table, cursor string, limit int) (RuntimePage, error) {
	out := RuntimePage{Items: []json.RawMessage{}}
	switch table {
	case "task", "scheduled_job", "alarm_session", "notification":
	default:
		return out, errors.New("UNKNOWN_RESOURCE")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return out, errors.New("INVALID_LIMIT")
	}
	c := runtimeCursor{Table: table, Limit: limit}
	e := s.Write(ctx, func(tx *sql.Tx) error {
		if cursor != "" {
			b, e := base64.RawURLEncoding.DecodeString(cursor)
			if e != nil {
				return errors.New("INVALID_CURSOR")
			}
			if e = json.Unmarshal(b, &c); e != nil || c.Table != table || c.Limit != limit || c.Last < 0 || c.Through < c.Last {
				return errors.New("CURSOR_QUERY_MISMATCH")
			}
		} else {
			if e := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(rowid),0) FROM "+table).Scan(&c.Through); e != nil {
				return e
			}
			if e := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM change_event`).Scan(&c.Seq); e != nil {
				return e
			}
		}
		out.SnapshotSeq = c.Seq
		rows, e := tx.QueryContext(ctx, "SELECT rowid,payload_json FROM "+table+" WHERE rowid>? AND rowid<=? ORDER BY rowid LIMIT ?", c.Last, c.Through, limit+1)
		if e != nil {
			return e
		}
		type entry struct {
			id  int
			raw string
		}
		var entries []entry
		for rows.Next() {
			var v entry
			if e = rows.Scan(&v.id, &v.raw); e != nil {
				rows.Close()
				return e
			}
			entries = append(entries, v)
		}
		rows.Close()
		for i, v := range entries {
			if i == limit {
				b, _ := json.Marshal(c)
				next := base64.RawURLEncoding.EncodeToString(b)
				out.NextCursor = &next
				break
			}
			c.Last = v.id
			if table == "notification" {
				var m runtimeObject
				_ = json.Unmarshal([]byte(v.raw), &m)
				if m["state"] == "PENDING" {
					m["state"] = "DELIVERED"
					m["delivered_at"] = contract.Now()
					v.raw = rtJSON(m)
					if _, e = tx.ExecContext(ctx, `UPDATE notification SET state='DELIVERED',payload_json=? WHERE id=?`, v.raw, m["id"]); e != nil {
						return e
					}
				}
			}
			out.Items = append(out.Items, json.RawMessage(v.raw))
		}
		return nil
	})
	return out, e
}
func (s *Store) AckNotification(ctx context.Context, id, request string) error {
	if request == "" {
		return errors.New("REQUEST_ID_REQUIRED")
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		hash := rtHash(runtimeObject{"operation": "notification_ack", "id": id})
		done, e := controlReceipt(ctx, tx, request, hash)
		if e != nil || done {
			return e
		}
		m, e := rtRead(ctx, tx, "notification", id)
		if e != nil {
			return e
		}
		if m["state"] != "ACKNOWLEDGED" {
			m["state"] = "ACKNOWLEDGED"
			m["acknowledged_at"] = contract.Now()
			if m["delivered_at"] == nil {
				m["delivered_at"] = contract.Now()
			}
			if _, e = tx.ExecContext(ctx, `UPDATE notification SET state='ACKNOWLEDGED',payload_json=? WHERE id=?`, rtJSON(m), id); e != nil {
				return e
			}
		}
		return saveControlReceipt(ctx, tx, request, hash, id)
	})
}
func (s *Store) RuntimeUnknownRuns(ctx context.Context) ([]contract.JobRun, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT payload_json FROM job_run WHERE state='RESULT_UNKNOWN' ORDER BY updated_at,id LIMIT 10`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []contract.JobRun{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var run contract.JobRun
		if e = contract.Decode("JobRun", []byte(raw), &run); e != nil {
			return nil, e
		}
		out = append(out, run)
	}
	return out, rows.Err()
}
