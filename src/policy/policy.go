// Package policy exposes the transaction-bound authority check for local commits.
package policy

import (
	"context"
	"database/sql"
	"secretarysimplified/store"
	"time"
)

func ConsumePermitTx(ctx context.Context, tx *sql.Tx, permitID, proposalHash, entityID, predicate, operation string, now time.Time) error {
	return store.ConsumePermitTx(ctx, tx, permitID, proposalHash, entityID, predicate, operation, now)
}
