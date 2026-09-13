package core

import (
	"errors"
	"net/http"
	"secretarysimplified/contract"
	"secretarysimplified/transport"
)

type typedRequest struct {
	SchemaVersion    int            `json:"schema_version"`
	RequestID        string         `json:"request_id"`
	SessionID        string         `json:"session_id"`
	ExpectedRevision *int           `json:"expected_revision"`
	Payload          map[string]any `json:"payload"`
}

func readTyped(r *http.Request) (typedRequest, error) {
	var q typedRequest
	if e := transport.Read(r, &q); e != nil {
		return q, e
	}
	if q.SchemaVersion != 1 || q.RequestID == "" || q.ExpectedRevision == nil || *q.ExpectedRevision < 0 {
		return q, errors.New("INVALID_SCHEMA")
	}
	if q.SessionID == "" {
		q.SessionID = "00000000-0000-4000-8000-000000000001"
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
			t, e := s.Typed(r.Context(), q.RequestID, q.SessionID, []contract.ActionProposal{{OperationKey: route.operation, Kind: route.kind, Payload: q.Payload}})
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
		_, e = s.Typed(r.Context(), q.RequestID, q.SessionID, []contract.ActionProposal{{OperationKey: "update_item", Kind: "UPDATE_ITEM", Payload: map[string]any{"item_id": r.PathValue("id"), "expected_revision": *q.ExpectedRevision, "changes": q.Payload}}})
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
		v, e := s.Store.UpdateJobRequest(r.Context(), r.PathValue("id"), q.RequestID, *q.ExpectedRevision, q.Payload)
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
