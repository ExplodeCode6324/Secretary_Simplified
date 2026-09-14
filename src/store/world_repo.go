package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"secretarysimplified/contract"
)

func (s *Store) PutProposalTx(ctx context.Context, tx *sql.Tx, p contract.WorldUpdateProposal) error {
	if _, e := contract.ReadClassification(p.Extensions); e != nil {
		return e
	}
	if e := contract.Validate("WorldUpdateProposal", p); e != nil {
		return e
	}
	if p.PolicyRevision != 1 {
		return errors.New("POLICY_REVISION_MISMATCH")
	}
	if p.Operation != "ASSERT" && p.ExpectedRevision == 0 {
		return errors.New("CONFLICT")
	}
	if len(p.Evidence) == 0 {
		return errors.New("EVIDENCE_REQUIRED")
	}
	for _, ref := range p.Evidence {
		var hash, class string
		if e := tx.QueryRowContext(ctx, "SELECT sha256,data_class FROM object_ref WHERE id=?", ref.ObjectID).Scan(&hash, &class); e != nil {
			return e
		}
		if hash != ref.SHA256 || class != ref.DataClass {
			return errors.New("INVALID_EVIDENCE")
		}
		var classErr error
		p.Extensions, classErr = contract.ClassifyExtensions(p.Extensions, class)
		if classErr != nil {
			return classErr
		}
	}
	b, _ := json.Marshal(p)
	hash, e := contract.ValueHash(p)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO world_proposal VALUES(?,?,?,?,?,?)", p.ID, p.RequestID, hash, "PENDING", string(b), contract.Now())
	return e
}
func (s *Store) GetProposal(ctx context.Context, id string) (contract.WorldUpdateProposal, error) {
	var p contract.WorldUpdateProposal
	var b []byte
	e := s.DB.QueryRowContext(ctx, "SELECT payload_json FROM world_proposal WHERE id=?", id).Scan(&b)
	if e == nil {
		e = contract.Decode("WorldUpdateProposal", b, &p)
	}
	return p, e
}
func (s *Store) CommitWorld(ctx context.Context, p contract.WorldUpdateProposal, check func(*sql.Tx) error) (contract.WorldFact, error) {
	return s.CommitWorldAtomic(ctx, p, check, nil)
}
func (s *Store) CommitWorldAtomic(ctx context.Context, p contract.WorldUpdateProposal, check func(*sql.Tx) error, finalize func(*sql.Tx, contract.WorldFact) error) (contract.WorldFact, error) {
	var result contract.WorldFact
	for _, evidence := range p.Evidence {
		bytes, e := s.ReadObject(ctx, evidence.ObjectID)
		if e != nil {
			return result, e
		}
		if contract.Hash(bytes) != evidence.SHA256 {
			return result, errors.New("INVALID_EVIDENCE")
		}
	}
	e := s.Write(ctx, func(tx *sql.Tx) error {
		if check == nil {
			return errors.New("PERMISSION_DENIED")
		}
		if e := check(tx); e != nil {
			return e
		}
		var raw []byte
		var state, hash string
		if e := tx.QueryRowContext(ctx, "SELECT payload_json,state,proposal_hash FROM world_proposal WHERE id=?", p.ID).Scan(&raw, &state, &hash); e != nil {
			return e
		}
		canonicalHash, err := contract.ValueHash(p)
		if err != nil {
			return err
		}
		if hash != canonicalHash || state != "PENDING" {
			return errors.New("PROPOSAL_CONFLICT")
		}
		var revision int
		err = tx.QueryRowContext(ctx, "SELECT revision FROM world_fact_head WHERE fact_id=?", p.FactID).Scan(&revision)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if revision != p.ExpectedRevision {
			return errors.New("CONFLICT")
		}
		var before any
		var previous *contract.VersionRef
		if revision > 0 {
			var old contract.WorldFact
			if err = tx.QueryRowContext(ctx, "SELECT payload_json FROM world_fact_version WHERE fact_id=? AND revision=?", p.FactID, revision).Scan(&raw); err != nil {
				return err
			}
			if err = contract.Decode("WorldFact", raw, &old); err != nil {
				return err
			}
			if old.EntityID != p.EntityID || old.Predicate != p.Predicate {
				return errors.New("SCOPE_CONFLICT")
			}
			before = old
			previous = &contract.VersionRef{ID: old.ID, Revision: old.Revision}
		}
		status := "CANDIDATE"
		if p.Basis == "MASTER_EXPLICIT" {
			status = "ACTIVE"
		}
		if p.Operation == "RETRACT" {
			status = "RETRACTED"
		}
		result = contract.WorldFact{SchemaVersion: 1, ID: p.FactID, Revision: revision + 1, EntityID: p.EntityID, Predicate: p.Predicate, Value: p.Value, Status: status, Evidence: p.Evidence, ProposalID: p.ID, AcceptanceRule: p.Basis, PolicyRevision: p.PolicyRevision, Supersedes: previous, CreatedAt: contract.Now(), Extensions: map[string]any{}}
		class, ce := contract.ReadClassification(p.Extensions)
		if ce != nil {
			return ce
		}
		if old, ok := before.(contract.WorldFact); ok {
			oldClass, e := contract.ReadClassification(old.Extensions)
			if e != nil {
				return e
			}
			class, ce = contract.JoinClass(class, oldClass)
			if ce != nil {
				return ce
			}
		}
		result.Extensions, ce = contract.ClassifyExtensions(result.Extensions, class)
		if ce != nil {
			return ce
		}
		if result.Status == "ACTIVE" && p.Operation == "ASSERT" && p.Predicate != "entity.relation" {
			rows, e := tx.QueryContext(ctx, "SELECT v.payload_json FROM world_fact_version v JOIN world_fact_head h ON h.fact_id=v.fact_id AND h.revision=v.revision WHERE v.entity_id=? AND v.predicate=? AND v.status IN ('ACTIVE','CONTESTED')", p.EntityID, p.Predicate)
			if e != nil {
				return e
			}
			others := []contract.WorldFact{}
			for rows.Next() {
				var x contract.WorldFact
				var rb []byte
				if e = rows.Scan(&rb); e != nil {
					rows.Close()
					return e
				}
				if e = contract.Decode("WorldFact", rb, &x); e != nil {
					rows.Close()
					return e
				}
				others = append(others, x)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
			for _, x := range others {
				if x.ID == result.ID || x.Value == nil || p.Value == nil {
					continue
				}
				if p.Predicate == "master.preference" && (*x.Value)["key"] != (*p.Value)["key"] {
					continue
				}
				left, _ := json.Marshal(x.Value)
				right, _ := json.Marshal(p.Value)
				if string(left) == string(right) {
					continue
				}
				group := contract.NewID()
				if x.ConflictGroup != nil {
					group = *x.ConflictGroup
				}
				result.Status = "CONTESTED"
				result.ConflictGroup = &group
				old := x
				oldClass, e := contract.ReadClassification(x.Extensions)
				if e != nil {
					return e
				}
				joined, e := contract.JoinClass(oldClass, class)
				if e != nil {
					return e
				}
				result.Extensions, e = contract.ClassifyExtensions(result.Extensions, joined)
				if e != nil {
					return e
				}
				x.Extensions, e = contract.ClassifyExtensions(x.Extensions, joined)
				if e != nil {
					return e
				}
				x.Revision++
				x.Status = "CONTESTED"
				x.ConflictGroup = &group
				x.ProposalID = p.ID
				x.Supersedes = &contract.VersionRef{ID: old.ID, Revision: old.Revision}
				x.CreatedAt = contract.Now()
				if e = writeFactTx(ctx, tx, x, old); e != nil {
					return e
				}
			}
		}
		if e := writeFactTx(ctx, tx, result, before); e != nil {
			return e
		}
		state = "ACCEPTED"
		if result.Status == "CONTESTED" {
			state = "CONTESTED"
		}
		_, e := tx.ExecContext(ctx, "UPDATE world_proposal SET state=? WHERE id=?", state, p.ID)
		if e != nil {
			return e
		}
		if finalize != nil {
			return finalize(tx, result)
		}
		return nil
	})
	return result, e
}
func writeFactTx(ctx context.Context, tx *sql.Tx, v contract.WorldFact, before any) error {
	if e := contract.Validate("WorldFact", v); e != nil {
		return e
	}
	b, _ := json.Marshal(v)
	if _, e := tx.ExecContext(ctx, "INSERT INTO world_fact_version VALUES(?,?,?,?,?,?,?,?)", v.ID, v.Revision, v.EntityID, v.Predicate, v.Status, v.ProposalID, string(b), v.CreatedAt); e != nil {
		return e
	}
	if _, e := tx.ExecContext(ctx, "INSERT INTO world_fact_head VALUES(?,?) ON CONFLICT(fact_id) DO UPDATE SET revision=excluded.revision", v.ID, v.Revision); e != nil {
		return e
	}
	return AppendEvent(ctx, tx, &contract.ChangeEvent{SchemaVersion: 1, ID: contract.NewID(), RootID: v.ProposalID, EntityType: "world_fact", EntityID: v.ID, EntityRevision: v.Revision, EventType: "world.updated", Origin: "world.update", CreatedAt: v.CreatedAt, Change: map[string]any{"before": before, "after": v, "evidence": v.Evidence}, Extensions: map[string]any{}})
}
