package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/contract"
	"time"
)

type runtimeObject map[string]any

func rtMap(v any) runtimeObject {
	b, _ := json.Marshal(v)
	var m runtimeObject
	_ = json.Unmarshal(b, &m)
	return m
}
func rtJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func rtHash(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		return ""
	}
	canonical, e := contract.CanonicalJSON(b)
	if e != nil {
		return ""
	}
	return contract.Hash(canonical)
}
func rtStr(v any) string { s, _ := v.(string); return s }
func rtInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}
func rtObj(v any) runtimeObject {
	switch x := v.(type) {
	case map[string]any:
		return x
	case runtimeObject:
		return x
	}
	return runtimeObject{}
}
func rtTime(t time.Time) string { return contract.Timestamp(t) }
func rtRead(ctx context.Context, tx *sql.Tx, table, id string) (runtimeObject, error) {
	var b string
	err := tx.QueryRowContext(ctx, "SELECT payload_json FROM "+table+" WHERE id=?", id).Scan(&b)
	var m runtimeObject
	if err == nil {
		err = json.Unmarshal([]byte(b), &m)
		if err == nil {
			typ := map[string]string{"task": "Task", "job_run": "JobRun", "authorization_grant": "AuthorizationGrant", "execution_permit": "ExecutionPermit", "alarm_session": "AlarmSession", "scheduled_job": "ScheduledJob"}[table]
			if typ != "" {
				err = contract.Validate(typ, m)
			}
		}
	}
	return m, err
}
func rtTyped[T any](m runtimeObject) (T, error) {
	var v T
	err := json.Unmarshal([]byte(rtJSON(m)), &v)
	return v, err
}
func rtBase() runtimeObject { return runtimeObject{"schema_version": 1, "extensions": runtimeObject{}} }
func rtEvent(ctx context.Context, tx *sql.Tx, root, entity, id, state string) error {
	after, err := rtRead(ctx, tx, entity, id)
	if err != nil {
		return err
	}
	eventType := entity + ".updated"

	e := contract.ChangeEvent{SchemaVersion: 1, ID: contract.NewID(), RootID: root, EntityType: entity, EntityID: id, EntityRevision: rtInt(after["revision"]), EventType: eventType, Origin: "runner", CreatedAt: contract.Now(), Change: map[string]any{"before": nil, "after": after, "evidence": []any{}}, Extensions: map[string]any{}}
	if entity == "task" && state == "created" && after["job_id"] != nil {
		e.Origin = "scheduler"
		job, readErr := rtRead(ctx, tx, "scheduled_job", rtStr(after["job_id"]))
		if readErr != nil {
			return readErr
		}
		if rtObj(job["schedule"])["kind"] == "event" {
			key := rtStr(after["occurrence_key"])
			prefix := rtStr(job["id"]) + ":"
			if len(key) > len(prefix) {
				cause := key[len(prefix):]
				e.CausationID = &cause
			}
		}
	}
	return AppendEvent(ctx, tx, &e)
}
func ensureBudget(ctx context.Context, tx *sql.Tx, root string) error {
	m := rtBase()
	for k, v := range (runtimeObject{"root_id": root, "revision": 1, "model_calls_used": 0, "retrievals_used": 0, "actions_used": 0, "replans_used": 0, "output_tokens_used": 0, "active_ms_used": 0, "cooldown_until": nil, "limits": runtimeObject{"model_calls": 8, "retrievals": 3, "actions": 10, "replans": 2, "output_tokens": 16000, "active_ms": 300000}}) {
		m[k] = v
	}
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO root_budget(root_id,revision,payload_json)VALUES(?,1,?)`, root, rtJSON(m))
	return err
}

// ChargeBudget reserves durable budget before work. Reservations survive crashes.
func (s *Store) ChargeBudget(ctx context.Context, root, kind string, amount int) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return chargeBudgetTx(ctx, tx, root, kind, amount) })
}
func chargeBudgetTx(ctx context.Context, tx *sql.Tx, root, kind string, amount int) error {
	if amount < 0 {
		return errors.New("negative budget reservation")
	}
	switch kind {
	case "model_calls", "retrievals", "actions", "replans", "output_tokens", "active_ms":
	default:
		return errors.New("unknown budget")
	}
	if err := ensureBudget(ctx, tx, root); err != nil {
		return err
	}
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT payload_json FROM root_budget WHERE root_id=?`, root).Scan(&raw); err != nil {
		return err
	}
	var b runtimeObject
	_ = json.Unmarshal([]byte(raw), &b)
	n := rtInt(b[kind+"_used"]) + amount
	if n > rtInt(rtObj(b["limits"])[kind]) {
		return errors.New("BUDGET_EXHAUSTED")
	}
	b[kind+"_used"] = n
	b["revision"] = rtInt(b["revision"]) + 1
	_, err := tx.ExecContext(ctx, `UPDATE root_budget SET revision=?,payload_json=? WHERE root_id=?`, b["revision"], rtJSON(b), root)
	return err
}
func (s *Store) PutGrant(ctx context.Context, g contract.AuthorizationGrant) error {
	if err := contract.Validate("AuthorizationGrant", g); err != nil {
		return err
	}
	m := rtMap(g)
	return s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO authorization_grant(id,principal_id,revision,revoked,expires_at,payload_json)VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,revoked=excluded.revoked,expires_at=excluded.expires_at,payload_json=excluded.payload_json WHERE excluded.revision>authorization_grant.revision`, m["id"], m["principal_id"], m["revision"], m["revoked"], m["expires_at"], rtJSON(m))
		return err
	})
}
func (s *Store) RegisterImmediate(ctx context.Context, root, intent string, c contract.Command, criteria []contract.Criterion, grant string) (contract.JobRun, error) {
	var out contract.JobRun
	err := s.Write(ctx, func(tx *sql.Tx) error {
		var e error
		out, e = s.RegisterImmediateTx(ctx, tx, root, intent, c, criteria, grant)
		return e
	})
	return out, err
}
func (s *Store) RegisterImmediateTx(ctx context.Context, tx *sql.Tx, root, intent string, c contract.Command, criteria []contract.Criterion, grant string) (contract.JobRun, error) {
	var out contract.JobRun
	if c.Capability == "memory.refresh" {
		return out, errors.New("MEMORY_REFRESH_REQUIRES_SLOT_CONTROLLER")
	}
	if err := contract.Validate("Command", c); err != nil {
		return out, err
	}
	if len(criteria) == 0 {
		return out, errors.New("criteria required")
	}
	for _, v := range criteria {
		if err := contract.Validate("Criterion", v); err != nil {
			return out, err
		}
	}
	err := func() error {
		authorization, authErr := rtRead(ctx, tx, "authorization_grant", grant)
		if authErr != nil {
			return authErr
		}
		if authErr = grantCheck(authorization, c.Capability, time.Now()); authErr != nil {
			return authErr
		}
		cm := rtMap(c)
		criterionSemantics := []any{}
		for _, criterion := range criteria {
			m := rtMap(criterion)
			delete(m, "id")
			criterionSemantics = append(criterionSemantics, m)
		}
		hash := rtHash(runtimeObject{"command": cm, "criteria": criterionSemantics})
		var oldHash, old string
		err := tx.QueryRowContext(ctx, `SELECT payload_hash,payload_json FROM command_ledger WHERE intent_id=? AND operation_key=?`, intent, cm["operation_key"]).Scan(&oldHash, &old)
		if err == nil {
			if oldHash != hash {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			var runRaw string
			if e := tx.QueryRowContext(ctx, `SELECT payload_json FROM job_run WHERE occurrence_key=?`, intent+":"+rtStr(cm["operation_key"])).Scan(&runRaw); e != nil {
				return e
			}
			return json.Unmarshal([]byte(runRaw), &out)
		}
		if err != sql.ErrNoRows {
			return err
		}
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM command_ledger WHERE intent_id=? AND json_extract(payload_json,'$.capability')=? AND json_extract(payload_json,'$.arguments')=?`, intent, cm["capability"], rtJSON(cm["arguments"])).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return errors.New("SEMANTIC_DUPLICATE")
		}
		r, e := createRunTx(ctx, tx, root, nil, intent+":"+rtStr(cm["operation_key"]), time.Now(), cm, rtMap(runtimeObject{"goal": "Delegated command", "criteria": criteria, "item_id": nil}), grant, "QUEUED")
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO command_ledger(id,intent_id,operation_key,payload_hash,task_id,payload_json,created_at)VALUES(?,?,?,?,?,?,?)`, contract.NewID(), intent, cm["operation_key"], hash, r["task_id"], rtJSON(cm), contract.Now())
		if e != nil {
			return e
		}
		out, e = rtTyped[contract.JobRun](r)
		return e
	}()
	return out, err
}
func createRunTx(ctx context.Context, tx *sql.Tx, root string, job runtimeObject, key string, due time.Time, command, template runtimeObject, grant, state string) (runtimeObject, error) {
	if err := ensureBudget(ctx, tx, root); err != nil {
		return nil, err
	}
	if err := chargeBudgetTx(ctx, tx, root, "actions", 1); err != nil {
		return nil, err
	}
	// Instance binding is performed before the criterion snapshot is frozen.
	if job != nil && command["capability"] == "notify.local" {
		command = rtMap(command)
		template = rtMap(template)
		args := rtObj(command["arguments"])
		original := rtStr(args["notification_key"])
		bound := "occ:v1:" + base64.RawURLEncoding.EncodeToString([]byte(original)) + ":" + base64.RawURLEncoding.EncodeToString([]byte(key))
		args["notification_key"] = bound
		command["arguments"] = args
		criteria, _ := template["criteria"].([]any)
		for _, c := range criteria {
			criterion := rtObj(c)
			if criterion["kind"] == "notification_recorded" {
				expected := rtObj(criterion["expected"])
				if expected["notification_key"] != original {
					return nil, errors.New("NOTIFICATION_TEMPLATE_KEY_MISMATCH")
				}
				expected["notification_key"] = bound
				criterion["expected"] = expected
			}
		}
	}
	now := contract.Now()
	task := rtBase()
	var jobID, rev, parent any
	if job != nil {
		jobID = job["id"]
		rev = job["revision"]
		parent = job["root_id"]
	}
	for k, v := range (runtimeObject{"id": contract.NewID(), "revision": 1, "root_id": root, "parent_root_id": parent, "item_id": template["item_id"], "job_id": jobID, "occurrence_key": key, "goal": template["goal"], "state": "PENDING", "criteria": template["criteria"], "criterion_hash": rtHash(template["criteria"]), "cancel_generation": 0, "deadline_at": nil, "created_at": now, "updated_at": now}) {
		task[k] = v
	}
	if state == "SKIPPED" {
		task["state"] = "CANCELLED"
	}
	if err := contract.Validate("Task", task); err != nil {
		return nil, err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO task(id,revision,root_id,item_id,job_id,occurrence_key,state,criterion_hash,payload_json,updated_at)VALUES(?,?,?,?,?,?,?,?,?,?)`, task["id"], 1, root, task["item_id"], jobID, key, task["state"], task["criterion_hash"], rtJSON(task), now)
	if err != nil {
		return nil, err
	}
	run := rtBase()
	for k, v := range (runtimeObject{"id": contract.NewID(), "task_id": task["id"], "job_id": jobID, "job_revision": rev, "occurrence_key": key, "scheduled_for": rtTime(due), "state": state, "attempt_no": 0, "fencing_token": 0, "lease_owner": nil, "lease_until": nil, "external_idempotency_key": contract.NewID(), "command": command, "updated_at": now, "extensions": runtimeObject{"runtime.authorization": runtimeObject{"grant_id": grant}}}) {
		run[k] = v
	}
	if err = contract.Validate("JobRun", run); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO job_run(id,task_id,job_id,job_revision,occurrence_key,scheduled_for,state,external_idempotency_key,payload_json,updated_at)VALUES(?,?,?,?,?,?,?,?,?,?)`, run["id"], task["id"], jobID, rev, key, run["scheduled_for"], state, run["external_idempotency_key"], rtJSON(run), now)
	if err == nil {
		err = rtEvent(ctx, tx, root, "task", rtStr(task["id"]), "created")
	}
	return run, err
}
func (s *Store) GetRun(ctx context.Context, id string) (contract.JobRun, error) {
	var raw string
	var v contract.JobRun
	err := s.DB.QueryRowContext(ctx, `SELECT payload_json FROM job_run WHERE id=?`, id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &v)
	}
	return v, err
}
func rtChange(ctx context.Context, tx *sql.Tx, root, entity, event string, before, after runtimeObject) error {
	revision := rtInt(after["revision"])
	if revision < 1 {
		revision = rtInt(after["fencing_token"]) + 1
	}
	var beforeValue any
	if before != nil {
		beforeValue = before
	}
	e := contract.ChangeEvent{SchemaVersion: 1, ID: contract.NewID(), RootID: root, EntityType: entity, EntityID: rtStr(after["id"]), EntityRevision: revision, EventType: event, Origin: "runner", CreatedAt: contract.Now(), Change: map[string]any{"before": beforeValue, "after": after, "evidence": []any{}}, Extensions: map[string]any{}}
	return AppendEvent(ctx, tx, &e)
}
func rtSaveRun(ctx context.Context, tx *sql.Tx, r runtimeObject) error {
	if e := contract.Validate("JobRun", r); e != nil {
		return e
	}
	before, e := rtRead(ctx, tx, "job_run", rtStr(r["id"]))
	if e != nil {
		return e
	}
	result, e := tx.ExecContext(ctx, `UPDATE job_run SET state=?,attempt_no=?,fencing_token=?,lease_owner=?,lease_until=?,scheduled_for=?,payload_json=?,updated_at=? WHERE id=? AND fencing_token=? AND attempt_no=?`, r["state"], r["attempt_no"], r["fencing_token"], r["lease_owner"], r["lease_until"], r["scheduled_for"], rtJSON(r), r["updated_at"], r["id"], before["fencing_token"], before["attempt_no"])
	if e != nil {
		return e
	}
	affected, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if affected != 1 {
		return errors.New("STALE_FENCE")
	}
	t, e := rtRead(ctx, tx, "task", rtStr(r["task_id"]))
	if e != nil {
		return e
	}
	return rtChange(ctx, tx, rtStr(t["root_id"]), "run", "run.updated", before, r)
}
func rtSaveTask(ctx context.Context, tx *sql.Tx, t runtimeObject) error {
	before, e := rtRead(ctx, tx, "task", rtStr(t["id"]))
	if e != nil {
		return e
	}
	if t["criterion_hash"] != before["criterion_hash"] || rtHash(t["criteria"]) != before["criterion_hash"] {
		return errors.New("IMMUTABLE_CRITERION")
	}
	t["revision"] = rtInt(t["revision"]) + 1
	t["updated_at"] = contract.Now()
	if e = contract.Validate("Task", t); e != nil {
		return e
	}
	result, e := tx.ExecContext(ctx, `UPDATE task SET state=?,revision=?,cancel_generation=?,payload_json=?,updated_at=? WHERE id=? AND criterion_hash=?`, t["state"], t["revision"], t["cancel_generation"], rtJSON(t), t["updated_at"], t["id"], t["criterion_hash"])
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
	return rtChange(ctx, tx, rtStr(t["root_id"]), "task", "task.updated", before, t)
}

var _ = fmt.Sprintf
