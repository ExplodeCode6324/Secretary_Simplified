// Package executor implements the independent persistent local Runner.
package executor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/platform"
	"secretarysimplified/store"
	"sync"
	"time"
)

type CoreBridge interface {
	Execute(context.Context, contract.JobRun, contract.ExecutionPermit) (contract.ExecutorReceipt, error)
}
type Options struct {
	ExecutorID, ArtifactDir string
	Clock                   platform.Clock
	Core                    CoreBridge
	Replay                  bool
	Frozen                  func() bool
}
type Runner struct {
	Store   *store.Store
	Options Options
	mu      sync.Mutex
	active  map[string]context.CancelFunc
}

func New(s *store.Store, o *Options) *Runner {
	v := Options{ExecutorID: "local-runner", ArtifactDir: filepath.Join(s.ObjectsDir, "artifacts"), Clock: platform.RealClock{}}
	if o != nil {
		v = *o
		if v.Clock == nil {
			v.Clock = platform.RealClock{}
		}
		if v.ExecutorID == "" {
			v.ExecutorID = "local-runner"
		}
		if v.ArtifactDir == "" {
			v.ArtifactDir = filepath.Join(s.ObjectsDir, "artifacts")
		}
	}
	return &Runner{Store: s, Options: v, active: map[string]context.CancelFunc{}}
}
func (r *Runner) Step(ctx context.Context) (bool, error) {
	if r.Frozen() {
		return false, nil
	}
	coreReady := r.Options.Core != nil
	if ready, ok := r.Options.Core.(interface{ Ready(context.Context) bool }); ok {
		coreReady = ready.Ready(ctx)
	}
	run, e := r.Store.ClaimRunReady(ctx, r.Options.ExecutorID, r.Options.Clock.Now(), coreReady)
	if e == sql.ErrNoRows {
		return r.reconcile(ctx, coreReady)
	}
	if e != nil {
		return false, e
	}
	permit, e := r.Store.DispatchRun(ctx, run, r.Options.ExecutorID, r.Options.Clock.Now())
	if e != nil {
		code := e.Error()
		receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: run.ID, AttemptNo: run.AttemptNo, FencingToken: run.FencingToken, ReceiptKey: fmt.Sprintf("attempt-%d-not-dispatched", run.AttemptNo), Status: "FAILED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ErrorCode: &code, ReceivedAt: contract.Timestamp(r.Options.Clock.Now()), Extensions: map[string]any{}}
		if saveErr := r.Store.RecordReceipt(ctx, receipt); saveErr != nil {
			return true, saveErr
		}
		return true, e
	}
	child, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r.mu.Lock()
	r.active[run.TaskID] = cancel
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.active, run.TaskID); r.mu.Unlock() }()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-child.Done():
				return
			case <-ticker.C:
				if r.Store.Heartbeat(child, run.ID, r.Options.ExecutorID, run.FencingToken, r.Options.Clock.Now()) != nil {
					cancel()
					return
				}
			}
		}
	}()
	receipt := contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: run.ID, AttemptNo: run.AttemptNo, FencingToken: run.FencingToken, ReceiptKey: fmt.Sprintf("attempt-%d-final", run.AttemptNo), Status: "SUCCEEDED", Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Timestamp(r.Options.Clock.Now()), Extensions: map[string]any{}}
	localEffect := true
	switch run.Command.Capability {
	case "notify.local":
		e = r.Store.RecordNotification(child, run)
	case "alarm.play", "alarm.stop", "alarm.snooze":
		e = r.Store.MutedAlarm(child, run)
	case "artifact.write":
		relative, _ := run.Command.Arguments["relative_path"].(string)
		content, _ := run.Command.Arguments["content"].(string)
		var path string
		path, e = store.SafeArtifactPath(r.Options.ArtifactDir, relative)
		if e == nil {
			e = r.Store.CheckArtifactScope(child, permit, path)
		}
		if e == nil {
			e = WriteArtifact(r.Options.ArtifactDir, relative, content)
			if e == nil {
				receipt.EffectObserved = true // The file write already committed, even if metadata persistence fails.
				class, classErr := contract.ReadClassification(run.Extensions)
				e = classErr
				if e == nil {
					var object contract.ObjectRef
					object, e = r.Store.PutObject(child, []byte(content), "text/plain", class)
					if e == nil {
						receipt.Artifacts = append(receipt.Artifacts, object)
					}
				}
			}
		}
	default:
		localEffect = false
		if r.Options.Core == nil {
			e = errors.New("CORE_UNAVAILABLE")
		} else {
			remote, remoteErr := r.Options.Core.Execute(child, run, permit)
			e = remoteErr
			if remoteErr == nil {
				if validErr := contract.Validate("ExecutorReceipt", remote); validErr != nil {
					e = validErr
				} else {
					receipt = remote
				}
			}
		}
	}
	if e != nil {
		receipt.Status = "RESULT_UNKNOWN"
		code := e.Error()
		receipt.ErrorCode = &code
	} else if localEffect {
		receipt.EffectObserved = true
	}
	receipt.ReceivedAt = contract.Timestamp(r.Options.Clock.Now())
	if saveErr := r.Store.RecordReceipt(ctx, receipt); saveErr != nil {
		return true, saveErr
	}
	if receipt.Status == "SUCCEEDED" {
		_, e = r.Store.VerifyTask(ctx, run.TaskID, r.Options.ArtifactDir)
	}
	return true, e
}
func (r *Runner) Cancel(ctx context.Context, taskID string) error {
	e := r.Store.CancelTask(ctx, taskID)
	if e == nil {
		r.cancelActive(taskID)
	}
	return e
}
func (r *Runner) Run(ctx context.Context) error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		if _, e := r.Step(ctx); e != nil {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (r *Runner) cancelActive(taskID string) {
	r.mu.Lock()
	cancel := r.active[taskID]
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
