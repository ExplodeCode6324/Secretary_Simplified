package diagnostics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"syscall"
	"testing"
	"time"
)

type issue1ReturnedModel struct {
	model.Client
	returned func()
}

func (m issue1ReturnedModel) Generate(_ context.Context, req model.Request) (model.Result, error) {
	out, err := model.Fixture(req)
	m.returned()
	return model.Result{Output: out}, err
}

func TestIssue1CancelledOutputArchiveHasBoundedLockWait(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(filepath.Join(dir, "db"), filepath.Join(dir, "objects"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := config.Default(dir)
	p := model.New(c)
	in := contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: contract.NewID(), PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: "invented cancellation evidence", DataClass: "SYNTHETIC", AttachmentRefs: []contract.ObjectRef{}, Extensions: map[string]any{}}
	turn, err := s.AcceptInput(context.Background(), in, 100)
	if err != nil {
		t.Fatal(err)
	}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, _, err := b.Build(context.Background(), turn, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(s.ObjectsDir, ".publish.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	acquired := make(chan error, 1)
	inner := issue1ReturnedModel{Client: p, returned: func() {
		// The request archive already completed. Another publisher now owns
		// the inode, just as cancellation races with a returned model result.
		err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		acquired <- err
		cancel()
	}}
	r := RecordingModel{Inner: inner, Store: s, Config: c, Dir: filepath.Join(dir, "reports", "model_calls")}
	done := make(chan error, 1)
	go func() { _, err := r.Generate(ctx, req); done <- err }()
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("fake model not reached")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("output archive did not report bounded persistence failure", err)
		}
	case <-time.After(8 * time.Second):
		// Release the owned lock and drain to avoid a leaked test goroutine.
		syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		<-done
		t.Fatal("cancelled output archive waited beyond persistence budget")
	}
}
