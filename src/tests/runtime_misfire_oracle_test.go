package tests

import (
	"context"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func misfireJob(t *testing.T, s *store.Store, g, kind, misfire string, grace int) contract.ScheduledJob {
	t.Helper()
	anchor := "2026-01-05T08:00:00.000Z"
	schedule := contract.Schedule{Kind: kind, Timezone: "UTC", Weekdays: []int{}}
	switch kind {
	case "once":
		schedule.At = &anchor
	case "interval":
		step := 60
		schedule.AnchorAt = &anchor
		schedule.EverySeconds = &step
	case "daily", "weekly":
		local := "08:00"
		schedule.LocalTime = &local
		if kind == "weekly" {
			schedule.Weekdays = []int{1}
		}
	}
	cmd := runtimeCommand()
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: schedule, Command: cmd, TaskTemplate: map[string]any{"goal": "fixed misfire oracle", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &anchor, Misfire: misfire, GraceSeconds: grace, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	if e = s.RegisterJob(context.Background(), job, g); e != nil {
		t.Fatal(e)
	}
	return job
}
func TestA11FixedMisfireAndGraceOracles(t *testing.T) {
	cases := []struct {
		name, kind, policy, now, state, scheduled, next string
		grace, omitted                                  int
	}{
		{"once exact zero grace", "once", "SKIP", "2026-01-05T08:00:00.000Z", "QUEUED", "2026-01-05T08:00:00.000Z", "", 0, 0},
		{"once 904ms zero grace", "once", "SKIP", "2026-01-05T08:00:00.904Z", "SKIPPED", "2026-01-05T08:00:00.000Z", "", 0, 0},
		{"once within five minutes", "once", "FIRE_ONCE_WITHIN_GRACE", "2026-01-05T08:04:59.999Z", "QUEUED", "2026-01-05T08:00:00.000Z", "", 300, 0},
		{"once exact grace boundary", "once", "FIRE_ONCE_WITHIN_GRACE", "2026-01-05T08:05:00.000Z", "QUEUED", "2026-01-05T08:00:00.000Z", "", 300, 0},
		{"once outside grace", "once", "FIRE_ONCE_WITHIN_GRACE", "2026-01-05T08:05:00.001Z", "SKIPPED", "2026-01-05T08:00:00.000Z", "", 300, 0},
		{"skip despite nonzero grace", "once", "SKIP", "2026-01-05T08:00:02.000Z", "SKIPPED", "2026-01-05T08:00:00.000Z", "", 300, 0},
		{"interval aggregates downtime", "interval", "FIRE_ONCE_WITHIN_GRACE", "2026-01-05T08:03:10.000Z", "QUEUED", "2026-01-05T08:03:00.000Z", "2026-01-05T08:04:00.000Z", 300, 3},
		{"interval latest outside grace", "interval", "FIRE_ONCE_WITHIN_GRACE", "2026-01-05T08:03:10.000Z", "SKIPPED", "2026-01-05T08:03:00.000Z", "2026-01-05T08:04:00.000Z", 5, 3},
		{"daily aggregates downtime", "daily", "FIRE_ONCE_WITHIN_GRACE", "2026-01-08T08:00:10.000Z", "QUEUED", "2026-01-08T08:00:00.000Z", "2026-01-09T08:00:00.000Z", 300, 3},
		{"weekly aggregates downtime", "weekly", "FIRE_ONCE_WITHIN_GRACE", "2026-01-19T08:00:10.000Z", "QUEUED", "2026-01-19T08:00:00.000Z", "2026-01-26T08:00:00.000Z", 300, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := runtimeDB(t)
			g := runtimeGrant(t, s)
			job := misfireJob(t, s, g, tc.kind, tc.policy, tc.grace)
			now, e := time.Parse(time.RFC3339Nano, tc.now)
			if e != nil {
				t.Fatal(e)
			}
			if n, e := s.ScheduleStep(context.Background(), now); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			var raw string
			s.DB.QueryRow(`SELECT payload_json FROM job_run WHERE job_id=?`, job.ID).Scan(&raw)
			var run contract.JobRun
			if e = contract.Decode("JobRun", []byte(raw), &run); e != nil {
				t.Fatal(e)
			}
			if run.State != tc.state || run.ScheduledFor != tc.scheduled {
				t.Fatal("fixed oracle", run.State, run.ScheduledFor, tc.state, tc.scheduled)
			}
			updated, e := s.RuntimeJob(context.Background(), job.ID)
			if e != nil {
				t.Fatal(e)
			}
			if tc.next == "" {
				if updated.NextDueAt != nil || updated.Enabled {
					t.Fatal("once still scheduled")
				}
			} else if updated.NextDueAt == nil || *updated.NextDueAt != tc.next {
				t.Fatal("next due", updated.NextDueAt, tc.next)
			}
			if tc.omitted > 0 {
				m := updated.Extensions["runtime.misfire"].(map[string]any)
				if m["omitted_occurrences"] != float64(tc.omitted) {
					t.Fatal("omitted oracle", m)
				}
			}
			if n, e := s.ScheduleStep(context.Background(), now); e != nil || n != 0 {
				t.Fatal("duplicate materialization", n, e)
			}
			var count int
			s.DB.QueryRow(`SELECT count(*) FROM job_run WHERE job_id=?`, job.ID).Scan(&count)
			if count != 1 {
				t.Fatal("catchup flooded queue", count)
			}
		})
	}
}

func TestA11OverlapSkipsEveryUnresolvedState(t *testing.T) {
	for _, state := range []string{"QUEUED", "CLAIMED", "RUNNING", "RESULT_UNKNOWN"} {
		t.Run(state, func(t *testing.T) {
			s, _ := runtimeDB(t)
			g := runtimeGrant(t, s)
			ctx := context.Background()
			job := misfireJob(t, s, g, "interval", "FIRE_ONCE_WITHIN_GRACE", 300)
			start, _ := time.Parse(time.RFC3339Nano, "2026-01-05T08:00:00.000Z")
			if _, e := s.ScheduleStep(ctx, start); e != nil {
				t.Fatal(e)
			}
			var first contract.JobRun
			var e error
			if state != "QUEUED" {
				first, e = s.ClaimRun(ctx, "oracle", start)
				if e != nil {
					t.Fatal(e)
				}
			}
			if state == "RUNNING" || state == "RESULT_UNKNOWN" {
				if _, e = s.DispatchRun(ctx, first, "oracle", start); e != nil {
					t.Fatal(e)
				}
			}
			if state == "RESULT_UNKNOWN" {
				s.ClaimRun(ctx, "recovery", start.Add(31*time.Second))
			}
			if n, e := s.ScheduleStep(ctx, start.Add(time.Minute)); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			var queued, skipped int
			s.DB.QueryRow(`SELECT count(*) FROM job_run WHERE job_id=? AND state='SKIPPED'`, job.ID).Scan(&skipped)
			s.DB.QueryRow(`SELECT count(*) FROM job_run WHERE job_id=? AND state IN ('QUEUED','CLAIMED','RUNNING','RESULT_UNKNOWN')`, job.ID).Scan(&queued)
			if skipped != 1 || queued != 1 {
				t.Fatal("overlap oracle", skipped, queued)
			}
			if n, e := s.ScheduleStep(ctx, start.Add(time.Minute)); e != nil || n != 0 {
				t.Fatal("duplicate skipped occurrence", n, e)
			}
			updated, _ := s.RuntimeJob(ctx, job.ID)
			if updated.NextDueAt == nil || *updated.NextDueAt != "2026-01-05T08:02:00.000Z" {
				t.Fatal("overlap failed to advance", updated.NextDueAt)
			}
		})
	}
}
