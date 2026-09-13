// Package world is the sole authority service for admitting facts.
package world

import (
	"context"
	"database/sql"

	"secretarysimplified/contract"
	"secretarysimplified/policy"
	"secretarysimplified/store"
	"time"
)

type Service struct{ Store *store.Store }

func (s *Service) Commit(ctx context.Context, p contract.WorldUpdateProposal, permitID string, now time.Time) (contract.WorldFact, error) {
	hash, e := contract.ValueHash(p)
	if e != nil {
		return contract.WorldFact{}, e
	}
	return s.Store.CommitWorld(ctx, p, func(tx *sql.Tx) error {
		return policy.ConsumePermitTx(ctx, tx, permitID, hash, p.EntityID, p.Predicate, p.Operation, now)
	})
}
