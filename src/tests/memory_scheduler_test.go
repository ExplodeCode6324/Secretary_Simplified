package tests

import (
	"context"
	"secretarysimplified/contract"
	"secretarysimplified/memory"
	"sync"
	"testing"
	"time"
)

func TestPersistentMemorySlotConcurrentAndExactVerification(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	epoch := time.Now()
	grant := contract.NewID()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.ScheduleMemorySlot(ctx, epoch, epoch, grant); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	var count int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM job_run").Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate memory job", count, e)
	}
	run, e := s.ScheduleMemorySlot(ctx, epoch, epoch, grant)
	if e != nil {
		t.Fatal(e)
	}
	mem := memory.Service{Store: s, Model: p, Config: c, Epoch: epoch}
	if _, e = mem.RefreshSlot(ctx, 1, epoch.Add(24*time.Hour)); e != nil {
		t.Fatal(e)
	}
	ok, e := s.VerifyTask(ctx, run.TaskID, "")
	if e != nil || ok {
		t.Fatal("future slot verified earlier target", ok, e)
	}
	if _, e = mem.RefreshSlot(ctx, 0, epoch.Add(24*time.Hour)); e == nil {
		t.Fatal("superseded slot regenerated")
	}
}
func TestPersistentMemorySlotSkipMissedAndFrozenCriterion(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	epoch := time.Now()
	grant := contract.NewID()
	first, e := s.ScheduleMemorySlot(ctx, epoch, epoch, grant)
	if e != nil {
		t.Fatal(e)
	}
	current, e := s.ScheduleMemorySlot(ctx, epoch, epoch.Add(72*time.Hour), grant)
	if e != nil {
		t.Fatal(e)
	}
	old, e := s.GetRun(ctx, first.ID)
	if e != nil || old.State != "CANCELLED" {
		t.Fatal("missed queued slot still dispatchable", old.State, e)
	}
	if current.Command.Arguments["slot"] != float64(3) {
		t.Fatal(current.Command.Arguments)
	}
	mem := memory.Service{Store: s, Model: p, Config: c, Epoch: epoch}
	v, e := mem.RefreshSlot(ctx, 3, epoch.Add(96*time.Hour))
	if e != nil || v.Slot != 3 {
		t.Fatal("command slot recalculated from clock", v, e)
	}
	ok, e := s.VerifyTask(ctx, current.TaskID, "")
	if e != nil || !ok {
		t.Fatal("exact target failed", ok, e)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM request_receipt WHERE principal_id='slot-controller'").Scan(&n)
	if n != 2 {
		t.Fatal("missing durable receipts", n)
	}
}
func TestEpochDriftRejectedByDurableObject(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	epoch := time.Now()
	m := memory.Service{Store: s, Model: p, Config: c, Epoch: epoch}
	if _, e := m.RefreshSlot(ctx, 0, epoch); e != nil {
		t.Fatal(e)
	}
	other := memory.Service{Store: s, Model: p, Config: c, Epoch: epoch.Add(-48 * time.Hour)}
	if _, e := other.Refresh(ctx, epoch.Add(24*time.Hour)); e == nil {
		t.Fatal("epoch changed silently")
	}
}
