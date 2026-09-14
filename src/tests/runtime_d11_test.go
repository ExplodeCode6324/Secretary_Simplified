package tests

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"strings"
	"sync/atomic"
	"testing"
)

func d11State(t *testing.T, s *store.Store, session string) contract.ConversationState {
	t.Helper()
	var raw string
	if e := s.DB.QueryRow(`SELECT payload_json FROM conversation_session WHERE id=?`, session).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var v contract.ConversationState
	if e := contract.Decode("ConversationState", []byte(raw), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func d11Reply(texts ...string) map[string]any {
	questions := []any{}
	for _, text := range texts {
		questions = append(questions, map[string]any{"text": text, "item_id": nil})
	}
	return map[string]any{"text": "Synthetic question lifecycle", "evidence": []any{}, "questions": questions}
}
func d11Create(t *testing.T, s *store.Store, session string, texts ...string) contract.InputTurn {
	t.Helper()
	in := input(session)
	turn, e := s.AcceptInput(context.Background(), in, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishDecisionTurn(context.Background(), turn.ID, d11Reply(texts...), []string{}, nil, "SYNTHETIC", nil); e != nil {
		t.Fatal(e)
	}
	turn, e = s.GetTurn(context.Background(), turn.ID)
	if e != nil {
		t.Fatal(e)
	}
	return turn
}
func d11Item() contract.Item {
	return contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "answer action", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Now(), UpdatedAt: contract.Now(), Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
}

func TestD11QuestionCreateAnswerTenRetriesAcrossReopen(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	session := contract.NewID()
	in := input(session)
	var id, question, replyHash string
	for i := 0; i < 10; i++ {
		turn, e := s.AcceptInput(ctx, in, 100)
		if e != nil {
			t.Fatal(i, e)
		}
		if id == "" {
			id = turn.ID
		} else if id != turn.ID {
			t.Fatal("turn identity changed")
		}
		if e = s.FinishDecisionTurn(ctx, id, d11Reply("Which exact due date?"), []string{}, nil, "SYNTHETIC", nil); e != nil {
			t.Fatal(e)
		}
		got, e := s.GetTurn(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		h, _ := contract.ValueHash(got.Reply)
		if i == 0 {
			replyHash = h
		} else if h != replyHash {
			t.Fatal("retry reply changed")
		}
		state := d11State(t, s, session)
		if len(state.PendingQuestions) != 1 {
			t.Fatal("duplicate question")
		}
		qid := state.PendingQuestions[0]["id"].(string)
		if question == "" {
			question = qid
		} else if question != qid {
			t.Fatal("question identity changed")
		}
		s.Close()
		s, e = store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
		if e != nil {
			t.Fatal(e)
		}
	}
	defer s.Close()
	state := d11State(t, s, session)
	var assistant int
	if e := s.DB.QueryRow(`SELECT sequence FROM conversation_event WHERE session_id=? AND role='ASSISTANT'`, session).Scan(&assistant); e != nil {
		t.Fatal(e)
	}
	if state.PendingQuestions[0]["created_sequence"] != float64(assistant) {
		t.Fatal("wrong question sequence")
	}
	answer := input(session)
	answer.AnswerToQuestionID = &question
	answer.Text = "The due date is explicitly supplied."
	id = ""
	replyHash = ""
	for i := 0; i < 10; i++ {
		turn, e := s.AcceptInput(ctx, answer, 100)
		if e != nil {
			t.Fatal("answer retry", i, e)
		}
		if id == "" {
			id = turn.ID
		} else if id != turn.ID {
			t.Fatal("answer identity changed")
		}
		if e = s.FinishDecisionTurn(ctx, id, d11Reply(), []string{}, nil, "SYNTHETIC", nil); e != nil {
			t.Fatal(e)
		}
		got, _ := s.GetTurn(ctx, id)
		h, _ := contract.ValueHash(got.Reply)
		if i == 0 {
			replyHash = h
		} else if replyHash != h {
			t.Fatal("answer reply changed")
		}
		s.Close()
		s, e = store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
		if e != nil {
			t.Fatal(e)
		}
	}
	defer s.Close()
	state = d11State(t, s, session)
	if state.PendingQuestions[0]["resolved"] != true {
		t.Fatal("not resolved")
	}
	var count int
	s.DB.QueryRow(`SELECT count(*) FROM conversation_event WHERE session_id=?`, session).Scan(&count)
	if count != 4 {
		t.Fatal("duplicate visible events", count)
	}
	changed := contract.NewID()
	answer.AnswerToQuestionID = &changed
	if _, e := s.AcceptInput(ctx, answer, 100); e == nil || !strings.Contains(e.Error(), "IDEMPOTENCY_CONFLICT") {
		t.Fatal("same key changed pointer", e)
	}
}

func TestD11ConcurrentAnswerOnlyWinnerCommitsActions(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	session := contract.NewID()
	d11Create(t, s, session, "Choose one answer.")
	state := d11State(t, s, session)
	qid := state.PendingQuestions[0]["id"].(string)
	inputs := []contract.InputEnvelope{input(session), input(session)}
	turns := []contract.InputTurn{}
	for i := range inputs {
		inputs[i].AnswerToQuestionID = &qid
		v, e := s.AcceptInput(ctx, inputs[i], 100)
		if e != nil {
			t.Fatal(e)
		}
		turns = append(turns, v)
	}
	other, e := store.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	gate := make(chan struct{})
	results := make(chan error, 2)
	var callbacks atomic.Int32
	for i, db := range []*store.Store{s, other} {
		go func(i int, db *store.Store) {
			<-gate
			item := d11Item()
			results <- db.FinishDecisionTurn(ctx, turns[i].ID, d11Reply(), []string{"answer-action"}, nil, "SYNTHETIC", func(tx *sql.Tx, _ contract.InputTurn) error {
				callbacks.Add(1)
				return store.PutItemTx(ctx, tx, item, 0)
			})
		}(i, db)
	}
	close(gate)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		e := <-results
		if e == nil {
			success++
		} else if strings.Contains(e.Error(), "QUESTION_ALREADY_RESOLVED") {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 || callbacks.Load() != 1 {
		t.Fatal("answer race", success, conflict, callbacks.Load())
	}
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM item`).Scan(&n)
	if n != 1 {
		t.Fatal("loser action persisted", n)
	}
	for i, in := range inputs {
		turn, _ := s.GetTurn(ctx, turns[i].ID)
		if turn.State != "COMMITTED" {
			again, e := s.AcceptInput(ctx, in, 100)
			if e != nil || again.ID != turn.ID {
				t.Fatal("loser request identity changed", e)
			}
			if e = s.FinishDecisionTurn(ctx, again.ID, d11Reply(), []string{}, nil, "SYNTHETIC", nil); e == nil || !strings.Contains(e.Error(), "QUESTION_ALREADY_RESOLVED") {
				t.Fatal("loser was later committed", e)
			}
		}
	}
}

func TestD11CapacityResolvedEvictionAndSummaryCAS(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	session := contract.NewID()
	for i := 0; i < 20; i += 3 {
		q := []string{}
		for k := i; k < i+3 && k < 20; k++ {
			q = append(q, fmt.Sprintf("question %02d", k))
		}
		d11Create(t, s, session, q...)
	}
	full := d11State(t, s, session)
	if len(full.PendingQuestions) != 20 {
		t.Fatal("not full")
	}
	turn, e := s.AcceptInput(ctx, input(session), 100)
	if e != nil {
		t.Fatal(e)
	}
	item := d11Item()
	if e = s.FinishDecisionTurn(ctx, turn.ID, d11Reply("overflow"), []string{}, nil, "SYNTHETIC", func(tx *sql.Tx, _ contract.InputTurn) error { return store.PutItemTx(ctx, tx, item, 0) }); e == nil {
		t.Fatal("unresolved question evicted")
	}
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM item`).Scan(&n)
	if n != 0 {
		t.Fatal("overflow action partially committed")
	}
	qid := full.PendingQuestions[0]["id"].(string)
	answer := input(session)
	answer.AnswerToQuestionID = &qid
	a, e := s.AcceptInput(ctx, answer, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.FinishDecisionTurn(ctx, a.ID, d11Reply(), []string{}, nil, "SYNTHETIC", nil); e != nil {
		t.Fatal(e)
	}
	stale := full
	stale.Revision++
	if e = s.SaveConversation(ctx, stale, full.Revision, full.ThroughSequence); e == nil {
		t.Fatal("old summary resurrected question")
	}
	if e = s.FinishDecisionTurn(ctx, turn.ID, d11Reply("overflow"), []string{}, nil, "SYNTHETIC", nil); e != nil {
		t.Fatal("resolved slot not reclaimable", e)
	}
	now := d11State(t, s, session)
	if len(now.PendingQuestions) != 20 {
		t.Fatal("capacity changed")
	}
	seen := map[string]bool{}
	for _, q := range now.PendingQuestions {
		seen[q["id"].(string)] = true
	}
	if seen[qid] {
		t.Fatal("resolved oldest not reclaimed")
	}
	for _, q := range full.PendingQuestions[1:] {
		if !seen[q["id"].(string)] {
			t.Fatal("unresolved evicted")
		}
	}
	again := input(session)
	again.AnswerToQuestionID = &qid
	if _, e = s.AcceptInput(ctx, again, 100); e == nil || !strings.Contains(e.Error(), "QUESTION_NOT_FOUND_IN_SESSION") {
		t.Fatal("recycled question revived", e)
	}
}

func TestD11QuestionDecisionRollbackBoundaries(t *testing.T) {
	for _, boundary := range []string{"question_state", "assistant_event", "input_turn", "request_receipt"} {
		t.Run(boundary, func(t *testing.T) {
			s, _ := runtimeDB(t)
			ctx := context.Background()
			session := contract.NewID()
			in := input(session)
			turn, e := s.AcceptInput(ctx, in, 100)
			if e != nil {
				t.Fatal(e)
			}
			snapshot := func() []string {
				values := []string{}
				queries := []string{`SELECT payload_json FROM conversation_session WHERE id='` + session + `'`, `SELECT payload_json FROM input_turn WHERE id='` + turn.ID + `'`, `SELECT response_json FROM request_receipt WHERE request_id='` + in.RequestID + `'`, `SELECT CAST(count(*) AS TEXT) FROM conversation_event`, `SELECT CAST(count(*) AS TEXT) FROM item`, `SELECT CAST(count(*) AS TEXT) FROM change_event`}
				for _, q := range queries {
					var v string
					if e := s.DB.QueryRow(q).Scan(&v); e != nil {
						t.Fatal(e)
					}
					values = append(values, v)
				}
				return values
			}
			before := snapshot()
			var stmt string
			switch boundary {
			case "question_state":
				stmt = `CREATE TRIGGER fail_question BEFORE UPDATE ON conversation_session BEGIN SELECT RAISE(ABORT,'INJECTED_QUESTION_STATE');END`
			case "assistant_event":
				stmt = `CREATE TRIGGER fail_question BEFORE INSERT ON conversation_event WHEN NEW.role='ASSISTANT' BEGIN SELECT RAISE(ABORT,'INJECTED_ASSISTANT_EVENT');END`
			case "input_turn":
				stmt = `CREATE TRIGGER fail_question BEFORE UPDATE ON input_turn WHEN NEW.state='COMMITTED' BEGIN SELECT RAISE(ABORT,'INJECTED_INPUT_COMMIT');END`
			case "request_receipt":
				stmt = `CREATE TRIGGER fail_question BEFORE UPDATE ON request_receipt WHEN NEW.state='COMMITTED' BEGIN SELECT RAISE(ABORT,'INJECTED_RECEIPT_COMMIT');END`
			}
			if _, e = s.DB.Exec(stmt); e != nil {
				t.Fatal(e)
			}
			item := d11Item()
			callback := func(tx *sql.Tx, _ contract.InputTurn) error { return store.PutItemTx(ctx, tx, item, 0) }
			if e = s.FinishDecisionTurn(ctx, turn.ID, d11Reply("Atomic question"), []string{"create"}, nil, "SYNTHETIC", callback); e == nil {
				t.Fatal("fault injection did not fire")
			}
			after := snapshot()
			for i := range before {
				if before[i] != after[i] {
					t.Fatal("partial decision", boundary, i)
				}
			}
			if _, e = s.DB.Exec(`DROP TRIGGER fail_question`); e != nil {
				t.Fatal(e)
			}
			if e = s.FinishDecisionTurn(ctx, turn.ID, d11Reply("Atomic question"), []string{"create"}, nil, "SYNTHETIC", callback); e != nil {
				t.Fatal(e)
			}
			state := d11State(t, s, session)
			if len(state.PendingQuestions) != 1 {
				t.Fatal("retry duplicated/lost question")
			}
			var n int
			s.DB.QueryRow(`SELECT count(*) FROM item`).Scan(&n)
			if n != 1 {
				t.Fatal("retry actions", n)
			}
		})
	}
}
