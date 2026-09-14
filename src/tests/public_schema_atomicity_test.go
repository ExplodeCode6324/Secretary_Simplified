package tests

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"testing"
)

// Public ingress must reject malformed nested contracts before any durable write.
// Snapshot every application table, including event and receipt contents, rather
// than just counting the requested Item.
func TestAcceptanceA01PublicMalformedBodiesHaveNoDurableWrites(t *testing.T) {
	s, c, p := setup(t)
	h := (&core.Service{Store: s, Config: c, Model: p}).Handler()
	snapshot := func() map[string]string {
		t.Helper()
		names, e := s.DB.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if e != nil {
			t.Fatal(e)
		}
		var tables []string
		for names.Next() {
			var name string
			if e = names.Scan(&name); e != nil {
				t.Fatal(e)
			}
			tables = append(tables, name)
		}
		names.Close()
		result := map[string]string{}
		for _, table := range tables {
			rows, e := s.DB.Query(`SELECT * FROM "` + table + `" ORDER BY rowid`)
			if e != nil {
				t.Fatal(e)
			}
			cols, _ := rows.Columns()
			all := [][]any{}
			for rows.Next() {
				values := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if e = rows.Scan(ptrs...); e != nil {
					t.Fatal(e)
				}
				all = append(all, values)
			}
			if e = rows.Err(); e != nil {
				t.Fatal(e)
			}
			rows.Close()
			b, _ := json.Marshal(all)
			result[table] = string(b)
		}
		return result
	}
	before := snapshot()
	request := contract.NewID()
	valid := createAction()
	fixtures := []struct{ name, path, body string }{
		{"nested-extra", "/v1/items", `{"schema_version":1,"request_id":"` + request + `","expected_revision":0,"payload":{"title":"synthetic","kind":"TASK","timezone":"UTC","domain":"work","priority":1,"due_at":null,"surprise":true}}`},
		{"invalid-enum", "/v1/items", `{"schema_version":1,"request_id":"` + request + `","expected_revision":0,"payload":{"title":"synthetic","kind":"INVALID_KIND","timezone":"UTC","domain":"work","priority":1,"due_at":null}}`},
		{"duplicate-key", "/v1/items", `{"schema_version":1,"schema_version":1,"request_id":"` + request + `","expected_revision":0,"payload":{}}`},
		{"invalid-nested-schedule", "/v1/jobs", `{"schema_version":1,"request_id":"` + request + `","expected_revision":0,"payload":{"schedule":{"kind":"once","at":"bad-time","surprise":true}}}`},
	}
	// Keep the rest of the job schema valid so the nested probes test their
	// named violation instead of merely failing on missing sibling fields.
	jobPayload := reminderActions(t)[1].Payload
	jobPayload["schedule"].(map[string]any)["surprise"] = true
	jobBody, _ := json.Marshal(map[string]any{"schema_version": 1, "request_id": contract.NewID(), "expected_revision": 0, "payload": jobPayload})
	fixtures[len(fixtures)-1].body = string(jobBody)
	delete(jobPayload["schedule"].(map[string]any), "surprise")
	jobPayload["schedule"].(map[string]any)["at"] = "invalid-date"
	jobBody, _ = json.Marshal(map[string]any{"schema_version": 1, "request_id": contract.NewID(), "expected_revision": 0, "payload": jobPayload})
	fixtures = append(fixtures, struct{ name, path, body string }{"invalid-schedule-time", "/v1/jobs", string(jobBody)})
	invalid := contract.ActionProposal{OperationKey: "second", Kind: "UPDATE_ITEM", Payload: map[string]any{"item_id": contract.NewID(), "expected_revision": 1, "changes": map[string]any{"status": "DONE", "surprise": true}}}
	body, _ := json.Marshal(map[string]any{"schema_version": 1, "request_id": contract.NewID(), "session_id": contract.NewID(), "actions": []contract.ActionProposal{valid, invalid}})
	fixtures = append(fixtures, struct{ name, path, body string }{"later-action-invalid", "/v1/actions", string(body)})
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("POST", f.path, bytes.NewBufferString(f.body)))
			if w.Code < 400 {
				t.Fatalf("accepted malformed request: %d", w.Code)
			}
			if after := snapshot(); !reflect.DeepEqual(before, after) {
				for table, b := range before {
					if after[table] != b {
						t.Errorf("rejected request changed %s", table)
					}
				}
			}
		})
	}
}
