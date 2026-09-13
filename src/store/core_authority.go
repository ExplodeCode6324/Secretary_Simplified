package store

import (
	"context"
	"database/sql"
	"errors"
	"secretarysimplified/contract"
	"time"
)

func CheckCoreAuthorityTx(ctx context.Context, tx *sql.Tx, grantID, principal string, now time.Time) (contract.AuthorizationGrant, error) {
	var g contract.AuthorizationGrant
	var raw []byte
	if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM authorization_grant WHERE id=?", grantID).Scan(&raw); e != nil {
		return g, errors.New("AUTHORIZATION_DENIED")
	}
	if e := contract.Decode("AuthorizationGrant", raw, &g); e != nil {
		return g, e
	}
	expires, e := time.Parse(time.RFC3339Nano, g.ExpiresAt)
	if e != nil || g.Revoked || g.PrincipalID != principal || g.PolicyRevision != 1 || !now.Before(expires) {
		return g, errors.New("AUTHORIZATION_DENIED")
	}
	return g, nil
}
func (s *Store) GrantedCapabilities(ctx context.Context, grantID, principal string, now time.Time) ([]string, error) {
	out := []string{}
	if grantID == "" {
		return out, nil
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	g, e := CheckCoreAuthorityTx(ctx, tx, grantID, principal, now)
	if e != nil {
		return out, nil
	}
	return g.CapabilityIDs, nil
}
func ChargeCoreActionTx(ctx context.Context, tx *sql.Tx, root string) error {
	return chargeBudgetTx(ctx, tx, root, "actions", 1)
}
