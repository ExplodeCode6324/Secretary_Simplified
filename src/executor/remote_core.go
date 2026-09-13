package executor

import (
	"context"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
	"secretarysimplified/transport"
)

type RemoteCore struct{ Client transport.Client }

func (c RemoteCore) Execute(ctx context.Context, run contract.JobRun, permit contract.ExecutionPermit) (contract.ExecutorReceipt, error) {
	var receipt contract.ExecutorReceipt
	v, status, e := c.Client.Call(ctx, "POST", "/internal/v1/work", map[string]any{"run": run, "permit": permit})
	if e != nil {
		return receipt, e
	}
	if status >= 400 {
		return receipt, errors.New("CORE_WORK_REJECTED")
	}
	b, e := json.Marshal(v.Result)
	if e == nil {
		e = contract.Decode("ExecutorReceipt", b, &receipt)
	}
	return receipt, e
}
