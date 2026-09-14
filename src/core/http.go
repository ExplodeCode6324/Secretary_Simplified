package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"secretarysimplified/contract"
	"secretarysimplified/diagnostics"
	"secretarysimplified/transport"
	"strconv"
	"strings"
	"time"
)

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/conversation", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.AuthorityState(r.Context(), s.Config.Limits.Queue)
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/conversation/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		parse := func(name string, def int) (int, error) {
			if !q.Has(name) {
				return def, nil
			}
			v := q.Get(name)
			if len(q[name]) != 1 || v == "" {
				return 0, errors.New("INVALID_QUERY")
			}
			for _, r := range v {
				if r < '0' || r > '9' {
					return 0, errors.New("INVALID_QUERY")
				}
			}
			n, e := strconv.Atoi(v)
			if e != nil {
				return 0, errors.New("INVALID_QUERY")
			}
			return n, nil
		}
		after, e1 := parse("after_sequence", 0)
		before, e2 := parse("before_sequence", 0)
		limit, e3 := parse("limit", 50)
		if e1 != nil || e2 != nil || e3 != nil {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_QUERY"))
			return
		}

		direction := q.Get("direction")
		if direction != "" && direction != "forward" && direction != "backward" {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_QUERY"))
			return
		}
		v, e := s.Store.AuthorityHistory(r.Context(), after, before, limit, direction == "backward")
		transport.Reply(w, status(e), "", v, e)
	})

	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		epoch, _ := time.Parse(time.RFC3339Nano, s.Config.Epoch)
		v, e := diagnostics.DoctorWithEpoch(r.Context(), s.Store, s.Config.DataDir, epoch)
		transport.Reply(w, 200, "", map[string]any{"core": "ONLINE", "model_profile": s.Config.Model.Profile, "model": "UNKNOWN: availability is established only by an actual call", "diagnostics": v}, e)
	})
	mux.HandleFunc("POST /v1/inputs", func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		if e := transport.Read(r, &raw); e != nil {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		// Validate the original field presence before decoding optional pointers;
		// explicit null must not collapse to an omitted answer target.
		if raw == nil {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		if e := contract.CheckNoClassification(raw); e != nil {
			transport.Reply(w, 400, "", nil, e)
			return
		}
		if _, present := raw["data_class"]; !present {
			raw["data_class"] = "PERSONAL"
		}
		raw["principal_id"] = "master"
		raw["origin"] = "MASTER_CLI"
		if _, present := raw["session_id"]; !present {
			request, _ := raw["request_id"].(string)
			session, e := s.Store.ResolveSession(r.Context(), "master", request, "")
			if e != nil {
				transport.Reply(w, status(e), request, nil, e)
				return
			}
			raw["session_id"] = session
		}

		b, _ := json.Marshal(raw)
		var in contract.InputEnvelope
		if e := contract.Decode("InputEnvelope", b, &in); e != nil {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		v, e := s.Store.AcceptInput(r.Context(), in, s.Config.Limits.Queue)
		code := 202
		if e != nil {
			code = 400
		}
		transport.Reply(w, code, in.RequestID, map[string]any{"turn_id": v.ID, "intent_id": v.IntentID, "state": v.State}, e)
	})
	mux.HandleFunc("GET /v1/turns/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.GetTurn(r.Context(), r.PathValue("id"))
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Request(r.Context(), "master", r.PathValue("id"))
		transport.Reply(w, status(e), r.PathValue("id"), v, e)
	})
	mux.HandleFunc("GET /v1/items", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		v, e := s.Store.PublicPage(r.Context(), "items", r.URL.Query().Get("domain"), r.URL.Query().Get("status"), r.URL.Query().Get("cursor"), limit)
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.GetItem(r.Context(), r.PathValue("id"))
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/world", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		v, e := s.Store.PublicPage(r.Context(), "world", r.URL.Query().Get("entity"), r.URL.Query().Get("predicate"), r.URL.Query().Get("cursor"), limit)
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/memory/consciousness", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.Snapshot(r.Context(), "00000000-0000-4000-8000-000000000000")
		transport.Reply(w, status(e), "", v.Consciousness, e)
	})
	mux.HandleFunc("POST /v1/memory/search", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			SchemaVersion int      `json:"schema_version"`
			Query         string   `json:"query"`
			EntityIDs     []string `json:"entity_ids"`
			Cursor        *string  `json:"cursor"`
		}
		if e := transport.Read(r, &q); e != nil || q.SchemaVersion != 1 {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		v, e := s.Store.SearchMemoryPage(r.Context(), q.Query, q.EntityIDs, q.Cursor)
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("POST /v1/actions", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			SchemaVersion int                       `json:"schema_version"`
			RequestID     string                    `json:"request_id"`
			SessionID     json.RawMessage           `json:"session_id,omitempty"`
			Actions       []contract.ActionProposal `json:"actions"`
			DataClass     json.RawMessage           `json:"data_class,omitempty"`
		}
		if e := transport.Read(r, &q); e != nil || q.SchemaVersion != 1 {
			transport.Reply(w, 400, "", nil, errors.New("INVALID_SCHEMA"))
			return
		}
		class, err := declaredClass(q.DataClass)
		if err != nil {
			transport.Reply(w, 400, q.RequestID, nil, err)
			return
		}
		var session string
		if q.SessionID != nil {
			if string(q.SessionID) == "null" || json.Unmarshal(q.SessionID, &session) != nil || session == "" {
				transport.Reply(w, 400, q.RequestID, nil, errors.New("INVALID_SCHEMA"))
				return
			}
		}
		v, e := s.TypedClass(r.Context(), q.RequestID, session, q.Actions, class)
		transport.Reply(w, status(e), q.RequestID, v, e)
	})
	mux.HandleFunc("GET /v1/context/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.GetManifest(r.Context(), r.PathValue("id"))
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/events", func(w http.ResponseWriter, r *http.Request) {
		after, _ := strconv.Atoi(r.URL.Query().Get("after_seq"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		v, e := s.Store.EventPage(r.Context(), after, limit)
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.RuntimeTask(r.Context(), r.PathValue("id"))
		transport.Reply(w, status(e), "", v, e)
	})
	mux.HandleFunc("GET /v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		v, e := s.Store.RuntimePage(r.Context(), "scheduled_job", r.URL.Query().Get("cursor"), limit)
		transport.Reply(w, status(e), "", v, e)
	})
	s.registerTypedRoutes(mux)
	return mux
}
func status(e error) int {
	if e == nil {
		return 200
	}
	if strings.Contains(e.Error(), "CONFLICT") {
		return 409
	}
	return 400
}
