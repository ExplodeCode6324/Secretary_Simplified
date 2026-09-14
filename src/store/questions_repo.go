package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/contract"
	"strings"
)

type questionQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// CheckAnswerTarget checks only the current session, never revealing a foreign question.
func CheckAnswerTarget(ctx context.Context, q questionQuery, in contract.InputEnvelope) error {
	if in.AnswerToQuestionID == nil {
		return nil
	}
	_, err := answerQuestion(ctx, q, in)
	return err
}
func answerQuestion(ctx context.Context, q questionQuery, in contract.InputEnvelope) (map[string]any, error) {
	if in.Origin != "MASTER_CLI" {
		return nil, errors.New("QUESTION_AUTHORITY_DENIED")
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil, errors.New("QUESTION_ANSWER_EMPTY")
	}
	missing := errors.New("QUESTION_NOT_FOUND_IN_SESSION")
	var raw []byte
	if err := q.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", in.SessionID).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, missing
		}
		return nil, err
	}
	var state contract.ConversationState
	if err := contract.Decode("ConversationState", raw, &state); err != nil {
		return nil, err
	}
	for _, p := range state.PendingQuestions {
		if p["id"] != *in.AnswerToQuestionID {
			continue
		}
		var eventRaw []byte
		if err := q.QueryRowContext(ctx, "SELECT payload_json FROM conversation_event WHERE session_id=? AND sequence=?", in.SessionID, p["created_sequence"]).Scan(&eventRaw); err != nil {
			return nil, missing
		}
		var ev contract.ConversationEvent
		if err := contract.Decode("ConversationEvent", eventRaw, &ev); err != nil {
			return nil, err
		}
		var principal string
		if ev.Role != "ASSISTANT" {
			return nil, missing
		}
		if err := q.QueryRowContext(ctx, "SELECT principal_id FROM input_turn WHERE id=? AND session_id=?", ev.TurnID, in.SessionID).Scan(&principal); err != nil || principal != in.PrincipalID {
			return nil, missing
		}
		if resolved, _ := p["resolved"].(bool); resolved {
			return nil, errors.New("QUESTION_ALREADY_RESOLVED")
		}
		return p, nil
	}
	return nil, missing
}

// FinishDecisionTurn admits question lifecycle changes only on a successful decision.
// The original model reply is never mutated; decision_record retains model proposals.
func (s *Store) FinishDecisionTurn(ctx context.Context, id string, reply map[string]any, keys []string, readSet []contract.ReadRef, class string, apply func(*sql.Tx, contract.InputTurn) error) error {
	raw, e := json.Marshal(reply)
	if e != nil {
		return e
	}
	var copied map[string]any
	if e = json.Unmarshal(raw, &copied); e != nil {
		return e
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		return finishTurnWithQuestionsTx(ctx, tx, id, copied, keys, readSet, class, apply, true)
	})
}

func prepareQuestionsTx(ctx context.Context, tx *sql.Tx, t contract.InputTurn, reply map[string]any, refs []contract.ReadRef, class string) (func(*contract.ConversationState, int) error, error) {
	answer, err := func() (map[string]any, error) {
		if t.Input.AnswerToQuestionID == nil {
			return nil, nil
		}
		return answerQuestion(ctx, tx, t.Input)
	}()
	if err != nil {
		return nil, err
	}
	proposals := []map[string]any{}
	if value, present := reply["questions"]; present {
		if t.Input.Origin != "MASTER_CLI" {
			return nil, errors.New("QUESTION_AUTHORITY_DENIED")
		}
		b, e := json.Marshal(value)
		if e != nil {
			return nil, e
		}
		if e = contract.Validate("QuestionProposals", value); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &proposals); e != nil {
			return nil, e
		}
	}
	seen := map[string]bool{}
	for _, p := range proposals {
		if strings.TrimSpace(p["text"].(string)) == "" {
			return nil, errors.New("QUESTION_TEXT_EMPTY")
		}
		key, _ := contract.ValueHash(p)
		if seen[key] {
			return nil, errors.New("QUESTION_DUPLICATE")
		}
		seen[key] = true
		if id, ok := p["item_id"].(string); ok {
			found := false
			for _, ref := range refs {
				if ref.EntityType == "Item" && ref.ID == id {
					if e := CheckReadSetTx(ctx, tx, []contract.ReadRef{ref}); e != nil {
						return nil, e
					}
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("QUESTION_ITEM_NOT_IN_CONTEXT")
			}
		}
	}
	return func(state *contract.ConversationState, seq int) error {
		if len(proposals) > 0 || answer != nil {
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
			var e error
			state.Extensions, e = contract.ClassifyExtensions(state.Extensions, class)
			if e != nil {
				return e
			}
		}

		if answer != nil {
			for _, p := range state.PendingQuestions {
				if p["id"] == answer["id"] {
					p["resolved"] = true
				}
			}
			reply["answered_question_id"] = answer["id"]
			reply["text"] = fmt.Sprint(reply["text"]) + fmt.Sprintf("\n[answered question_id=%s session_id=%s]\n%s", answer["id"], t.SessionID, answer["text"])
		}
		for len(state.PendingQuestions)+len(proposals) > 20 {
			oldest := -1
			minSeq := int(^uint(0) >> 1)
			for i, p := range state.PendingQuestions {
				if p["resolved"] == true {
					n := int(p["created_sequence"].(float64))
					if n < minSeq {
						oldest = i
						minSeq = n
					}
				}
			}
			if oldest < 0 {
				return errors.New("QUESTION_CAPACITY_EXCEEDED")
			}
			state.PendingQuestions = append(state.PendingQuestions[:oldest], state.PendingQuestions[oldest+1:]...)
		}
		admitted := []map[string]any{}
		for i, p := range proposals {
			id := contract.DeriveID(fmt.Sprintf("pending-question:v1:%s:%s:%s:%d", t.PrincipalID, t.SessionID, t.RequestID, i))
			v := map[string]any{"id": id, "text": p["text"], "item_id": p["item_id"], "created_sequence": seq, "resolved": false}
			state.PendingQuestions = append(state.PendingQuestions, v)
			admitted = append(admitted, v)
			reply["text"] = fmt.Sprint(reply["text"]) + fmt.Sprintf("\n[question_id=%s session_id=%s]\n%s", id, t.SessionID, p["text"])
		}
		if _, present := reply["questions"]; present {
			reply["questions"] = admitted
		}
		return contract.Validate("ConversationState", state)
	}, nil
}
