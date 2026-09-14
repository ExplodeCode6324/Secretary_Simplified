package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/contract"
)

func emptyConversation(id string) contract.ConversationState {
	return contract.ConversationState{SchemaVersion: 1, ID: id, Revision: 1, RecentEventIDs: []string{}, FocusEntityIDs: []string{}, PendingQuestions: []map[string]any{}, CommitmentItemIDs: []string{}, Extensions: map[string]any{}}
}
func (s *Store) AcceptInput(ctx context.Context, in contract.InputEnvelope, limit int) (contract.InputTurn, error) {
	var out contract.InputTurn
	if e := contract.Validate("InputEnvelope", in); e != nil {
		return out, e
	}
	if e := contract.CheckNoClassification(in); e != nil {
		return out, e
	}
	hash, e := inputSemanticHash(in)
	if e != nil {
		return out, e
	}
	var priorHash string
	lookupErr := s.DB.QueryRowContext(ctx, "SELECT payload_hash FROM request_receipt WHERE principal_id=? AND request_id=?", in.PrincipalID, in.RequestID).Scan(&priorHash)
	if lookupErr == nil && priorHash != hash {
		return out, errors.New("IDEMPOTENCY_CONFLICT")
	}
	if lookupErr != nil && lookupErr != sql.ErrNoRows {
		return out, lookupErr
	}
	if lookupErr == nil {
		var raw []byte
		if e = s.DB.QueryRowContext(ctx, "SELECT payload_json FROM input_turn WHERE principal_id=? AND request_id=?", in.PrincipalID, in.RequestID).Scan(&raw); e != nil {
			return out, e
		}
		e = contract.Decode("InputTurn", raw, &out)
		return out, e
	}
	if e = CheckAnswerTarget(ctx, s.DB, in); e != nil {
		return out, e
	}
	in, e = s.archiveInput(ctx, in)
	if e != nil {
		return out, e
	}
	err := s.Write(ctx, func(tx *sql.Tx) error { var e error; out, e = acceptInputTx(ctx, tx, in, hash, limit); return e })
	return out, err
}
func acceptInputTx(ctx context.Context, tx *sql.Tx, in contract.InputEnvelope, hash string, limit int) (out contract.InputTurn, err error) {
	err = func() error {

		var oldHash string
		err := tx.QueryRowContext(ctx, "SELECT payload_hash FROM request_receipt WHERE principal_id=? AND request_id=?", in.PrincipalID, in.RequestID).Scan(&oldHash)
		if err == nil {
			if oldHash != hash {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			var raw []byte
			err = tx.QueryRowContext(ctx, "SELECT payload_json FROM input_turn WHERE principal_id=? AND request_id=?", in.PrincipalID, in.RequestID).Scan(&raw)
			if err != nil {
				return err
			}
			return contract.Decode("InputTurn", raw, &out)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err = CheckAnswerTarget(ctx, tx, in); err != nil {
			return err
		}
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM input_turn WHERE state IN ('PENDING','PROCESSING')").Scan(&count); err != nil {
			return err
		}
		if count >= limit {
			return errors.New("BACKPRESSURE")
		}
		session := emptyConversation(in.SessionID)
		sb, _ := json.Marshal(session)
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO conversation_session(id,revision,through_sequence,payload_json) VALUES(?,1,0,?)", in.SessionID, string(sb)); err != nil {
			return err
		}
		out = contract.InputTurn{SchemaVersion: 1, ID: contract.NewID(), SessionID: in.SessionID, PrincipalID: in.PrincipalID, RequestID: in.RequestID, IntentID: contract.NewID(), State: "PENDING", Input: in, CommittedOperationKeys: []string{}, UpdatedAt: in.ReceivedAt, Extensions: map[string]any{}}
		receipt, _ := json.Marshal(map[string]any{"turn_id": out.ID, "intent_id": out.IntentID, "state": "ACCEPTED"})
		if _, err = tx.ExecContext(ctx, "INSERT INTO request_receipt VALUES(?,?,?,?,?,?,?)", in.PrincipalID, in.RequestID, hash, out.IntentID, "ACCEPTED", string(receipt), in.ReceivedAt); err != nil {
			return err
		}
		tb, _ := json.Marshal(out)
		if _, err = tx.ExecContext(ctx, "INSERT INTO input_turn VALUES(?,?,?,?,?,?,?,?)", out.ID, in.SessionID, in.PrincipalID, in.RequestID, out.IntentID, out.State, string(tb), out.UpdatedAt); err != nil {
			return err
		}
		return appendConversationTx(ctx, tx, out, "MASTER", in.Text)
	}()
	return out, err
}
func appendConversationTx(ctx context.Context, tx *sql.Tx, t contract.InputTurn, role, text string) error {
	return appendConversationWithQuestionsTx(ctx, tx, t, role, text, t.Input.DataClass, nil, nil)
}
func appendConversationWithQuestionsTx(ctx context.Context, tx *sql.Tx, t contract.InputTurn, role, text, class string, reply map[string]any, mutate func(*contract.ConversationState, int) error) error {
	var seq int
	if e := tx.QueryRowContext(ctx, "SELECT coalesce(max(sequence),0)+1 FROM conversation_event WHERE session_id=?", t.SessionID).Scan(&seq); e != nil {
		return e
	}
	var raw []byte
	if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", t.SessionID).Scan(&raw); e != nil {
		return e
	}
	var session contract.ConversationState
	if e := contract.Decode("ConversationState", raw, &session); e != nil {
		return e
	}
	if mutate != nil {
		if e := mutate(&session, seq); e != nil {
			return e
		}
		text, _ = reply["text"].(string)
		if session.Summary != "" || len(session.PendingQuestions) > 0 {
			stateClass, e := contract.ReadClassification(session.Extensions)
			if e != nil {
				return e
			}
			class, e = contract.JoinClass(class, stateClass)
			if e != nil {
				return e
			}
		}

	}
	v := contract.ConversationEvent{SchemaVersion: 1, ID: contract.NewID(), SessionID: t.SessionID, Sequence: seq, TurnID: t.ID, Role: role, Text: text, CreatedAt: t.UpdatedAt, DeliveryState: "RECORDED", Evidence: []contract.EvidenceRef{}, Extensions: map[string]any{}}
	if role == "MASTER" {
		for _, ref := range t.Input.AttachmentRefs {
			if ref.MediaType == "text/plain" && ref.SHA256 == contract.Hash([]byte(text)) {
				v.Evidence = append(v.Evidence, contract.EvidenceRef{ObjectID: ref.ID, SHA256: ref.SHA256, Locator: "full_text", OriginID: ref.ID, DataClass: ref.DataClass})
			}
		}
	}
	var classErr error
	v.Extensions, classErr = contract.ClassifyExtensions(v.Extensions, class)
	if classErr != nil {
		return classErr
	}
	if e := contract.Validate("ConversationEvent", v); e != nil {
		return e
	}
	b, _ := json.Marshal(v)
	_, e := tx.ExecContext(ctx, "INSERT INTO conversation_event VALUES(?,?,?,?,?,?)", v.ID, v.SessionID, v.Sequence, v.Role, string(b), v.CreatedAt)
	if e != nil {
		return e
	}
	oldRevision := session.Revision
	session.Revision++
	raw, _ = json.Marshal(session)
	r, e := tx.ExecContext(ctx, "UPDATE conversation_session SET revision=?,payload_json=? WHERE id=? AND revision=?", session.Revision, string(raw), session.ID, oldRevision)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("CONFLICT")
	}
	return nil
}
func (s *Store) GetTurn(ctx context.Context, id string) (contract.InputTurn, error) {
	var v contract.InputTurn
	var b []byte
	e := s.DB.QueryRowContext(ctx, "SELECT payload_json FROM input_turn WHERE id=?", id).Scan(&b)
	if e == nil {
		e = contract.Decode("InputTurn", b, &v)
	}
	return v, e
}
func (s *Store) PendingTurns(ctx context.Context) ([]contract.InputTurn, error) {
	rows, e := s.DB.QueryContext(ctx, "SELECT payload_json FROM input_turn WHERE state IN ('PENDING','PROCESSING') ORDER BY updated_at LIMIT 100")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []contract.InputTurn{}
	for rows.Next() {
		var b []byte
		var v contract.InputTurn
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = contract.Decode("InputTurn", b, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Request(ctx context.Context, principal, id string) (map[string]any, error) {
	var b []byte
	var v map[string]any
	e := s.DB.QueryRowContext(ctx, "SELECT response_json FROM request_receipt WHERE principal_id=? AND request_id=?", principal, id).Scan(&b)
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) FinishTurn(ctx context.Context, id string, reply map[string]any, keys []string, apply func(*sql.Tx, contract.InputTurn) error) error {
	return s.Write(ctx, func(tx *sql.Tx) error { return finishTurnTx(ctx, tx, id, reply, keys, apply) })
}
func (s *Store) FinishTurnClass(ctx context.Context, id string, reply map[string]any, keys []string, class string, apply func(*sql.Tx, contract.InputTurn) error) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		return finishTurnWithQuestionsTx(ctx, tx, id, reply, keys, nil, class, apply, false)
	})
}
func finishTurnTx(ctx context.Context, tx *sql.Tx, id string, reply map[string]any, keys []string, apply func(*sql.Tx, contract.InputTurn) error) error {
	return finishTurnWithQuestionsTx(ctx, tx, id, reply, keys, nil, "SYNTHETIC", apply, false)
}
func finishTurnWithQuestionsTx(ctx context.Context, tx *sql.Tx, id string, reply map[string]any, keys []string, refs []contract.ReadRef, class string, apply func(*sql.Tx, contract.InputTurn) error, questions bool) error {

	var t contract.InputTurn
	var b []byte
	if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM input_turn WHERE id=?", id).Scan(&b); e != nil {
		return e
	}
	if e := contract.Decode("InputTurn", b, &t); e != nil {
		return e
	}
	if t.State == "COMMITTED" {
		return nil
	}
	if questions {
		var stateRaw []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", t.SessionID).Scan(&stateRaw); e != nil {
			return e
		}
		var state contract.ConversationState
		if e := contract.Decode("ConversationState", stateRaw, &state); e != nil {
			return e
		}
		if state.Summary != "" || len(state.PendingQuestions) > 0 {
			old, e := contract.ReadClassification(state.Extensions)
			if e != nil {
				return e
			}
			class, e = contract.JoinClass(class, old)
			if e != nil {
				return e
			}
		}
	}
	var mutate func(*contract.ConversationState, int) error
	if questions {
		var e error
		mutate, e = prepareQuestionsTx(ctx, tx, t, reply, refs, class)
		if e != nil {
			return e
		}
	}
	if apply != nil {
		if e := apply(tx, t); e != nil {
			return e
		}
	}
	var classErr error
	t.Extensions, classErr = contract.ClassifyExtensions(t.Extensions, class)
	if classErr != nil {
		return classErr
	}
	t.State = "COMMITTED"
	t.Reply = &reply
	if keys == nil {
		keys = []string{}
	}
	t.CommittedOperationKeys = keys
	t.UpdatedAt = contract.Now()
	text, _ := reply["text"].(string)
	if e := appendConversationWithQuestionsTx(ctx, tx, t, "ASSISTANT", text, class, reply, mutate); e != nil {
		return e
	}

	if e := contract.Validate("InputTurn", t); e != nil {
		return e
	}
	b, _ = json.Marshal(t)
	if _, e := tx.ExecContext(ctx, "UPDATE input_turn SET state=?,payload_json=?,updated_at=? WHERE id=?", t.State, string(b), t.UpdatedAt, id); e != nil {
		return e
	}
	r, _ := json.Marshal(map[string]any{"turn_id": id, "intent_id": t.IntentID, "state": "COMMITTED", "reply": reply, "operation_keys": keys, "extensions": t.Extensions})
	if _, e := tx.ExecContext(ctx, "UPDATE request_receipt SET state='COMMITTED',response_json=? WHERE principal_id=? AND request_id=?", string(r), t.PrincipalID, t.RequestID); e != nil {
		return e
	}
	return nil
}
func (s *Store) SaveManifest(ctx context.Context, v contract.ContextManifest) error {
	if e := contract.Validate("ContextManifest", v); e != nil {
		return e
	}
	b, _ := json.Marshal(v)
	_, e := s.DB.ExecContext(ctx, "INSERT INTO context_manifest(id,intent_id,snapshot_seq,request_hash,payload_json,created_at) VALUES(?,?,?,?,?,?)", v.ID, v.IntentID, v.SnapshotSeq, v.RequestHash, string(b), v.AsOf)
	return e
}
func CheckReadSetTx(ctx context.Context, tx *sql.Tx, refs []contract.ReadRef) error {
	for _, r := range refs {
		table, ok := map[string]string{"Item": "item", "Task": "task", "WorldFact": "world_fact_head", "ConversationState": "conversation_session", "SourceState": "source_state", "ConsciousnessState": "consciousness_snapshot"}[r.EntityType]
		if !ok {
			return fmt.Errorf("UNKNOWN_READ_TYPE")
		}
		col := "id"
		if table == "world_fact_head" {
			col = "fact_id"
		}
		var revision int
		if e := tx.QueryRowContext(ctx, "SELECT revision FROM "+table+" WHERE "+col+"=?", r.ID).Scan(&revision); e != nil {
			return e
		}
		if revision != r.Revision {
			return errors.New("CONFLICT")
		}
	}
	return nil
}

// AcceptTyped atomically accepts and commits program-validated commands. A
// rejected typed command can never be picked up by the language-model scanner.
func (s *Store) AcceptTyped(ctx context.Context, in contract.InputEnvelope, limit int, reply map[string]any, keys []string, apply func(*sql.Tx, contract.InputTurn) error) (out contract.InputTurn, err error) {
	if err = contract.Validate("InputEnvelope", in); err != nil {
		return
	}
	if e := contract.CheckNoClassification(in); e != nil {
		return out, e
	}
	hash, e := inputSemanticHash(in)
	if e != nil {
		return out, e
	}
	var priorHash string
	lookupErr := s.DB.QueryRowContext(ctx, "SELECT payload_hash FROM request_receipt WHERE principal_id=? AND request_id=?", in.PrincipalID, in.RequestID).Scan(&priorHash)
	if lookupErr == nil && priorHash != hash {
		return out, errors.New("IDEMPOTENCY_CONFLICT")
	}
	if lookupErr != nil && lookupErr != sql.ErrNoRows {
		return out, lookupErr
	}
	in, e = s.archiveInput(ctx, in)
	if e != nil {
		return out, e
	}
	err = s.Write(ctx, func(tx *sql.Tx) error {
		var e error
		out, e = acceptInputTx(ctx, tx, in, hash, limit)
		if e != nil {
			return e
		}
		return finishTurnTx(ctx, tx, out.ID, reply, keys, apply)
	})
	if err == nil {
		return s.GetTurn(ctx, out.ID)
	}
	return
}

func (s *Store) archiveInput(ctx context.Context, in contract.InputEnvelope) (contract.InputEnvelope, error) {
	ref, e := s.PutObject(ctx, []byte(in.Text), "text/plain", in.DataClass)
	if e != nil {
		return in, e
	}
	for _, v := range in.AttachmentRefs {
		if v.ID == ref.ID {
			return in, nil
		}
	}
	in.AttachmentRefs = append(append([]contract.ObjectRef{}, in.AttachmentRefs...), ref)
	return in, nil
}

func (s *Store) ShouldSummarize(ctx context.Context, session string) (bool, error) {
	var count, bytes int
	e := s.DB.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(length(CAST(json_extract(e.payload_json,'$.text') AS BLOB))),0) FROM conversation_event e JOIN conversation_session c ON e.session_id=c.id WHERE e.session_id=? AND e.sequence>c.through_sequence", session).Scan(&count, &bytes)
	return count > 40 || bytes > 12*1024, e
}

// The original text itself is semantic; its automatic archival copy is derived
// evidence, not a second user attachment. This produces the same identity from
// the submitted envelope and the final stored envelope.
func inputSemanticHash(in contract.InputEnvelope) (string, error) {
	in.ReceivedAt = ""
	refs := []contract.ObjectRef{}
	for _, ref := range in.AttachmentRefs {
		if ref.MediaType == "text/plain" && ref.SHA256 == contract.Hash([]byte(in.Text)) {
			continue
		}
		refs = append(refs, ref)
	}
	in.AttachmentRefs = refs
	return contract.ValueHash(in)
}
