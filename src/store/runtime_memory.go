package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"secretarysimplified/contract"
	"time"
)

// ScheduleMemorySlot is the sole programmatic admission path for memory.refresh.
// It registers only the current elapsed-time slot and never reopens an exhausted run.
func (s *Store) ScheduleMemorySlot(ctx context.Context, epoch, now time.Time, grantID string) (out contract.JobRun, err error) {
	if err = s.PinEpoch(ctx, epoch); err != nil {
		return out, err
	}
	slot := int(now.Sub(epoch) / (24 * time.Hour))
	if slot < 0 {
		return out, nil
	}
	root := contract.DeriveID(fmt.Sprintf("secretary.memory.slot.v1:%s:%d", contract.Timestamp(epoch), slot))
	intent := contract.DeriveID(root + ":intent")
	request := contract.DeriveID(root + ":request")
	err = s.Write(ctx, func(tx *sql.Tx) error {
		var max sql.NullInt64
		if e := tx.QueryRowContext(ctx, "SELECT MAX(slot) FROM consciousness_snapshot").Scan(&max); e != nil {
			return e
		}
		if max.Valid && max.Int64 >= int64(slot) {
			return nil
		}
		var raw []byte
		e := tx.QueryRowContext(ctx, "SELECT r.payload_json FROM command_ledger c JOIN job_run r ON r.task_id=c.task_id WHERE c.intent_id=? AND c.operation_key='memory_refresh'", intent).Scan(&raw)
		if e == nil {
			return contract.Decode("JobRun", raw, &out)
		}
		if e != sql.ErrNoRows {
			return e
		}
		rows, e := tx.QueryContext(ctx, "SELECT task_id FROM job_run WHERE state='QUEUED' AND json_extract(payload_json,'$.command.capability')='memory.refresh' AND json_extract(payload_json,'$.command.arguments.slot')<?", slot)
		if e != nil {
			return e
		}
		old := []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			old = append(old, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, id := range old {
			if e = s.CancelTaskTx(ctx, tx, id); e != nil {
				return e
			}
		}
		command := contract.Command{SchemaVersion: 1, OperationKey: "memory_refresh", Capability: "memory.refresh", CapabilityVersion: 1, Arguments: map[string]any{"slot": slot}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{}}
		criterion := contract.Criterion{ID: contract.DeriveID(root + ":criterion"), Kind: "consciousness_slot_committed", Expected: map[string]any{"slot": slot}, EvidencePolicy: "exact persisted ConsciousnessState slot, schema validated"}
		command.Extensions, e = contract.ClassifyExtensions(command.Extensions, "SYNTHETIC")
		if e != nil {
			return e
		}
		if e = contract.Validate("Command", command); e != nil {
			return e
		}
		if e = contract.Validate("Criterion", criterion); e != nil {
			return e
		}
		run, e := createRunTx(ctx, tx, root, nil, intent+":memory_refresh", now, rtMap(command), runtimeObject{"goal": fmt.Sprintf("Commit consciousness slot %d", slot), "criteria": []contract.Criterion{criterion}, "item_id": nil}, grantID, "QUEUED")
		if e != nil {
			return e
		}
		payloadHash := rtHash(runtimeObject{"command": command, "criteria": []contract.Criterion{criterion}})
		if _, e = tx.ExecContext(ctx, "INSERT INTO command_ledger(id,intent_id,operation_key,payload_hash,task_id,payload_json,created_at) VALUES(?,?,?,?,?,?,?)", contract.DeriveID(root+":ledger"), intent, "memory_refresh", payloadHash, run["task_id"], rtJSON(command), contract.Timestamp(now)); e != nil {
			return e
		}
		response := rtJSON(runtimeObject{"state": "COMMITTED", "root_id": root, "run_id": run["id"], "slot": slot})
		if _, e = tx.ExecContext(ctx, "INSERT INTO request_receipt(principal_id,request_id,payload_hash,intent_id,state,response_json,created_at) VALUES('slot-controller',?,?,?,'COMMITTED',?,?)", request, payloadHash, intent, response, contract.Timestamp(now)); e != nil {
			return e
		}
		raw, _ = json.Marshal(run)
		return contract.Decode("JobRun", raw, &out)
	})
	return
}

// ReconcileMemoryWork closes a crash gap using only this controller's exact slot.
// GET remains read-only. This writes no model output and never executes work.
func (s *Store) ReconcileMemoryWork(ctx context.Context, run contract.JobRun) (out *contract.ExecutorReceipt, err error) {
	if run.Command.Capability != "memory.refresh" {
		return nil, nil
	}
	err = s.Write(ctx, func(tx *sql.Tx) error {
		var raw []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM core_work WHERE run_id=?", run.ID).Scan(&raw); e != nil {
			if e == sql.ErrNoRows {
				return nil
			}
			return e
		}
		var work contract.CoreWork
		if e := contract.Decode("CoreWork", raw, &work); e != nil {
			return e
		}
		hash, e := contract.ValueHash(run.Command)
		if e != nil {
			return e
		}
		if work.RunID != run.ID || work.AttemptNo != run.AttemptNo || work.FencingToken != run.FencingToken || work.CommandHash != hash {
			return errors.New("STALE_FENCE")
		}
		if work.Receipt != nil && work.State != "RESULT_UNKNOWN" {
			out = work.Receipt
			return nil
		}
		slot := rtInt(run.Command.Arguments["slot"])
		if slot < 0 || fmt.Sprint(run.Command.Arguments["slot"]) != fmt.Sprint(slot) {
			return errors.New("INVALID_SLOT")
		}
		if e = tx.QueryRowContext(ctx, "SELECT payload_json FROM consciousness_snapshot WHERE slot=?", slot).Scan(&raw); e != nil {
			if e == sql.ErrNoRows {
				return nil
			}
			return e
		}
		var snapshot contract.ConsciousnessState
		if e = contract.Decode("ConsciousnessState", raw, &snapshot); e != nil {
			return e
		}
		if snapshot.Slot != slot {
			return errors.New("INVALID_SLOT")
		}
		if _, e = contract.ReadClassification(snapshot.Extensions); e != nil {
			return e
		}
		var path, epochHash string
		if e = tx.QueryRowContext(ctx, "SELECT relative_path,sha256 FROM object_ref WHERE id=?", contract.DeriveID("secretary.deployment.epoch.v1")).Scan(&path, &epochHash); e != nil {
			return e
		}
		root, e := os.OpenRoot(s.ObjectsDir)
		if e != nil {
			return e
		}
		defer root.Close()
		epochBytes, e := root.ReadFile(path)
		if e != nil {
			return e
		}
		if contract.Hash(epochBytes) != epochHash {
			return errors.New("STORAGE_CORRUPTION: epoch object")
		}
		var epoch struct {
			SchemaVersion int    `json:"schema_version"`
			Epoch         string `json:"epoch"`
		}
		if e = json.Unmarshal(epochBytes, &epoch); e != nil {
			return e
		}
		if epoch.SchemaVersion != 1 {
			return errors.New("INVALID_EPOCH")
		}
		expectedRoot := contract.DeriveID(fmt.Sprintf("secretary.memory.slot.v1:%s:%d", epoch.Epoch, slot))
		task, e := rtRead(ctx, tx, "task", run.TaskID)
		if e != nil {
			return e
		}
		if task["root_id"] != expectedRoot {
			return errors.New("MEMORY_REFRESH_REQUIRES_SLOT_CONTROLLER")
		}
		receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.DeriveID(fmt.Sprintf("memory-reconcile:%s:%d:%d", run.ID, run.AttemptNo, run.FencingToken)), RunID: run.ID, AttemptNo: run.AttemptNo, FencingToken: run.FencingToken, ReceiptKey: fmt.Sprintf("exact-slot-committed:%d:%d", slot, run.FencingToken), Status: "SUCCEEDED", EffectObserved: true, Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Now(), Extensions: snapshot.Extensions}
		if e = FinishWorkTx(ctx, tx, run, receipt); e != nil {
			return e
		}
		out = &receipt
		return nil
	})
	return
}
