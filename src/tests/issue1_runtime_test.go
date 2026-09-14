package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/executor"
	"secretarysimplified/ingest"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"secretarysimplified/transport"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type issueTimedRemote struct {
	executor.RemoteCore
	timeout time.Duration
}

func (b issueTimedRemote) Execute(ctx context.Context, r contract.JobRun, p contract.ExecutionPermit) (contract.ExecutorReceipt, error) {
	c, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()
	return b.RemoteCore.Execute(c, r, p)
}

type issueBlockedModel struct {
	inner   model.Client
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (m *issueBlockedModel) Encode(r model.Request) ([]byte, error) { return m.inner.Encode(r) }
func (m *issueBlockedModel) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	m.calls.Add(1)
	select {
	case m.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	if m.release != nil {
		<-m.release
	}
	return model.Result{}, ctx.Err()
}
func issueSocket(t *testing.T, h http.Handler) executor.RemoteCore {
	t.Helper()
	dir, e := os.MkdirTemp("/tmp", "ss-issue1-")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	socket := filepath.Join(dir, "core.sock")
	go func() { done <- transport.Serve(ctx, socket, "synthetic-test-token", h) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("socket server did not stop")
		}
		os.RemoveAll(dir)
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, e = os.Stat(socket); e == nil {
			return executor.RemoteCore{Client: transport.Client{Socket: socket, Token: "synthetic-test-token"}}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("socket not ready")
	return executor.RemoteCore{}
}
func issueMemoryRun(t *testing.T, s *store.Store, c config.Config) contract.JobRun {
	t.Helper()
	epoch, e := time.Parse(time.RFC3339Nano, c.Epoch)
	if e != nil {
		t.Fatal(e)
	}
	g := probeGrant(t, s, []string{"memory.refresh"}, []string{})
	run, e := s.ScheduleMemorySlot(context.Background(), epoch, time.Now(), g)
	if e != nil {
		t.Fatal(e)
	}
	return run
}
func issueAwaitWork(t *testing.T, s *store.Store, id string) contract.CoreWork {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		w, e := s.GetCoreWork(context.Background(), id)
		if e == nil && w.Receipt != nil {
			return w
		}
		time.Sleep(5 * time.Millisecond)
	}
	w, e := s.GetCoreWork(context.Background(), id)
	t.Fatalf("no durable final CoreWork: %+v %v", w, e)
	return w
}
func TestIssue1CancelledSocketWorkSettlesAfterRunnerUnknown(t *testing.T) {
	s, dir := runtimeDB(t)
	c := config.Default(dir)
	m := &issueBlockedModel{inner: model.New(c), started: make(chan struct{}, 1), release: make(chan struct{})}
	run := issueMemoryRun(t, s, c)
	svc := core.Service{Store: s, Config: c, Model: m}
	remote := issueSocket(t, svc.InternalHandler())
	runner := executor.New(s, &executor.Options{Core: issueTimedRemote{RemoteCore: remote, timeout: time.Second}})
	finished := make(chan error, 1)
	go func() { _, e := runner.Step(context.Background()); finished <- e }()
	select {
	case <-m.started:
	case err := <-finished:
		t.Fatal("Runner returned before fake model", err)
	case <-time.After(5 * time.Second):
		t.Fatal("model not started")
	}
	if e := <-finished; e == nil {
		t.Fatal("expected timed out HTTP")
	}
	current, e := s.GetRun(context.Background(), run.ID)
	if e != nil || current.State != "RESULT_UNKNOWN" {
		t.Fatal(current, e)
	}
	var activeBefore int
	if e = s.DB.QueryRow(`SELECT json_extract(b.payload_json,'$.active_ms_used') FROM root_budget b JOIN task t ON t.root_id=b.root_id WHERE t.id=?`, run.TaskID).Scan(&activeBefore); e != nil {
		t.Fatal(e)
	}
	close(m.release)
	work := issueAwaitWork(t, s, run.ID)
	if work.State != "FAILED" || work.Receipt.EffectObserved {
		t.Fatal(work)
	}
	if _, e = runner.Step(context.Background()); e != nil {
		t.Fatal(e)
	}
	current, e = s.GetRun(context.Background(), run.ID)
	if e != nil || current.State != "QUEUED" || current.AttemptNo != 1 {
		t.Fatal(current, e)
	}
	var activeAfter int
	if e = s.DB.QueryRow(`SELECT json_extract(b.payload_json,'$.active_ms_used') FROM root_budget b JOIN task t ON t.root_id=b.root_id WHERE t.id=?`, run.TaskID).Scan(&activeAfter); e != nil || activeAfter != activeBefore {
		t.Fatal("active_ms settled twice", activeBefore, activeAfter, e)
	}
	due, e := time.Parse(time.RFC3339Nano, current.ScheduledFor)
	if e != nil || time.Until(due) < 4*time.Minute {
		t.Fatal("memory retry backoff lost", due, e)
	}
	if m.calls.Load() != 1 {
		t.Fatal("query redispatched", m.calls.Load())
	}
}

// AUD04 failures occur before the adapter touches source state or cursor.
func TestIssue1OversizedSourceSocketDoesNotAdvanceCursor(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	c := config.Default(dir)
	source := contract.NewID()
	file := filepath.Join(dir, "oversized.json")
	f, e := os.Create(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(64 << 20); e != nil {
		t.Fatal(e)
	}
	f.Close()
	adapter := ingest.Service{Store: s, QuarantineDir: filepath.Join(dir, "quarantine")}
	if _, e = adapter.Sync(ctx, source, []ingest.FixtureRecord{{ExternalID: "retained", Version: "v1", Value: json.RawMessage(`{"title":"synthetic retained","domain":"work","due_at":null,"status":"OPEN"}`)}}); e != nil {
		t.Fatal(e)
	}
	var originalSource string
	var originalEvents int
	s.DB.QueryRow(`SELECT payload_json FROM source_state WHERE id=?`, source).Scan(&originalSource)
	s.DB.QueryRow(`SELECT count(*) FROM change_event WHERE event_type='source.synced'`).Scan(&originalEvents)
	c.SourceConfigs = []config.SourceConfig{{ID: source, FixturePath: file}}
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"source.sync"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{source}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e = s.PutGrant(ctx, g); e != nil {
		t.Fatal(e)
	}
	cmd := contract.Command{SchemaVersion: 1, OperationKey: "source", Capability: "source.sync", CapabilityVersion: 1, Arguments: map[string]any{"source_id": source}, ExpectedRevisions: []contract.ReadRef{}, Extensions: d12Ext("SYNTHETIC")}
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g.ID)
	if e != nil {
		t.Fatal(e)
	}
	svc := core.Service{Store: s, Model: model.New(c), Config: c}
	remote := issueSocket(t, svc.InternalHandler())
	runner := executor.New(s, &executor.Options{Core: remote})
	if _, e = runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	work := issueAwaitWork(t, s, run.ID)
	if work.State != "FAILED" || work.Receipt.EffectObserved || work.Receipt.ErrorCode == nil || *work.Receipt.ErrorCode != "INPUT_TOO_LARGE" {
		t.Fatal(work)
	}
	var count int
	for _, table := range []string{"source_state", "source_record"} {
		if e = s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&count); e != nil || count != 1 {
			t.Fatal(table, count, e)
		}
	}
	var afterSource string
	var afterEvents int
	s.DB.QueryRow(`SELECT payload_json FROM source_state WHERE id=?`, source).Scan(&afterSource)
	s.DB.QueryRow(`SELECT count(*) FROM change_event WHERE event_type='source.synced'`).Scan(&afterEvents)
	if afterSource != originalSource || afterEvents != originalEvents {
		t.Fatal("source cursor or success events changed")
	}
}

func TestIssue1QueuedBackgroundCancellationNeverCallsModelHTTP(t *testing.T) {
	s, dir := runtimeDB(t)
	c := config.Default(dir)
	var calls atomic.Int32
	entered := make(chan struct{}, 1)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		select {
		case entered <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer fake.Close()
	c.Model.Profile = "opencode-go"
	c.Model.Endpoint = fake.URL
	c.Model.SecretRef = filepath.Join(dir, "fake-key")
	if e := os.WriteFile(c.Model.SecretRef, []byte("local-test-placeholder-not-a-real-key"), 0600); e != nil {
		t.Fatal(e)
	}
	provider := model.New(c)
	frontTurn, e := s.AcceptInput(context.Background(), input(contract.NewID()), 100)
	if e != nil {
		t.Fatal(e)
	}
	builder := ctxbuild.Builder{Store: s, Model: provider, Config: c}
	frontRequest, _, e := builder.Build(context.Background(), frontTurn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	frontCtx, frontCancel := context.WithCancel(context.Background())
	defer frontCancel()
	frontDone := make(chan error, 1)
	go func() {
		defer close(frontDone)
		_, err := provider.Generate(frontCtx, frontRequest)
		frontDone <- err
	}()
	select {
	case <-entered:
	case err := <-frontDone:
		t.Fatal("foreground failed before HTTP", err)
	case <-time.After(5 * time.Second):
		t.Fatal("foreground fake HTTP not entered")
	}
	run := issueMemoryRun(t, s, c)
	svc := core.Service{Store: s, Config: c, Model: provider}
	remote := issueSocket(t, svc.InternalHandler())
	runner := executor.New(s, &executor.Options{Core: issueTimedRemote{RemoteCore: remote, timeout: time.Second}})
	if _, e := runner.Step(context.Background()); e == nil {
		t.Fatal("expected cancelled queued work")
	}
	work := issueAwaitWork(t, s, run.ID)
	if work.Receipt.Status != "FAILED" || work.Receipt.EffectObserved {
		t.Fatal(work)
	}
	if calls.Load() != 1 {
		t.Fatal("queued background reached HTTP", calls.Load())
	}
	frontCancel()
	<-frontDone
	if _, e := runner.Step(context.Background()); e != nil {
		t.Fatal(e)
	}
	got, e := s.GetRun(context.Background(), run.ID)
	if e != nil || got.State != "QUEUED" || got.AttemptNo != 1 {
		t.Fatal(got, e)
	}
}

type issueCountingModel struct {
	model.Client
	calls atomic.Int32
}

func (m *issueCountingModel) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	m.calls.Add(1)
	return m.Client.Generate(ctx, r)
}

func TestIssue1CommittedMemoryCrashGapReconcilesAfterReopen(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	c := config.Default(dir)
	m := &issueCountingModel{Client: model.New(c)}
	run := issueMemoryRun(t, s, c)
	if _, e := s.DB.Exec(`CREATE TRIGGER block_work_finish BEFORE UPDATE ON core_work BEGIN SELECT RAISE(ABORT,'injected settlement failure'); END`); e != nil {
		t.Fatal(e)
	}
	svc := core.Service{Store: s, Config: c, Model: m}
	remote := issueSocket(t, svc.InternalHandler())
	runner := executor.New(s, &executor.Options{Core: remote})
	if _, e := runner.Step(ctx); e == nil {
		t.Fatal("expected injected finish failure")
	}
	var n int
	if e := s.DB.QueryRow(`SELECT count(*) FROM consciousness_snapshot`).Scan(&n); e != nil || n != 1 {
		t.Fatal("snapshot was not committed", n, e)
	}
	work, e := s.GetCoreWork(ctx, run.ID)
	if e != nil || work.State != "RUNNING" || work.Receipt != nil {
		t.Fatal(work, e)
	}
	if _, e = s.DB.Exec(`DROP TRIGGER block_work_finish`); e != nil {
		t.Fatal(e)
	}
	current, e := s.GetRun(ctx, run.ID)
	if e != nil {
		t.Fatal(e)
	}
	var snapshotRaw string
	var slot int
	if e = s.DB.QueryRow(`SELECT slot,payload_json FROM consciousness_snapshot`).Scan(&slot, &snapshotRaw); e != nil {
		t.Fatal(e)
	}
	// Only the exact slot is proof: a future valid snapshot cannot satisfy this run.
	var snapshot contract.ConsciousnessState
	if e = contract.Decode("ConsciousnessState", []byte(snapshotRaw), &snapshot); e != nil {
		t.Fatal(e)
	}
	snapshot.Slot++
	future, _ := json.Marshal(snapshot)
	if _, e = s.DB.Exec(`UPDATE consciousness_snapshot SET slot=?,payload_json=?`, snapshot.Slot, string(future)); e != nil {
		t.Fatal(e)
	}
	if receipt, err := s.ReconcileMemoryWork(ctx, current); err != nil || receipt != nil {
		t.Fatal("future slot accepted", receipt, err)
	}
	if _, e = s.DB.Exec(`UPDATE consciousness_snapshot SET slot=?,payload_json=?`, slot, snapshotRaw); e != nil {
		t.Fatal(e)
	}
	old := current
	old.FencingToken++
	if _, err := s.ReconcileMemoryWork(ctx, old); err == nil {
		t.Fatal("wrong fence accepted")
	}
	// A new Store and authenticated socket represent restarted Core state; the old handler is idle.
	fresh, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close()
	restarted := core.Service{Store: fresh, Config: c, Model: m}
	next := issueSocket(t, restarted.InternalHandler())
	runner = executor.New(fresh, &executor.Options{Core: next})
	if _, e = runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	task, e := fresh.RuntimeTask(ctx, run.TaskID)
	if e != nil || task.State != "SUCCEEDED" {
		t.Fatal(task, e)
	}
	work = issueAwaitWork(t, fresh, run.ID)
	if work.State != "SUCCEEDED" || !strings.HasPrefix(work.Receipt.ReceiptKey, "exact-slot-committed:") {
		t.Fatal(work)
	}
	if m.calls.Load() != 1 {
		t.Fatal("reconcile repeated model", m.calls.Load())
	}
}

func TestIssue1LostBriefingResponseUsesAtomicArtifactWithoutModelReplay(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	c := config.Default(dir)
	m := &issueCountingModel{Client: model.New(c)}
	cmd := d08Command(t, s, "briefing.build")
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, probeGrant(t, s, []string{"briefing.build"}, []string{}))
	if e != nil {
		t.Fatal(e)
	}
	svc := core.Service{Store: s, Config: c, Model: m}
	inner := svc.InternalHandler()
	remote := issueSocket(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			inner.ServeHTTP(w, r)
			return
		}
		recorder := httptest.NewRecorder()
		inner.ServeHTTP(recorder, r)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	runner := executor.New(s, &executor.Options{Core: remote})
	if _, e = runner.Step(ctx); e == nil {
		t.Fatal("response was not lost")
	}
	work := issueAwaitWork(t, s, run.ID)
	if work.State != "SUCCEEDED" || len(work.Receipt.Artifacts) != 1 {
		t.Fatal(work)
	}
	fresh, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close()
	restarted := core.Service{Store: fresh, Config: c, Model: m}
	next := issueSocket(t, restarted.InternalHandler())
	runner = executor.New(fresh, &executor.Options{Core: next})
	if _, e = runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	task, e := fresh.RuntimeTask(ctx, run.TaskID)
	if e != nil || task.State != "SUCCEEDED" {
		t.Fatal(task, e)
	}
	if m.calls.Load() != 1 {
		t.Fatal("reconcile repeated model", m.calls.Load())
	}
}

func TestIssue1DuplicateSocketPostAndLateCancelledReceipt(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	c := config.Default(dir)
	m := &issueBlockedModel{inner: model.New(c), started: make(chan struct{}, 1), release: make(chan struct{})}
	registered := issueMemoryRun(t, s, c)
	claimed, e := s.ClaimRun(ctx, "test-worker", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	permit, e := s.DispatchRun(ctx, claimed, "test-worker", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.GetRun(ctx, registered.ID)
	if e != nil {
		t.Fatal(e)
	}
	svc := core.Service{Store: s, Config: c, Model: m}
	remote := issueSocket(t, svc.InternalHandler())
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := remote.Execute(callCtx, run, permit); done <- err }()
	select {
	case <-m.started:
	case <-time.After(5 * time.Second):
		t.Fatal("model not entered")
	}
	if _, e = remote.Execute(ctx, run, permit); e == nil {
		t.Fatal("duplicate pending work accepted")
	}
	if m.calls.Load() != 1 {
		t.Fatal("duplicate model", m.calls.Load())
	}
	if e = s.CancelTask(ctx, run.TaskID); e != nil {
		t.Fatal(e)
	}
	cancel()
	<-done
	close(m.release)
	work := issueAwaitWork(t, s, run.ID)
	if work.Receipt.EffectObserved || work.Receipt.Extensions["runtime.cancellation"] == nil {
		t.Fatal(work)
	}
	if _, e = executor.New(s, &executor.Options{Core: remote}).Step(ctx); e != nil {
		t.Fatal(e)
	}
	task, e := s.RuntimeTask(ctx, run.TaskID)
	if e != nil || task.State != "CANCELLED" {
		t.Fatal("cancelled task resurrected", task, e)
	}
	// A stale generation cannot turn this durable failure into a fresh retry proof.
	newer := run
	newer.AttemptNo++
	newer.FencingToken++
	if _, known, e := remote.Query(ctx, newer); e != nil || known {
		t.Fatal("old failure rebound", known, e)
	}
}
