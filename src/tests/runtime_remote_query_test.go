package tests

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/executor"
	"secretarysimplified/transport"
	"testing"
	"time"
)

func TestRuntimeRemoteQueryRebindsEvidence(t *testing.T) {
	dir, e := os.MkdirTemp("/tmp", "ss-query-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := runtimeCommand()
	hash, _ := contract.ValueHash(command)
	id := contract.NewID()
	old := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: id, AttemptNo: 1, FencingToken: 1, ReceiptKey: "old-proof", Status: "SUCCEEDED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, EffectObserved: true, ReceivedAt: contract.Now(), Extensions: map[string]any{}}
	work := contract.CoreWork{SchemaVersion: 1, RunID: id, AttemptNo: 1, FencingToken: 1, CommandHash: hash, State: "SUCCEEDED", Receipt: &old, UpdatedAt: contract.Now(), Extensions: map[string]any{}}
	socket := filepath.Join(dir, "core.sock")
	token := "synthetic-internal-token-0000000000"
	done := make(chan error, 1)
	go func() {
		done <- transport.Serve(ctx, socket, token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { transport.Reply(w, 200, "", work, nil) }))
	}()
	defer func() { cancel(); <-done }()
	for i := 0; i < 100; i++ {
		if _, e = os.Stat(socket); e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	remote := executor.RemoteCore{Client: transport.Client{Socket: socket, Token: token}}
	run := contract.JobRun{ID: id, AttemptNo: 2, FencingToken: 3, Command: command}
	got, known, e := remote.Query(ctx, run)
	if e != nil || !known {
		t.Fatalf("query %v %v", known, e)
	}
	if got.ID == old.ID || got.AttemptNo != 2 || got.FencingToken != 3 || got.RunID != id || !got.EffectObserved {
		t.Fatal("old receipt reused rather than reconciled", got)
	}
	run.Command.Arguments["text"] = "tampered"
	if _, _, e = remote.Query(ctx, run); e == nil {
		t.Fatal("foreign command proof accepted")
	}
}
