package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"secretarysimplified/contract"
	"strings"
	"time"
)

func has(v any, s string) bool {
	a, _ := v.([]any)
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
func grantCheck(g runtimeObject, cap string, now time.Time) error {
	exp, e := time.Parse(time.RFC3339Nano, rtStr(g["expires_at"]))
	if e != nil || !exp.After(now) || g["revoked"] == true || g["principal_id"] != "master" || !has(g["capability_ids"], cap) {
		return errors.New("AUTHORIZATION_DENIED")
	}
	return nil
}

// ConsumePermitTx rechecks authority and the current fence within the fact transaction.
func ConsumePermitTx(ctx context.Context, tx *sql.Tx, id, hash, entity, predicate, operation string, now time.Time) error {
	p, e := rtRead(ctx, tx, "execution_permit", id)
	if e != nil {
		return e
	}
	if p["capability"] != "world.update" {
		return errors.New("PERMIT_CAPABILITY_MISMATCH")
	}
	if p["consumed_at"] != nil || p["proposal_hash"] != hash {
		return errors.New("PERMIT_BINDING_MISMATCH")
	}
	exp, e := time.Parse(time.RFC3339Nano, rtStr(p["expires_at"]))
	if e != nil || !exp.After(now) {
		return errors.New("PERMIT_EXPIRED")
	}
	r, e := rtRead(ctx, tx, "job_run", rtStr(p["run_id"]))
	if e != nil {
		return e
	}
	if rtObj(r["command"])["capability"] != "world.update" {
		return errors.New("PERMIT_CAPABILITY_MISMATCH")
	}
	t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
	if e != nil {
		return e
	}
	g, e := rtRead(ctx, tx, "authorization_grant", rtStr(p["grant_id"]))
	if e != nil {
		return e
	}
	if e = grantCheck(g, rtStr(p["capability"]), now); e != nil {
		return e
	}
	if rtInt(g["revision"]) != rtInt(p["grant_revision"]) || rtInt(r["fencing_token"]) != rtInt(p["fencing_token"]) || rtInt(t["cancel_generation"]) != rtInt(p["cancel_generation"]) || r["state"] != "RUNNING" {
		return errors.New("STALE_PERMIT")
	}
	scope := rtObj(g["scope"])
	if !has(scope["entity_ids"], entity) || !has(scope["predicates"], predicate) || !has(scope["operations"], operation) {
		return errors.New("SCOPE_DENIED")
	}
	p["consumed_at"] = rtTime(now)
	_, e = tx.ExecContext(ctx, `UPDATE execution_permit SET consumed_at=?,payload_json=? WHERE id=?`, p["consumed_at"], rtJSON(p), id)
	return e
}
func (s *Store) ClaimRun(ctx context.Context, owner string, now time.Time) (contract.JobRun, error) {
	return s.ClaimRunReady(ctx, owner, now, true)
}
func (s *Store) ClaimRunReady(ctx context.Context, owner string, now time.Time, coreReady bool) (contract.JobRun, error) {
	var out contract.JobRun
	empty := false
	e := s.Write(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM job_run WHERE state IN ('CLAIMED','RUNNING') AND lease_until<=?`, rtTime(now))
		if e != nil {
			return e
		}
		var expired []runtimeObject
		for rows.Next() {
			var raw string
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return e
			}
			var r runtimeObject
			_ = json.Unmarshal([]byte(raw), &r)
			expired = append(expired, r)
		}
		rows.Close()
		for _, r := range expired {
			var dispatch string
			e = tx.QueryRowContext(ctx, `SELECT dispatch_state FROM execution_attempt WHERE run_id=? AND attempt_no=?`, r["id"], r["attempt_no"]).Scan(&dispatch)
			if e != nil {
				return e
			}
			if dispatch == "PREPARED" {
				r["state"] = "QUEUED"
			} else {
				r["state"] = "RESULT_UNKNOWN"
				t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
				if e != nil {
					return e
				}
				t["state"] = "NEEDS_ATTENTION"
				if e = rtSaveTask(ctx, tx, t); e != nil {
					return e
				}
			}
			if e = rtSaveRun(ctx, tx, r); e != nil {
				return e
			}
		}
		var raw string
		e = tx.QueryRowContext(ctx, `SELECT payload_json FROM job_run WHERE state='QUEUED' AND scheduled_for<=? AND (? OR json_extract(payload_json,'$.command.capability') IN('notify.local','artifact.write','alarm.play','alarm.stop','alarm.snooze')) ORDER BY scheduled_for,id LIMIT 1`, rtTime(now), coreReady).Scan(&raw)
		if e == sql.ErrNoRows {
			empty = true
			return nil
		}
		if e != nil {
			return e
		}
		var r runtimeObject
		_ = json.Unmarshal([]byte(raw), &r)
		t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
		if e != nil {
			return e
		}
		if t["state"] == "CANCELLED" {
			return errors.New("TASK_CANCELLED")
		}
		r["attempt_no"] = rtInt(r["attempt_no"]) + 1
		r["fencing_token"] = rtInt(r["fencing_token"]) + 1
		r["lease_owner"] = owner
		r["lease_until"] = rtTime(now.Add(30 * time.Second))
		r["state"] = "CLAIMED"
		r["updated_at"] = rtTime(now)
		if e = rtSaveRun(ctx, tx, r); e != nil {
			return e
		}
		a := rtBase()
		for k, v := range (runtimeObject{"run_id": r["id"], "attempt_no": r["attempt_no"], "fencing_token": r["fencing_token"], "dispatch_state": "PREPARED", "prepared_at": rtTime(now), "dispatched_at": nil, "finished_at": nil, "executor_id": owner, "command_hash": rtHash(r["command"]), "last_error": nil}) {
			a[k] = v
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO execution_attempt(run_id,attempt_no,fencing_token,dispatch_state,payload_json)VALUES(?,?,?,'PREPARED',?)`, r["id"], r["attempt_no"], r["fencing_token"], rtJSON(a))
		if e != nil {
			return e
		}
		out, e = rtTyped[contract.JobRun](r)
		return e
	})
	if e == nil && empty {
		return out, sql.ErrNoRows
	}
	return out, e
}
func (s *Store) DispatchRun(ctx context.Context, run contract.JobRun, owner string, now time.Time) (contract.ExecutionPermit, error) {
	var out contract.ExecutionPermit
	in := rtMap(run)
	e := s.Write(ctx, func(tx *sql.Tx) error {
		r, e := rtRead(ctx, tx, "job_run", rtStr(in["id"]))
		if e != nil {
			return e
		}
		if r["state"] != "CLAIMED" || r["lease_owner"] != owner || rtInt(r["fencing_token"]) != rtInt(in["fencing_token"]) {
			return errors.New("STALE_FENCE")
		}
		lease, _ := time.Parse(time.RFC3339Nano, rtStr(r["lease_until"]))
		if !lease.After(now) {
			return errors.New("LEASE_EXPIRED")
		}
		t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
		if e != nil {
			return e
		}
		if t["state"] == "CANCELLED" || t["state"] == "WAITING" {
			return errors.New("TASK_NOT_DISPATCHABLE")
		}
		if d := rtStr(t["deadline_at"]); d != "" {
			deadline, _ := time.Parse(time.RFC3339Nano, d)
			if !deadline.After(now) {
				return errors.New("DEADLINE_EXPIRED")
			}
		}
		if e = chargeBudgetTx(ctx, tx, rtStr(t["root_id"]), "active_ms", 30000); e != nil {
			return e
		}
		grant := rtStr(rtObj(rtObj(r["extensions"])["runtime.authorization"])["grant_id"])
		g, e := rtRead(ctx, tx, "authorization_grant", grant)
		if e != nil {
			return e
		}
		cmd := rtObj(r["command"])
		if e = contract.Validate("Command", cmd); e != nil {
			return e
		}
		dto, decodeErr := rtTyped[contract.Command](cmd)
		if decodeErr != nil {
			return decodeErr
		}
		if e = CheckReadSetTx(ctx, tx, dto.ExpectedRevisions); e != nil {
			return e
		}
		if e = grantCheck(g, rtStr(cmd["capability"]), now); e != nil {
			return e
		}
		args := rtObj(cmd["arguments"])
		scope := rtObj(g["scope"])
		if source := rtStr(args["source_id"]); source != "" && !has(scope["source_ids"], source) {
			return errors.New("SOURCE_SCOPE_DENIED")
		}
		var proposal any
		if cmd["capability"] == "world.update" {
			e = tx.QueryRowContext(ctx, `SELECT proposal_hash FROM world_proposal WHERE id=?`, args["proposal_id"]).Scan(&proposal)
			if e != nil {
				return e
			}
		}
		p := rtBase()
		for k, v := range (runtimeObject{"id": contract.NewID(), "run_id": r["id"], "grant_id": grant, "grant_revision": g["revision"], "fencing_token": r["fencing_token"], "cancel_generation": t["cancel_generation"], "capability": cmd["capability"], "proposal_hash": proposal, "expires_at": rtTime(now.Add(30 * time.Second)), "consumed_at": nil}) {
			p[k] = v
		}
		if e = contract.Validate("ExecutionPermit", p); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO execution_permit(id,run_id,grant_id,grant_revision,fencing_token,cancel_generation,expires_at,proposal_hash,payload_json)VALUES(?,?,?,?,?,?,?,?,?)`, p["id"], r["id"], grant, g["revision"], r["fencing_token"], t["cancel_generation"], p["expires_at"], proposal, rtJSON(p))
		if e != nil {
			return e
		}
		var aRaw string
		if e = tx.QueryRowContext(ctx, `SELECT payload_json FROM execution_attempt WHERE run_id=? AND attempt_no=?`, r["id"], r["attempt_no"]).Scan(&aRaw); e != nil {
			return e
		}
		var a runtimeObject
		_ = json.Unmarshal([]byte(aRaw), &a)
		a["dispatch_state"] = "DISPATCHED"
		a["dispatched_at"] = rtTime(now)
		_, e = tx.ExecContext(ctx, `UPDATE execution_attempt SET dispatch_state='DISPATCHED',payload_json=? WHERE run_id=? AND attempt_no=?`, rtJSON(a), r["id"], r["attempt_no"])
		if e != nil {
			return e
		}
		r["state"] = "RUNNING"
		t["state"] = "RUNNING"
		if e = rtSaveTask(ctx, tx, t); e != nil {
			return e
		}
		if e = rtSaveRun(ctx, tx, r); e != nil {
			return e
		}
		out, e = rtTyped[contract.ExecutionPermit](p)
		return e
	})
	return out, e
}
func (s *Store) Heartbeat(ctx context.Context, id, owner string, fence int, now time.Time) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		r, e := rtRead(ctx, tx, "job_run", id)
		if e != nil {
			return e
		}
		if r["lease_owner"] != owner || rtInt(r["fencing_token"]) != fence || r["state"] != "RUNNING" {
			return errors.New("STALE_FENCE")
		}
		r["lease_until"] = rtTime(now.Add(30 * time.Second))
		return rtSaveRun(ctx, tx, r)
	})
}
func (s *Store) RecordReceipt(ctx context.Context, receipt contract.ExecutorReceipt) error {
	if e := contract.Validate("ExecutorReceipt", receipt); e != nil {
		return e
	}
	p := rtMap(receipt)
	return s.Write(ctx, func(tx *sql.Tx) error {
		durableRun, readErr := rtRead(ctx, tx, "job_run", rtStr(p["run_id"]))
		if readErr != nil {
			return readErr
		}
		durableTask, readErr := rtRead(ctx, tx, "task", rtStr(durableRun["task_id"]))
		if readErr != nil {
			return readErr
		}
		if durableTask["state"] == "CANCELLED" && p["effect_observed"] == true {
			ext := rtObj(p["extensions"])
			ext["runtime.cancellation"] = runtimeObject{"effect_observed": true, "cancel_generation": durableTask["cancel_generation"]}
			p["extensions"] = ext
		}
		var prior string
		e := tx.QueryRowContext(ctx, `SELECT payload_hash FROM executor_receipt WHERE run_id=? AND receipt_key=?`, p["run_id"], p["receipt_key"]).Scan(&prior)
		if e == nil {
			if prior != rtHash(p) {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO executor_receipt(id,run_id,attempt_no,receipt_key,payload_hash,received_at,payload_json)VALUES(?,?,?,?,?,?,?)`, p["id"], p["run_id"], p["attempt_no"], p["receipt_key"], rtHash(p), p["received_at"], rtJSON(p))
		if e != nil {
			return e
		}
		r, e := rtRead(ctx, tx, "job_run", rtStr(p["run_id"]))
		if e != nil {
			return e
		}
		if rtInt(r["fencing_token"]) != rtInt(p["fencing_token"]) || rtInt(r["attempt_no"]) != rtInt(p["attempt_no"]) {
			return nil
		}
		r["state"] = p["status"]
		retry := false
		currentTask, taskErr := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
		if taskErr != nil {
			return taskErr
		}
		if p["status"] == "FAILED" && p["effect_observed"] == false && currentTask["state"] != "CANCELLED" {
			var dispatched string
			if e = tx.QueryRowContext(ctx, `SELECT dispatch_state FROM execution_attempt WHERE run_id=? AND attempt_no=?`, r["id"], r["attempt_no"]).Scan(&dispatched); e != nil {
				return e
			}
			maxAttempts := 3
			if r["job_id"] != nil {
				j, je := rtRead(ctx, tx, "scheduled_job", rtStr(r["job_id"]))
				if je != nil {
					return je
				}
				maxAttempts = rtInt(j["max_attempts"])
			}
			if dispatched == "DISPATCHED" && rtInt(r["attempt_no"]) < maxAttempts {
				retry = true
				r["state"] = "QUEUED"
				delay := time.Second
				if rtInt(r["attempt_no"]) >= 2 {
					delay = 5 * time.Second
				}
				if rtObj(r["command"])["capability"] == "memory.refresh" {
					delay = 5 * time.Minute
					if rtInt(r["attempt_no"]) >= 2 {
						delay = 30 * time.Minute
					}
				}
				at, pe := time.Parse(time.RFC3339Nano, rtStr(p["received_at"]))
				if pe != nil {
					return pe
				}
				r["scheduled_for"] = rtTime(at.Add(delay))
				r["lease_owner"] = nil
				r["lease_until"] = nil
			}
		}
		r["updated_at"] = p["received_at"]
		if e = rtSaveRun(ctx, tx, r); e != nil {
			return e
		}
		var raw string
		if e = tx.QueryRowContext(ctx, `SELECT payload_json FROM execution_attempt WHERE run_id=? AND attempt_no=?`, r["id"], r["attempt_no"]).Scan(&raw); e != nil {
			return e
		}
		var a runtimeObject
		_ = json.Unmarshal([]byte(raw), &a)
		// Settle the dispatch reservation only once with a persisted final receipt.
		if p["status"] != "RUNNING" && a["dispatch_state"] != "FINISHED" {
			started, parseErr := time.Parse(time.RFC3339Nano, rtStr(a["dispatched_at"]))
			finished, endErr := time.Parse(time.RFC3339Nano, rtStr(p["received_at"]))
			if parseErr == nil && endErr == nil {
				used := int(finished.Sub(started) / time.Millisecond)
				if used < 0 {
					used = 0
				}
				if used > 30000 {
					used = 30000
				}
				task, budgetErr := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
				if budgetErr != nil {
					return budgetErr
				}
				var budgetRaw string
				if budgetErr = tx.QueryRowContext(ctx, `SELECT payload_json FROM root_budget WHERE root_id=?`, task["root_id"]).Scan(&budgetRaw); budgetErr != nil {
					return budgetErr
				}
				var budget runtimeObject
				_ = json.Unmarshal([]byte(budgetRaw), &budget)
				remaining := rtInt(budget["active_ms_used"]) - (30000 - used)
				if remaining < 0 {
					return errors.New("BUDGET_RESERVATION_MISMATCH")
				}
				budget["active_ms_used"] = remaining
				budget["revision"] = rtInt(budget["revision"]) + 1
				if _, budgetErr = tx.ExecContext(ctx, `UPDATE root_budget SET revision=?,payload_json=? WHERE root_id=?`, budget["revision"], rtJSON(budget), task["root_id"]); budgetErr != nil {
					return budgetErr
				}
			}
		}
		a["dispatch_state"] = "FINISHED"
		a["finished_at"] = p["received_at"]
		_, e = tx.ExecContext(ctx, `UPDATE execution_attempt SET dispatch_state='FINISHED',payload_json=? WHERE run_id=? AND attempt_no=?`, rtJSON(a), r["id"], r["attempt_no"])
		if e != nil {
			return e
		}
		t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
		if e != nil {
			return e
		}
		switch p["status"] {
		case "SUCCEEDED":
			t["state"] = "VERIFYING"
		case "RESULT_UNKNOWN":
			t["state"] = "NEEDS_ATTENTION"
		case "FAILED":
			t["state"] = "FAILED"
		case "CANCELLED":
			t["state"] = "CANCELLED"
		}
		if retry {
			t["state"] = "PENDING"
		}
		if currentTask["state"] == "CANCELLED" {
			t["state"] = "CANCELLED"
		}
		return rtSaveTask(ctx, tx, t)
	})
}
func (s *Store) CancelTask(ctx context.Context, id string) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return s.CancelTaskTx(ctx, tx, id) })
}
func (s *Store) CancelTaskTx(ctx context.Context, tx *sql.Tx, id string) error {
	return func() error {
		t, e := rtRead(ctx, tx, "task", id)
		if e != nil {
			return e
		}
		if t["state"] == "SUCCEEDED" || t["state"] == "FAILED" || t["state"] == "CANCELLED" {
			return nil
		}
		t["cancel_generation"] = rtInt(t["cancel_generation"]) + 1
		t["state"] = "CANCELLED"
		if e = rtSaveTask(ctx, tx, t); e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM job_run WHERE task_id=? AND state IN('QUEUED','CLAIMED')`, id)
		if e != nil {
			return e
		}
		var runs []runtimeObject
		for rows.Next() {
			var raw string
			rows.Scan(&raw)
			var r runtimeObject
			_ = json.Unmarshal([]byte(raw), &r)
			runs = append(runs, r)
		}
		rows.Close()
		for _, r := range runs {
			r["state"] = "CANCELLED"
			if e = rtSaveRun(ctx, tx, r); e != nil {
				return e
			}
		}
		return rtEvent(ctx, tx, rtStr(t["root_id"]), "task", id, "cancel_requested")
	}()
}

// CheckArtifactScope requires an explicitly granted root and never accepts a sibling prefix.
func (s *Store) CheckArtifactScope(ctx context.Context, permit contract.ExecutionPermit, target string) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		persisted, permitErr := rtRead(ctx, tx, "execution_permit", permit.ID)
		if permitErr != nil {
			return permitErr
		}
		if rtHash(persisted) != rtHash(permit) {
			return errors.New("PERMIT_BINDING_MISMATCH")
		}
		if persisted["consumed_at"] != nil {
			return errors.New("PERMIT_ALREADY_CONSUMED")
		}
		g, e := rtRead(ctx, tx, "authorization_grant", permit.GrantID)
		if e != nil {
			return e
		}
		if e = grantCheck(g, "artifact.write", time.Now()); e != nil {
			return e
		}
		if rtInt(g["revision"]) != permit.GrantRevision {
			return errors.New("STALE_PERMIT")
		}
		exp, parseErr := time.Parse(time.RFC3339Nano, permit.ExpiresAt)
		if parseErr != nil || !exp.After(time.Now()) {
			return errors.New("PERMIT_EXPIRED")
		}
		run, readErr := rtRead(ctx, tx, "job_run", permit.RunID)
		if readErr != nil {
			return readErr
		}
		if rtInt(run["fencing_token"]) != permit.FencingToken || run["state"] != "RUNNING" {
			return errors.New("STALE_FENCE")
		}
		task, readErr := rtRead(ctx, tx, "task", rtStr(run["task_id"]))
		if readErr != nil {
			return readErr
		}
		if rtInt(task["cancel_generation"]) != permit.CancelGeneration || task["state"] == "CANCELLED" {
			return errors.New("STALE_PERMIT")
		}

		scope := rtObj(g["scope"])
		roots, _ := scope["path_roots"].([]any)
		for _, root := range roots {
			base, e := filepath.Abs(rtStr(root))
			if e != nil {
				continue
			}
			rel, e := filepath.Rel(base, target)
			if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				persisted["consumed_at"] = contract.Now()
				_, e = tx.ExecContext(ctx, `UPDATE execution_permit SET consumed_at=?,payload_json=? WHERE id=? AND consumed_at IS NULL`, persisted["consumed_at"], rtJSON(persisted), permit.ID)
				return e
			}
		}
		return errors.New("PATH_SCOPE_DENIED")
	})
}

// ReconcileLocal proves a previously unknown local effect from authoritative state.
// It never redispatches the command merely because a worker disappeared.
func (s *Store) ReconcileLocal(ctx context.Context, id, artifactRoot string) (bool, error) {
	r, e := s.GetRun(ctx, id)
	if e != nil {
		return false, e
	}
	if r.State != "RESULT_UNKNOWN" {
		return false, errors.New("RUN_NOT_UNKNOWN")
	}
	if r.Command.Capability != "notify.local" && r.Command.Capability != "artifact.write" {
		return false, errors.New("CAPABILITY_REQUIRES_EXECUTOR_QUERY")
	}
	ok, e := s.VerifyTask(ctx, r.TaskID, artifactRoot)
	if e != nil || !ok {
		return ok, e
	}
	receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: r.ID, AttemptNo: r.AttemptNo, FencingToken: r.FencingToken, ReceiptKey: "reconciled-local-effect", Status: "SUCCEEDED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, EffectObserved: true, ReceivedAt: contract.Now(), Extensions: map[string]any{}}
	if e = s.RecordReceipt(ctx, receipt); e != nil {
		return false, e
	}
	return s.VerifyTask(ctx, r.TaskID, artifactRoot)
}
func checkLocalDispatchTx(ctx context.Context, tx *sql.Tx, run runtimeObject) error {
	r, e := rtRead(ctx, tx, "job_run", rtStr(run["id"]))
	if e != nil {
		return e
	}
	if rtHash(r["command"]) != rtHash(run["command"]) {
		return errors.New("COMMAND_HASH_MISMATCH")
	}
	if r["state"] != "RUNNING" || rtInt(r["fencing_token"]) != rtInt(run["fencing_token"]) {
		return errors.New("STALE_FENCE")
	}
	var raw string
	e = tx.QueryRowContext(ctx, `SELECT payload_json FROM execution_permit WHERE run_id=? AND fencing_token=? ORDER BY expires_at DESC LIMIT 1`, r["id"], r["fencing_token"]).Scan(&raw)
	if e != nil {
		return e
	}
	var p runtimeObject
	if e = json.Unmarshal([]byte(raw), &p); e != nil {
		return e
	}
	exp, parseErr := time.Parse(time.RFC3339Nano, rtStr(p["expires_at"]))
	if parseErr != nil || !exp.After(time.Now()) {
		return errors.New("PERMIT_EXPIRED")
	}
	if p["capability"] != rtObj(r["command"])["capability"] || rtInt(r["attempt_no"]) != rtInt(run["attempt_no"]) {
		return errors.New("PERMIT_CAPABILITY_OR_ATTEMPT_MISMATCH")
	}
	if p["consumed_at"] != nil {
		return errors.New("PERMIT_ALREADY_CONSUMED")
	}
	g, e := rtRead(ctx, tx, "authorization_grant", rtStr(p["grant_id"]))
	if e != nil {
		return e
	}
	if e = grantCheck(g, rtStr(p["capability"]), time.Now()); e != nil {
		return e
	}
	t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
	if e != nil {
		return e
	}
	if rtInt(g["revision"]) != rtInt(p["grant_revision"]) || rtInt(t["cancel_generation"]) != rtInt(p["cancel_generation"]) {
		return errors.New("STALE_PERMIT")
	}
	p["consumed_at"] = contract.Now()
	_, e = tx.ExecContext(ctx, `UPDATE execution_permit SET consumed_at=?,payload_json=? WHERE id=? AND consumed_at IS NULL`, p["consumed_at"], rtJSON(p), p["id"])
	return e
}
