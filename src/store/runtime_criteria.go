package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"secretarysimplified/contract"
)

// Runtime criteria consume program-owned durable evidence, never model assertions.
func (s *Store) verifyRuntimeCriterion(ctx context.Context, tx *sql.Tx, task, kind string, expected runtimeObject) bool {
	rows, e := tx.QueryContext(ctx, `SELECT id,attempt_no,fencing_token,state,payload_json FROM job_run WHERE task_id=? ORDER BY rowid DESC`, task)
	if e != nil {
		return false
	}
	var run contract.JobRun
	count := 0
	for rows.Next() {
		var id, state, raw string
		var attempt, fence int
		if rows.Scan(&id, &attempt, &fence, &state, &raw) != nil {
			rows.Close()
			return false
		}
		var candidate contract.JobRun
		if contract.Decode("JobRun", []byte(raw), &candidate) != nil || candidate.ID != id || candidate.TaskID != task || candidate.AttemptNo != attempt || candidate.FencingToken != fence || candidate.State != state {
			rows.Close()
			return false
		}
		if count == 0 {
			run = candidate
		} else {
			switch candidate.State {
			case "QUEUED", "CLAIMED", "RUNNING", "RESULT_UNKNOWN":
				rows.Close()
				return false
			}
		}
		count++
	}
	e = rows.Err()
	rows.Close()
	if e != nil || count < 1 || run.State == "RESULT_UNKNOWN" {
		return false
	}
	switch kind {
	case "alarm_session_recorded":
		args := rtObj(rtMap(run.Command)["arguments"])
		if run.Command.Capability != "alarm.play" || args["device_id"] != expected["device_id"] || rtObj(args["audio_ref"])["id"] != expected["audio_ref"] {
			return false
		}
		rows, e := tx.QueryContext(ctx, `SELECT id,state,payload_json FROM alarm_session WHERE run_id=?`, run.ID)
		if e != nil {
			return false
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var id, state, raw string
			var a contract.AlarmSession
			if rows.Scan(&id, &state, &raw) != nil || contract.Decode("AlarmSession", []byte(raw), &a) != nil {
				return false
			}
			m := rtMap(a)
			if a.ID != id || a.RunID != run.ID || a.State != state || (state != "PLAYING" && state != "STOPPED") || m["device_id"] != expected["device_id"] || rtObj(m["audio_ref"])["id"] != expected["audio_ref"] {
				return false
			}
			count++
		}
		return rows.Err() == nil && count > 0
	case "briefing_artifact_recorded":
		if run.Command.Capability != "briefing.build" || expected["media_type"] != "text/plain" {
			return false
		}
		rows, e := tx.QueryContext(ctx, `SELECT id,attempt_no,payload_hash,payload_json FROM executor_receipt WHERE run_id=? ORDER BY rowid DESC`, run.ID)
		if e != nil {
			return false
		}
		var receipts []contract.ExecutorReceipt
		for rows.Next() {
			var id, hash, raw string
			var attempt int
			var receipt contract.ExecutorReceipt
			if rows.Scan(&id, &attempt, &hash, &raw) != nil || contract.Decode("ExecutorReceipt", []byte(raw), &receipt) != nil || receipt.ID != id || receipt.RunID != run.ID || receipt.AttemptNo != attempt || rtHash(receipt) != hash {
				rows.Close()
				return false
			}
			if receipt.AttemptNo == run.AttemptNo && receipt.FencingToken == run.FencingToken {
				receipts = append(receipts, receipt)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil || len(receipts) == 0 {
			return false
		}
		receipt := receipts[0]
		if receipt.Status != "SUCCEEDED" || !receipt.EffectObserved || len(receipt.Artifacts) == 0 {
			return false
		}
		for _, a := range receipt.Artifacts {
			var path, hash, media, classification, created string
			var size int
			if tx.QueryRowContext(ctx, `SELECT relative_path,sha256,media_type,byte_size,data_class,created_at FROM object_ref WHERE id=?`, a.ID).Scan(&path, &hash, &media, &size, &classification, &created) != nil {
				return false
			}
			if a.RelativePath != path || a.SHA256 != hash || a.MediaType != media || a.ByteSize != size || a.DataClass != classification || a.CreatedAt != created || media != "text/plain" || size < 1 {
				return false
			}
			safe, e := SafeArtifactPath(s.ObjectsDir, path)
			if e != nil {
				return false
			}
			b, e := os.ReadFile(safe)
			if e != nil || len(b) != size || contract.Hash(b) != hash {
				return false
			}
		}
		return true
	case "source_sync_recorded":
		source := rtStr(expected["source_id"])
		if run.Command.Capability != "source.sync" || run.Command.Arguments["source_id"] != source {
			return false
		}
		var id, kind, raw string
		var rev, stale int
		var cursor, last *string
		if tx.QueryRowContext(ctx, `SELECT id,kind,revision,cursor_json,last_success_at,stale_after_seconds,payload_json FROM source_state WHERE id=?`, source).Scan(&id, &kind, &rev, &cursor, &last, &stale, &raw) != nil || cursor == nil {
			return false
		}
		var state contract.SourceState
		var stored any
		if contract.Decode("SourceState", []byte(raw), &state) != nil || json.Unmarshal([]byte(*cursor), &stored) != nil || state.ID != id || state.Kind != kind || state.Revision != rev || state.StaleAfterSeconds != stale || rtHash(state.Cursor) != rtHash(stored) || (last == nil) != (state.LastSuccessAt == nil) || (last != nil && *last != *state.LastSuccessAt) {
			return false
		}
		events, e := tx.QueryContext(ctx, `SELECT payload_json FROM change_event WHERE entity_id=? AND event_type='source.synced' AND json_extract(payload_json,'$.extensions."runtime.source_sync".run_id')=?`, source, run.ID)
		if e != nil {
			return false
		}
		defer events.Close()
		matched := false
		for events.Next() {
			var raw string
			var event contract.ChangeEvent
			if events.Scan(&raw) != nil || contract.Decode("ChangeEvent", []byte(raw), &event) != nil || contract.ValidateEvent(event) != nil {
				return false
			}
			p := rtObj(event.Extensions["runtime.source_sync"])
			if event.Origin != "IngestService" || event.EntityID != source || p["run_id"] != run.ID {
				return false
			}
			if rtInt(p["attempt_no"]) != run.AttemptNo || rtInt(p["fencing_token"]) != run.FencingToken {
				continue
			}
			after := rtObj(event.Change["after"])
			if after["id"] != source || rtInt(after["revision"]) != event.EntityRevision || rtInt(rtObj(after["cursor"])["processed"]) != rtInt(p["records_processed"]) {
				return false
			}
			matched = true
		}
		return events.Err() == nil && matched
	}
	return false
}
