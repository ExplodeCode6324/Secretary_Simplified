package tests

import (
	"context"
	"database/sql"
	"secretarysimplified/contract"
	"secretarysimplified/world"
	"testing"
	"time"
)

func TestWorldPermitCorrectionAndHistory(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	entity := contract.NewID()
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"world.update"}, Scope: map[string]any{"entity_ids": []string{entity}, "predicates": []string{"master.preference"}, "operations": []string{"ASSERT", "CORRECT", "RETRACT"}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e := s.PutGrant(ctx, g); e != nil {
		t.Fatal(e)
	}
	obj, e := s.PutObject(ctx, []byte("Synthetic Master preference: coffee, then tea, then retracted."), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	value := map[string]any{"key": "drink", "value": "coffee"}
	p := contract.WorldUpdateProposal{SchemaVersion: 1, ID: contract.NewID(), RequestID: contract.NewID(), EntityID: entity, Predicate: "master.preference", Operation: "ASSERT", FactID: contract.NewID(), Value: &value, Evidence: []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, Locator: "full", OriginID: obj.ID, DataClass: "SYNTHETIC"}}, Basis: "MASTER_EXPLICIT", PolicyRevision: 1, Reason: "synthetic test", Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	svc := world.Service{Store: s}
	if _, e = svc.Commit(ctx, p, "", time.Now()); e == nil {
		t.Fatal("missing permit accepted")
	}
	commit := func(p contract.WorldUpdateProposal) contract.WorldFact {
		t.Helper()
		if e := s.Write(ctx, func(tx *sql.Tx) error { return s.PutProposalTx(ctx, tx, p) }); e != nil {
			t.Fatal(e)
		}
		cmd := contract.Command{SchemaVersion: 1, OperationKey: "world", Capability: "world.update", CapabilityVersion: 1, Arguments: map[string]any{"proposal_id": p.ID}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
		criteria := []contract.Criterion{{ID: contract.NewID(), Kind: "world_revision_matches", Expected: map[string]any{"fact_id": p.FactID, "revision": p.ExpectedRevision + 1}, EvidencePolicy: "WorldCommitService"}}
		if _, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g.ID); e != nil {
			t.Fatal(e)
		}
		now := time.Now().Add(time.Second)
		run, e := s.ClaimRun(ctx, "world-test", now)
		if e != nil {
			t.Fatal(e)
		}
		permit, e := s.DispatchRun(ctx, run, "world-test", now)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = svc.Commit(ctx, p, permit.ID, now.Add(time.Minute)); e == nil {
			t.Fatal("expired accepted")
		}
		original := p.EntityID
		p.EntityID = contract.NewID()
		if _, e = svc.Commit(ctx, p, permit.ID, now); e == nil {
			t.Fatal("wrong proposal accepted")
		}
		p.EntityID = original
		v, e := svc.Commit(ctx, p, permit.ID, now)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = svc.Commit(ctx, p, permit.ID, now); e == nil {
			t.Fatal("consumed accepted")
		}
		return v
	}
	v := commit(p)
	if v.Status != "ACTIVE" || v.Revision != 1 {
		t.Fatal(v)
	}
	p.ID = contract.NewID()
	p.RequestID = contract.NewID()
	p.Operation = "CORRECT"
	p.ExpectedRevision = 1
	value = map[string]any{"key": "drink", "value": "tea"}
	p.Value = &value
	v = commit(p)
	if v.Revision != 2 || (*v.Value)["value"] != "tea" {
		t.Fatal(v)
	}
	p.ID = contract.NewID()
	p.RequestID = contract.NewID()
	p.Operation = "RETRACT"
	p.ExpectedRevision = 2
	p.Value = nil
	v = commit(p)
	if v.Status != "RETRACTED" {
		t.Fatal(v)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM world_fact_version WHERE fact_id=?", p.FactID).Scan(&n)
	if n != 3 {
		t.Fatal(n)
	}
}

// Repository-only projection test. Permit enforcement is exercised separately above.
func TestWorldCandidateDoesNotOverrideAndConflictGroupUsesPreferenceKey(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	obj, e := s.PutObject(ctx, []byte("independent synthetic evidence"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	entity := contract.NewID()
	apply := func(key, value, basis string) contract.WorldFact {
		t.Helper()
		v := map[string]any{"key": key, "value": value}
		p := contract.WorldUpdateProposal{SchemaVersion: 1, ID: contract.NewID(), RequestID: contract.NewID(), EntityID: entity, Predicate: "master.preference", Operation: "ASSERT", FactID: contract.NewID(), Value: &v, Evidence: []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, OriginID: obj.ID, Locator: "full", DataClass: "SYNTHETIC"}}, Basis: basis, PolicyRevision: 1, Reason: "fixture", Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
		if e := s.Write(ctx, func(tx *sql.Tx) error { return s.PutProposalTx(ctx, tx, p) }); e != nil {
			t.Fatal(e)
		}
		f, e := s.CommitWorld(ctx, p, func(tx *sql.Tx) error { return nil })
		if e != nil {
			t.Fatal(e)
		}
		return f
	}
	a := apply("drink", "tea", "MASTER_EXPLICIT")
	b := apply("drink", "coffee", "INFERENCE")
	if a.Status != "ACTIVE" || b.Status != "CANDIDATE" {
		t.Fatal(a.Status, b.Status)
	}
	c := apply("language", "Chinese", "MASTER_EXPLICIT")
	if c.Status != "ACTIVE" || c.ConflictGroup != nil {
		t.Fatal("different key falsely contested")
	}
	d := apply("drink", "coffee", "MASTER_EXPLICIT")
	if d.Status != "CONTESTED" || d.ConflictGroup == nil {
		t.Fatal("conflict missing")
	}
	snap, e := s.Snapshot(ctx, contract.NewID())
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range snap.Facts {
		if f.ID == a.ID && (f.Status != "CONTESTED" || f.Revision != 2) {
			t.Fatal("old head did not append contested revision")
		}
	}
}
