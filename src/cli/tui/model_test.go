package tui

import (
	"context"
	"encoding/json"
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/transport"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeCaller struct {
	mu    sync.Mutex
	calls []string
	fn    func(string, string, any) (transport.Envelope, int, error)
}

func (f *fakeCaller) Call(_ context.Context, method, path string, body any) (transport.Envelope, int, error) {
	f.mu.Lock()
	f.calls = append(f.calls, method+" "+path)
	f.mu.Unlock()
	if f.fn != nil {
		return f.fn(method, path, body)
	}
	return transport.Envelope{Result: map[string]any{}}, 200, nil
}
func testModel(t *testing.T) Model {
	f := &fakeCaller{}
	m := New(context.Background(), Options{Core: f, Runner: f, Config: config.Default(t.TempDir()), RecoveryDir: t.TempDir(), DataClass: "SYNTHETIC", CallTimeout: 20 * time.Millisecond})
	m.initialized = true
	m.coreStatus = "已连接"
	m.authority = Authority{InstanceID: contract.NewID(), SessionID: contract.NewID()}
	return m
}
func step(m Model, msg tea.Msg) (Model, tea.Cmd) { out, cmd := m.Update(msg); return out.(Model), cmd }
func TestIssue2TUIChinesePasteResizeDraftAndDelayedWork(t *testing.T) {
	m := testModel(t)
	paste := "第一行中文\n第二行草稿\x13不是发送"
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(paste), Paste: true})
	if len(m.pending) != 0 || m.input.Value() != "第一行中文\n第二行草稿不是发送" {
		t.Fatal("paste submitted or corrupted", m.input.Value())
	}
	m, _ = step(m, tea.WindowSizeMsg{Width: 42, Height: 18})
	if !strings.Contains(m.input.Value(), "第二行") {
		t.Fatal("resize lost draft")
	}
	cmd := m.send()
	if cmd == nil || len(m.pending) != 1 || m.input.Value() != "" {
		t.Fatal("submit failed")
	}
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("等候时继续写中文")})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyPgUp})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyF3})
	if m.input.Value() != "等候时继续写中文" || m.panel != "tasks" {
		t.Fatal("pending work blocked UI")
	}
	// A timeout does not manufacture terminal business failure or a fresh request.
	id := ""
	for k := range m.pending {
		id = k
	}
	m, _ = step(m, sentMsg{id: id, err: errors.New("DEPENDENCY_UNAVAILABLE")})
	if len(m.pending) != 1 || m.pending[id] == "" {
		t.Fatal("lost stable pending ID")
	}
}
func TestIssue2TUISafeRenderAllExternalText(t *testing.T) {
	bad := "中文\x1b]52;c;ZXZpbA==\a\x1b[2J\x1b[31m红色\u009b31m\x00\x7f"
	safe := SafeText(bad)
	if strings.ContainsAny(safe, "\x1b\u009b\x00\x7f") || strings.Contains(safe, "ZXZpbA") {
		t.Fatal(safe)
	}
	if !strings.Contains(safe, "中文") || !strings.Contains(safe, "红色") {
		t.Fatal("lost safe unicode")
	}
	m := testModel(t)
	m.authority.State.Summary = bad
	m.panel = "status"
	if strings.Contains(m.View(), "\x1b]52") {
		t.Fatal("summary escape leaked")
	}
	m.panel = "items"
	m.rows = []map[string]any{{"id": bad, "title": bad, "revision": 1.0}}
	m.debug = true
	if strings.Contains(m.View(), "\x1b]52") {
		t.Fatal("panel escape leaked")
	}
}
func TestIssue2TUIRecoveryIDsOnlyAndInstanceIsolation(t *testing.T) {
	dir := t.TempDir()
	instance := contract.NewID()
	id := contract.NewID()
	if e := savePending(dir, instance, id); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(dir, id+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]any
	if json.Unmarshal(b, &v) != nil || len(v) != 2 {
		t.Fatal("not minimal ID metadata")
	}
	st, _ := os.Stat(filepath.Join(dir, id+".json"))
	if st.Mode().Perm() != 0600 {
		t.Fatal(st.Mode())
	}
	if len(loadPending(dir, contract.NewID())) != 0 || len(loadPending(dir, instance)) != 1 {
		t.Fatal("instance mixed")
	}
	m := testModel(t)
	m.opts.RecoveryDir = dir
	m.initialized = false
	m.opts.Core = &fakeCaller{fn: func(method, path string, body any) (transport.Envelope, int, error) {
		switch {
		case path == "/v1/conversation":
			return transport.Envelope{Result: Authority{InstanceID: instance, SessionID: contract.NewID()}}, 200, nil
		case strings.Contains(path, "history"):
			return transport.Envelope{Result: History{}}, 200, nil
		case strings.Contains(path, "/requests/"):
			return transport.Envelope{Result: map[string]any{"turn_id": "known-turn"}}, 200, nil
		default:
			return transport.Envelope{Result: contract.InputTurn{ID: "known-turn", RequestID: id, State: "COMMITTED"}}, 200, nil
		}
	}}
	msg := m.sync()().(syncMsg)
	m, _ = step(m, msg)
	if len(m.pending) != 0 || len(loadPending(dir, instance)) != 0 {
		t.Fatal("recovered request not completed")
	}
	f := m.opts.Core.(*fakeCaller)
	for _, call := range f.calls {
		if strings.HasPrefix(call, "POST") {
			t.Fatal("recovery redispatched", call)
		}
	}
}
func TestIssue2TUIStructuredQuestionAndExplicitControl(t *testing.T) {
	m := testModel(t)
	id := contract.NewID()
	m.authority.State.PendingQuestions = []map[string]any{{"id": id, "text": "截止日期？", "resolved": false}}
	m.showPanel("questions")
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.answerID != id {
		t.Fatal("question not bound")
	}
	m.input.SetValue("明天下午")
	m.send()
	for _, in := range m.envelopes {
		if in.AnswerToQuestionID == nil || *in.AnswerToQuestionID != id || in.SessionID != m.authority.SessionID {
			t.Fatal("wrong answer identity")
		}
	}
	m.showPanel("notifications")
	m.rows = []map[string]any{{"id": "notification", "text": "待确认"}}
	f := m.opts.Runner.(*fakeCaller)
	if len(f.calls) != 0 {
		t.Fatal("browse acknowledged")
	}
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirm == nil || len(f.calls) != 0 {
		t.Fatal("no explicit confirmation")
	}
	m, cmd := step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("missing control")
	}
	cmd()
	if len(f.calls) != 1 || f.calls[0] != "POST /v1/notifications/notification/ack" {
		t.Fatal(f.calls)
	}
}
func TestIssue2TUIAuthorityDriftAndRejectedInput(t *testing.T) {
	m := testModel(t)
	original := m.authority.InstanceID
	m, _ = step(m, syncMsg{authority: Authority{InstanceID: contract.NewID(), SessionID: contract.NewID()}})
	if m.authority.InstanceID != original || m.coreStatus != "实例已变化" {
		t.Fatal("instance silently replaced")
	}
	m.input.SetValue("新草稿")
	if m.send() != nil {
		t.Fatal("drift submitted")
	}
	m.coreStatus = "已连接"
	m.send()
	id := ""
	for k := range m.pending {
		id = k
	}
	m, _ = step(m, sentMsg{id: id, err: &APIError{403, "DISCLOSURE_DENIED"}})
	if len(m.pending) != 0 || m.input.Value() != "新草稿" || !strings.Contains(m.notice, "拒绝") {
		t.Fatal("rejection shown pending or lost draft")
	}
}
func TestIssue2TUIHistoryOrderedDeduplicated(t *testing.T) {
	m := testModel(t)
	m.mergeHistory(History{Events: []contract.ConversationEvent{{Sequence: 2, Role: "ASSISTANT", Text: "回复二"}, {Sequence: 1, Role: "MASTER", Text: "输入一"}}})
	m.mergeHistory(History{Events: []contract.ConversationEvent{{Sequence: 2, Role: "ASSISTANT", Text: "回复二"}}})
	if len(m.events) != 2 || strings.Count(m.history.View(), "回复二") != 1 || strings.Index(m.history.View(), "输入一") > strings.Index(m.history.View(), "回复二") {
		t.Fatal("history fork or duplicate")
	}
}

func TestIssue2TUIJobControlsRetainSnapshotRevisionAndConflict(t *testing.T) {
	m := testModel(t)
	m.panel = "jobs"
	m.rows = []map[string]any{{"id": "job-a", "enabled": true, "revision": 7.0, "name": "合成计划"}}
	m.prepareControl()
	target := *m.confirm
	m.rows[0]["revision"] = 8.0
	if target.revision != 7 || target.verb != "pause" || target.id != "job-a" {
		t.Fatal("confirmation did not capture target")
	}
	f := &fakeCaller{fn: func(method, path string, body any) (transport.Envelope, int, error) {
		v := body.(map[string]any)
		if v["expected_revision"] != 7 || v["request_id"] != target.request {
			t.Fatal("changed control identity")
		}
		return transport.Envelope{Error: map[string]any{"code": "REVISION_CONFLICT"}}, 409, nil
	}}
	m.opts.Runner = f
	msg := m.perform(target)()
	m, _ = step(m, msg)
	if !strings.Contains(m.notice, "REVISION_CONFLICT") || len(f.calls) != 1 {
		t.Fatal("conflict hidden or auto retried")
	}
	m.rows[0]["enabled"] = false
	m.prepareControl()
	if m.confirm.verb != "resume" || m.confirm.revision != 8 {
		t.Fatal("resume not based on refreshed row")
	}
}
func TestIssue2TUIWrapPreservesLongChineseAndDraft(t *testing.T) {
	m := testModel(t)
	text := strings.Repeat("中文很长的原话", 30)
	m, _ = step(m, tea.WindowSizeMsg{Width: 30, Height: 16})
	m.mergeHistory(History{Events: []contract.ConversationEvent{{Sequence: 1, Text: text}}})
	if m.history.TotalLineCount() < 10 {
		t.Fatal("long original text was truncated rather than wrapped")
	}
	m.input.SetValue("窗口缩放中的草稿")
	m, _ = step(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.input.Value() != "窗口缩放中的草稿" {
		t.Fatal("resize discarded draft")
	}
}

func TestIssue2TUIClassificationPreservedAndPrivateDraftNotArchived(t *testing.T) {
	for _, class := range []string{"PERSONAL", "SECRET"} {
		t.Run(class, func(t *testing.T) {
			m := testModel(t)
			m.opts.DataClass = class
			canary := "invented-private-canary-" + class
			m.input.SetValue(canary)
			m.send()
			for id, in := range m.envelopes {
				if in.DataClass != class {
					t.Fatal("classification downgraded")
				}
				b, e := os.ReadFile(filepath.Join(m.opts.RecoveryDir, id+".json"))
				if e != nil || bytesContain(b, canary) {
					t.Fatal("private body archived", e)
				}
			}
		})
	}
}
func bytesContain(b []byte, s string) bool { return strings.Contains(string(b), s) }

type delayedCaller struct{}

func (delayedCaller) Call(ctx context.Context, method, path string, body any) (transport.Envelope, int, error) {
	<-ctx.Done()
	return transport.Envelope{}, 0, ctx.Err()
}
func TestIssue2TUIInjectedShortTimeoutNeverEndsObservationOrChangesRequest(t *testing.T) {
	m := testModel(t)
	m.opts.Core = delayedCaller{}
	m.opts.CallTimeout = 20 * time.Millisecond
	m.input.SetValue("合成慢调用")
	cmd := m.send()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("延迟期间编辑草稿")})
	time.Sleep(60 * time.Millisecond)
	m, _ = step(m, <-done)
	if len(m.pending) != 1 || m.input.Value() != "延迟期间编辑草稿" {
		t.Fatal("timeout stopped client observation")
	}
	for id := range m.pending {
		if _, ok := m.envelopes[id]; !ok {
			t.Fatal("request identity replaced")
		}
	}
	if !strings.Contains(m.notice, "不更换 request_id") {
		t.Fatal("failure misrepresented")
	}
}

// This child-only helper wraps the actual TUI Model and Run's unchanged Bubble Tea
// options, injecting a synthetic panic from Update. The production CLI has no hook.
type issuePanicModel struct{ Model }

func (m issuePanicModel) Init() tea.Cmd { return nil }
func (m issuePanicModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+x" {
		panic("synthetic-issue2-catchable-panic")
	}
	next, cmd := m.Model.Update(msg)
	m.Model = next.(Model)
	return m, cmd
}
func TestIssue2TUIPanicPTYHelper(t *testing.T) {
	if os.Getenv("SECRETARY_ISSUE2_PANIC_PTY") != "1" {
		t.Skip("child-only PTY fault injection")
	}
	m := issuePanicModel{New(context.Background(), Options{})}
	_, err := tea.NewProgram(m, tea.WithContext(context.Background()), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err == nil {
		t.Fatal("injected panic did not terminate program")
	}
}
