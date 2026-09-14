package tests

import (
	"context"
	"encoding/json"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/memory"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestAcceptanceA08ExactDayBoundaryReopen(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := contract.NewID()
	first, e := s.ScheduleMemorySlot(ctx, epoch, epoch, g)
	if e != nil {
		t.Fatal(e)
	}
	mem := memory.Service{Store: s, Model: p, Config: c, Epoch: epoch}
	snapshot, e := mem.Refresh(ctx, epoch)
	if e != nil {
		t.Fatal(e)
	}
	before := epoch.Add(23*time.Hour + 59*time.Minute)
	same, e := s.ScheduleMemorySlot(ctx, epoch, before, g)
	if e != nil || same.ID != "" && same.ID != first.ID {
		t.Fatal("23:59 new slot", e)
	}
	sameSnapshot, e := mem.Refresh(ctx, before)
	if e != nil || sameSnapshot.ID != snapshot.ID {
		t.Fatal("23:59 refreshed", e)
	}
	s.Close()
	s, e = store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	at := epoch.Add(24 * time.Hour)
	next, e := s.ScheduleMemorySlot(ctx, epoch, at, g)
	if e != nil || next.ID == first.ID {
		t.Fatal("24:00 missing slot", e)
	}
	mem.Store = s
	fresh, e := mem.Refresh(ctx, at)
	if e != nil || fresh.Slot != 1 {
		t.Fatal("24:00 snapshot", fresh, e)
	}
	s.Close()
	s, e = store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	mem.Store = s
	retry, e := s.ScheduleMemorySlot(ctx, epoch, at, g)
	if e != nil || retry.ID != "" && retry.ID != next.ID {
		t.Fatal("restart duplicated slot", e)
	}
	again, e := mem.Refresh(ctx, at)
	if e != nil || again.ID != fresh.ID {
		t.Fatal("restart duplicated snapshot", e)
	}
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM consciousness_snapshot`).Scan(&n)
	if n != 2 {
		t.Fatal("snapshots", n)
	}
}

func TestAcceptanceA02TenRetriesAcrossReopen(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	request, session := contract.NewID(), contract.NewID()
	cmd := runtimeCommand()
	cmd.Extensions = map[string]any{} // Authenticated synthetic typed fixture supplies data_class separately.
	criteria, _ := store.DeriveCriteria(cmd)
	due := contract.Timestamp(time.Now().Add(time.Hour))
	actions := []contract.ActionProposal{createAction(), {OperationKey: "immediate", Kind: "SUBMIT_TASK", Payload: map[string]any{"goal": "retry immediate", "item_id": nil, "item_operation_key": nil, "criteria": criteria, "command": cmd, "deadline_at": nil}}, {OperationKey: "scheduled", Kind: "CREATE_JOB", Payload: map[string]any{"schedule": contract.Schedule{Kind: "once", At: &due, Timezone: "UTC", Weekdays: []int{}}, "command": cmd, "task_template": map[string]any{"goal": "retry scheduled", "item_id": nil, "item_operation_key": nil, "criteria": criteria}, "misfire": "FIRE_ONCE_WITHIN_GRACE", "grace_seconds": 300}}}
	rawActions, _ := json.Marshal(actions)
	var id string
	for i := 0; i < 10; i++ {
		svc := core.Service{Store: s, Model: p, Config: c, GrantID: g}
		var retryActions []contract.ActionProposal
		json.Unmarshal(rawActions, &retryActions)
		turn, e := svc.TypedClass(ctx, request, session, retryActions, "SYNTHETIC")
		if e != nil {
			t.Fatal(i, e)
		}
		if i == 0 {
			id = turn.ID
		} else if turn.ID != id {
			t.Fatal("identity changed")
		}
		s.Close()
		s, e = store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
		if e != nil {
			t.Fatal(e)
		}
	}
	defer s.Close()
	for table, want := range map[string]int{"item": 1, "task": 1, "scheduled_job": 1, "job_run": 1, "input_turn": 1} {
		var n int
		if e := s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != want {
			t.Fatal(table, n, e)
		}
	}
	actions[0].Payload["title"] = "different payload"
	svc := core.Service{Store: s, Model: p, Config: c, GrantID: g}
	if _, e := svc.TypedClass(ctx, request, session, actions, "SYNTHETIC"); e == nil {
		t.Fatal("different request accepted")
	}
}

func TestAcceptanceA15SixWaitPaths(t *testing.T) {
	for _, mode := range []string{"already_satisfied", "registration_race", "lost_notification", "restart", "old_generation", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, dir := runtimeDB(t)
			ctx := context.Background()
			g := runtimeGrant(t, s)
			run := runtimeRegister(t, s, g)
			now := time.Now()
			item := contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "wait dependency", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
			if e := s.PutItem(ctx, item, 0); e != nil {
				t.Fatal(e)
			}
			mark := func(db *store.Store) error {
				v := item
				v.Status = "DONE"
				v.Revision = 2
				return db.PutItem(ctx, v, 1)
			}
			w := contract.WaitSubscription{SchemaVersion: 1, ID: contract.NewID(), TaskID: run.TaskID, Generation: 1, EventType: "item.updated", EntityID: item.ID, ExpectedState: "DONE", DeadlineAt: contract.Timestamp(now.Add(time.Hour)), State: "ARMED", Extensions: map[string]any{}}
			if mode == "already_satisfied" {
				if e := mark(s); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "registration_race" {
				other, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
				if e != nil {
					t.Fatal(e)
				}
				defer other.Close()
				gate := make(chan struct{})
				result := make(chan error, 2)
				go func() { <-gate; result <- s.Wait(ctx, w) }()
				go func() { <-gate; result <- mark(other) }()
				close(gate)
				for i := 0; i < 2; i++ {
					if e := <-result; e != nil {
						t.Fatal(e)
					}
				}
			} else {
				if e := s.Wait(ctx, w); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "old_generation" {
				w.ID = contract.NewID()
				if e := s.Wait(ctx, w); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "restart" {
				s.Close()
				var e error
				s, e = store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
				if e != nil {
					t.Fatal(e)
				}
				defer s.Close()
			}
			if mode != "already_satisfied" && mode != "registration_race" && mode != "timeout" {
				if e := mark(s); e != nil {
					t.Fatal(e)
				}
			}
			// No notification callback is invoked: the durable predicate scan alone must recover.
			if mode == "timeout" {
				now = now.Add(2 * time.Hour)
			}
			if _, e := s.WakeWaits(ctx, now); e != nil {
				t.Fatal(e)
			}
			if n, e := s.WakeWaits(ctx, now); e != nil || n != 0 {
				t.Fatal("duplicate continuation", n, e)
			}
			task, e := s.RuntimeTask(ctx, run.TaskID)
			if e != nil || task.State != "PENDING" {
				t.Fatal(task.State, e)
			}
			var n int
			if e = s.DB.QueryRow(`SELECT count(*) FROM wait_subscription WHERE task_id=? AND state IN ('WOKEN','EXPIRED')`, run.TaskID).Scan(&n); e != nil || n != 1 {
				t.Fatal("terminal generation count", n, e)
			}
			if mode == "old_generation" {
				s.DB.QueryRow(`SELECT count(*) FROM wait_subscription WHERE task_id=? AND state='CANCELLED'`, run.TaskID).Scan(&n)
				if n != 1 {
					t.Fatal("old generation not cancelled", n)
				}
			}
		})
	}
}
