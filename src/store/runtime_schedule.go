package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/contract"
	"time"
)

// NextOccurrence returns the earliest valid occurrence strictly after after.
// Scanning UTC minutes gives the first fold and skips nonexistent civil times.
func NextOccurrence(schedule contract.Schedule, after time.Time) (*time.Time, error) {
	m := rtMap(schedule)
	kind := rtStr(m["kind"])
	switch kind {
	case "event":
		return nil, nil
	case "immediate":
		t := after.Add(time.Nanosecond)
		return &t, nil
	case "once":
		t, e := time.Parse(time.RFC3339Nano, rtStr(m["at"]))
		if e != nil {
			return nil, e
		}
		if t.After(after) {
			return &t, nil
		}
		return nil, nil
	case "interval":
		anchor, e := time.Parse(time.RFC3339Nano, rtStr(m["anchor_at"]))
		if e != nil {
			return nil, e
		}
		step := time.Duration(rtInt(m["every_seconds"])) * time.Second
		if step < time.Minute {
			return nil, errors.New("interval below 60 seconds")
		}
		if anchor.After(after) {
			return &anchor, nil
		}
		t := anchor.Add((after.Sub(anchor)/step + 1) * step)
		return &t, nil
	case "daily", "weekly":
		loc, e := time.LoadLocation(rtStr(m["timezone"]))
		if e != nil {
			return nil, e
		}
		for d := 0; d < 370; d++ {
			local := after.In(loc).AddDate(0, 0, d)
			y, mo, day := local.Date()
			if kind == "weekly" {
				weekday := int(local.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				found := false
				arr, _ := m["weekdays"].([]any)
				for _, w := range arr {
					if rtInt(w) == weekday {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			base := time.Date(y, mo, day, 0, 0, 0, 0, time.UTC).Add(-15 * time.Hour)
			var first *time.Time
			for i := 0; i < 54*60; i++ {
				u := base.Add(time.Duration(i) * time.Minute)
				v := u.In(loc)
				vy, vm, vd := v.Date()
				if vy == y && vm == mo && vd == day && v.Format("15:04") == rtStr(m["local_time"]) {
					first = &u
					break
				}
			}
			if first != nil && first.After(after) {
				return first, nil
			}
		}
		return nil, errors.New("no calendar occurrence within year")
	default:
		return nil, errors.New("unknown schedule")
	}
}
func (s *Store) RegisterJob(ctx context.Context, j contract.ScheduledJob, grant string) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return s.RegisterJobTx(ctx, tx, j, grant) })
}
func (s *Store) RegisterJobTx(ctx context.Context, tx *sql.Tx, j contract.ScheduledJob, grant string) error {
	if e := contract.Validate("ScheduledJob", j); e != nil {
		return e
	}
	authorization, authErr := rtRead(ctx, tx, "authorization_grant", grant)
	if authErr != nil {
		return authErr
	}
	if authErr = grantCheck(authorization, j.Command.Capability, time.Now()); authErr != nil {
		return authErr
	}
	m := rtMap(j)
	if rtObj(m["command"])["capability"] == "notify.local" {
		key := rtObj(rtObj(m["command"])["arguments"])["notification_key"]
		criteria, _ := rtObj(m["task_template"])["criteria"].([]any)
		found := false
		for _, c := range criteria {
			cm := rtObj(c)
			if cm["kind"] == "notification_recorded" {
				found = true
				if rtObj(cm["expected"])["notification_key"] != key {
					return errors.New("NOTIFICATION_TEMPLATE_KEY_MISMATCH")
				}
			}
		}
		if !found {
			return errors.New("NOTIFICATION_CRITERION_REQUIRED")
		}
	}

	if m["next_due_at"] == nil && j.Enabled {
		schedule := rtObj(m["schedule"])
		switch schedule["kind"] {
		case "once":
			m["next_due_at"] = schedule["at"]
		case "interval":
			m["next_due_at"] = schedule["anchor_at"]
		case "immediate":
			m["next_due_at"] = contract.Now()
		case "daily", "weekly":
			next, err := NextOccurrence(j.Schedule, time.Now().Add(-time.Millisecond))
			if err != nil {
				return err
			}
			if next != nil {
				m["next_due_at"] = rtTime(*next)
			}
		}
	}
	ext := rtObj(m["extensions"])
	ext["runtime.authorization"] = runtimeObject{"grant_id": grant}
	m["extensions"] = ext
	if e := ensureBudget(ctx, tx, rtStr(m["root_id"])); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO scheduled_job(id,revision,root_id,enabled,next_due_at,payload_json,updated_at)VALUES(?,?,?,?,?,?,?)`, m["id"], m["revision"], m["root_id"], m["enabled"], m["next_due_at"], rtJSON(m), m["updated_at"])
	if e == nil {
		e = rtEvent(ctx, tx, rtStr(m["root_id"]), "scheduled_job", rtStr(m["id"]), "created")
	}
	return e
}
func (s *Store) ScheduleStep(ctx context.Context, now time.Time) (int, error) {
	n := 0
	e := s.Write(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM scheduled_job WHERE enabled=1`)
		if e != nil {
			return e
		}
		var jobs []runtimeObject
		for rows.Next() {
			var raw string
			rows.Scan(&raw)
			var m runtimeObject
			_ = json.Unmarshal([]byte(raw), &m)
			jobs = append(jobs, m)
		}
		rows.Close()
		for _, j := range jobs {
			beforeJob := rtMap(j)
			gapDates := []string{}
			sm := rtObj(j["schedule"])
			kind := rtStr(sm["kind"])
			grant := rtStr(rtObj(rtObj(j["extensions"])["runtime.authorization"])["grant_id"])
			if kind == "event" {
				events, e := tx.QueryContext(ctx, `SELECT seq,id,root_id,entity_id,payload_json FROM change_event WHERE seq>? AND event_type=? ORDER BY seq LIMIT 100`, rtInt(sm["after_seq"]), sm["event_type"])
				if e != nil {
					return e
				}
				type ev struct {
					seq                   int
					id, root, entity, raw string
				}
				var list []ev
				for events.Next() {
					var v ev
					events.Scan(&v.seq, &v.id, &v.root, &v.entity, &v.raw)
					list = append(list, v)
				}
				events.Close()
				for _, v := range list {
					sm["after_seq"] = v.seq
					var payload runtimeObject
					_ = json.Unmarshal([]byte(v.raw), &payload)
					if payload["origin"] == "scheduler.calendar" {
						continue
					}
					change := rtObj(payload["change"])
					after := rtObj(change["after"])
					if (payload["origin"] == "runner" || payload["origin"] == "scheduler") && after["job_id"] == j["id"] {
						continue
					}
					beforeEffect := rtMap(change["before"])
					afterEffect := rtMap(change["after"])
					for _, key := range []string{"revision", "updated_at", "created_at"} {
						delete(beforeEffect, key)
						delete(afterEffect, key)
					}
					if rtHash(beforeEffect) == rtHash(afterEffect) {
						continue
					}
					effectHash := rtHash(afterEffect)
					match := true
					for k, x := range rtObj(sm["filter"]) {
						if rtJSON(payload[k]) != rtJSON(x) {
							match = false
						}
					}
					if !match {
						continue
					}
					var allowed sql.NullString
					var count int
					var oldEffect sql.NullString
					err := tx.QueryRowContext(ctx, `SELECT next_allowed_at,no_progress_count,json_extract(payload_json,'$.last_effect_hash') FROM rule_state WHERE rule_id=? AND root_id=?`, j["id"], v.root).Scan(&allowed, &count, &oldEffect)
					if err != nil && err != sql.ErrNoRows {
						return err
					}
					if oldEffect.Valid && oldEffect.String == effectHash {
						count++
					} else {
						count = 0
					}
					if count >= 3 || (allowed.Valid && allowed.String > rtTime(now)) {
						continue
					}
					key := rtStr(j["id"]) + ":" + v.id
					var exists int
					tx.QueryRowContext(ctx, `SELECT count(*) FROM job_run WHERE occurrence_key=?`, key).Scan(&exists)
					if exists == 0 {
						_, e = createRunTx(ctx, tx, v.root, j, key, now, rtObj(j["command"]), rtObj(j["task_template"]), grant, "QUEUED")
						if e != nil {
							return e
						}
						n++
					}
					rule := rtBase()
					for k, x := range (runtimeObject{"rule_id": j["id"], "root_id": v.root, "cursor_seq": v.seq, "next_allowed_at": rtTime(now.Add(time.Minute)), "no_progress_count": count, "last_effect_hash": effectHash}) {
						rule[k] = x
					}
					_, e = tx.ExecContext(ctx, `INSERT INTO rule_state(rule_id,root_id,next_allowed_at,no_progress_count,cursor_seq,payload_json)VALUES(?,?,?,?,?,?) ON CONFLICT(rule_id,root_id) DO UPDATE SET next_allowed_at=excluded.next_allowed_at,no_progress_count=excluded.no_progress_count,cursor_seq=excluded.cursor_seq,payload_json=excluded.payload_json`, j["id"], v.root, rtTime(now.Add(time.Minute)), count, v.seq, rtJSON(rule))
					if e != nil {
						return e
					}
				}
				j["schedule"] = sm
				if _, e = tx.ExecContext(ctx, `INSERT INTO consumer_cursor(consumer_id,last_seq,revision)VALUES(?,?,1) ON CONFLICT(consumer_id) DO UPDATE SET last_seq=excluded.last_seq,revision=consumer_cursor.revision+1`, "scheduler:"+rtStr(j["id"]), sm["after_seq"]); e != nil {
					return e
				}
			} else {
				dueRaw := rtStr(j["next_due_at"])
				if dueRaw == "" {
					continue
				}
				due, e := time.Parse(time.RFC3339Nano, dueRaw)
				if e != nil {
					return e
				}
				if due.After(now) {
					continue
				}
				schedule, e := rtTyped[contract.Schedule](sm)
				if e != nil {
					return e
				}
				latest := due
				missed := 0
				if kind == "interval" {
					step := time.Duration(rtInt(sm["every_seconds"])) * time.Second
					if step < time.Minute {
						return errors.New("invalid interval")
					}
					missed = int(now.Sub(due) / step)
					latest = due.Add(time.Duration(missed) * step)
				} else if kind == "daily" || kind == "weekly" {
					for i := 0; i < 37000; i++ {
						next, e := NextOccurrence(schedule, latest)
						if e != nil {
							return e
						}
						if next == nil || next.After(now) {
							break
						}
						latest = *next
						missed++
					}
				}
				state := "QUEUED"
				if now.Sub(latest) > time.Duration(rtInt(j["grace_seconds"]))*time.Second || (j["misfire"] == "SKIP" && now.Sub(latest) > time.Second) {
					state = "SKIPPED"
				}
				var active int
				if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM job_run WHERE job_id=? AND state IN('QUEUED','CLAIMED','RUNNING','RESULT_UNKNOWN')`, j["id"]).Scan(&active); e != nil {
					return e
				}
				if active > 0 {
					state = "SKIPPED"
				}
				key := fmt.Sprint(j["id"]) + ":" + rtTime(latest)
				var exists int
				tx.QueryRowContext(ctx, `SELECT count(*) FROM job_run WHERE occurrence_key=?`, key).Scan(&exists)
				if exists == 0 {
					_, e = createRunTx(ctx, tx, contract.NewID(), j, key, latest, rtObj(j["command"]), rtObj(j["task_template"]), grant, state)
					if e != nil {
						return e
					}
					n++
				}
				if missed > 0 {
					ext := rtObj(j["extensions"])
					ext["runtime.misfire"] = runtimeObject{"omitted_occurrences": missed}
					j["extensions"] = ext
				}
				if kind == "immediate" || kind == "once" {
					j["enabled"] = false
					j["next_due_at"] = nil
				} else {
					next, e := NextOccurrence(schedule, now)
					if e != nil {
						return e
					}
					if next == nil {
						j["next_due_at"] = nil
					} else {
						j["next_due_at"] = rtTime(*next)
						gapDates = calendarGapDates(sm, latest, *next)
					}
				}
			}
			j["updated_at"] = rtTime(now)
			_, e = tx.ExecContext(ctx, `UPDATE scheduled_job SET enabled=?,next_due_at=?,payload_json=?,updated_at=? WHERE id=?`, j["enabled"], j["next_due_at"], rtJSON(j), j["updated_at"], j["id"])
			if e != nil {
				return e
			}
			for _, date := range gapDates {
				eventID := contract.DeriveID("calendar-gap:" + rtStr(j["id"]) + ":" + date + ":" + rtHash(sm))
				skip := runtimeObject{"local_date": date, "timezone": sm["timezone"], "local_time": sm["local_time"], "reason": "DST_GAP"}
				var oldRaw string
				oldErr := tx.QueryRowContext(ctx, `SELECT payload_json FROM change_event WHERE id=?`, eventID).Scan(&oldRaw)
				if oldErr == nil {
					var old runtimeObject
					_ = json.Unmarshal([]byte(oldRaw), &old)
					if rtHash(rtObj(old["extensions"])["runtime.calendar_skip"]) != rtHash(skip) || rtHash(rtObj(rtObj(old["change"])["after"])["schedule"]) != rtHash(sm) {
						return errors.New("CALENDAR_SKIP_IDEMPOTENCY_CONFLICT")
					}
					continue
				}
				if oldErr != sql.ErrNoRows {
					return oldErr
				}
				event := contract.ChangeEvent{SchemaVersion: 1, ID: eventID, RootID: rtStr(j["root_id"]), EntityType: "scheduled_job", EntityID: rtStr(j["id"]), EntityRevision: rtInt(j["revision"]), EventType: "scheduled_job.skipped", Origin: "scheduler.calendar", CreatedAt: rtTime(now), Change: map[string]any{"before": beforeJob, "after": j, "evidence": []any{}}, Extensions: map[string]any{"runtime.calendar_skip": skip}}
				if e = AppendEvent(ctx, tx, &event); e != nil {
					return e
				}
			}
		}
		return nil
	})
	return n, e
}

// UpdateJob changes the business revision and cancels undispatched old snapshots.
// Consumed occurrence identities remain permanently occupied.
func (s *Store) UpdateJob(ctx context.Context, j contract.ScheduledJob, expected int) error {
	if e := contract.Validate("ScheduledJob", j); e != nil {
		return e
	}
	return s.Write(ctx, func(tx *sql.Tx) error { return s.UpdateJobTx(ctx, tx, j, expected) })
}

func (s *Store) UpdateJobTx(ctx context.Context, tx *sql.Tx, j contract.ScheduledJob, expected int) error {
	if e := contract.Validate("ScheduledJob", j); e != nil {
		return e
	}
	old, e := rtRead(ctx, tx, "scheduled_job", j.ID)
	if e != nil {
		return e
	}
	if rtInt(old["revision"]) != expected || j.Revision != expected+1 {
		return errors.New("REVISION_CONFLICT")
	}
	m := rtMap(j)
	if rtObj(m["command"])["capability"] == "notify.local" {
		key := rtObj(rtObj(m["command"])["arguments"])["notification_key"]
		criteria, _ := rtObj(m["task_template"])["criteria"].([]any)
		found := false
		for _, c := range criteria {
			cm := rtObj(c)
			if cm["kind"] == "notification_recorded" {
				found = true
				if rtObj(cm["expected"])["notification_key"] != key {
					return errors.New("NOTIFICATION_TEMPLATE_KEY_MISMATCH")
				}
			}
		}
		if !found {
			return errors.New("NOTIFICATION_CRITERION_REQUIRED")
		}
	}

	if m["root_id"] != old["root_id"] {
		return errors.New("IMMUTABLE_JOB_ROOT")
	}
	ext := rtObj(m["extensions"])
	ext["runtime.authorization"] = rtObj(old["extensions"])["runtime.authorization"]
	m["extensions"] = ext
	rows, e := tx.QueryContext(ctx, `SELECT payload_json FROM job_run WHERE job_id=? AND state IN('QUEUED','CLAIMED')`, j.ID)
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
		task, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
		if e != nil {
			return e
		}
		task["cancel_generation"] = rtInt(task["cancel_generation"]) + 1
		task["state"] = "CANCELLED"
		if e = rtSaveTask(ctx, tx, task); e != nil {
			return e
		}
	}
	result, e := tx.ExecContext(ctx, `UPDATE scheduled_job SET revision=?,enabled=?,next_due_at=?,payload_json=?,updated_at=? WHERE id=? AND revision=?`, j.Revision, j.Enabled, j.NextDueAt, rtJSON(m), j.UpdatedAt, j.ID, expected)
	if e != nil {
		return e
	}
	affected, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if affected != 1 {
		return errors.New("REVISION_CONFLICT")
	}
	return rtChange(ctx, tx, j.RootID, "scheduled_job", "scheduled_job.updated", old, m)
}

func calendarGapDates(schedule runtimeObject, from, to time.Time) []string {
	out := []string{}
	kind := rtStr(schedule["kind"])
	if kind != "daily" && kind != "weekly" {
		return out
	}
	loc, e := time.LoadLocation(rtStr(schedule["timezone"]))
	if e != nil {
		return out
	}
	fy, fm, fd := from.In(loc).Date()
	ty, tm, td := to.In(loc).Date()
	start := time.Date(fy, fm, fd, 12, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	end := time.Date(ty, tm, td, 12, 0, 0, 0, time.UTC)
	hour, minute := 0, 0
	fmt.Sscanf(rtStr(schedule["local_time"]), "%d:%d", &hour, &minute)
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		if kind == "weekly" {
			day := int(d.Weekday())
			if day == 0 {
				day = 7
			}
			allowed := false
			days, _ := schedule["weekdays"].([]any)
			for _, x := range days {
				if rtInt(x) == day {
					allowed = true
				}
			}
			if !allowed {
				continue
			}
		}
		y, m, day := d.Date()
		candidate := time.Date(y, m, day, hour, minute, 0, 0, loc)
		cy, cm, cd := candidate.Date()
		if cy != y || cm != m || cd != day || candidate.Hour() != hour || candidate.Minute() != minute {
			out = append(out, d.Format("2006-01-02"))
		}
	}
	return out
}
