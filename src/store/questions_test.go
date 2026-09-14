package store

import (
	"context"
	"database/sql"
	"secretarysimplified/contract"
	"strings"
	"testing"
)

func questionInput() contract.InputEnvelope {
	return contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: contract.NewID(), PrincipalID: contract.NewID(), Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: "请确认日期", AttachmentRefs: []contract.ObjectRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}
}
func questionReply(p any) map[string]any {
	return map[string]any{"text": "请确认", "evidence": []any{}, "questions": p}
}
func TestQuestionAdmissionRejectsWholeDecision(t *testing.T) {
	for _, tc := range []struct {
		name, origin string
		p            any
	}{
		{"duplicate", "MASTER_CLI", []any{map[string]any{"text": "日期？", "item_id": nil}, map[string]any{"text": "日期？", "item_id": nil}}},
		{"forgedid", "MASTER_CLI", []any{map[string]any{"text": "日期？", "item_id": nil, "id": contract.NewID()}}},
		{"unknownitem", "MASTER_CLI", []any{map[string]any{"text": "日期？", "item_id": contract.NewID()}}},
		{"source", "SOURCE", []any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := foundationDB(t)
			ctx := context.Background()
			in := questionInput()
			in.Origin = tc.origin
			turn, e := s.AcceptInput(ctx, in, 100)
			if e != nil {
				t.Fatal(e)
			}
			called := false
			e = s.FinishDecisionTurn(ctx, turn.ID, questionReply(tc.p), nil, nil, "SYNTHETIC", func(*sql.Tx, contract.InputTurn) error { called = true; return nil })
			if e == nil || called {
				t.Fatalf("invalid admitted/callback %v %v", e, called)
			}
			got, _ := s.GetTurn(ctx, turn.ID)
			if got.State != "PENDING" {
				t.Fatal(got.State)
			}
		})
	}
}
func TestQuestionFallbackAndForeignPreArchive(t *testing.T) {
	s := foundationDB(t)
	ctx := context.Background()
	in := questionInput()
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	reply := questionReply([]any{map[string]any{"text": "具体哪一天？", "item_id": nil}})
	if e = s.FinishDecisionTurn(ctx, turn.ID, reply, nil, nil, "SYNTHETIC", nil); e != nil {
		t.Fatal(e)
	}
	got, e := s.GetTurn(ctx, turn.ID)
	if e != nil {
		t.Fatal(e)
	}
	q := (*got.Reply)["questions"].([]any)[0].(map[string]any)
	qid := q["id"].(string)
	if _, ok := reply["questions"].([]any)[0].(map[string]any)["id"]; ok {
		t.Fatal("mutated model reply")
	}
	var before int
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&before)
	foreign := in
	foreign.RequestID = contract.NewID()
	foreign.PrincipalID = contract.NewID()
	foreign.Text = "foreign-secret-marker"
	foreign.AnswerToQuestionID = &qid
	if _, e = s.AcceptInput(ctx, foreign, 100); e == nil || !strings.Contains(e.Error(), "QUESTION_NOT_FOUND_IN_SESSION") {
		t.Fatal(e)
	}
	var after int
	s.DB.QueryRow("SELECT count(*) FROM object_ref").Scan(&after)
	if after != before {
		t.Fatal("rejected target archived")
	}
	answer := in
	answer.RequestID = contract.NewID()
	answer.AnswerToQuestionID = &qid
	answer.Text = "明天"
	at, e := s.AcceptInput(ctx, answer, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishTurn(ctx, at.ID, map[string]any{"text": "model failed", "evidence": []any{}}, nil, nil); e != nil {
		t.Fatal(e)
	}
	if e = CheckAnswerTarget(ctx, s.DB, answer); e != nil {
		t.Fatal("fallback resolved", e)
	}
}
