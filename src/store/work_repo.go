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
				if old.AttemptNo != run.AttemptNo || old.FencingToken != run.FencingToken {
					return errors.New("RESULT_UNKNOWN")
				}
				out = old.Receipt
				return nil
			}
			if old.FencingToken != run.FencingToken {
				return errors.New("RESULT_UNKNOWN")
			}
			return errors.New("WORK_IN_PROGRESS")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		v := contract.CoreWork{SchemaVersion: 1, RunID: run.ID, AttemptNo: run.AttemptNo, FencingToken: run.FencingToken, CommandHash: hash, State: "RUNNING", UpdatedAt: contract.Now(), Extensions: map[string]any{}}
		class, classErr := contract.ReadClassification(run.Extensions)
		if classErr != nil {
			return classErr
		}
		v.Extensions, classErr = contract.ClassifyExtensions(v.Extensions, class)
		if classErr != nil {
			return classErr
		}
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
	current, e := rtRead(ctx, tx, "job_run", run.ID)
	if e != nil {
		return e
	}
	var sqlTask, sqlState string
	var sqlAttempt, sqlFence int
	if e = tx.QueryRowContext(ctx, "SELECT task_id,state,attempt_no,fencing_token FROM job_run WHERE id=?", run.ID).Scan(&sqlTask, &sqlState, &sqlAttempt, &sqlFence); e != nil {
		return e
	}
	if current["task_id"] != sqlTask || current["state"] != sqlState || rtInt(current["attempt_no"]) != sqlAttempt || rtInt(current["fencing_token"]) != sqlFence {
		return errors.New("STALE_FENCE")
	}
	if current["state"] != "RUNNING" && current["state"] != "RESULT_UNKNOWN" {
		return errors.New("STALE_FENCE")
	}
	hash, e := contract.ValueHash(run.Command)
	if e != nil {
		return e
	}
	if rtStr(current["task_id"]) != run.TaskID || rtInt(current["fencing_token"]) != run.FencingToken || rtInt(current["attempt_no"]) != run.AttemptNo || rtHash(current["command"]) != hash || current["external_idempotency_key"] != run.ExternalIdempotencyKey || current["occurrence_key"] != run.OccurrenceKey {
		return errors.New("STALE_FENCE")
	}
	if r.RunID != run.ID || r.AttemptNo != run.AttemptNo || r.FencingToken != run.FencingToken {
		return errors.New("STALE_FENCE")
	}
	if r.Status != "SUCCEEDED" && r.Status != "FAILED" && r.Status != "CANCELLED" && r.Status != "RESULT_UNKNOWN" {
		return errors.New("INVALID_WORK_RECEIPT")
	}
	task, e := rtRead(ctx, tx, "task", run.TaskID)
	if e != nil {
		return e
	}
	var permitRaw string
	if e = tx.QueryRowContext(ctx, "SELECT payload_json FROM execution_permit WHERE run_id=? AND fencing_token=? ORDER BY rowid DESC LIMIT 1", run.ID, run.FencingToken).Scan(&permitRaw); e != nil {
		return e
	}
	var permit contract.ExecutionPermit
	if e = contract.Decode("ExecutionPermit", []byte(permitRaw), &permit); e != nil {
		return e
	}
	if permit.RunID != run.ID || permit.FencingToken != run.FencingToken || permit.Capability != run.Command.Capability {
		return errors.New("STALE_PERMIT")
	}
	generation := rtInt(task["cancel_generation"])
	if generation != permit.CancelGeneration && !(generation > permit.CancelGeneration && task["state"] == "CANCELLED") {
		return errors.New("STALE_PERMIT")
	}
	proof := rtMap(r)
	if e = rtInherit(proof, current, task); e != nil {
		return e
	}
	for _, ref := range r.Artifacts {
		class, e := rtObjectClassTx(ctx, tx, ref.ID, ref.SHA256)
		if e != nil {
			return e
		}
		if class != ref.DataClass {
			return errors.New("ARTIFACT_REFERENCE_MISMATCH")
		}
		if e = rtClass(proof, class); e != nil {
			return e
		}
	}
	if generation != permit.CancelGeneration {
		ext := rtObj(proof["extensions"])
		ext["runtime.cancellation"] = runtimeObject{"effect_observed": r.EffectObserved, "cancel_generation": generation}
		proof["extensions"] = ext
	}
	if r, e = rtTyped[contract.ExecutorReceipt](proof); e != nil {
		return e
	}
	var raw []byte
	if e = tx.QueryRowContext(ctx, "SELECT payload_json FROM core_work WHERE run_id=?", run.ID).Scan(&raw); e != nil {
		return e
	}
	var v contract.CoreWork
	if e = contract.Decode("CoreWork", raw, &v); e != nil {
		return e
	}
	if v.RunID != run.ID || v.AttemptNo != run.AttemptNo || v.FencingToken != run.FencingToken || v.CommandHash != hash {
		return errors.New("STALE_FENCE")
	}
	if v.Receipt != nil && v.State != "RESULT_UNKNOWN" {
		if rtHash(v.Receipt) == rtHash(r) {
			return nil
		}
		return errors.New("WORK_ALREADY_SETTLED")
	}
	v.State = r.Status
	v.Receipt = &r
	v.UpdatedAt = contract.Now()
	class, e := contract.ReadClassification(r.Extensions)
	if e != nil {
		return e
	}
	v.Extensions, e = contract.ClassifyExtensions(v.Extensions, class)
	if e != nil {
		return e
	}
	if e = contract.Validate("CoreWork", v); e != nil {
		return e
	}
	raw, _ = json.Marshal(v)
	res, e := tx.ExecContext(ctx, "UPDATE core_work SET state=?,payload_json=?,updated_at=? WHERE run_id=? AND fencing_token=? AND attempt_no=? AND command_hash=?", v.State, string(raw), v.UpdatedAt, run.ID, run.FencingToken, run.AttemptNo, hash)
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
