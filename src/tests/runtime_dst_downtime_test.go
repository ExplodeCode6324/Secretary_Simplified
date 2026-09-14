package tests

import (
	"context"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestRuntimeDSTGapDuringDowntimeAuditedOnce(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	cmd := runtimeCommand()
	criteria, _ := store.DeriveCriteria(cmd)
	local := "02:30"
	due := "2026-03-07T07:30:00.000Z"
	now, _ := time.Parse(time.RFC3339, "2026-03-10T06:30:00.000Z")
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
	if after["next_due_at"] != "2026-03-11T06:30:00.000Z" || before["next_due_at"] == after["next_due_at"] {
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
