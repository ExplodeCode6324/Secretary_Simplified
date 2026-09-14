package store

import (
	"context"
	"database/sql"
	"errors"
	"secretarysimplified/contract"
	"time"
)

// UpdateJobRequest atomically applies an allowlisted patch and stores its exact response.
func (s *Store) UpdateJobRequest(ctx context.Context, id, request string, expected int, patch map[string]any, outputClasses ...string) (out contract.ScheduledJob, err error) {
	if request == "" || expected < 1 || len(patch) == 0 {
		return out, errors.New("INVALID_CONTROL")
	}
	for k := range patch {
		switch k {
		case "schedule", "enabled", "misfire", "grace_seconds", "overlap", "max_attempts":
		default:
			return out, errors.New("IMMUTABLE_JOB_FIELD")
		}
	}
	err = s.Write(ctx, func(tx *sql.Tx) error {
		hash := rtHash(runtimeObject{"operation": "job_update", "job_id": id, "expected_revision": expected, "patch": patch, "data_classes": outputClasses})
		done, e := controlReceipt(ctx, tx, request, hash)
		if e != nil {
			return e
		}
		if done {
			var raw string
			if e = tx.QueryRowContext(ctx, `SELECT response_json FROM request_receipt WHERE principal_id='master' AND request_id=?`, request).Scan(&raw); e != nil {
				return e
			}
			return contract.Decode("ScheduledJob", []byte(raw), &out)
		}
		m, e := rtRead(ctx, tx, "scheduled_job", id)
		if e != nil {
			return e
		}
		if e = rtInherit(m, m); e != nil {
			return e
		}
		for _, class := range outputClasses {
			if e = rtClass(m, class); e != nil {
				return e
			}
		}
		if rtInt(m["revision"]) != expected {
			return errors.New("REVISION_CONFLICT")
		}
		for k, v := range patch {
			m[k] = v
		}
		m["revision"] = expected + 1
		m["updated_at"] = contract.Now()
		m["next_due_at"] = nil
		var j contract.ScheduledJob
		if e = contract.Decode("ScheduledJob", []byte(rtJSON(m)), &j); e != nil {
			return e
		}
		if j.Enabled && j.Schedule.Kind != "event" {
			next, e := NextOccurrence(j.Schedule, time.Now().Add(-time.Millisecond))
			if e != nil {
				return e
			}
			if next != nil {
				v := rtTime(*next)
				j.NextDueAt = &v
			}
		}
		if e = s.UpdateJobTx(ctx, tx, j, expected); e != nil {
			return e
		}
		raw, e := rtRead(ctx, tx, "scheduled_job", id)
		if e != nil {
			return e
		}
		out, e = rtTyped[contract.ScheduledJob](raw)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO request_receipt(principal_id,request_id,payload_hash,intent_id,state,response_json,created_at)VALUES('master',?,?,?,'COMMITTED',?,?)`, request, hash, j.RootID, rtJSON(out), contract.Now())
		return e
	})
	return
}
