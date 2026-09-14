package tui

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"net/url"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/transport"
	"strings"
	"sync"
	"testing"
	"time"
)

type residualCaller struct {
	submitCode                string
	successStatus             int
	invalidReceipt            bool
	invalidate                bool
	mu                        sync.Mutex
	calls                     []string
	bodies                    []contract.InputEnvelope
	submitStatus, queryStatus int
	timeout, lost             bool
	done                      bool
	authority                 Authority
	turnID                    string
	revision                  int
	removed                   string
}

func (f *residualCaller) Call(ctx context.Context, method, path string, body any) (transport.Envelope, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method+" "+path)
	if method == "POST" && path == "/v1/inputs" {
		in := body.(contract.InputEnvelope)
		f.bodies = append(f.bodies, in)
		if f.lost {
			return transport.Envelope{}, 0, context.DeadlineExceeded
		}
		if f.submitStatus >= 400 {
			code := f.submitCode
			if code == "" {
				code = "INVALID_SCHEMA"
			}
			return transport.Envelope{Error: map[string]any{"code": code}}, f.submitStatus, nil
		}
		status := f.successStatus
		if status == 0 {
			status = 202
		}
		turnID := f.turnID
		if f.invalidReceipt {
			turnID = "not-a-uuid"
		}
		return transport.Envelope{Result: map[string]any{"turn_id": turnID}}, status, nil
	}
	if path == "/v1/conversation" {
		return transport.Envelope{Result: f.authority}, 200, nil
	}
	if strings.HasPrefix(path, "/v1/conversation/history") {
		return transport.Envelope{Result: History{SessionID: f.authority.SessionID, Events: []contract.ConversationEvent{{Sequence: 1, ID: "one", TurnID: f.turnID, Text: "固定完成回复", Role: "ASSISTANT"}}}}, 200, nil
	}
	if strings.HasPrefix(path, "/v1/requests/") {
		return transport.Envelope{Result: map[string]any{"turn_id": f.turnID}}, 200, nil
	}
	if strings.HasPrefix(path, "/v1/turns/") {
		if f.timeout {
			<-ctx.Done()
			return transport.Envelope{}, 0, ctx.Err()
		}
		if f.queryStatus >= 400 {
			return transport.Envelope{Error: map[string]any{"code": "QUERY_DENIED"}}, f.queryStatus, nil
		}
		rid := ""
		if len(f.bodies) > 0 {
			rid = f.bodies[0].RequestID
		}
		state := "ACCEPTED"
		if f.done {
			state = "COMMITTED"
		}
		return transport.Envelope{Result: contract.InputTurn{ID: f.turnID, RequestID: rid, State: state, SessionID: f.authority.SessionID}}, 200, nil
	}
	u, _ := url.Parse(path)
	name := strings.TrimPrefix(u.Path, "/v1/")
	if strings.HasPrefix(u.Path, "/v1/items/") {
		id := strings.TrimPrefix(u.Path, "/v1/items/")
		if id == f.removed {
			return transport.Envelope{Error: map[string]any{"code": "NOT_FOUND"}}, 404, nil
		}
		return transport.Envelope{Result: map[string]any{"id": id, "title": id, "revision": float64(f.revision)}}, 200, nil
	}
	if name == "items" && u.Query().Get("cursor") != "" && f.invalidate {
		return transport.Envelope{Error: map[string]any{"code": "CONFLICT"}}, 409, nil
	}

	first := 0
	next := "page2"
	if u.Query().Get("cursor") == "page2" {
		first = 50
		next = "page3"
	}
	rows := []map[string]any{}
	for i := first; i < first+50; i++ {
		id := fmt.Sprintf("%s-%02d", name, i)
		if id != f.removed {
			rows = append(rows, map[string]any{"id": id, "title": id, "goal": id, "text": id, "revision": float64(f.revision), "enabled": true, "state": "RUNNING"})
		}
	}
	return transport.Envelope{Result: Page{Items: rows, NextCursor: &next}}, 200, nil
}
func residualSetup(t *testing.T) (Model, *residualCaller) {
	m := testModel(t)
	f := &residualCaller{turnID: contract.NewID(), authority: m.authority, revision: 1}
	m.opts.Core = f
	m.opts.Runner = f
	return m, f
}
func residualPendingFile(m Model, id string) bool {
	_, e := os.Stat(filepath.Join(m.opts.RecoveryDir, id+".json"))
	return e == nil
}
func residualKnownTurn(m Model, id, turn string) bool {
	v, ok := m.turns[turn]
	return ok && v.ID == turn && v.RequestID == id
}
func residualPanelMessage(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			if child == nil {
				continue
			}
			v := child()
			if _, ok := v.(panelMsg); ok {
				return v
			}
		}
		t.Fatal("no panel query in tick")
	}
	return msg
}
func residualLoadTwo(t *testing.T, name string) (Model, *residualCaller) {
	t.Helper()
	m, f := residualSetup(t)
	cmd := m.showPanel(name)
	m, _ = step(m, cmd())
	m, cmd = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = step(m, cmd())
	if len(m.rows) != 100 {
		t.Fatal("precondition: two pages", len(m.rows))
	}
	m.selected = 75
	return m, f
}
func residualSelected(m Model) string {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return ""
	}
	id, _ := m.rows[m.selected]["id"].(string)
	return id
}

func TestIssue2ResidualR101AcceptedQuery4xx(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			m, f := residualSetup(t)
			f.queryStatus = code
			m.input.SetValue("原始合成输入")
			cmd := m.send()
			msg := cmd()
			id := f.bodies[0].RequestID
			m, _ = step(m, msg)
			if !residualPendingFile(m, id) || m.pending[id] == "" || !residualKnownTurn(m, id, f.turnID) || m.input.Value() != "" || !strings.Contains(m.notice, "已受理") || strings.Contains(m.notice, "被后端拒绝") || !strings.Contains(m.notice, "QUERY_DENIED") {
				t.Fatalf("accepted observation was downgraded: pending=%v metadata=%v knownTurn=%v draft=%q notice=%q", m.pending, residualPendingFile(m, id), residualKnownTurn(m, id, f.turnID), m.input.Value(), m.notice)
			}
			if len(f.bodies) != 1 || len(f.calls) != 2 || f.calls[0] != "POST /v1/inputs" || f.calls[1] != "GET /v1/turns/"+f.turnID {
				t.Fatal(f.calls)
			}
		})
	}
}
func TestIssue2ResidualR102UnknownObservationRebuildAndLateResult(t *testing.T) {
	for _, mode := range []string{"timeout", "server_error"} {
		t.Run(mode, func(t *testing.T) {
			m, f := residualSetup(t)
			m.opts.CallTimeout = 5 * time.Millisecond
			f.timeout = mode == "timeout"
			if !f.timeout {
				f.queryStatus = 503
			}
			m.input.SetValue("唯一提交")
			cmd := m.send()
			late := cmd()
			id := f.bodies[0].RequestID
			m, _ = step(m, late)
			if !residualPendingFile(m, id) || !residualKnownTurn(m, id, f.turnID) || !strings.Contains(m.notice, "已受理") {
				t.Fatal("accepted identity lost")
			}
			f.timeout = false
			f.queryStatus = 403
			rebuiltWhileDenied := New(context.Background(), m.opts)
			rebuiltWhileDenied, _ = step(rebuiltWhileDenied, rebuiltWhileDenied.sync()())
			if !residualKnownTurn(rebuiltWhileDenied, id, f.turnID) || !residualPendingFile(rebuiltWhileDenied, id) || !strings.Contains(rebuiltWhileDenied.pending[id], "已受理") {
				t.Fatal("request receipt identity lost while result still forbidden")
			}
			f.queryStatus = 0
			f.done = true
			rebuilt := New(context.Background(), m.opts)
			rebuilt, _ = step(rebuilt, rebuilt.sync()())
			if residualPendingFile(rebuilt, id) || len(rebuilt.pending) != 0 || rebuilt.turns[f.turnID].State != "COMMITTED" {
				t.Fatal("terminal recovery failed")
			}
			rebuilt, _ = step(rebuilt, late)
			rebuilt, _ = step(rebuilt, syncMsg{authority: f.authority, turns: []contract.InputTurn{{ID: f.turnID, RequestID: id, State: "ACCEPTED"}}, unresolved: []string{id}})
			if len(rebuilt.pending) != 0 || rebuilt.turns[f.turnID].State != "COMMITTED" || rebuilt.input.Value() != "" || strings.Contains(rebuilt.notice, "拒绝") {
				t.Fatal("late observation regressed terminal")
			}
			rebuilt.mergeHistory(History{Events: []contract.ConversationEvent{{Sequence: 1, ID: "one", TurnID: f.turnID, Text: "固定完成回复", Role: "ASSISTANT"}}})
			if len(rebuilt.events) != 1 || len(f.bodies) != 1 {
				t.Fatal("duplicate turn display or POST")
			}
			if !strings.Contains(strings.Join(f.calls, "\n"), "GET /v1/requests/"+id) {
				t.Fatal("did not query original request")
			}
		})
	}
}
func TestIssue2ResidualR103SubmissionRejectionVersusLostResponse(t *testing.T) {
	for _, mode := range []string{"rejected_empty", "rejected_new_draft", "lost"} {
		t.Run(mode, func(t *testing.T) {
			m, f := residualSetup(t)
			f.lost = mode == "lost"
			if !f.lost {
				f.submitStatus = 400
			}
			m.input.SetValue("原输入")
			cmd := m.send()
			if mode == "rejected_new_draft" {
				m.input.SetValue("另一份未发草稿")
			}
			msg := cmd()
			id := f.bodies[0].RequestID
			m, _ = step(m, msg)
			if f.lost {
				if !residualPendingFile(m, id) || m.pending[id] == "" || m.input.Value() != "" {
					t.Fatal("unknown POST lost tracking")
				}
			} else {
				expected := "原输入"
				if mode == "rejected_new_draft" {
					expected = "另一份未发草稿"
				}
				if residualPendingFile(m, id) || len(m.pending) != 0 || m.input.Value() != expected {
					t.Fatal("definite rejection mishandled")
				}
			}
			if len(f.bodies) != 1 || len(f.calls) != 1 {
				t.Fatal("submission automatically retried or queried after rejection")
			}
		})
	}
}
func TestIssue2ResidualR201PanelRefreshKeepsSecondPage(t *testing.T) {
	for _, name := range []string{"tasks", "jobs", "items", "notifications"} {
		t.Run(name, func(t *testing.T) {
			m, f := residualLoadTwo(t, name)
			id := residualSelected(m)
			m, cmd := step(m, tickMsg{})
			m, _ = step(m, residualPanelMessage(t, cmd))
			if len(m.rows) != 100 || residualSelected(m) != id || m.panelCursor == nil || *m.panelCursor != "page3" {
				t.Fatalf("refresh replaced loaded range: count=%d selected=%s", len(m.rows), residualSelected(m))
			}
			seen := map[string]bool{}
			for _, row := range m.rows {
				id := row["id"].(string)
				if seen[id] {
					t.Fatal("duplicate ID")
				}
				seen[id] = true
			}
			for _, call := range f.calls {
				if strings.HasPrefix(call, "POST") {
					t.Fatal("refresh wrote state")
				}
			}
		})
	}
}
func TestIssue2ResidualR202PanelResponsesIgnoreOldScope(t *testing.T) {
	for _, name := range []string{"tasks", "jobs", "items", "notifications"} {
		t.Run(name, func(t *testing.T) {
			m, _ := residualSetup(t)
			cmd := m.showPanel(name)
			m, _ = step(m, cmd())
			m, refresh := step(m, tickMsg{})
			old := residualPanelMessage(t, refresh)
			m, next := step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
			newPage := next()
			m, _ = step(m, newPage)
			m.selected = 75
			id := residualSelected(m)
			m, _ = step(m, old)
			m, _ = step(m, newPage)
			if len(m.rows) != 100 || residualSelected(m) != id || *m.panelCursor != "page3" {
				t.Fatal("late or duplicate response replaced newer page")
			}
			stale := m.openPanel(name, true)()
			cmd = m.showPanel(name)
			m, _ = step(m, cmd())
			m, _ = step(m, stale)
			if len(m.rows) != 50 || *m.panelCursor != "page2" {
				t.Fatal("previous panel visit response applied after reopen")
			}
		})
	}
}
func TestIssue2ResidualR203RefreshVersionConfirmationAndRemovedTarget(t *testing.T) {
	for _, name := range []string{"tasks", "jobs", "items", "notifications"} {
		t.Run(name, func(t *testing.T) {
			m, f := residualLoadTwo(t, name)
			id := residualSelected(m)
			m.prepareControl()
			captured := m.confirm
			f.revision = 2
			cmd := m.openPanel(name, false)
			m, _ = step(m, cmd())
			if residualSelected(m) != id || m.rows[m.selected]["revision"] != float64(2) {
				t.Fatal("visible target version not refreshed")
			}
			if captured != nil && (m.confirm == nil || m.confirm.id != id || m.confirm.revision != 1) {
				t.Fatal("confirmation rebound on refresh")
			}
			f.removed = id
			cmd = m.openPanel(name, false)
			m, _ = step(m, cmd())
			if residualSelected(m) != "" || !strings.Contains(m.notice, "目标") {
				t.Fatal("removed target silently selected a different object")
			}
			if captured != nil && (m.confirm.id != id || m.confirm.revision != 1) {
				t.Fatal("old confirmation identity changed")
			}
			for _, call := range f.calls {
				if strings.HasPrefix(call, "POST") {
					t.Fatal("refresh auto mutation")
				}
			}
		})
	}
}

func TestIssue2ResidualR203ItemsInvalidatedCursorKeepsExplicitNavigation(t *testing.T) {
	m, f := residualLoadTwo(t, "items")
	id := residualSelected(m)
	f.revision = 2
	f.invalidate = true
	cmd := m.openPanel("items", false)
	m, _ = step(m, cmd())
	if len(m.rows) != 100 || residualSelected(m) != id || m.rows[m.selected]["revision"] != float64(2) || !strings.Contains(m.notice, "失效") {
		t.Fatal("invalidated cursor silently left selected item stale or replaced range")
	}
	count := len(f.calls)
	m, cmd = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd != nil {
		cmd()
	}
	if len(f.calls) != count {
		t.Fatal("n followed invalidated cursor")
	}
	m, cmd = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Fatal("explicit reset missing")
	}
	m, _ = step(m, cmd())
	if len(m.rows) != 50 || m.panelCursor == nil || *m.panelCursor != "page2" || residualSelected(m) != "" {
		t.Fatal("explicit reset did not obtain first-page snapshot or rebound different ID")
	}
}

func TestIssue2ResidualR103AmbiguousSubmissionCodesAndUnexpectedSuccess(t *testing.T) {
	for _, code := range []string{"IDEMPOTENCY_CONFLICT", "REQUEST_REJECTED", "BUDGET_EXHAUSTED", "CONTEXT_REQUIRED_OVERFLOW"} {
		t.Run(code, func(t *testing.T) {
			m, f := residualSetup(t)
			f.submitStatus = 409
			f.submitCode = code
			m.input.SetValue("不可因不确定回执重提")
			cmd := m.send()
			m, _ = step(m, cmd())
			id := f.bodies[0].RequestID
			if !residualPendingFile(m, id) || m.pending[id] == "" || m.input.Value() != "" || len(f.bodies) != 1 {
				t.Fatal("ambiguous submit treated as definite refusal")
			}
		})
	}
	for _, mode := range []string{"unexpected_200", "bad_202_receipt"} {
		t.Run(mode, func(t *testing.T) {
			m, f := residualSetup(t)
			if mode == "unexpected_200" {
				f.successStatus = 200
			} else {
				f.invalidReceipt = true
			}
			m.input.SetValue("未知受理形状")
			cmd := m.send()
			m, _ = step(m, cmd())
			id := f.bodies[0].RequestID
			if !residualPendingFile(m, id) || m.pending[id] == "" || m.input.Value() != "" || len(f.calls) != 1 {
				t.Fatal("unexpected response advanced to known acceptance")
			}
		})
	}
}
func TestIssue2ResidualR203ItemsStaleTicksAndMissingTarget(t *testing.T) {
	m, f := residualLoadTwo(t, "items")
	id := residualSelected(m)
	f.invalidate = true
	m, _ = step(m, m.openPanel("items", false)())
	start := len(f.calls)
	f.revision = 3
	m.syncing = false
	m, cmd := step(m, tickMsg{})
	m, _ = step(m, residualPanelMessage(t, cmd))
	itemCalls := []string{}
	for _, call := range f.calls[start:] {
		if strings.Contains(call, "/v1/items") {
			itemCalls = append(itemCalls, call)
		}
	}
	if len(itemCalls) != 1 || itemCalls[0] != "GET /v1/items/"+id || m.rows[m.selected]["revision"] != float64(3) {
		t.Fatal("stale tick repeated dead cursor or missed new target version", itemCalls)
	}
	f.removed = id
	m, _ = step(m, m.openPanel("items", false)())
	if residualSelected(m) != "" || !strings.Contains(m.notice, "目标已不存在") {
		t.Fatal("404 quietly selected another target")
	}
}

func TestIssue2ResidualR202AppendWithoutNewIDsStopsNavigation(t *testing.T) {
	for _, name := range []string{"tasks", "jobs", "items", "notifications"} {
		t.Run(name, func(t *testing.T) {
			m, f := residualLoadTwo(t, name)
			id := m.selectedID()
			cmd := m.openPanel(name, true)
			// The synthetic server's page3 repeats page1 with another nonempty cursor.
			m, _ = step(m, cmd())
			if len(m.rows) != 100 || m.selectedID() != id || m.panelCursor != nil || !strings.Contains(m.notice, "新 ID") {
				t.Fatal("zero-progress append was not stopped", len(m.rows), m.notice)
			}
			count := len(f.calls)
			m, cmd = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
			if cmd != nil {
				cmd()
			}
			if len(f.calls) != count {
				t.Fatal("continued cursor after zero-progress append")
			}
			cmd = m.openPanel(name, false)
			m, _ = step(m, cmd())
			if m.panelCursor != nil {
				t.Fatal("tick silently re-enabled stopped pagination")
			}
		})
	}
}

func TestIssue2ResidualR203ResetFailureRetainsSelectionIntent(t *testing.T) {
	for _, selected := range []int{25, 75} {
		t.Run(fmt.Sprint(selected), func(t *testing.T) {
			m, _ := residualLoadTwo(t, "items")
			m.selected = selected
			id := m.selectedID()
			m.panelInvalid = true
			cmd := m.resetPanel()
			v := cmd().(panelMsg)
			v.err = &APIError{Status: 503, Code: "UNAVAILABLE"}
			m, _ = step(m, v)
			if m.panelSelection != id || m.selectedID() != "" {
				t.Fatal("failed reset consumed target intent")
			}
			cmd = m.openPanel("items", false)
			m, _ = step(m, cmd())
			if selected == 25 && m.selectedID() != id {
				t.Fatal("retry did not restore original ID")
			}
			if selected == 75 && (m.selectedID() != "" || !strings.Contains(m.notice, "原选中")) {
				t.Fatal("retry selected unrelated first row")
			}
			if m.panelSelection != "" {
				t.Fatal("successful reset did not consume target intent")
			}
		})
	}
}

func TestIssue2ResidualR203RepeatedResetRetainsSelectionIntent(t *testing.T) {
	for _, selected := range []int{25, 75} {
		t.Run(fmt.Sprint(selected), func(t *testing.T) {
			m, _ := residualLoadTwo(t, "items")
			m.selected = selected
			id := m.selectedID()
			m.panelInvalid = true
			cmd := m.resetPanel()
			v := cmd().(panelMsg)
			v.err = &APIError{Status: 503, Code: "UNAVAILABLE"}
			m, _ = step(m, v)
			cmd = m.resetPanel()
			m, _ = step(m, cmd())
			if selected == 25 && m.selectedID() != id {
				t.Fatal("repeated reset lost present original ID")
			}
			if selected == 75 && (m.selectedID() != "" || !strings.Contains(m.notice, "原选中")) {
				t.Fatal("repeated reset selected unrelated row")
			}
		})
	}
}
