package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"secretarysimplified/contract"
)

// ApplyControlTx composes control effects with Core's decision transaction.
func (s *Store) ApplyControlTx(ctx context.Context, tx *sql.Tx, control contract.Control, rootID, artifactRoot string, outputClasses ...string) error {
	if e := contract.Validate("Control", control); e != nil {
		return e
	}
	if control.TaskID == nil {
		return errors.New("TASK_CONTROL_REQUIRES_TASK")
	}
	id := *control.TaskID
	t, e := rtRead(ctx, tx, "task", id)
	if e != nil {
		return e
	}
	switch t["state"] {
	case "SUCCEEDED", "FAILED", "CANCELLED":
		return errors.New("TERMINAL_TASK")
	}
	if e = rtInherit(t, t); e != nil {
		return e
	}
	for _, class := range outputClasses {
		if e = rtClass(t, class); e != nil {
			return e
		}
	}
	if len(outputClasses) > 0 {
		if e = rtSaveTask(ctx, tx, t); e != nil {
			return e
		}
	}
	payload := control.Payload
	switch control.Kind {
	case "WAIT":
		w := contract.WaitSubscription{SchemaVersion: 1, ID: contract.NewID(), TaskID: id, Generation: 1, EventType: rtStr(payload["event_type"]), EntityID: rtStr(payload["entity_id"]), ExpectedState: rtStr(payload["expected_state"]), DeadlineAt: rtStr(payload["deadline_at"]), State: "ARMED", Extensions: map[string]any{}}
		w.Extensions, e = contract.ClassifyExtensions(w.Extensions, rtStr(rtObj(rtObj(t["extensions"])[contract.ClassificationKey])["data_class"]))
		if e != nil {
			return e
		}
		return s.WaitTx(ctx, tx, w)
	case "REQUEST_COMPLETION":
		if payload["criterion_hash"] != t["criterion_hash"] {
			return errors.New("CRITERION_HASH_MISMATCH")
		}
		_, e := s.VerifyTaskTx(ctx, tx, id, artifactRoot)
		return e
	case "SUBMIT_ARTIFACT":
		var artifact contract.ObjectRef
		if e = json.Unmarshal([]byte(rtJSON(payload["artifact"])), &artifact); e != nil {
			return e
		}
		if e = contract.Validate("ObjectRef", artifact); e != nil {
			return e
		}
		var path, hash, class string
		var size int
		if e = tx.QueryRowContext(ctx, `SELECT relative_path,sha256,byte_size,data_class FROM object_ref WHERE id=?`, artifact.ID).Scan(&path, &hash, &size, &class); e != nil {
			return e
		}
		if artifact.RelativePath != path || artifact.SHA256 != hash || artifact.ByteSize != size || artifact.DataClass != class {
			return errors.New("ARTIFACT_REFERENCE_MISMATCH")
		}
		safe, e := SafeArtifactPath(s.ObjectsDir, path)
		if e != nil {
			return e
		}
		content, e := os.ReadFile(safe)
		if e != nil {
			return e
		}
		if contract.Hash(content) != hash || len(content) != size {
			return errors.New("ARTIFACT_HASH_MISMATCH")
		}
		if e = rtClass(t, class); e != nil {
			return e
		}
		ext := rtObj(t["extensions"])
		existing := rtObj(ext["runtime.artifacts"])
		refs, _ := existing["refs"].([]any)
		for _, ref := range refs {
			if rtObj(ref)["id"] == artifact.ID {
				return nil
			}
		}
		if len(refs) >= 20 {
			return errors.New("ARTIFACT_BUDGET_EXHAUSTED")
		}
		refs = append(refs, rtMap(artifact))
		ext["runtime.artifacts"] = runtimeObject{"refs": refs}
		t["extensions"] = ext
		return rtSaveTask(ctx, tx, t)
	case "REPLAN":
		var command contract.Command
		if e = json.Unmarshal([]byte(rtJSON(payload["command"])), &command); e != nil {
			return e
		}
		for _, class := range outputClasses {
			command.Extensions, e = contract.ClassifyExtensions(command.Extensions, class)
			if e != nil {
				return e
			}
		}
		if e = contract.Validate("Command", command); e != nil {
			return e
		}
		if command.Capability == "memory.refresh" {
			return errors.New("MEMORY_REFRESH_REQUIRES_SLOT_CONTROLLER")
		}
		var active int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM job_run WHERE task_id=? AND state IN('RUNNING','RESULT_UNKNOWN')`, id).Scan(&active); e != nil {
			return e
		}
		if active > 0 {
			return errors.New("UNRESOLVED_EXECUTION")
		}
		var raw string
		if e = tx.QueryRowContext(ctx, `SELECT payload_json FROM job_run WHERE task_id=? ORDER BY rowid DESC LIMIT 1`, id).Scan(&raw); e != nil {
			return e
		}
		var previous runtimeObject
		_ = json.Unmarshal([]byte(raw), &previous)
		if e = rtInherit(t, t, previous, rtMap(command)); e != nil {
			return e
		}
		if e = rtSaveTask(ctx, tx, t); e != nil {
			return e
		}
		if e = s.ReplanTx(ctx, tx, id); e != nil {
			return e
		}
		if e = chargeBudgetTx(ctx, tx, rtStr(t["root_id"]), "actions", 1); e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM job_run WHERE task_id=? AND state IN('QUEUED','CLAIMED')`, id)
		if e != nil {
			return e
		}
		var oldRuns []runtimeObject
		for rows.Next() {
			var raw string
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return e
			}
			var run runtimeObject
			_ = json.Unmarshal([]byte(raw), &run)
			oldRuns = append(oldRuns, run)
		}
		rows.Close()
		for _, run := range oldRuns {
			run["state"] = "CANCELLED"
			if e = rtSaveRun(ctx, tx, run); e != nil {
				return e
			}
		}
		run := rtMap(previous)
		run["id"] = contract.NewID()
		cm := rtMap(command)
		if e = rtInherit(cm, t, previous, cm); e != nil {
			return e
		}
		run["command"] = cm
		if e = rtInherit(run, t, cm); e != nil {
			return e
		}
		run["occurrence_key"] = rtStr(previous["occurrence_key"]) + ":replan:" + rtStr(run["id"])
		run["external_idempotency_key"] = contract.NewID()
		run["attempt_no"] = 0
		run["fencing_token"] = 0
		run["lease_owner"] = nil
		run["lease_until"] = nil
		run["state"] = "QUEUED"
		run["scheduled_for"] = contract.Now()
		run["updated_at"] = contract.Now()
		if e = contract.Validate("JobRun", run); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO job_run(id,task_id,job_id,job_revision,occurrence_key,scheduled_for,state,external_idempotency_key,payload_json,updated_at)VALUES(?,?,?,?,?,?,'QUEUED',?,?,?)`, run["id"], id, run["job_id"], run["job_revision"], run["occurrence_key"], run["scheduled_for"], run["external_idempotency_key"], rtJSON(run), run["updated_at"])
		if e != nil {
			return e
		}
		return rtChange(ctx, tx, rtStr(t["root_id"]), "run", "run.updated", nil, run)
	default:
		return errors.New("READ_MEMORY_REQUIRES_QUERY_CHANNEL")
	}
}
