package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/transport"
	"testing"
)

func apiCall(t *testing.T, h http.Handler, method, path string, body any) (int, transport.Envelope) {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out transport.Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatalf("%s %s: %s", method, path, w.Body.String())
	}
	return w.Code, out
}
func TestPublicTypedItemsPaginationAndNoPartialWrites(t *testing.T) {
	s, c, p := setup(t)
	svc := core.Service{Store: s, Config: c, Model: p}
	h := svc.Handler()
	request := contract.NewID()
	body := map[string]any{"schema_version": 1, "request_id": request, "expected_revision": 0, "payload": createAction().Payload}
	code, v := apiCall(t, h, "POST", "/v1/items", body)
	if code != 201 {
		t.Fatal(code, v)
	}
	m := v.Result.(map[string]any)
	id := m["id"].(string)
	code, v = apiCall(t, h, "POST", "/v1/items", body)
	if code != 201 || v.Result.(map[string]any)["id"] != id {
		t.Fatal("retry ID", code, v)
	}
	bad := map[string]any{"schema_version": 1, "request_id": contract.NewID(), "expected_revision": 1, "payload": map[string]any{"status": "DONE", "unexpected": "reject me"}}
	code, _ = apiCall(t, h, "PATCH", "/v1/items/"+id, bad)
	if code < 400 {
		t.Fatal("unknown patch accepted")
	}
	item, _ := s.GetItem(context.Background(), id)
	if item.Revision != 1 || item.Status != "OPEN" {
		t.Fatal("partial mutation", item)
	}
	bad["payload"] = map[string]any{"status": "DONE"}
	code, v = apiCall(t, h, "PATCH", "/v1/items/"+id, bad)
	if code != 200 || v.Result.(map[string]any)["status"] != "DONE" {
		t.Fatal(code, v)
	}
	code, v = apiCall(t, h, "POST", "/v1/items", body)
	if code != 201 || v.Result.(map[string]any)["revision"] != float64(1) || v.Result.(map[string]any)["status"] != "OPEN" {
		t.Fatal("retry must return original revision", code, v)
	}
	body["request_id"] = contract.NewID()
	code, _ = apiCall(t, h, "POST", "/v1/items", body)
	if code != 201 {
		t.Fatal(code)
	}
	code, v = apiCall(t, h, "GET", "/v1/items?domain=work&limit=1", nil)
	if code != 200 {
		t.Fatal(code, v)
	}
	page := v.Result.(map[string]any)
	cursor := page["next_cursor"].(string)
	code, v = apiCall(t, h, "GET", "/v1/items?domain=work&limit=1&cursor="+cursor, nil)
	if code != 200 || len(v.Result.(map[string]any)["items"].([]any)) != 1 {
		t.Fatal(code, v)
	}
	code, _ = apiCall(t, h, "GET", "/v1/items?domain=life&limit=1&cursor="+cursor, nil)
	if code < 400 {
		t.Fatal("cursor filter mismatch accepted")
	}
	body["request_id"] = contract.NewID()
	apiCall(t, h, "POST", "/v1/items", body)
	code, _ = apiCall(t, h, "GET", "/v1/items?domain=work&limit=1&cursor="+cursor, nil)
	if code != 409 {
		t.Fatal("changed snapshot accepted", code)
	}
}

func TestPublicSearchRequiresSchemaVersion(t *testing.T) {
	s, c, p := setup(t)
	svc := core.Service{Store: s, Config: c, Model: p}
	code, _ := apiCall(t, svc.Handler(), "POST", "/v1/memory/search", map[string]any{"query": "test"})
	if code != 400 {
		t.Fatal(code)
	}
	code, _ = apiCall(t, svc.Handler(), "POST", "/v1/memory/search", map[string]any{"schema_version": 1, "query": "test"})
	if code != 200 {
		t.Fatal(code)
	}
}
