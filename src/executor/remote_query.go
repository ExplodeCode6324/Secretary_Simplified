package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/contract"
	"time"
)

// Query treats old Core receipts as evidence and produces a fresh reconciliation
// receipt for the current attempt/fence; it never redispatches saved work.
func (c RemoteCore) Query(ctx context.Context, run contract.JobRun) (contract.ExecutorReceipt, bool, error) {
	var out contract.ExecutorReceipt
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ctx = bounded
	v, status, e := c.Client.Call(ctx, "GET", "/internal/v1/work/"+run.ID, nil)
	if e != nil {
		return out, false, e
	}
	if status == 400 || status == 404 {
		return out, false, nil
	}
	if status != 200 {
		return out, false, errors.New("CORE_QUERY_REJECTED")
	}
	raw, e := json.Marshal(v.Result)
	if e != nil {
		return out, false, e
	}
	var work contract.CoreWork
	if e = contract.Decode("CoreWork", raw, &work); e != nil {
		return out, false, e
	}
	hash, e := contract.ValueHash(run.Command)
	if e != nil {
		return out, false, e
	}
	if work.RunID != run.ID || work.CommandHash != hash {
		return out, false, errors.New("CORE_QUERY_IDENTITY_MISMATCH")
	}
	if work.Receipt == nil || (work.State != "SUCCEEDED" && work.State != "FAILED" && work.State != "CANCELLED") {
		return out, false, nil
	}
	old := work.Receipt
	if old.RunID != run.ID || old.AttemptNo != work.AttemptNo || old.FencingToken != work.FencingToken || old.Status != work.State {
		return out, false, errors.New("CORE_QUERY_RECEIPT_MISMATCH")
	}
	out = *old
	out.ID = contract.NewID()
	out.AttemptNo = run.AttemptNo
	out.FencingToken = run.FencingToken
	out.ReceiptKey = fmt.Sprintf("reconciled-core:%s:%d", old.ID, run.FencingToken)
	out.ReceivedAt = contract.Now()
	return out, true, nil
}
func (r *Runner) reconcile(ctx context.Context, coreReady bool) (bool, error) {
	if !coreReady {
		return false, nil
	}
	query, ok := r.Options.Core.(interface {
		Query(context.Context, contract.JobRun) (contract.ExecutorReceipt, bool, error)
	})
	if !ok {
		return false, nil
	}
	runs, e := r.Store.RuntimeUnknownRuns(ctx)
	if e != nil {
		return false, e
	}
	for _, run := range runs {
		switch run.Command.Capability {
		case "notify.local", "artifact.write", "alarm.play", "alarm.stop", "alarm.snooze":
			continue
		}
		receipt, known, e := query.Query(ctx, run)
		if e != nil {
			return false, e
		}
		if !known {
			continue
		}
		if e = r.Store.RecordReceipt(ctx, receipt); e != nil {
			return false, e
		}
		if receipt.Status == "SUCCEEDED" {
			if _, e = r.Store.VerifyTask(ctx, run.TaskID, r.Options.ArtifactDir); e != nil {
				return false, e
			}
		}
		return true, nil
	}
	return false, nil
}
