package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"strings"
	"sync"
	"testing"
	"time"
)

type issue2Model struct {
	model.Client
	mu        sync.Mutex
	requests  []model.Request
	entered   chan struct{}
	release   chan struct{}
	questions bool
}

func (m *issue2Model) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	m.mu.Lock()
	n := len(m.requests)
	m.requests = append(m.requests, r)
	m.mu.Unlock()
	if n == 0 && m.entered != nil {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return model.Result{}, ctx.Err()
		}
	}
	b, e := model.Fixture(r)
	if e != nil {
		return model.Result{}, e
	}
	var d contract.DecisionEnvelope
	json.Unmarshal(b, &d)
	d.Reply["text"] = "ISSUE2_COMMITTED_REPLY"
	if m.questions && n == 0 {
		d.Reply["questions"] = []any{map[string]any{"text": "Synthetic shared question?", "item_id": nil}}
	}
	b, e = json.Marshal(d)
	return model.Result{Output: b}, e
}
func issue2HTTP(t *testing.T, h http.Handler, method, path string, v any) (int, map[string]any) {
	t.Helper()
	var b []byte
	if v != nil {
		var e error
		b, e = json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e, w.Body.String())
	}
	return w.Code, out
}
func issue2Data(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	d, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatal(out)
	}
	return d
}
func TestIssue2AUTHSharedAuthorityAndFrozenOrdering(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	session, e := s.AuthoritySession(ctx)
	if e != nil {
		t.Fatal(e)
	}
	m := &issue2Model{Client: p, entered: make(chan struct{}), release: make(chan struct{})}
	svc := &core.Service{Store: s, Model: m, Config: c}
	h := svc.Handler()
	_, a := issue2HTTP(t, h, "GET", "/v1/conversation", nil)
	_, b := issue2HTTP(t, h, "GET", "/v1/conversation", nil)
	if issue2Data(t, a)["session_id"] != session || issue2Data(t, b)["instance_id"] != issue2Data(t, a)["instance_id"] {
		t.Fatal(a, b)
	}
	first := input(session)
	first.Text = "FIRST_AUTH_INPUT"
	ft, e := s.AcceptInput(ctx, first, 100)
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 2)
	go func() { done <- svc.Process(ctx, ft) }()
	select {
	case <-m.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("model not entered")
	}
	later := input(session)
	later.Text = "FUTURE_AUTH_CANARY"
	later.ReceivedAt = "2000-01-01T00:00:00Z"
	later.DataClass = "PERSONAL"
	lt, e := s.AcceptInput(ctx, later, 100)
	if e != nil {
		t.Fatal(e)
	}
	// A deterministic public command cannot overtake the already accepted head.
	if _, e = svc.TypedClass(ctx, contract.NewID(), session, []contract.ActionProposal{}, "SYNTHETIC"); e == nil || e.Error() != "AUTHORITY_BUSY" {
		t.Fatal("Typed overtook head", e)
	}
	go func() { done <- svc.Process(ctx, lt) }()
	m.mu.Lock()
	calls := len(m.requests)
	m.mu.Unlock()
	if calls != 1 {
		t.Fatal("parallel model calls", calls)
	}
	close(m.release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	// Later PERSONAL input is correctly denied, but did not contaminate the earlier SYNTHETIC request.
	m.mu.Lock()
	req := m.requests[0]
	calls = len(m.requests)
	m.mu.Unlock()
	raw, _ := json.Marshal(req.Input)
	if bytes.Contains(raw, []byte("FUTURE_AUTH_CANARY")) || req.DataClass != "SYNTHETIC" || calls != 1 {
		t.Fatal("future input contaminated prefix", req.DataClass, calls)
	}
	prior, e := s.GetTurn(ctx, ft.ID)
	if e != nil || prior.State != "COMMITTED" {
		t.Fatal(prior, e)
	}
	// A third client shares the already committed preceding reply without uploading history.
	c.Policy.AllowedClasses = []string{"SYNTHETIC", "PERSONAL"}
	svc.Config = c
	m.Client = model.New(c)
	third := input(session)
	third.Text = "THIRD_AUTH_INPUT"
	tt, e := s.AcceptInput(ctx, third, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, tt); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	last := m.requests[len(m.requests)-1]
	m.mu.Unlock()
	raw, _ = json.Marshal(last.Input)
	if !bytes.Contains(raw, []byte("ISSUE2_COMMITTED_REPLY")) {
		got, _ := s.GetTurn(ctx, tt.ID)
		t.Fatalf("third=%v calls=%d", got.Reply, len(m.requests))
	}
	code, hist := issue2HTTP(t, h, "GET", "/v1/conversation/history?direction=backward&limit=2", nil)
	if code != 200 || len(issue2Data(t, hist)["events"].([]any)) != 2 || issue2Data(t, hist)["has_more"] != true {
		t.Fatal(hist)
	}
	again, e := s.AcceptInput(ctx, first, 100)
	if e != nil || again.ID != ft.ID {
		t.Fatal("lost response replay", again, e)
	}
	bad := input(contract.NewID())
	code, denied := issue2HTTP(t, h, "POST", "/v1/inputs", bad)
	if code != 403 || !strings.Contains(string(mustIssue2JSON(denied)), "AUTHORITY_SESSION_MISMATCH") {
		t.Fatal(code, denied)
	}
	if _, e = s.AcceptTyped(ctx, bad, 100, map[string]any{"text": "bad", "evidence": []any{}}, []string{}, nil); e == nil {
		t.Fatal("direct Typed foreign session accepted")
	}
}
func mustIssue2JSON(v any) []byte { b, _ := json.Marshal(v); return b }
func TestIssue2AUTHSharedQuestionsRestartAndWatermarks(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	session, _ := s.AuthoritySession(ctx)
	m := &issue2Model{Client: p, questions: true}
	svc := &core.Service{Store: s, Model: m, Config: c, GrantID: runtimeGrant(t, s)}
	in := input(session)
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	a, e := s.AuthorityState(ctx, 100)
	if e != nil || len(a.State.PendingQuestions) != 1 {
		t.Fatal(a, e)
	}
	qid := a.State.PendingQuestions[0]["id"].(string)
	answer := input(session)
	answer.Text = "Synthetic answer"
	answer.AnswerToQuestionID = &qid
	other := answer
	other.RequestID = contract.NewID()
	at, e := s.AcceptInput(ctx, answer, 100)
	if e != nil {
		t.Fatal(e)
	}
	bt, e := s.AcceptInput(ctx, other, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, at); e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, bt); e != nil {
		t.Fatal(e)
	}
	loser, e := s.GetTurn(ctx, bt.ID)
	if e != nil || loser.Reply == nil || !strings.Contains((*loser.Reply)["text"].(string), "QUESTION_ALREADY_RESOLVED") {
		t.Fatal("competing answer lacks stable rejection", loser, e)
	}
	calls := len(m.requests)
	if e = svc.Process(ctx, bt); e != nil || len(m.requests) != calls {
		t.Fatal("failed answer replay reran model", e)
	}
	a, e = s.AuthorityState(ctx, 100)
	if e != nil || a.State.PendingQuestions[0]["resolved"] != true {
		t.Fatal(a, e)
	}
	// A bounded preset summary CAS validates resynchronization, not model summary quality.
	st := a.State
	st.Revision++
	st.Summary = "Synthetic shared summary"
	st.SummaryFromSequence = 1
	st.ThroughSequence = a.HistorySequence
	st.Extensions, _ = contract.ClassifyExtensions(st.Extensions, "SYNTHETIC")
	if e = s.SaveConversation(ctx, st, a.State.Revision, a.State.ThroughSequence); e != nil {
		t.Fatal(e)
	}
	var path string
	s.DB.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path)
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := store.Open(path, s.ObjectsDir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	b, e := reopened.AuthorityState(ctx, 100)
	if e != nil || b.InstanceID != a.InstanceID || b.SessionID != a.SessionID || b.State.Summary != st.Summary || b.SummaryThroughSequence != st.ThroughSequence {
		t.Fatal(b, e)
	}
	retry, e := reopened.AcceptInput(ctx, answer, 100)
	if e != nil || retry.ID != at.ID {
		t.Fatal(retry, e)
	}
	_, out := issue2HTTP(t, (&core.Service{Store: reopened, Config: c, Model: p}).Handler(), "GET", "/v1/conversation", nil)
	if issue2Data(t, out)["revision"] != float64(st.Revision) {
		t.Fatal(out)
	}
}

func TestIssue2AUTHTUI13PublicEntryDisclosureAndOmittedSession(t *testing.T) {
	for _, class := range []string{"PERSONAL", "SECRET"} {
		t.Run(class, func(t *testing.T) {
			s, c, p := setup(t)
			ctx := context.Background()
			session, _ := s.AuthoritySession(ctx)
			m := &issue2Model{Client: p}
			svc := &core.Service{Store: s, Config: c, Model: m}
			h := svc.Handler()
			in := input(session)
			in.Text = "VIRTUAL_" + class + "_CANARY_ONLY"
			in.DataClass = class
			var body map[string]any
			json.Unmarshal(mustIssue2JSON(in), &body)
			delete(body, "session_id")
			if class == "PERSONAL" {
				delete(body, "data_class")
			}
			code, out := issue2HTTP(t, h, "POST", "/v1/inputs", body)
			if code != 202 {
				t.Fatal(code, out)
			}
			id := issue2Data(t, out)["turn_id"].(string)
			turn, e := s.GetTurn(ctx, id)
			if e != nil || turn.SessionID != session || turn.Input.DataClass != class {
				t.Fatal(turn, e)
			}
			if e = svc.Process(ctx, turn); e != nil {
				t.Fatal(e)
			}
			if len(m.requests) != 0 {
				t.Fatal("unapproved model call", class)
			}
			code, out = issue2HTTP(t, h, "POST", "/v1/inputs", body)
			if code != 202 || issue2Data(t, out)["turn_id"] != id {
				t.Fatal("omitted session replay", out)
			}
			action := map[string]any{"schema_version": 1, "request_id": contract.NewID(), "session_id": nil, "actions": []any{}, "data_class": "SYNTHETIC"}
			code, _ = issue2HTTP(t, h, "POST", "/v1/actions", action)
			if code != 400 {
				t.Fatal("null session accepted", code)
			}
		})
	}
}

func TestIssue2AUTHInternalTaskContextIsNotSecondConversation(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	a, e := s.AuthorityState(ctx, 100)
	if e != nil {
		t.Fatal(e)
	}
	internal := contract.NewID()
	in := input(internal)
	in.PrincipalID = "system"
	in.Origin = "SYSTEM"
	in.Text = "Synthetic read-only internal role"
	turn := contract.InputTurn{SchemaVersion: 1, ID: internal, SessionID: internal, IntentID: internal, RequestID: internal, Input: in}
	b := ctxbuild.Builder{Store: s, Model: p, Config: c, TaskLocal: true}
	req, _, e := b.Build(ctx, turn, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	body := map[string]any{}
	json.Unmarshal(mustIssue2JSON(req.Input), &body)
	if body["conversation"].(map[string]any)["id"] != a.SessionID {
		t.Fatal("internal cognitive branch")
	}
	after, e := s.AuthorityState(ctx, 100)
	if e != nil || after.Revision != a.Revision || after.HistorySequence != a.HistorySequence || len(after.LegacySessions) != 0 {
		t.Fatal("internal role wrote conversation", after, e)
	}
	turn.Input.Origin = "MASTER_CLI"
	if _, _, e = b.Build(ctx, turn, time.Now()); e == nil || e.Error() != "AUTHORITY_TASK_CONTEXT_DENIED" {
		t.Fatal("public task bypass", e)
	}
}

func TestIssue2AUTHHistoryQueryParametersStrict(t *testing.T) {
	s, c, p := setup(t)
	h := (&core.Service{Store: s, Config: c, Model: p}).Handler()
	for _, q := range []string{"after_sequence=oops", "before_sequence=-1", "after_sequence=", "after_sequence=%2B1", "after_sequence=1&after_sequence=2", "limit=invalid", "limit=0", "limit=101", "limit=99999999999999999999999999999"} {
		code, _ := issue2HTTP(t, h, "GET", "/v1/conversation/history?"+q, nil)
		if code != 400 {
			t.Fatal(q, code)
		}
	}
}
