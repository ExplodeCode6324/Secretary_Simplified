package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/model"
	"strings"
	"testing"
)

type coreQuestionModel struct {
	model.Client
	calls        int
	ask          bool
	action       bool
	questionText string
}

func (m *coreQuestionModel) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	m.calls++
	raw, e := model.Fixture(r)
	if e != nil {
		return model.Result{}, e
	}
	var d contract.DecisionEnvelope
	json.Unmarshal(raw, &d)
	if m.ask {
		text := m.questionText
		if text == "" {
			text = "What is the deadline?"
		}
		d.Reply["questions"] = []any{map[string]any{"text": text, "item_id": nil}}
	}
	if m.action {
		d.Actions = []contract.ActionProposal{createAction()}
	}
	raw, e = json.Marshal(d)
	return model.Result{Output: raw}, e
}
func TestD11CoreCreatesAnswersAndPreservesFailedAnswerReceipt(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	m := &coreQuestionModel{Client: p, ask: true}
	svc := core.Service{Store: s, Config: c, Model: m, GrantID: g}
	session := contract.NewID()
	turn, e := s.AcceptInput(ctx, input(session), 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	turn, e = s.GetTurn(ctx, turn.ID)
	if e != nil {
		t.Fatal(e)
	}
	qs, ok := (*turn.Reply)["questions"].([]any)
	if !ok || len(qs) != 1 {
		t.Fatal("normal Core did not register question", turn.Reply)
	}
	q := qs[0].(map[string]any)
	id := q["id"].(string)
	if q["resolved"] != false || q["created_sequence"] != float64(2) || !strings.Contains((*turn.Reply)["text"].(string), id) {
		t.Fatal(q, turn.Reply)
	}
	var original string
	s.DB.QueryRow("SELECT payload_json FROM decision_record").Scan(&original)
	if strings.Contains(original, id) {
		t.Fatal("program-assigned ID polluted raw model decision")
	}
	m.ask = false
	m.action = true
	a := input(session)
	a.AnswerToQuestionID = &id
	a.Text = "synthetic explicit answer one"
	b := input(session)
	b.AnswerToQuestionID = &id
	b.Text = "synthetic explicit answer two"
	first, e := s.AcceptInput(ctx, a, 100)
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.AcceptInput(ctx, b, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, first); e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, second); e != nil {
		t.Fatal(e)
	}
	failed, e := s.GetTurn(ctx, second.ID)
	if e != nil || failed.State != "COMMITTED" || !strings.Contains((*failed.Reply)["text"].(string), "QUESTION_ALREADY_RESOLVED") || len(failed.CommittedOperationKeys) != 0 {
		t.Fatal(failed, e)
	}
	calls := m.calls
	if e = svc.Process(ctx, failed); e != nil || m.calls != calls {
		t.Fatal("failed answer reran model", e, m.calls, calls)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM item").Scan(&n)
	if n != 1 {
		t.Fatal("losing answer mutated business", n)
	}
	replay, e := s.AcceptInput(ctx, a, 100)
	if e != nil || replay.ID != first.ID {
		t.Fatal("same request rejected after resolution", e)
	}
}
func TestD11PublicAnswerNullAndUnknownTargetAreRejected(t *testing.T) {
	s, c, p := setup(t)
	h := (&core.Service{Store: s, Config: c, Model: p}).Handler()
	in := input(contract.NewID())
	base, _ := json.Marshal(in)
	var body map[string]any
	json.Unmarshal(base, &body)
	for _, tc := range []struct {
		name   string
		target any
		code   int
		reason string
	}{{"null", nil, 400, "INVALID_SCHEMA"}, {"unknown", contract.NewID(), 404, "QUESTION_NOT_FOUND_IN_SESSION"}} {
		t.Run(tc.name, func(t *testing.T) {
			body["answer_to_question_id"] = tc.target
			raw, _ := json.Marshal(body)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/inputs", bytes.NewReader(raw)))
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.reason) {
				t.Fatal(w.Code, w.Body.String())
			}
			for _, table := range []string{"input_turn", "request_receipt", "conversation_event", "object_ref"} {
				var n int
				s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
				if n != 0 {
					t.Fatal("invalid answer persisted", table, n)
				}
			}
		})
	}
}
func TestD11QuestionOnlyDecisionRequiresCurrentMasterAuthority(t *testing.T) {
	s, c, p := setup(t)
	m := &coreQuestionModel{Client: p, ask: true}
	svc := core.Service{Store: s, Config: c, Model: m, GrantID: runtimeGrant(t, s)}
	in := input(contract.NewID())
	in.PrincipalID = "synthetic-other-principal"
	turn, e := s.AcceptInput(context.Background(), in, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(context.Background(), turn); e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(context.Background(), in.SessionID)
	if e != nil || len(snap.Conversation.PendingQuestions) != 0 {
		t.Fatal("forged principal registered questions", e)
	}
}

func TestD11WhitespaceTextCannotAskOrAnswer(t *testing.T) {
	for _, blank := range []string{"", " ", "\t\n", "\u00a0\u2003\u3000"} {
		t.Run("answer_"+blank, func(t *testing.T) {
			s, c, p := setup(t)
			h := (&core.Service{Store: s, Config: c, Model: p}).Handler()
			in := input(contract.NewID())
			id := contract.NewID()
			in.AnswerToQuestionID = &id
			in.Text = blank
			code, v := apiCall(t, h, "POST", "/v1/inputs", in)
			if code != 400 || v.Error.(map[string]any)["code"] != "QUESTION_ANSWER_EMPTY" {
				t.Fatal(code, v)
			}
			for _, table := range []string{"object_ref", "input_turn", "request_receipt", "conversation_event"} {
				var n int
				s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
				if n != 0 {
					t.Fatal("blank answer persisted", table)
				}
			}
		})
		if blank == "" {
			continue
		}
		t.Run("ask_"+blank, func(t *testing.T) {
			s, c, p := setup(t)
			m := &coreQuestionModel{Client: p, ask: true, action: true, questionText: blank}
			svc := core.Service{Store: s, Config: c, Model: m, GrantID: runtimeGrant(t, s)}
			in := input(contract.NewID())
			turn, e := s.AcceptInput(context.Background(), in, 100)
			if e != nil {
				t.Fatal(e)
			}
			if e = svc.Process(context.Background(), turn); e != nil {
				t.Fatal(e)
			}
			turn, e = s.GetTurn(context.Background(), turn.ID)
			if e != nil || !strings.Contains((*turn.Reply)["text"].(string), "QUESTION_TEXT_EMPTY") {
				t.Fatal(turn, e)
			}
			var n int
			s.DB.QueryRow("SELECT count(*) FROM item").Scan(&n)
			if n != 0 {
				t.Fatal("partially committed blank question decision")
			}
			snap, _ := s.Snapshot(context.Background(), in.SessionID)
			if len(snap.Conversation.PendingQuestions) != 0 {
				t.Fatal("blank question registered")
			}
		})
	}
}
