package main

import (
	"context"
	"database/sql"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"secretarysimplified/world"
	"time"
)

// WorldSeed is frozen synthetic authority setup, not an evaluated model action.
type WorldSeed struct {
	FactID           string          `json:"fact_id"`
	Operation        string          `json:"operation"`
	ExpectedRevision int             `json:"expected_revision"`
	Value            *map[string]any `json:"value"`
	EvidenceText     string          `json:"evidence_text"`
}

func seedWorld(ctx context.Context, s *store.Store, v WorldSeed) error {
	entity := "00000000-0000-4000-8000-000000000100"
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"world.update"}, Scope: map[string]any{"entity_ids": []string{entity}, "predicates": []string{"master.preference"}, "operations": []string{"ASSERT", "RETRACT"}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e := s.PutGrant(ctx, g); e != nil {
		return e
	}
	obj, e := s.PutObject(ctx, []byte(v.EvidenceText), "text/plain", "SYNTHETIC")
	if e != nil {
		return e
	}
	p := contract.WorldUpdateProposal{SchemaVersion: 1, ID: contract.NewID(), RequestID: contract.NewID(), EntityID: entity, Predicate: "master.preference", Operation: v.Operation, FactID: v.FactID, ExpectedRevision: v.ExpectedRevision, Value: v.Value, Evidence: []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, OriginID: obj.ID, Locator: "full", DataClass: "SYNTHETIC"}}, Basis: "MASTER_EXPLICIT", PolicyRevision: 1, Reason: "Frozen synthetic authority fixture; model evaluates resulting read semantics only", Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
	if e = s.Write(ctx, func(tx *sql.Tx) error { return s.PutProposalTx(ctx, tx, p) }); e != nil {
		return e
	}
	cmd := contract.Command{SchemaVersion: 1, OperationKey: "world_fixture", Capability: "world.update", CapabilityVersion: 1, Arguments: map[string]any{"proposal_id": p.ID}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}}
	criteria := []contract.Criterion{{ID: contract.NewID(), Kind: "world_revision_matches", Expected: map[string]any{"fact_id": p.FactID, "revision": p.ExpectedRevision + 1}, EvidencePolicy: "WorldCommitService"}}
	if _, e = s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g.ID); e != nil {
		return e
	}
	now := time.Now().Add(time.Second)
	run, e := s.ClaimRun(ctx, "world-fixture", now)
	if e != nil {
		return e
	}
	permit, e := s.DispatchRun(ctx, run, "world-fixture", now)
	if e != nil {
		return e
	}
	svc := world.Service{Store: s}
	_, e = svc.Commit(ctx, p, permit.ID, now)
	return e
}
