package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/model"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Real Core, SQLite and provider wire paths; only HTTP responses are synthetic.
func TestProviderSwitchPreservesAuthorityAndFailedRequestIsolation(t *testing.T) {
	s, c, _ := setup(t)
	ctx := context.Background()
	grant := runtimeGrant(t, s)
	key := filepath.Join(t.TempDir(), "test.key")
	if e := os.WriteFile(key, []byte("synthetic-provider-switch-key"), 0600); e != nil {
		t.Fatal(e)
	}
	c.Model.Profile = "opencode-go"
	c.Model.SecretRef = key
	goProvider := model.New(c)
	limited := false
	goCalls := 0
	response := func(r *http.Request, action contract.ActionProposal) *http.Response {
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		content := body["input"].([]any)[1].(map[string]any)["content"].(string)
		var input map[string]any
		if e := json.Unmarshal([]byte(content), &input); e != nil {
			t.Fatal(e)
		}
		raw, e := model.Fixture(model.Request{ContextID: input["context_id"].(string), OutputType: "DecisionEnvelope", Input: input})
		if e != nil {
			t.Fatal(e)
		}
		var decision map[string]any
		json.Unmarshal(raw, &decision)
		decision["actions"] = []contract.ActionProposal{action}
		out, _ := json.Marshal(decision)
		wire, _ := json.Marshal(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(out)}}}}, "usage": map[string]any{"input_tokens": 100, "output_tokens": 100}})
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(wire)))}
	}
	goProvider.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		goCalls++
		if r.URL.String() != "https://opencode.ai/zen/go/v1/responses" || r.Header.Get("x-opencode-session") == "" {
			t.Fatal("incorrect Go route")
		}
		if limited {
			return &http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("synthetic rate limit"))}, nil
		}
		return response(r, createAction()), nil
	})}
	svc := core.Service{Store: s, Config: c, Model: goProvider, GrantID: grant}
	first := input(contract.NewID())
	first.Text = "synthetic create"
	turn, e := s.AcceptInput(ctx, first, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	items, _ := s.ListItems(ctx)
	if len(items) != 1 {
		t.Fatal("first provider did not commit", items)
	}
	id := items[0].ID
	limited = true
	blocked := input(first.SessionID)
	turn, e = s.AcceptInput(ctx, blocked, 100)
	if e != nil {
		t.Fatal(e)
	}
	svc.Process(ctx, turn)
	failed, _ := s.GetTurn(ctx, turn.ID)
	if failed.State != "COMMITTED" || failed.Reply == nil || !strings.Contains((*failed.Reply)["text"].(string), "MODEL_HTTP_429") || len(failed.CommittedOperationKeys) != 0 {
		t.Fatal("rate-limited turn not isolated", failed.State)
	}
	nextConfig := c
	nextConfig.Model.Profile = "deepseek"
	nextConfig.Model.Model = "deepseek-flash"
	nextConfig.Model.Endpoint = "https://api.deepseek.com/responses"
	if e = nextConfig.Validate(); e != nil {
		t.Fatal(e)
	}
	official := model.New(nextConfig)
	officialCalls := 0
	official.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		officialCalls++
		if r.URL.String() != nextConfig.Model.Endpoint || r.Header.Get("x-opencode-session") != "" || r.Header.Get("Authorization") != "Bearer synthetic-provider-switch-key" {
			t.Fatal("provider credentials or route leaked")
		}
		return response(r, contract.ActionProposal{OperationKey: "change", Kind: "UPDATE_ITEM", Payload: map[string]any{"item_id": id, "expected_revision": 1, "changes": map[string]any{"status": "DONE"}}}), nil
	})}
	svc = core.Service{Store: s, Config: nextConfig, Model: official, GrantID: grant}
	next := input(first.SessionID)
	next.Text = "synthetic update " + id
	turn, e = s.AcceptInput(ctx, next, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	items, _ = s.ListItems(ctx)
	if len(items) != 1 || items[0].ID != id || items[0].Revision != 2 || items[0].Status != "DONE" {
		t.Fatal("switch lost authority or duplicated state", items)
	}
	if officialCalls != 1 || goCalls != 4 {
		t.Fatal("unexpected transport calls", goCalls, officialCalls)
	}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	if officialCalls != 1 {
		t.Fatal("switch repeated committed effect")
	}
}
