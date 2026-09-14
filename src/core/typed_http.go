package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"secretarysimplified/contract"
	"secretarysimplified/transport"
)

type typedRequest struct {
	SchemaVersion    int             `json:"schema_version"`
	RequestID        string          `json:"request_id"`
	SessionID        string          `json:"-"`
	SessionIDRaw     json.RawMessage `json:"session_id,omitempty"`
	ExpectedRevision *int            `json:"expected_revision"`
	Payload          map[string]any  `json:"payload"`
	DataClassRaw     json.RawMessage `json:"data_class,omitempty"`
	DataClass        string          `json:"-"`
}

func readTyped(r *http.Request) (typedRequest, error) {
	var q typedRequest
	if e := transport.Read(r, &q); e != nil {
		return q, e
	}
	if e := contract.CheckNoClassification(q); e != nil {
		return q, e
	}
	class, err := declaredClass(q.DataClassRaw)
	if err != nil {
		return q, err
	}
	q.DataClass = class
	if q.SchemaVersion != 1 || q.RequestID == "" || q.ExpectedRevision == nil || *q.ExpectedRevision < 0 {
		return q, errors.New("INVALID_SCHEMA")
	}
	if q.SessionIDRaw != nil {
		if string(q.SessionIDRaw) == "null" || json.Unmarshal(q.SessionIDRaw, &q.SessionID) != nil || q.SessionID == "" {
			return q, errors.New("INVALID_SCHEMA")
		}
	}
	return q, nil
}
func (s *Service) registerTypedRoutes(mux *http.ServeMux) {
	for _, route := range []struct{ path, kind, operation string }{
		{"POST /v1/items", "CREATE_ITEM", "create_item"},
		{"POST /v1/jobs", "CREATE_JOB", "create_job"},
		{"POST /v1/world/proposals", "WORLD_PROPOSAL", "world_proposal"},
	} {
		route := route
		mux.HandleFunc(route.path, func(w http.ResponseWriter, r *http.Request) {
			q, e := readTyped(r)
			if e == nil && *q.ExpectedRevision != 0 {
				e = errors.New("INVALID_ARGUMENT")
			}
			if e != nil {
				transport.Reply(w, 400, q.RequestID, nil, e)
				return
			}
			t, e := s.TypedClass(r.Context(), q.RequestID, q.SessionID, []contract.ActionProposal{{OperationKey: route.operation, Kind: route.kind, Payload: q.Payload}}, q.DataClass)
			var result any = t
			if e == nil {
				switch route.kind {
				case "CREATE_ITEM":
					result, e = s.Store.ItemRevision(r.Context(), typedItemID(q.RequestID, route.operation), 1)
				case "CREATE_JOB":
					var raw []byte
					e = s.Store.DB.QueryRowContext(r.Context(), "SELECT payload_json FROM scheduled_job WHERE root_id=?", t.IntentID).Scan(&raw)
					if e == nil {
						var j contract.ScheduledJob
						e = contract.Decode("ScheduledJob", raw, &j)
						result = j
					}
				case "WORLD_PROPOSAL":
					var task string
					e = s.Store.DB.QueryRowContext(r.Context(), "SELECT id FROM task WHERE root_id=?", t.IntentID).Scan(&task)
					result = map[string]any{"task_id": task, "turn_id": t.ID, "execution_state": "QUEUED"}
				}
			}
			transport.Reply(w, 201, q.RequestID, result, e)
		})
	}
	mux.HandleFunc("PATCH /v1/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		q, e := readTyped(r)
		if e != nil {
			transport.Reply(w, 400, q.RequestID, nil, e)
			return
		}
		_, e = s.TypedClass(r.Context(), q.RequestID, q.SessionID, []contract.ActionProposal{{OperationKey: "update_item", Kind: "UPDATE_ITEM", Payload: map[string]any{"item_id": r.PathValue("id"), "expected_revision": *q.ExpectedRevision, "changes": q.Payload}}}, q.DataClass)
		var result any
		if e == nil {
			result, e = s.Store.ItemRevision(r.Context(), r.PathValue("id"), *q.ExpectedRevision+1)
		}
		transport.Reply(w, 200, q.RequestID, result, e)
	})
	mux.HandleFunc("PATCH /v1/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		q, e := readTyped(r)
		if e != nil {
			transport.Reply(w, 400, q.RequestID, nil, e)
			return
		}
		v, e := s.Store.UpdateJobRequest(r.Context(), r.PathValue("id"), q.RequestID, *q.ExpectedRevision, q.Payload, q.DataClass)
		transport.Reply(w, 200, q.RequestID, v, e)
	})
	mux.HandleFunc("POST /v1/tasks/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		q, e := readTyped(r)
		if e != nil {
			transport.Reply(w, 400, q.RequestID, nil, e)
			return
		}
		e = s.Store.CancelTaskRequest(r.Context(), r.PathValue("id"), q.RequestID, *q.ExpectedRevision)
		var result any
		if e == nil {
			result, e = s.Store.RuntimeTask(r.Context(), r.PathValue("id"))
		}
		transport.Reply(w, 200, q.RequestID, result, e)
	})
}

func declaredClass(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "PERSONAL", nil
	}
	var class string
	if err := json.Unmarshal(raw, &class); err != nil {
		return "", errors.New("INVALID_DATA_CLASS")
	}
	if _, err := contract.JoinClass(class); err != nil {
		return "", errors.New("INVALID_DATA_CLASS")
	}
	return class, nil
}
