package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"time"
)

// SafeArtifactPath rejects traversal and all existing symlink path components.
func SafeArtifactPath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) || relative == "" {
		return "", errors.New("PATH_DENIED")
	}
	clean := filepath.Clean(relative)
	if clean == ".." || len(clean) > 3 && clean[:3] == "../" {
		return "", errors.New("PATH_DENIED")
	}
	base, e := filepath.Abs(root)
	if e != nil {
		return "", e
	}
	target := filepath.Join(base, clean)
	for p := target; ; p = filepath.Dir(p) {
		fi, e := os.Lstat(p)
		if e == nil && fi.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("SYMLINK_DENIED")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		if p == base {
			break
		}
		if filepath.Dir(p) == p {
			return "", errors.New("PATH_DENIED")
		}
	}
	return target, nil
}
func (s *Store) RecordNotification(ctx context.Context, run contract.JobRun) error {
	r := rtMap(run)
	a := rtObj(rtObj(r["command"])["arguments"])
	return s.Write(ctx, func(tx *sql.Tx) error {
		if e := checkLocalDispatchTx(ctx, tx, r); e != nil {
			if e.Error() == "PERMIT_ALREADY_CONSUMED" {
				return nil
			}
			return e
		}

		var raw string
		e := tx.QueryRowContext(ctx, `SELECT payload_json FROM notification WHERE notification_key=?`, a["notification_key"]).Scan(&raw)
		if e == nil {
			var old runtimeObject
			_ = json.Unmarshal([]byte(raw), &old)
			if old["text"] != a["text"] {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			return nil
		}
		if e != sql.ErrNoRows {
			return e
		}
		m := rtBase()
		if e := rtInherit(m, r); e != nil {
			return e
		}
		for k, v := range (runtimeObject{"id": contract.NewID(), "run_id": r["id"], "notification_key": a["notification_key"], "state": "PENDING", "text": a["text"], "created_at": contract.Now(), "delivered_at": nil, "acknowledged_at": nil}) {
			m[k] = v
		}
		if e = contract.Validate("Notification", m); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO notification(id,run_id,notification_key,state,payload_json,created_at)VALUES(?,?,?,'PENDING',?,?)`, m["id"], r["id"], m["notification_key"], rtJSON(m), m["created_at"])
		return e
	})
}
func (s *Store) Notifications(ctx context.Context, ackID string) ([]contract.Notification, error) {
	out := []contract.Notification{}
	e := s.Write(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM notification ORDER BY created_at`)
		if e != nil {
			return e
		}
		var ms []runtimeObject
		for rows.Next() {
			var raw string
			rows.Scan(&raw)
			var m runtimeObject
			_ = json.Unmarshal([]byte(raw), &m)
			ms = append(ms, m)
		}
		rows.Close()
		for _, m := range ms {
			if m["id"] == ackID {
				m["state"] = "ACKNOWLEDGED"
				m["acknowledged_at"] = contract.Now()
			} else if m["state"] == "PENDING" {
				m["state"] = "DELIVERED"
				m["delivered_at"] = contract.Now()
			}
			_, e = tx.ExecContext(ctx, `UPDATE notification SET state=?,payload_json=? WHERE id=?`, m["state"], rtJSON(m), m["id"])
			if e != nil {
				return e
			}
			v, e := rtTyped[contract.Notification](m)
			if e != nil {
				return e
			}
			out = append(out, v)
		}
		return nil
	})
	return out, e
}
func (s *Store) MutedAlarm(ctx context.Context, run contract.JobRun) error {
	r := rtMap(run)
	cmd := rtObj(r["command"])
	a := rtObj(cmd["arguments"])
	return s.Write(ctx, func(tx *sql.Tx) error {
		if e := checkLocalDispatchTx(ctx, tx, r); e != nil {
			if e.Error() == "PERMIT_ALREADY_CONSUMED" {
				return nil
			}
			return e
		}

		if cmd["capability"] == "alarm.play" {
			m := rtBase()
			if e := rtInherit(m, r); e != nil {
				return e
			}
			for k, v := range (runtimeObject{"id": contract.NewID(), "run_id": r["id"], "revision": 1, "state": "PLAYING", "device_id": a["device_id"], "audio_ref": a["audio_ref"], "playback_handle": "muted:" + rtStr(r["id"]), "saved_settings": runtimeObject{"output_volume": 0, "muted": true, "device_id": a["device_id"]}, "started_at": contract.Now(), "stopped_at": nil, "stop_reason": nil}) {
				m[k] = v
			}
			_, e := tx.ExecContext(ctx, `INSERT OR IGNORE INTO alarm_session(id,run_id,revision,state,payload_json)VALUES(?,?,1,'PLAYING',?)`, m["id"], r["id"], rtJSON(m))
			return e
		}
		m, e := rtRead(ctx, tx, "alarm_session", rtStr(a["alarm_session_id"]))
		if e != nil {
			return e
		}
		if e = rtInherit(m, m, r); e != nil {
			return e
		}
		m["state"] = "STOPPED"
		m["revision"] = rtInt(m["revision"]) + 1
		m["stopped_at"] = contract.Now()
		m["stop_reason"] = cmd["capability"]
		_, e = tx.ExecContext(ctx, `UPDATE alarm_session SET state='STOPPED',revision=?,payload_json=? WHERE id=?`, m["revision"], rtJSON(m), m["id"])
		if e != nil {
			return e
		}
		if cmd["capability"] == "alarm.snooze" {
			original, e := rtRead(ctx, tx, "job_run", rtStr(m["run_id"]))
			if e != nil {
				return e
			}
			task, e := rtRead(ctx, tx, "task", rtStr(original["task_id"]))
			if e != nil {
				return e
			}
			due := time.Now().Add(time.Duration(rtInt(a["delay_seconds"])) * time.Second)
			job := rtBase()
			if e := rtInherit(job, original, task, r); e != nil {
				return e
			}
			for k, v := range (runtimeObject{"id": contract.NewID(), "revision": 1, "root_id": task["root_id"], "enabled": true, "schedule": runtimeObject{"kind": "once", "at": rtTime(due), "anchor_at": nil, "every_seconds": nil, "local_time": nil, "timezone": "UTC", "weekdays": []any{}, "event_type": nil, "filter": nil, "after_seq": nil}, "command": original["command"], "task_template": runtimeObject{"goal": task["goal"], "criteria": task["criteria"], "item_id": task["item_id"], "item_operation_key": nil}, "next_due_at": rtTime(due), "misfire": "FIRE_ONCE_WITHIN_GRACE", "grace_seconds": 300, "overlap": "SKIP", "max_attempts": 3, "updated_at": contract.Now()}) {
				job[k] = v
			}
			dto, e := rtTyped[contract.ScheduledJob](job)
			if e != nil {
				return e
			}
			grant := rtStr(rtObj(rtObj(original["extensions"])["runtime.authorization"])["grant_id"])
			return s.RegisterJobTx(ctx, tx, dto, grant)
		}
		return nil
	})
}
func (s *Store) VerifyTask(ctx context.Context, id, artifactDir string) (bool, error) {
	var passed bool
	e := s.Write(ctx, func(tx *sql.Tx) error {
		var err error
		passed, err = s.VerifyTaskTx(ctx, tx, id, artifactDir)
		return err
	})
	return passed, e
}
func (s *Store) VerifyTaskTx(ctx context.Context, tx *sql.Tx, id, artifactDir string) (bool, error) {
	passed := true
	e := func() error {
		t, e := rtRead(ctx, tx, "task", id)
		if e != nil {
			return e
		}
		if rtHash(t["criteria"]) != t["criterion_hash"] {
			return errors.New("CRITERION_TAMPERED")
		}
		results := []any{}
		criteria, _ := t["criteria"].([]any)
		for _, c := range criteria {
			cm := rtObj(c)
			expected := rtObj(cm["expected"])
			ok := false
			switch cm["kind"] {
			case "alarm_session_recorded", "briefing_artifact_recorded", "source_sync_recorded":
				ok = s.verifyRuntimeCriterion(ctx, tx, id, rtStr(cm["kind"]), expected)
			case "notification_recorded":
				var n int
				e = tx.QueryRowContext(ctx, `SELECT count(*) FROM notification WHERE notification_key=?`, expected["notification_key"]).Scan(&n)
				ok = e == nil && n > 0
			case "artifact_hash_matches":
				var p string
				p, e = SafeArtifactPath(artifactDir, rtStr(expected["relative_path"]))
				if e == nil {
					var b []byte
					b, e = os.ReadFile(p)
					h := sha256.Sum256(b)
					ok = e == nil && hex.EncodeToString(h[:]) == expected["sha256"]
				}
			case "world_revision_matches":
				var revision int
				e = tx.QueryRowContext(ctx, `SELECT revision FROM world_fact_head WHERE fact_id=?`, expected["fact_id"]).Scan(&revision)
				ok = e == nil && revision == rtInt(expected["revision"])
			case "source_cursor_committed":
				var raw string
				e = tx.QueryRowContext(ctx, `SELECT cursor_json FROM source_state WHERE id=?`, expected["source_id"]).Scan(&raw)
				var cursor any
				if e == nil {
					e = json.Unmarshal([]byte(raw), &cursor)
				}
				ok = e == nil && rtHash(cursor) == expected["cursor_hash"]
			case "consciousness_slot_committed":
				var raw string
				e = tx.QueryRowContext(ctx, `SELECT payload_json FROM consciousness_snapshot WHERE slot=?`, expected["slot"]).Scan(&raw)
				if e == nil {
					var snapshot contract.ConsciousnessState
					e = contract.Decode("ConsciousnessState", []byte(raw), &snapshot)
					ok = e == nil && snapshot.Slot == rtInt(expected["slot"])
				}
			case "master_confirmed":
				var n int
				e = tx.QueryRowContext(ctx, `SELECT count(*) FROM change_event WHERE entity_id=? AND event_type='item.master_confirmed' AND json_extract(payload_json,'$.origin')='authenticated_master'`, expected["item_id"]).Scan(&n)
				ok = e == nil && n > 0
			}
			verdict := "UNKNOWN"
			reason := "Required independent evidence is absent"
			if ok {
				verdict = "PASS"
				reason = "Deterministic authoritative evidence matched"
			} else {
				passed = false
			}
			results = append(results, runtimeObject{"criterion_id": cm["id"], "verdict": verdict, "evidence": []any{}, "reason": reason})
		}
		v := rtBase()
		for k, x := range (runtimeObject{"id": contract.NewID(), "task_id": id, "criterion_hash": t["criterion_hash"], "results": results, "verifier_version": "1", "created_at": contract.Now()}) {
			v[k] = x
		}
		if e = contract.Validate("Verification", v); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO verification(id,task_id,criterion_hash,payload_json,created_at)VALUES(?,?,?,?,?)`, v["id"], id, t["criterion_hash"], rtJSON(v), v["created_at"])
		if e != nil {
			return e
		}
		if t["state"] != "CANCELLED" {
			if passed {
				t["state"] = "SUCCEEDED"
			} else {
				t["state"] = "NEEDS_ATTENTION"
			}
			return rtSaveTask(ctx, tx, t)
		}
		return nil
	}()
	return passed, e
}

// DeriveCriteria constructs objective checks from already validated command semantics.
func DeriveCriteria(c contract.Command) ([]contract.Criterion, error) {
	m := rtMap(c)
	a := rtObj(m["arguments"])
	var kind string
	expected := runtimeObject{}
	switch m["capability"] {
	case "alarm.play":
		kind = "alarm_session_recorded"
		expected["device_id"] = a["device_id"]
		expected["audio_ref"] = rtObj(a["audio_ref"])["id"]
	case "briefing.build":
		kind = "briefing_artifact_recorded"
		expected["media_type"] = "text/plain"
	case "source.sync":
		kind = "source_sync_recorded"
		expected["source_id"] = a["source_id"]
	case "notify.local":
		kind = "notification_recorded"
		expected["notification_key"] = a["notification_key"]
	case "artifact.write":
		kind = "artifact_hash_matches"
		expected["relative_path"] = a["relative_path"]
		h := sha256.Sum256([]byte(rtStr(a["content"])))
		expected["sha256"] = hex.EncodeToString(h[:])
	default:
		return nil, errors.New("explicit independently verifiable criteria required for capability")
	}
	v, e := rtTyped[contract.Criterion](runtimeObject{"id": contract.NewID(), "kind": kind, "expected": expected, "evidence_policy": "Authoritative persisted state or artifact bytes"})
	return []contract.Criterion{v}, e
}

var _ = time.Second

func (s *Store) ExpireMutedAlarms(ctx context.Context, now time.Time) error {
	var reconcile []contract.JobRun
	err := s.Write(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT a.payload_json,r.payload_json FROM alarm_session a JOIN job_run r ON r.id=a.run_id WHERE a.state='PLAYING' OR (a.state='STOPPED' AND r.state IN('RUNNING','RESULT_UNKNOWN'))`)
		if e != nil {
			return e
		}
		type pair struct{ a, r runtimeObject }
		var entries []pair
		for rows.Next() {
			var ar, rr string
			if e = rows.Scan(&ar, &rr); e != nil {
				rows.Close()
				return e
			}
			var a, r runtimeObject
			_ = json.Unmarshal([]byte(ar), &a)
			_ = json.Unmarshal([]byte(rr), &r)
			entries = append(entries, pair{a, r})
		}
		rows.Close()
		for _, v := range entries {
			a, r := v.a, v.r
			if a["state"] == "PLAYING" {
				started, e := time.Parse(time.RFC3339Nano, rtStr(a["started_at"]))
				max := rtInt(rtObj(rtObj(r["command"])["arguments"])["max_duration_seconds"])
				if e != nil || max < 1 || started.Add(time.Duration(max)*time.Second).After(now) {
					continue
				}
				a["state"] = "STOPPED"
				a["revision"] = rtInt(a["revision"]) + 1
				a["stopped_at"] = rtTime(now)
				a["stop_reason"] = "MAX_DURATION"
				if _, e = tx.ExecContext(ctx, `UPDATE alarm_session SET state='STOPPED',revision=?,payload_json=? WHERE id=?`, a["revision"], rtJSON(a), a["id"]); e != nil {
					return e
				}
			}
			if r["state"] == "RUNNING" || r["state"] == "RESULT_UNKNOWN" {
				r["state"] = "RESULT_UNKNOWN"
				if e = rtSaveRun(ctx, tx, r); e != nil {
					return e
				}
				task, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
				if e != nil {
					return e
				}
				if task["state"] != "CANCELLED" {
					task["state"] = "NEEDS_ATTENTION"
					if e = rtSaveTask(ctx, tx, task); e != nil {
						return e
					}
				}
				dto, e := rtTyped[contract.JobRun](r)
				if e != nil {
					return e
				}
				reconcile = append(reconcile, dto)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, run := range reconcile {
		receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: run.ID, AttemptNo: run.AttemptNo, FencingToken: run.FencingToken, ReceiptKey: "alarm-expiry-evidence", Status: "SUCCEEDED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, EffectObserved: true, ReceivedAt: rtTime(now), Extensions: map[string]any{}}
		if err = s.RecordReceipt(ctx, receipt); err != nil {
			return err
		}
		if _, err = s.VerifyTask(ctx, run.TaskID, ""); err != nil {
			return err
		}
	}
	return nil
}
