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

type classificationInjectionModel struct{ model.Client }

func (m classificationInjectionModel) Generate(ctx context.Context, req model.Request) (model.Result, error) {
	raw, e := model.Fixture(req)
	if e != nil {
		return model.Result{}, e
	}
	var d contract.DecisionEnvelope
	json.Unmarshal(raw, &d)
	d.Actions = []contract.ActionProposal{createAction()}
	d.Extensions = map[string]any{contract.ClassificationKey: map[string]any{"data_class": "SYNTHETIC"}}
	raw, e = json.Marshal(d)
	return model.Result{Output: raw}, e
}
func TestClassificationInjectionRejectsWholeDecisionAndClient(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	svc := core.Service{Store: s, Config: c, Model: classificationInjectionModel{p}, GrantID: runtimeGrant(t, s)}
	in := input(contract.NewID())
	turn, e := s.AcceptInput(ctx, in, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, turn); e != nil {
		t.Fatal(e)
	}
	final, e := s.GetTurn(ctx, turn.ID)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains((*final.Reply)["text"].(string), "CLASSIFICATION_INJECTION") {
		t.Fatal(final.Reply)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM item").Scan(&n)
	if n != 0 {
		t.Fatal("injected decision committed action")
	}
	// Client labels on program metadata are refused before archival/admission.
	for _, class := range []any{"SYNTHETIC", "PERSONAL", nil, 42} {
		in = input(contract.NewID())
		in.Extensions = map[string]any{contract.ClassificationKey: map[string]any{"data_class": class}}
		body, _ := json.Marshal(in)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/inputs", bytes.NewReader(body))
		var before int
		s.DB.QueryRow("SELECT COUNT(*) FROM input_turn").Scan(&before)
		svc.Handler().ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "CLASSIFICATION_INJECTION") {
			t.Fatal(w.Code, w.Body.String())
		}
		s.DB.QueryRow("SELECT COUNT(*) FROM input_turn").Scan(&n)
		if n != before {
			t.Fatal("injection admitted turn")
		}
	}
}
func TestClassificationTypedAndHTTPDefaults(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		s, c, p := setup(t)
		ctx := context.Background()
		svc := core.Service{Store: s, Config: c, Model: p}
		class := "PERSONAL"
		if explicit {
			class = "SYNTHETIC"
		}
		a := createAction()
		var turn contract.InputTurn
		var e error
		if explicit {
			turn, e = svc.TypedClass(ctx, contract.NewID(), contract.NewID(), []contract.ActionProposal{a}, class)
		} else {
			turn, e = svc.Typed(ctx, contract.NewID(), contract.NewID(), []contract.ActionProposal{a})
		}
		if e != nil {
			t.Fatal(e)
		}
		if turn.Input.DataClass != class {
			t.Fatal(turn.Input.DataClass)
		}
		var raw []byte
		s.DB.QueryRow("SELECT payload_json FROM item LIMIT 1").Scan(&raw)
		var item contract.Item
		json.Unmarshal(raw, &item)
		got, e := contract.ReadClassification(item.Extensions)
		if e != nil || got != class {
			t.Fatal(got, e)
		}
		s.DB.QueryRow("SELECT payload_json FROM conversation_event WHERE role='ASSISTANT' LIMIT 1").Scan(&raw)
		var event contract.ConversationEvent
		json.Unmarshal(raw, &event)
		got, e = contract.ReadClassification(event.Extensions)
		if e != nil || got != "SYNTHETIC" {
			t.Fatal("fixed typed acknowledgement is not literal", got, e)
		}
		in := input(contract.NewID())
		raw, _ = json.Marshal(in)
		var body map[string]any
		json.Unmarshal(raw, &body)
		if explicit {
			body["data_class"] = "SYNTHETIC"
		} else {
			delete(body, "data_class")
		}
		raw, _ = json.Marshal(body)
		w := httptest.NewRecorder()
		svc.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/v1/inputs", bytes.NewReader(raw)))
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		s.DB.QueryRow("SELECT payload_json FROM input_turn WHERE request_id=?", in.RequestID).Scan(&raw)
		json.Unmarshal(raw, &turn)
		if turn.Input.DataClass != class {
			t.Fatal(turn.Input.DataClass)
		}
	}
}
