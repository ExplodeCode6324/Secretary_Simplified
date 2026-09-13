package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
	"time"
)

func (s *Store) BeginWork(ctx context.Context, run contract.JobRun) (*contract.ExecutorReceipt, error) {
	var out *contract.ExecutorReceipt
	e := s.Write(ctx, func(tx *sql.Tx) error {
		var fence, attempt int
		if e := tx.QueryRowContext(ctx, "SELECT fencing_token,attempt_no FROM job_run WHERE id=? AND state='RUNNING'", run.ID).Scan(&fence, &attempt); e != nil {
			return e
		}
		if fence != run.FencingToken || attempt != run.AttemptNo {
			return errors.New("STALE_FENCE")
		}
		hash, hashErr := contract.ValueHash(run.Command)
		if hashErr != nil {
			return hashErr
		}
		var raw []byte
		err := tx.QueryRowContext(ctx, "SELECT payload_json FROM core_work WHERE run_id=?", run.ID).Scan(&raw)
		if err == nil {
			var old contract.CoreWork
			if err = contract.Decode("CoreWork", raw, &old); err != nil {
				return err
			}
			if old.CommandHash != hash {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			if old.Receipt != nil && old.Receipt.Status == "FAILED" && !old.Receipt.EffectObserved && run.AttemptNo > old.AttemptNo {
				old.AttemptNo = run.AttemptNo
				old.FencingToken = run.FencingToken
				old.State = "RUNNING"
				old.Receipt = nil
				old.UpdatedAt = contract.Now()
				raw, _ = json.Marshal(old)
				_, err = tx.ExecContext(ctx, "UPDATE core_work SET attempt_no=?,fencing_token=?,state=?,payload_json=?,updated_at=? WHERE run_id=?", old.AttemptNo, old.FencingToken, old.State, string(raw), old.UpdatedAt, run.ID)
				return err
			}
			if old.Receipt != nil {
				out = old.Receipt
				return nil
			}
			if old.FencingToken != run.FencingToken {
				return errors.New("RESULT_UNKNOWN")
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		v := contract.CoreWork{SchemaVersion: 1, RunID: run.ID, AttemptNo: run.AttemptNo, FencingToken: run.FencingToken, CommandHash: hash, State: "RUNNING", UpdatedAt: contract.Now(), Extensions: map[string]any{}}
		raw, _ = json.Marshal(v)
		_, err = tx.ExecContext(ctx, "INSERT INTO core_work VALUES(?,?,?,?,?,?,?)", v.RunID, v.AttemptNo, v.FencingToken, v.CommandHash, v.State, string(raw), v.UpdatedAt)
		return err
	})
	return out, e
}
func (s *Store) FinishWork(ctx context.Context, run contract.JobRun, r contract.ExecutorReceipt) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return FinishWorkTx(ctx, tx, run, r) })
}
func FinishWorkTx(ctx context.Context, tx *sql.Tx, run contract.JobRun, r contract.ExecutorReceipt) error {
	var currentFence, currentAttempt int
	if e := tx.QueryRowContext(ctx, "SELECT fencing_token,attempt_no FROM job_run WHERE id=? AND state='RUNNING'", run.ID).Scan(&currentFence, &currentAttempt); e != nil {
		return e
	}
	if currentFence != run.FencingToken || currentAttempt != run.AttemptNo {
		return errors.New("STALE_FENCE")
	}
	if r.RunID != run.ID || r.AttemptNo != run.AttemptNo || r.FencingToken != run.FencingToken {
		return errors.New("STALE_FENCE")
	}
	var raw []byte
	if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM core_work WHERE run_id=?", run.ID).Scan(&raw); e != nil {
		return e
	}
	var v contract.CoreWork
	if e := contract.Decode("CoreWork", raw, &v); e != nil {
		return e
	}
	if v.FencingToken != run.FencingToken {
		return errors.New("STALE_FENCE")
	}
	v.State = r.Status
	v.Receipt = &r
	v.UpdatedAt = contract.Now()
	if e := contract.Validate("CoreWork", v); e != nil {
		return e
	}
	raw, _ = json.Marshal(v)
	res, e := tx.ExecContext(ctx, "UPDATE core_work SET state=?,payload_json=?,updated_at=? WHERE run_id=? AND fencing_token=?", v.State, string(raw), v.UpdatedAt, run.ID, run.FencingToken)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return errors.New("STALE_FENCE")
	}
	return nil
}

func (s *Store) ValidateIncomingWork(ctx context.Context, run contract.JobRun, p contract.ExecutionPermit) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		var rb, pb []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM job_run WHERE id=?", run.ID).Scan(&rb); e != nil {
			return e
		}
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM execution_permit WHERE id=?", p.ID).Scan(&pb); e != nil {
			return e
		}
		var current contract.JobRun
		var permit contract.ExecutionPermit
		if e := contract.Decode("JobRun", rb, &current); e != nil {
			return e
		}
		if e := contract.Decode("ExecutionPermit", pb, &permit); e != nil {
			return e
		}
		a, _ := json.Marshal(current.Command)
		b, _ := json.Marshal(run.Command)
		if string(a) != string(b) || current.FencingToken != run.FencingToken || current.AttemptNo != run.AttemptNo || current.State != "RUNNING" || current.TaskID != run.TaskID || current.ExternalIdempotencyKey != run.ExternalIdempotencyKey || current.OccurrenceKey != run.OccurrenceKey {
			return errors.New("STALE_FENCE")
		}
		a, _ = json.Marshal(permit)
		b, _ = json.Marshal(p)
		if string(a) != string(b) || p.RunID != run.ID {
			return errors.New("PERMISSION_DENIED")
		}
		expiry, err := time.Parse(time.RFC3339Nano, p.ExpiresAt)
		if err != nil || !expiry.After(time.Now()) {
			return errors.New("PERMIT_EXPIRED")
		}
		g, err := rtRead(ctx, tx, "authorization_grant", p.GrantID)
		if err != nil {
			return err
		}
		if err = grantCheck(g, p.Capability, time.Now()); err != nil {
			return err
		}
		task, err := rtRead(ctx, tx, "task", run.TaskID)
		if err != nil {
			return err
		}
		if rtInt(g["revision"]) != p.GrantRevision || rtInt(task["cancel_generation"]) != p.CancelGeneration {
			return errors.New("STALE_PERMIT")
		}
		return nil
	})
}

func (s *Store) GetCoreWork(ctx context.Context, id string) (contract.CoreWork, error) {
	var v contract.CoreWork
	var b []byte
	e := s.DB.QueryRowContext(ctx, "SELECT payload_json FROM core_work WHERE run_id=?", id).Scan(&b)
	if e == nil {
		e = contract.Decode("CoreWork", b, &v)
	}
	return v, e
}
