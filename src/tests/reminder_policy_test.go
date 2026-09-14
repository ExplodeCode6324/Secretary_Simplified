package tests

import (
	"context"
	"encoding/json"
	"os"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"strings"
	"testing"
	"time"
)

type reminderPolicyModel struct {
	model.Client
	actions  []contract.ActionProposal
	calls    int
	feedback bool
}

func (m *reminderPolicyModel) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	m.calls++
	b, _ := json.Marshal(r.Input)
	if strings.Contains(string(b), "REMINDER_POLICY_UNSUPPORTED") {
		m.feedback = true
	}
	raw, e := model.Fixture(r)
	if e != nil {
		return model.Result{}, e
	}
	var d contract.DecisionEnvelope
	if e = json.Unmarshal(raw, &d); e != nil {
		return model.Result{}, e
	}
	d.Actions = m.actions
	raw, e = json.Marshal(d)
	return model.Result{Output: raw}, e
}
func reminderActions(t *testing.T) []contract.ActionProposal {
	t.Helper()
	b, e := os.ReadFile("../../docs/examples/decision-create-reminder.json")
	if e != nil {
		t.Fatal(e)
	}
	var d contract.DecisionEnvelope
	if e = json.Unmarshal(b, &d); e != nil {
		t.Fatal(e)
	}
	stamp := contract.Timestamp(time.Now().Add(time.Minute))
	d.Actions[0].Payload["due_at"] = stamp
	d.Actions[1].Payload["schedule"].(map[string]any)["at"] = stamp
	return d.Actions
}
func TestNaturalReminderPolicyDefaultsAndAtomicRejection(t *testing.T) {
	for _, tc := range []struct {
		name, cap, misfire string
		grace              int
		accepted           bool
	}{
		{"default-notify", "notify.local", "FIRE_ONCE_WITHIN_GRACE", 300, true},
		{"skip-zero-notify", "notify.local", "SKIP", 0, false},
		{"custom-grace-notify", "notify.local", "FIRE_ONCE_WITHIN_GRACE", 60, false},
		{"skip-alarm", "alarm.play", "SKIP", 0, false},
		{"default-alarm", "alarm.play", "FIRE_ONCE_WITHIN_GRACE", 300, true},
		{"other-capability", "briefing.build", "SKIP", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c, p := setup(t)
			ctx := context.Background()
			g := probeGrant(t, s, []string{"notify.local", "alarm.play", "briefing.build"}, []string{})
			actions := reminderActions(t)
			job := actions[1].Payload
			job["misfire"] = tc.misfire
			job["grace_seconds"] = tc.grace
			if tc.cap != "notify.local" {
				cmd := d08Command(t, s, tc.cap)
				// Model proposals contain no program-owned metadata.
				delete(cmd.Extensions, contract.ClassificationKey)
				job["command"] = cmd
				criteria, e := store.DeriveCriteria(cmd)
				if e != nil {
					t.Fatal(e)
				}
				job["task_template"].(map[string]any)["criteria"] = criteria
			}
			m := &reminderPolicyModel{Client: p, actions: actions}
			svc := core.Service{Store: s, Config: c, Model: m, GrantID: g}
			turn, e := s.AcceptInput(ctx, input(contract.NewID()), 100)
			if e != nil {
				t.Fatal(e)
			}
			if e = svc.Process(ctx, turn); e != nil {
				t.Fatal(e)
			}
			got, e := s.GetTurn(ctx, turn.ID)
			if e != nil {
				t.Fatal(e)
			}
			for _, table := range []string{"item", "scheduled_job"} {
				var n int
				s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
				want := 0
				if tc.accepted {
					want = 1
				}
				if n != want {
					t.Fatalf("%s count=%d want=%d", table, n, want)
				}
			}
			if tc.accepted {
				if m.calls != 1 {
					t.Fatal("default retried", m.calls)
				}
				var policy string
				var grace int
				s.DB.QueryRow("SELECT json_extract(payload_json,'$.misfire'),json_extract(payload_json,'$.grace_seconds') FROM scheduled_job").Scan(&policy, &grace)
				if policy != tc.misfire || grace != tc.grace {
					t.Fatal(policy, grace)
				}
			} else {
				if m.calls != 3 || !m.feedback || got.Reply == nil || !strings.Contains((*got.Reply)["text"].(string), "REMINDER_POLICY_UNSUPPORTED") {
					t.Fatalf("incorrect bounded rejection: calls=%d feedback=%v reply=%v", m.calls, m.feedback, got.Reply)
				}
				for i := 0; i < 2; i++ {
					s.ScheduleStep(ctx, time.Now().Add(2*time.Minute))
				}
				for _, table := range []string{"task", "job_run", "notification"} {
					var n int
					s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
					if n != 0 {
						t.Fatalf("rejected decision created %s", table)
					}
				}
				if e = svc.Process(ctx, got); e != nil || m.calls != 3 {
					t.Fatal("rejected committed turn repeated", e, m.calls)
				}
			}
		})
	}
}
func TestTypedReminderExplicitSkipRemainsUnchanged(t *testing.T) {
	s, c, p := setup(t)
	actions := reminderActions(t)
	actions[1].Payload["misfire"] = "SKIP"
	actions[1].Payload["grace_seconds"] = 0
	svc := core.Service{Store: s, Config: c, Model: p, GrantID: runtimeGrant(t, s)}
	if _, e := svc.TypedClass(context.Background(), contract.NewID(), contract.NewID(), actions, "SYNTHETIC"); e != nil {
		t.Fatal(e)
	}
	var raw []byte
	s.DB.QueryRow("SELECT payload_json FROM scheduled_job").Scan(&raw)
	var job contract.ScheduledJob
	if e := json.Unmarshal(raw, &job); e != nil {
		t.Fatal(e)
	}
	if job.Misfire != "SKIP" || job.GraceSeconds != 0 {
		t.Fatal("typed policy altered")
	}
	due, _ := time.Parse(time.RFC3339Nano, *job.Schedule.At)
	if _, e := s.ScheduleStep(context.Background(), due.Add(904*time.Millisecond)); e != nil {
		t.Fatal(e)
	}
	var state string
	s.DB.QueryRow("SELECT state FROM job_run").Scan(&state)
	if state != "SKIPPED" {
		t.Fatal(state)
	}
}
