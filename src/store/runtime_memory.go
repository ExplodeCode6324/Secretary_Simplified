package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
