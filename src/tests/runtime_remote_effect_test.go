package tests

import (
	"context"
	"fmt"
	"secretarysimplified/contract"
	"secretarysimplified/executor"
	"secretarysimplified/platform"
	"secretarysimplified/store"
	"testing"
	"time"
)

type failedEffectBridge struct {
	effect bool
	calls  int
}

func (b *failedEffectBridge) Execute(ctx context.Context, r contract.JobRun, p contract.ExecutionPermit) (contract.ExecutorReceipt, error) {
	b.calls++
	code := "fixture_failure"
	return contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: r.ID, AttemptNo: r.AttemptNo, FencingToken: r.FencingToken, ReceiptKey: fmt.Sprintf("attempt-%d-final", r.AttemptNo), Status: "FAILED", EffectObserved: b.effect, ErrorCode: &code, Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Now(), Extensions: map[string]any{}}, nil
}
func TestRemoteFailurePreservesEffectAndRetryCeiling(t *testing.T) {
	for _, effect := range []bool{false, true} {
		t.Run(fmt.Sprint(effect), func(t *testing.T) {
			s, _ := runtimeDB(t)
			g := probeGrant(t, s, []string{"briefing.build"}, []string{})
			cmd := d08Command(t, s, "briefing.build")
			criteria, _ := store.DeriveCriteria(cmd)
			run, e := s.RegisterImmediate(context.Background(), contract.NewID(), contract.NewID(), cmd, criteria, g)
			if e != nil {
				t.Fatal(e)
			}
			clock := &platform.ManualClock{T: time.Now().Add(time.Second)}
			bridge := &failedEffectBridge{effect: effect}
			r := executor.New(s, &executor.Options{Clock: clock, Core: bridge})
			for i := 0; i < 4; i++ {
				if _, e = r.Step(context.Background()); e != nil {
					t.Fatal(e)
				}
				clock.Advance(10 * time.Second)
			}
			want := 3
			if effect {
				want = 1
			}
			if bridge.calls != want {
				t.Fatal("wrong retry calls", bridge.calls, want)
			}
			got, _ := s.GetRun(context.Background(), run.ID)
			if got.State != "FAILED" || got.AttemptNo != want {
				t.Fatal(got.State, got.AttemptNo)
			}
			rows, e := s.DB.Query(`SELECT payload_json FROM executor_receipt WHERE run_id=?`, run.ID)
			if e != nil {
				t.Fatal(e)
			}
			defer rows.Close()
			for rows.Next() {
				var raw string
				rows.Scan(&raw)
				var receipt contract.ExecutorReceipt
				if e = contract.Decode("ExecutorReceipt", []byte(raw), &receipt); e != nil || receipt.EffectObserved != effect {
					t.Fatal("overwritten remote effect", receipt.EffectObserved, e)
				}
			}
		})
	}
}
