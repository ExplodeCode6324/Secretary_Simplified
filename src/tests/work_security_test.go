package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"testing"
	"time"
)

func TestIncomingPermitBoundToExactRun(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	runtimeRegister(t, s, g)
	a, e := s.ClaimRun(ctx, "worker", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.DispatchRun(ctx, a, "worker", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	fake := a
	fake.ID = contract.NewID()
	raw, _ := json.Marshal(map[string]any{"run": fake, "permit": p})
	w := httptest.NewRecorder()
	(&core.Service{Store: s}).InternalHandler().ServeHTTP(w, httptest.NewRequest("POST", "/internal/v1/work", bytes.NewReader(raw)))
	if w.Code != 403 {
		t.Fatalf("expected403 got%d %s", w.Code, w.Body.String())
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM core_work").Scan(&n)
	if n != 0 {
		t.Fatal("mismatched request left work")
	}
}
func TestStaleWorkerCannotCommitReceipt(t *testing.T) {
	s, _ := runtimeDB(t)
	g := runtimeGrant(t, s)
	ctx := context.Background()
	runtimeRegister(t, s, g)
	a, e := s.ClaimRun(ctx, "worker", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DispatchRun(ctx, a, "worker", time.Now().Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.BeginWork(ctx, a); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("UPDATE job_run SET fencing_token=fencing_token+1 WHERE id=?", a.ID); e != nil {
		t.Fatal(e)
	}
	r := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: a.ID, AttemptNo: a.AttemptNo, FencingToken: a.FencingToken, ReceiptKey: "final", Status: "SUCCEEDED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Now(), Extensions: map[string]any{}}
	if e = s.FinishWork(ctx, a, r); e == nil {
		t.Fatal("stale worker accepted")
	}
	var state string
	s.DB.QueryRow("SELECT state FROM core_work WHERE run_id=?", a.ID).Scan(&state)
	if state != "RUNNING" {
		t.Fatal(state)
	}
}
