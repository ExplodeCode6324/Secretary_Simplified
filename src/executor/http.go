package executor

import (
	"errors"
	"net/http"
	"secretarysimplified/transport"
	"strconv"
)

type controlInput struct {
	SchemaVersion    int    `json:"schema_version"`
	RequestID        string `json:"request_id"`
	ExpectedRevision int    `json:"expected_revision"`
	DelaySeconds     int    `json:"delay_seconds"`
}

func readControl(q *http.Request) (controlInput, error) {
	var in controlInput
	e := transport.Read(q, &in)
	if e == nil && (in.SchemaVersion != 1 || in.RequestID == "") {
		e = errors.New("SCHEMA_VERSION_AND_REQUEST_ID_REQUIRED")
	}
	return in, e
}
func (r *Runner) Handler() http.Handler {
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, id string, v any, e error) {
		status := 200
		if e != nil {
			status = 400
			if e.Error() == "REVISION_CONFLICT" || e.Error() == "IDEMPOTENCY_CONFLICT" {
				status = 409
			}
		}
		transport.Reply(w, status, id, v, e)
	}
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, q *http.Request) {
		reply(w, "", map[string]any{"runner": "ONLINE", "replay": r.Options.Replay}, nil)
	})
	for path, table := range map[string]string{"tasks": "task", "jobs": "scheduled_job", "alarms": "alarm_session", "notifications": "notification"} {
		path, table := path, table
		mux.HandleFunc("GET /v1/"+path, func(w http.ResponseWriter, q *http.Request) {
			limit := 50
			if raw := q.URL.Query().Get("limit"); raw != "" {
				n, e := strconv.Atoi(raw)
				if e != nil {
					reply(w, "", nil, errors.New("INVALID_LIMIT"))
					return
				}
				limit = n
			}
			v, e := r.Store.RuntimePage(q.Context(), table, q.URL.Query().Get("cursor"), limit)
			reply(w, "", v, e)
		})
	}
	mux.HandleFunc("GET /v1/tasks/{id}", func(w http.ResponseWriter, q *http.Request) {
		v, e := r.Store.RuntimeTask(q.Context(), q.PathValue("id"))
		reply(w, "", v, e)
	})
	mux.HandleFunc("GET /v1/jobs/{id}", func(w http.ResponseWriter, q *http.Request) {
		v, e := r.Store.RuntimeJob(q.Context(), q.PathValue("id"))
		reply(w, "", v, e)
	})
	mux.HandleFunc("GET /v1/runs/{id}", func(w http.ResponseWriter, q *http.Request) {
		v, e := r.Store.GetRun(q.Context(), q.PathValue("id"))
		reply(w, "", v, e)
	})
	mux.HandleFunc("POST /v1/notifications/{id}/ack", func(w http.ResponseWriter, q *http.Request) {
		in, e := readControl(q)
		if e != nil {
			reply(w, in.RequestID, nil, e)
			return
		}
		e = r.Store.AckNotification(q.Context(), q.PathValue("id"), in.RequestID)
		reply(w, in.RequestID, map[string]any{"acknowledged": e == nil}, e)
	})
	mux.HandleFunc("POST /v1/tasks/{id}/cancel", func(w http.ResponseWriter, q *http.Request) {
		in, e := readControl(q)
		if e != nil {
			reply(w, in.RequestID, nil, e)
			return
		}
		e = r.Store.CancelTaskRequest(q.Context(), q.PathValue("id"), in.RequestID, in.ExpectedRevision)
		if e == nil {
			r.cancelActive(q.PathValue("id"))
		}
		reply(w, in.RequestID, map[string]any{"cancel_ack": e == nil}, e)
	})
	mux.HandleFunc("POST /v1/runs/{id}/cancel", func(w http.ResponseWriter, q *http.Request) {
		in, e := readControl(q)
		if e != nil {
			reply(w, in.RequestID, nil, e)
			return
		}
		run, e := r.Store.GetRun(q.Context(), q.PathValue("id"))
		if e == nil {
			e = r.Store.CancelTaskRequest(q.Context(), run.TaskID, in.RequestID, in.ExpectedRevision)
			if e == nil {
				r.cancelActive(run.TaskID)
			}
		}
		reply(w, in.RequestID, map[string]any{"cancel_ack": e == nil}, e)
	})
	for _, action := range []string{"pause", "resume"} {
		action := action
		mux.HandleFunc("POST /v1/jobs/{id}/"+action, func(w http.ResponseWriter, q *http.Request) {
			in, e := readControl(q)
			if e != nil {
				reply(w, in.RequestID, nil, e)
				return
			}
			if action == "resume" && r.Frozen() {
				reply(w, in.RequestID, nil, errors.New("EXECUTION_FROZEN"))
				return
			}
			e = r.Store.SetJobEnabled(q.Context(), q.PathValue("id"), in.RequestID, in.ExpectedRevision, action == "resume")
			reply(w, in.RequestID, map[string]any{"updated": e == nil}, e)
		})
	}
	mux.HandleFunc("POST /v1/jobs/{id}/trigger", func(w http.ResponseWriter, q *http.Request) {
		in, e := readControl(q)
		if e != nil {
			reply(w, in.RequestID, nil, e)
			return
		}
		if r.Frozen() {
			reply(w, in.RequestID, nil, errors.New("EXECUTION_FROZEN"))
			return
		}
		v, e := r.Store.TriggerJob(q.Context(), q.PathValue("id"), in.RequestID, r.Options.Clock.Now(), in.ExpectedRevision)
		reply(w, in.RequestID, v, e)
	})
	for _, action := range []string{"stop", "snooze"} {
		action := action
		mux.HandleFunc("POST /v1/alarms/{id}/"+action, func(w http.ResponseWriter, q *http.Request) {
			in, e := readControl(q)
			if e != nil {
				reply(w, in.RequestID, nil, e)
				return
			}
			if action == "snooze" && (in.DelaySeconds < 1 || r.Frozen()) {
				reply(w, in.RequestID, nil, errors.New("INVALID_OR_FROZEN_SNOOZE"))
				return
			}
			if action == "stop" {
				in.DelaySeconds = 0
			}
			e = r.Store.AlarmControl(q.Context(), q.PathValue("id"), in.RequestID, in.DelaySeconds, r.Options.Clock.Now())
			reply(w, in.RequestID, map[string]any{"cancel_ack": e == nil, "adapter": "muted"}, e)
		})
	}
	return mux
}
func (r *Runner) Frozen() bool {
	if r.Options.Replay {
		return true
	}
	if r.Options.Frozen != nil {
		return r.Options.Frozen()
	}
	return false
}
