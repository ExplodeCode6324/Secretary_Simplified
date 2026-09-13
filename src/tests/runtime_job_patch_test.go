package tests

import (
	"context"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestRuntimeJobPatchAtomicIdempotency(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	cmd := runtimeCommand()
	criteria, _ := store.DeriveCriteria(cmd)
	due := contract.Timestamp(time.Now().Add(time.Hour))
	j := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "once", At: &due, Timezone: "UTC", Weekdays: []int{}}, Command: cmd, TaskTemplate: map[string]any{"goal": "patch", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, NextDueAt: &due, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: contract.Now(), Extensions: map[string]any{}}
	if e := s.RegisterJob(ctx, j, g); e != nil {
		t.Fatal(e)
	}
	run, e := s.TriggerJob(ctx, j.ID, contract.NewID(), time.Now(), 1)
	if e != nil {
		t.Fatal(e)
	}
	request := contract.NewID()
	patch := map[string]any{"enabled": false}
	got, e := s.UpdateJobRequest(ctx, j.ID, request, 1, patch)
	if e != nil || got.Revision != 2 || got.NextDueAt != nil {
		t.Fatalf("patch %+v %v", got, e)
	}
	r, _ := s.GetRun(ctx, run.ID)
	if r.State != "CANCELLED" {
		t.Fatal(r.State)
	}
	if _, e = s.UpdateJobRequest(ctx, j.ID, contract.NewID(), 2, map[string]any{"enabled": true}); e != nil {
		t.Fatal(e)
	}
	retry, e := s.UpdateJobRequest(ctx, j.ID, request, 1, patch)
	if e != nil || retry.Revision != 2 {
		t.Fatalf("stable response %+v %v", retry, e)
	}
	if _, e = s.UpdateJobRequest(ctx, j.ID, request, 1, map[string]any{"enabled": true}); e == nil {
		t.Fatal("key conflict accepted")
	}
	if _, e = s.UpdateJobRequest(ctx, j.ID, contract.NewID(), 3, map[string]any{"command": cmd}); e == nil {
		t.Fatal("command mutation")
	}
	bad := contract.NewID()
	if _, e = s.UpdateJobRequest(ctx, j.ID, bad, 3, map[string]any{"max_attempts": 0}); e == nil {
		t.Fatal("invalid accepted")
	}
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM request_receipt WHERE request_id=?`, bad).Scan(&n)
	if n != 0 {
		t.Fatal("failed patch receipt committed")
	}
	current, _ := s.RuntimeJob(ctx, j.ID)
	if current.Revision != 3 {
		t.Fatal("failed patch changed revision")
	}
}
