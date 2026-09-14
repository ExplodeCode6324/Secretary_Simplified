package tests

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestAcceptanceA12CancellationFourStages(t *testing.T) {
	for _, stage := range []string{"before_claim", "after_claim_before_permit", "after_dispatch_before_effect", "after_effect_before_receipt"} {
		t.Run(stage, func(t *testing.T) {
			s, _ := runtimeDB(t)
			ctx := context.Background()
			g := runtimeGrant(t, s)
			run := runtimeRegister(t, s, g)
			now := time.Now().Add(time.Second)
			var claimed contract.JobRun
			var e error
			if stage != "before_claim" {
				claimed, e = s.ClaimRun(ctx, "cancel-worker", now)
				if e != nil {
					t.Fatal(e)
				}
			}
			if stage == "after_dispatch_before_effect" || stage == "after_effect_before_receipt" {
				if _, e = s.DispatchRun(ctx, claimed, "cancel-worker", now); e != nil {
					t.Fatal(e)
				}
			}
			if stage == "after_effect_before_receipt" {
				if e = s.RecordNotification(ctx, claimed); e != nil {
					t.Fatal(e)
				}
			}
			if e = s.CancelTask(ctx, run.TaskID); e != nil {
				t.Fatal(e)
			}
			switch stage {
			case "before_claim":
				if _, e = s.ClaimRun(ctx, "late", now); e != sql.ErrNoRows {
					t.Fatal(e)
				}
			case "after_claim_before_permit":
				if _, e = s.DispatchRun(ctx, claimed, "cancel-worker", now); e == nil {
					t.Fatal("cancelled permit issued")
				}
			case "after_dispatch_before_effect":
				if e = s.RecordNotification(ctx, claimed); e == nil {
					t.Fatal("cancelled effect issued")
				}
			case "after_effect_before_receipt":
				receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: run.ID, AttemptNo: claimed.AttemptNo, FencingToken: claimed.FencingToken, ReceiptKey: "late-proof", Status: "SUCCEEDED", EffectObserved: true, Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Now(), Extensions: map[string]any{}}
				if e = s.RecordReceipt(ctx, receipt); e != nil {
					t.Fatal(e)
				}
				var raw string
				s.DB.QueryRow(`SELECT payload_json FROM executor_receipt WHERE id=?`, receipt.ID).Scan(&raw)
				var got contract.ExecutorReceipt
				contract.Decode("ExecutorReceipt", []byte(raw), &got)
				if !got.EffectObserved || got.Extensions["runtime.cancellation"] == nil {
					t.Fatal("effect race hidden")
				}
			}
			var n int
			s.DB.QueryRow(`SELECT count(*) FROM notification`).Scan(&n)
			want := 0
			if stage == "after_effect_before_receipt" {
				want = 1
			}
			if n != want {
				t.Fatal("effects", n, want)
			}
			task, _ := s.RuntimeTask(ctx, run.TaskID)
			if task.State != "CANCELLED" {
				t.Fatal(task.State)
			}
		})
	}
}

func TestAcceptanceA13ProcessExitsAfterEffectBeforeReceipt(t *testing.T) {
	if dir := os.Getenv("SECRETARY_CRASH_FIXTURE_DIR"); dir != "" {
		s, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
		if e != nil {
			t.Fatal(e)
		}
		ctx := context.Background()
		now := time.Now()
		run, e := s.ClaimRun(ctx, "crash-child", now)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.DispatchRun(ctx, run, "crash-child", now); e != nil {
			t.Fatal(e)
		}
		if e = s.RecordNotification(ctx, run); e != nil {
			t.Fatal(e)
		}
		os.Exit(0)
	}
	s, dir := runtimeDB(t)
	g := runtimeGrant(t, s)
	run := runtimeRegister(t, s, g)
	s.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestAcceptanceA13ProcessExitsAfterEffectBeforeReceipt$")
	child.Env = append(os.Environ(), "SECRETARY_CRASH_FIXTURE_DIR="+dir)
	if out, e := child.CombinedOutput(); e != nil {
		t.Fatalf("child %v %s", e, out)
	}
	s, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	var count int
	s.DB.QueryRow(`SELECT count(*) FROM executor_receipt`).Scan(&count)
	if count != 0 {
		t.Fatal("child wrote receipt")
	}
	if _, e = s.ClaimRun(ctx, "recovery", time.Now().Add(time.Minute)); e != sql.ErrNoRows {
		t.Fatal("unknown was redispatched", e)
	}
	got, _ := s.GetRun(ctx, run.ID)
	if got.State != "RESULT_UNKNOWN" {
		t.Fatal(got.State)
	}
	if ok, e := s.ReconcileLocal(ctx, run.ID, dir); e != nil || !ok {
		t.Fatal(ok, e)
	}
	s.DB.QueryRow(`SELECT count(*) FROM notification`).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate notification", count)
	}
	got, _ = s.GetRun(ctx, run.ID)
	if got.AttemptNo != 1 || got.State != "SUCCEEDED" {
		t.Fatal(got.State, got.AttemptNo)
	}
}

func TestAcceptanceA18AllBudgetFieldsSurviveReopen(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	root := contract.NewID()
	limits := map[string]int{"model_calls": 8, "retrievals": 3, "actions": 10, "replans": 2, "output_tokens": 16000, "active_ms": 300000}
	for kind, n := range limits {
		if e := s.ChargeBudget(ctx, root, kind, n); e != nil {
			t.Fatal(kind, e)
		}
	}
	s.Close()
	s, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for kind := range limits {
		if e = s.ChargeBudget(ctx, root, kind, 1); e == nil {
			t.Fatal("budget reset", kind)
		}
	}
	if e = s.ChargeBudget(ctx, contract.NewID(), "model_calls", 1); e != nil {
		t.Fatal("independent root blocked", e)
	}
}
