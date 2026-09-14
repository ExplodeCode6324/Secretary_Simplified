package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/executor"
	"secretarysimplified/platform"
	"secretarysimplified/scheduler"
	"secretarysimplified/store"
	"strings"
	"testing"
	"time"
)

func runtimeDB(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, e := store.Init(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}
func runtimeGrant(t *testing.T, s *store.Store) string {
	t.Helper()
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"notify.local", "artifact.write"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e := s.PutGrant(context.Background(), g); e != nil {
		t.Fatal(e)
	}
	return g.ID
}
func runtimeCommand() contract.Command {
	return contract.Command{SchemaVersion: 1, OperationKey: "notify", Capability: "notify.local", CapabilityVersion: 1, Arguments: map[string]any{"text": "fixture", "notification_key": "fixture-notice"}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
}
func runtimeRegister(t *testing.T, s *store.Store, g string) contract.JobRun {
	t.Helper()
	cmd := runtimeCommand()
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.RegisterImmediate(context.Background(), contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	return run
}
func TestRuntimeNotificationPersistenceVerifier(t *testing.T) {
	s, dir := runtimeDB(t)
	g := runtimeGrant(t, s)
	run := runtimeRegister(t, s, g)
	r := executor.New(s, &executor.Options{ArtifactDir: dir})
	if worked, e := r.Step(context.Background()); e != nil || !worked {
		t.Fatalf("step: %v %v", worked, e)
	}
	got, e := s.GetRun(context.Background(), run.ID)
	if e != nil || got.State != "SUCCEEDED" {
		t.Fatalf("run: %+v %v", got, e)
	}
	var state string
	s.DB.QueryRow(`SELECT state FROM task WHERE id=?`, run.TaskID).Scan(&state)
	if state != "SUCCEEDED" {
		t.Fatal(state)
	}
	notices, e := s.Notifications(context.Background(), "")
	if e != nil || len(notices) != 1 || notices[0].State != "DELIVERED" {
		t.Fatalf("notices %v %v", notices, e)
	}
	s.Notifications(context.Background(), notices[0].ID)
	notices, _ = s.Notifications(context.Background(), "")
	if notices[0].State != "ACKNOWLEDGED" {
		t.Fatal(notices)
	}
	if _, e = r.Step(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestRuntimeDurableFenceAndUnknown(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	original := runtimeRegister(t, s, g)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	a, e := s.ClaimRun(ctx, "old", now)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.ClaimRun(ctx, "new", now.Add(31*time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if b.FencingToken <= a.FencingToken {
		t.Fatal("fence not advanced")
	}
	if _, e = s.DispatchRun(ctx, a, "old", now.Add(31*time.Second)); e == nil {
		t.Fatal("old dispatch accepted")
	}
	if _, e = s.DispatchRun(ctx, b, "new", now.Add(31*time.Second)); e != nil {
		t.Fatal(e)
	}
	_, e = s.ClaimRun(ctx, "third", now.Add(62*time.Second))
	if e != sql.ErrNoRows {
		t.Fatal(e)
	}
	got, _ := s.GetRun(ctx, original.ID)
	if got.State != "RESULT_UNKNOWN" {
		t.Fatal(got.State)
	}
}
func TestRuntimeCancelBeforeDispatch(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	run := runtimeRegister(t, s, g)
	if e := s.CancelTask(context.Background(), run.TaskID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ClaimRun(context.Background(), "x", time.Now().Add(time.Second)); e != sql.ErrNoRows {
		t.Fatal(e)
	}
}
func TestRuntimeBudgetNoReset(t *testing.T) {
	s, _ := runtimeDB(t)
	root := contract.NewID()
	ctx := context.Background()
	if e := s.ChargeBudget(ctx, root, "model_calls", 8); e != nil {
		t.Fatal(e)
	}
	if e := s.ChargeBudget(ctx, root, "model_calls", 1); e == nil {
		t.Fatal("budget reset")
	}
}
func TestRuntimeCalendarDST(t *testing.T) {
	local := "02:30"
	s := contract.Schedule{Kind: "daily", LocalTime: &local, Timezone: "America/New_York", Weekdays: []int{}}
	after, _ := time.Parse(time.RFC3339, "2026-03-08T00:00:00Z")
	next, e := scheduler.Next(s, after)
	if e != nil || next.Format(time.RFC3339) != "2026-03-09T06:30:00Z" {
		t.Fatalf("spring %v %v", next, e)
	}
	local = "01:30"
	after, _ = time.Parse(time.RFC3339, "2026-11-01T00:00:00Z")
	next, e = scheduler.Next(s, after)
	if e != nil || next.Format(time.RFC3339) != "2026-11-01T05:30:00Z" {
		t.Fatalf("fall %v %v", next, e)
	}
	next, e = scheduler.Next(s, *next)
	if e != nil || next.Format(time.RFC3339) != "2026-11-02T06:30:00Z" {
		t.Fatalf("fold duplicate %v %v", next, e)
	}
}
func TestRuntimeArtifactTraversal(t *testing.T) {
	root := t.TempDir()
	if _, e := store.SafeArtifactPath(root, "../escape"); e == nil {
		t.Fatal("traversal accepted")
	}
	if e := os.Symlink(t.TempDir(), filepath.Join(root, "link")); e != nil {
		t.Fatal(e)
	}
	if _, e := store.SafeArtifactPath(root, "link/file"); e == nil {
		t.Fatal("symlink accepted")
	}
}
func TestRuntimeWaitTimeoutPersistsAndReplanCriterion(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	run := runtimeRegister(t, s, g)
	ctx := context.Background()
	var hash string
	s.DB.QueryRow(`SELECT criterion_hash FROM task WHERE id=?`, run.TaskID).Scan(&hash)
	w := contract.WaitSubscription{SchemaVersion: 1, ID: contract.NewID(), TaskID: run.TaskID, Generation: 1, EventType: "task.updated", EntityID: contract.NewID(), ExpectedState: "SUCCEEDED", DeadlineAt: contract.Timestamp(time.Now().Add(time.Second)), State: "ARMED", Extensions: map[string]any{}}
	if e := s.Wait(ctx, w); e != nil {
		t.Fatal(e)
	}
	if n, e := s.WakeWaits(ctx, time.Now().Add(2*time.Second)); e != nil || n != 1 {
		t.Fatalf("wake %d %v", n, e)
	}
	if e := s.Replan(ctx, run.TaskID); e != nil {
		t.Fatal(e)
	}
	var after string
	s.DB.QueryRow(`SELECT criterion_hash FROM task WHERE id=?`, run.TaskID).Scan(&after)
	if hash != after {
		t.Fatal("criterion changed")
	}
	if e := s.Replan(ctx, run.TaskID); e != nil {
		t.Fatal(e)
	}
	if e := s.Replan(ctx, run.TaskID); e == nil {
		t.Fatal("replan budget reset")
	}
}
func TestRuntimeScheduleOccurrenceStableAcrossRestart(t *testing.T) {
	s, dir := runtimeDB(t)
	g := runtimeGrant(t, s)
	cmd := runtimeCommand()
	criteria, _ := store.DeriveCriteria(cmd)
	now := time.Now().UTC().Truncate(time.Second)
	due := contract.Timestamp(now.Add(-time.Second))
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "once", At: &due, Timezone: "UTC", Weekdays: []int{}}, Command: cmd, TaskTemplate: map[string]any{"goal": "notice", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &due, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	if e := s.RegisterJob(context.Background(), job, g); e != nil {
		t.Fatal(e)
	}
	if n, e := s.ScheduleStep(context.Background(), now); e != nil || n != 1 {
		t.Fatalf("schedule %d %v", n, e)
	}
	s.Close()
	again, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if n, e := again.ScheduleStep(context.Background(), now); e != nil || n != 0 {
		t.Fatalf("duplicate %d %v", n, e)
	}
	var count int
	again.DB.QueryRow(`SELECT count(*) FROM job_run`).Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
}
func TestRuntimeScheduledJobEventAtomic(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	cmd := runtimeCommand()
	criteria, _ := store.DeriveCriteria(cmd)
	due := contract.Timestamp(time.Now().Add(time.Minute))
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "once", At: &due, Timezone: "UTC", Weekdays: []int{}}, Command: cmd, TaskTemplate: map[string]any{"goal": "notice", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &due, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	if e := s.RegisterJob(context.Background(), job, g); e != nil {
		t.Fatal(e)
	}
	var raw string
	if e := s.DB.QueryRow(`SELECT payload_json FROM change_event WHERE entity_id=? AND event_type='scheduled_job.updated'`, job.ID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var event contract.ChangeEvent
	if e := contract.Decode("ChangeEvent", []byte(raw), &event); e != nil {
		t.Fatal(e)
	}
	if e := contract.ValidateEvent(event); e != nil {
		t.Fatal(e)
	}
	event.EventType = "unregistered.event"
	if e := contract.ValidateEvent(event); e == nil {
		t.Fatal("unknown event accepted")
	}
}
func TestRuntimeUnknownLocalReconciliation(t *testing.T) {
	s, dir := runtimeDB(t)
	g := runtimeGrant(t, s)
	original := runtimeRegister(t, s, g)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	claimed, e := s.ClaimRun(ctx, "crashed-worker", now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DispatchRun(ctx, claimed, "crashed-worker", now); e != nil {
		t.Fatal(e)
	}
	if e = s.RecordNotification(ctx, claimed); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ClaimRun(ctx, "recovery-worker", now.Add(31*time.Second)); e != sql.ErrNoRows {
		t.Fatal(e)
	}
	got, _ := s.GetRun(ctx, original.ID)
	if got.State != "RESULT_UNKNOWN" {
		t.Fatal(got.State)
	}
	ok, e := s.ReconcileLocal(ctx, got.ID, dir)
	if e != nil || !ok {
		t.Fatalf("reconciliation: %v %v", ok, e)
	}
	notices, _ := s.Notifications(ctx, "")
	if len(notices) != 1 {
		t.Fatalf("duplicate effect %d", len(notices))
	}
}
func TestRuntimeVirtual72HoursAnd30DayChanges(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		steps  int
		period time.Duration
	}{{"72-hour-continuity", 72, time.Hour}, {"30-day-three-change-points", 30, 24 * time.Hour}} {
		t.Run(scenario.name, func(t *testing.T) {
			s, dir := runtimeDB(t)
			ctx := context.Background()
			start := time.Now().UTC().Truncate(time.Second)
			g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"artifact.write"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{dir}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(start.Add(90 * 24 * time.Hour)), Extensions: map[string]any{}}
			if e := s.PutGrant(ctx, g); e != nil {
				t.Fatal(e)
			}
			command := contract.Command{SchemaVersion: 1, OperationKey: "write-fixture", Capability: "artifact.write", CapabilityVersion: 1, Arguments: map[string]any{"relative_path": "timeline.txt", "content": "initial synthetic fixture"}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
			criteria, _ := store.DeriveCriteria(command)
			anchor := contract.Timestamp(start)
			every := int(scenario.period / time.Second)
			job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "interval", AnchorAt: &anchor, EverySeconds: &every, Timezone: "UTC", Weekdays: []int{}}, Command: command, TaskTemplate: map[string]any{"goal": "synthetic timeline artifact", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &anchor, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
			if e := s.RegisterJob(ctx, job, g.ID); e != nil {
				t.Fatal(e)
			}
			clock := &platform.ManualClock{T: start}
			r := executor.New(s, &executor.Options{ArtifactDir: dir, Clock: clock})
			for i := 0; i < scenario.steps; i++ {
				now := start.Add(time.Duration(i) * scenario.period)
				if scenario.steps == 30 && (i == 9 || i == 19 || i == 24) {
					job.Command.Arguments["content"] = fmt.Sprintf("synthetic revision at day %d", i+1)
					criteria, _ = store.DeriveCriteria(job.Command)
					job.TaskTemplate["criteria"] = criteria
					job.Revision++
					at := contract.Timestamp(now)
					job.NextDueAt = &at
					job.UpdatedAt = at
					if e := s.UpdateJob(ctx, job, job.Revision-1); e != nil {
						t.Fatal(e)
					}
				}
				if n, e := s.ScheduleStep(ctx, now); e != nil || n != 1 {
					t.Fatalf("step %d schedule %d: %v", i, n, e)
				}
				if worked, e := r.Step(ctx); e != nil || !worked {
					t.Fatalf("step %d execution %v: %v", i, worked, e)
				}
				clock.Advance(scenario.period)
			}
			var succeeded, roots int
			if e := s.DB.QueryRow(`SELECT count(*) FROM task WHERE state='SUCCEEDED'`).Scan(&succeeded); e != nil {
				t.Fatal(e)
			}
			s.DB.QueryRow(`SELECT count(DISTINCT root_id) FROM task`).Scan(&roots)
			if succeeded != scenario.steps || roots != scenario.steps {
				t.Fatalf("success %d roots %d expected %d", succeeded, roots, scenario.steps)
			}
			if scenario.steps == 30 {
				var revisions int
				s.DB.QueryRow(`SELECT count(DISTINCT job_revision) FROM job_run`).Scan(&revisions)
				if revisions != 4 {
					t.Fatalf("expected initial plus three changes, got %d", revisions)
				}
			}
		})
	}
}
func TestRuntimeEventRuleCooldownSurvivesDatabaseState(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	command := runtimeCommand()
	criteria, _ := store.DeriveCriteria(command)
	eventType := "task.updated"
	seq := 0
	filter := map[string]any{}
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "event", Timezone: "UTC", Weekdays: []int{}, EventType: &eventType, Filter: &filter, AfterSeq: &seq}, Command: command, TaskTemplate: map[string]any{"goal": "event fixture", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, Misfire: "SKIP", GraceSeconds: 0, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	if e := s.RegisterJob(ctx, job, g); e != nil {
		t.Fatal(e)
	}
	runtimeRegister(t, s, g)
	now := time.Now().Add(time.Second)
	if n, e := s.ScheduleStep(ctx, now); e != nil || n != 1 {
		t.Fatalf("initial %d %v", n, e)
	}
	if n, e := s.ScheduleStep(ctx, now); e != nil || n != 0 {
		t.Fatalf("cooldown %d %v", n, e)
	}
	var count int
	if e := s.DB.QueryRow(`SELECT count(*) FROM rule_state WHERE rule_id=? AND no_progress_count=0 AND next_allowed_at IS NOT NULL`, job.ID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("durable cooldown %d %v", count, e)
	}
}
func TestRuntimeRecurringNotificationBinding(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	cmd := runtimeCommand()
	criteria, _ := store.DeriveCriteria(cmd)
	start := time.Now().UTC().Truncate(time.Second)
	anchor := contract.Timestamp(start)
	every := 60
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "interval", AnchorAt: &anchor, EverySeconds: &every, Timezone: "UTC", Weekdays: []int{}}, Command: cmd, TaskTemplate: map[string]any{"goal": "repeat", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &anchor, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	if e := s.RegisterJob(ctx, job, g); e != nil {
		t.Fatal(e)
	}
	clock := &platform.ManualClock{T: start}
	r := executor.New(s, &executor.Options{Clock: clock})
	keys := []string{}
	for i := 0; i < 2; i++ {
		if n, e := s.ScheduleStep(ctx, clock.Now()); e != nil || n != 1 {
			t.Fatalf("schedule %d %v", n, e)
		}
		var raw string
		if e := s.DB.QueryRow(`SELECT payload_json FROM job_run WHERE state='QUEUED'`).Scan(&raw); e != nil {
			t.Fatal(e)
		}
		var run contract.JobRun
		if e := contract.Decode("JobRun", []byte(raw), &run); e != nil {
			t.Fatal(e)
		}
		key := run.Command.Arguments["notification_key"].(string)
		keys = append(keys, key)
		if !strings.HasPrefix(key, "occ:v1:") {
			t.Fatal(key)
		}
		var taskRaw string
		s.DB.QueryRow(`SELECT payload_json FROM task WHERE id=?`, run.TaskID).Scan(&taskRaw)
		var task contract.Task
		contract.Decode("Task", []byte(taskRaw), &task)
		if task.Criteria[0].Expected["notification_key"] != key {
			t.Fatal("command/criterion mismatch")
		}
		before := task.CriterionHash
		if e := s.Replan(ctx, task.ID); e != nil {
			t.Fatal(e)
		}
		s.DB.QueryRow(`SELECT criterion_hash FROM task WHERE id=?`, task.ID).Scan(&taskRaw)
		if before != taskRaw {
			t.Fatal("replan changed key hash")
		}
		if _, e := r.Step(ctx); e != nil {
			t.Fatal(e)
		}
		if n, e := s.ScheduleStep(ctx, clock.Now()); e != nil || n != 0 {
			t.Fatalf("duplicate trigger %d %v", n, e)
		}
		clock.Advance(time.Minute)
	}
	if keys[0] == keys[1] {
		t.Fatal("occurrence keys reused")
	}
	jobs, e := s.RuntimeJobs(ctx)
	if e != nil || jobs[0].Command.Arguments["notification_key"] != "fixture-notice" {
		t.Fatal("template mutated")
	}
	notices, e := s.Notifications(ctx, "")
	if e != nil || len(notices) != 2 {
		t.Fatalf("wanted two notifications: %d %v", len(notices), e)
	}
	bad := job
	bad.ID = contract.NewID()
	bad.TaskTemplate = map[string]any{"goal": "bad", "criteria": []contract.Criterion{{ID: contract.NewID(), Kind: "notification_recorded", Expected: map[string]any{"notification_key": "wrong"}, EvidencePolicy: "persisted"}}, "item_id": nil, "item_operation_key": nil}
	if e = s.RegisterJob(ctx, bad, g); e == nil {
		t.Fatal("mismatch accepted")
	}
}
func TestRuntimeKnownNoEffectRetryBounded(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	original := runtimeRegister(t, s, g)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	for i := 1; i <= 3; i++ {
		run, e := s.ClaimRun(ctx, "worker", now)
		if e != nil {
			t.Fatal(e)
		}
		if run.ID != original.ID || run.ExternalIdempotencyKey != original.ExternalIdempotencyKey || run.AttemptNo != i {
			t.Fatal("retry identity changed")
		}
		if _, e = s.DispatchRun(ctx, run, "worker", now); e != nil {
			t.Fatal(e)
		}
		receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: run.ID, AttemptNo: i, FencingToken: run.FencingToken, ReceiptKey: fmt.Sprintf("known-failure-%d", i), Status: "FAILED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Timestamp(now), Extensions: map[string]any{}}
		if e = s.RecordReceipt(ctx, receipt); e != nil {
			t.Fatal(e)
		}
		got, _ := s.GetRun(ctx, run.ID)
		if i < 3 {
			if got.State != "QUEUED" {
				t.Fatal(got.State)
			}
			if _, e = s.ClaimRun(ctx, "too-soon", now); e != sql.ErrNoRows {
				t.Fatal("retry had no backoff", e)
			}
		} else if got.State != "FAILED" {
			t.Fatal(got.State)
		}
		now = now.Add(6 * time.Second)
	}
	if _, e := s.ClaimRun(ctx, "fourth", now); e != sql.ErrNoRows {
		t.Fatal("more than three attempts", e)
	}
}
func TestRuntimeControlCASAndPagination(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	first := runtimeRegister(t, s, g)
	runtimeRegister(t, s, g)
	page, e := s.RuntimePage(ctx, "task", "", 1)
	if e != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatalf("first page %+v %v", page, e)
	}
	runtimeRegister(t, s, g)
	next, e := s.RuntimePage(ctx, "task", *page.NextCursor, 1)
	if e != nil || len(next.Items) != 1 || next.NextCursor != nil {
		t.Fatalf("watermark page %+v %v", next, e)
	}
	if _, e = s.RuntimePage(ctx, "scheduled_job", *page.NextCursor, 1); e == nil {
		t.Fatal("cursor query mismatch accepted")
	}
	task, e := s.RuntimeTask(ctx, first.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	request := contract.NewID()
	if e = s.CancelTaskRequest(ctx, task.ID, request, task.Revision); e != nil {
		t.Fatal(e)
	}
	if e = s.CancelTaskRequest(ctx, task.ID, request, task.Revision); e != nil {
		t.Fatal("same request did not replay", e)
	}
	if e = s.CancelTaskRequest(ctx, task.ID, contract.NewID(), task.Revision); e == nil {
		t.Fatal("stale revision accepted")
	}
	after, _ := s.RuntimeTask(ctx, task.ID)
	if after.CancelGeneration != 1 {
		t.Fatal("duplicate cancel incremented generation")
	}
}
func TestRuntimeImmediateRetryIgnoresProgramAssignedCriterionID(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	root := contract.NewID()
	cmd := runtimeCommand()
	first, _ := store.DeriveCriteria(cmd)
	a, e := s.RegisterImmediate(ctx, root, root, cmd, first, g)
	if e != nil {
		t.Fatal(e)
	}
	second, _ := store.DeriveCriteria(cmd)
	b, e := s.RegisterImmediate(ctx, root, root, cmd, second, g)
	if e != nil || a.ID != b.ID {
		t.Fatalf("program ID disrupted retry %v", e)
	}
	cmd.Arguments["text"] = "different semantics"
	if _, e = s.RegisterImmediate(ctx, root, root, cmd, second, g); e == nil {
		t.Fatal("different payload accepted")
	}
}
func TestRuntimeDSTGapHasAtomicAudit(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	cmd := runtimeCommand()
	criteria, _ := store.DeriveCriteria(cmd)
	local := "02:30"
	due := "2026-03-07T07:30:00.000Z"
	now, _ := time.Parse(time.RFC3339, due)
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "daily", LocalTime: &local, Timezone: "America/New_York", Weekdays: []int{}}, Command: cmd, TaskTemplate: map[string]any{"goal": "DST fixture", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &due, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	if e := s.RegisterJob(ctx, job, g); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ScheduleStep(ctx, now); e != nil {
		t.Fatal(e)
	}
	var raw string
	if e := s.DB.QueryRow(`SELECT payload_json FROM change_event WHERE entity_id=? AND event_type='scheduled_job.skipped'`, job.ID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var event contract.ChangeEvent
	if e := contract.Decode("ChangeEvent", []byte(raw), &event); e != nil {
		t.Fatal(e)
	}
	if e := contract.ValidateEvent(event); e != nil {
		t.Fatal(e)
	}
	skip := event.Extensions["runtime.calendar_skip"].(map[string]any)
	if skip["local_date"] != "2026-03-08" || event.Origin != "scheduler.calendar" {
		t.Fatal(event)
	}
	after := event.Change["after"].(map[string]any)
	before := event.Change["before"].(map[string]any)
	if after["next_due_at"] != "2026-03-09T06:30:00.000Z" || before["next_due_at"] == after["next_due_at"] {
		t.Fatal("invalid advancement audit")
	}
	if _, e := s.ScheduleStep(ctx, now); e != nil {
		t.Fatal(e)
	}
	var count int
	s.DB.QueryRow(`SELECT count(*) FROM change_event WHERE event_type='scheduled_job.skipped'`).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate gap audit")
	}
}
func TestRuntimeCoreOfflineDoesNotBlockLocalQueue(t *testing.T) {
	s, _ := runtimeDB(t)
	g := probeGrant(t, s, []string{"notify.local", "memory.search"}, []string{})
	ctx := context.Background()
	command := contract.Command{SchemaVersion: 1, OperationKey: "remote", Capability: "memory.search", CapabilityVersion: 1, Arguments: map[string]any{"query": "synthetic", "entity_ids": []string{}, "cursor": nil}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	criteria := []contract.Criterion{{ID: contract.NewID(), Kind: "notification_recorded", Expected: map[string]any{"notification_key": "remote-proof"}, EvidencePolicy: "fixture"}}
	remote, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), command, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	local := runtimeRegister(t, s, g)
	r := executor.New(s, &executor.Options{})
	if worked, e := r.Step(ctx); e != nil || !worked {
		t.Fatalf("local step %v %v", worked, e)
	}
	got, _ := s.GetRun(ctx, remote.ID)
	if got.State != "QUEUED" || got.AttemptNo != 0 {
		t.Fatal("offline P1 was dispatched", got)
	}
	localRun, _ := s.GetRun(ctx, local.ID)
	if localRun.State != "SUCCEEDED" {
		t.Fatal(localRun.State)
	}
}
func TestRuntimeCanonicalCriterionHashUTF8(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	cmd := runtimeCommand()
	cmd.Arguments["notification_key"] = "<>&中文"
	criteria, _ := store.DeriveCriteria(cmd)
	run, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(criteria)
	canonical, e := contract.CanonicalJSON(raw)
	if e != nil {
		t.Fatal(e)
	}
	task, e := s.RuntimeTask(ctx, run.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	if task.CriterionHash != contract.Hash(canonical) {
		t.Fatalf("noncanonical hash %s", task.CriterionHash)
	}
	if _, e = s.ClaimRun(ctx, "worker", time.Now().Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	var attemptRaw string
	s.DB.QueryRow(`SELECT payload_json FROM execution_attempt WHERE run_id=?`, run.ID).Scan(&attemptRaw)
	var attempt contract.ExecutionAttempt
	if e = contract.Decode("ExecutionAttempt", []byte(attemptRaw), &attempt); e != nil {
		t.Fatal(e)
	}
	raw, _ = json.Marshal(cmd)
	canonical, _ = contract.CanonicalJSON(raw)
	if attempt.CommandHash != contract.Hash(canonical) {
		t.Fatal("noncanonical command hash")
	}
}
func TestRuntimeControlReplanAndCompletionTransaction(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	run := runtimeRegister(t, s, g)
	task, _ := s.RuntimeTask(ctx, run.TaskID)
	cmd := runtimeCommand()
	cmd.OperationKey = "replanned"
	control := contract.Control{Kind: "REPLAN", TaskID: &task.ID, Payload: map[string]any{"reason": "synthetic alternate approach", "command": cmd}}
	if e := s.Write(ctx, func(tx *sql.Tx) error { return s.ApplyControlTx(ctx, tx, control, task.RootID, s.ObjectsDir) }); e != nil {
		t.Fatal(e)
	}
	after, _ := s.RuntimeTask(ctx, task.ID)
	if after.CriterionHash != task.CriterionHash {
		t.Fatal("replan changed criterion")
	}
	old, _ := s.GetRun(ctx, run.ID)
	if old.State != "CANCELLED" {
		t.Fatal("old plan dispatchable")
	}
	r := executor.New(s, nil)
	if _, e := r.Step(ctx); e != nil {
		t.Fatal(e)
	}
	after, _ = s.RuntimeTask(ctx, task.ID)
	if after.State != "SUCCEEDED" {
		t.Fatal(after.State)
	}
}
func TestRuntimeMemorySlotRetryFiveAndThirtyMinutes(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"memory.refresh"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(now.Add(2 * time.Hour)), Extensions: map[string]any{}}
	if e := s.PutGrant(ctx, g); e != nil {
		t.Fatal(e)
	}
	initial, e := s.ScheduleMemorySlot(ctx, now.Add(-time.Hour), now, g.ID)
	if e != nil {
		t.Fatal(e)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		run, e := s.ClaimRun(ctx, "worker", now)
		if e != nil {
			t.Fatal(e)
		}
		if run.ID != initial.ID || run.AttemptNo != attempt {
			t.Fatal("memory retry reset identity")
		}
		if _, e = s.DispatchRun(ctx, run, "worker", now); e != nil {
			t.Fatal(e)
		}
		receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: run.ID, AttemptNo: attempt, FencingToken: run.FencingToken, ReceiptKey: fmt.Sprintf("memory-failure-%d", attempt), Status: "FAILED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Timestamp(now), Extensions: map[string]any{}}
		if e = s.RecordReceipt(ctx, receipt); e != nil {
			t.Fatal(e)
		}
		got, _ := s.GetRun(ctx, run.ID)
		if attempt < 3 {
			delay := 5 * time.Minute
			if attempt == 2 {
				delay = 30 * time.Minute
			}
			expected := contract.Timestamp(now.Add(delay))
			if got.ScheduledFor != expected || got.State != "QUEUED" {
				t.Fatalf("backoff got %s want %s", got.ScheduledFor, expected)
			}
			now = now.Add(delay)
		} else if got.State != "FAILED" {
			t.Fatal("memory retry exhausted state", got.State)
		}
	}
}

type runtimeBlockingCore struct{ started chan struct{} }

func (c runtimeBlockingCore) Execute(ctx context.Context, run contract.JobRun, permit contract.ExecutionPermit) (contract.ExecutorReceipt, error) {
	close(c.started)
	<-ctx.Done()
	return contract.ExecutorReceipt{}, ctx.Err()
}
func TestRuntimeDispatchedCancelSignalsExecutor(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"memory.search"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e := s.PutGrant(ctx, g); e != nil {
		t.Fatal(e)
	}
	command := contract.Command{SchemaVersion: 1, OperationKey: "blocking", Capability: "memory.search", CapabilityVersion: 1, Arguments: map[string]any{"query": "fixture", "entity_ids": []string{}, "cursor": nil}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	criteria := []contract.Criterion{{ID: contract.NewID(), Kind: "notification_recorded", Expected: map[string]any{"notification_key": "blocking-proof"}, EvidencePolicy: "fixture"}}
	run, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), command, criteria, g.ID)
	if e != nil {
		t.Fatal(e)
	}
	bridge := runtimeBlockingCore{started: make(chan struct{})}
	r := executor.New(s, &executor.Options{Core: bridge})
	done := make(chan error, 1)
	go func() { _, e := r.Step(ctx); done <- e }()
	select {
	case <-bridge.started:
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not start")
	}
	if e = r.Cancel(ctx, run.TaskID); e != nil {
		t.Fatal(e)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not signal active execution")
	}
	task, _ := s.RuntimeTask(ctx, run.TaskID)
	if task.State != "CANCELLED" {
		t.Fatal(task.State)
	}
	got, _ := s.GetRun(ctx, run.ID)
	if got.State != "RESULT_UNKNOWN" {
		t.Fatal("unproven effect was asserted absent", got.State)
	}
}
