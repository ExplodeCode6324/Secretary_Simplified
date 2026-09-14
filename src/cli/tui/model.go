package tui

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"secretarysimplified/contract"
)

type Model struct {
	accepted                                   map[string]string
	done                                       map[string]bool
	panelPages                                 []loadedPanelPage
	panelGeneration                            uint64
	panelApplied                               uint64
	panelInvalid                               bool
	panelSelection                             string
	panelOffset                                int
	opts                                       Options
	ctx                                        context.Context
	input                                      textarea.Model
	history                                    viewport.Model
	authority                                  Authority
	events                                     map[int]contract.ConversationEvent
	turns                                      map[string]contract.InputTurn
	pending                                    map[string]string
	envelopes                                  map[string]contract.InputEnvelope
	initialized, syncing, runnerOnline         bool
	coreStatus, notice, panel                  string
	rows                                       []map[string]any
	selected                                   int
	panelCursor                                *string
	panelNoProgress                            bool
	panelBusy                                  bool
	width, height, lastSequence, firstSequence int
	previousMore                               bool
	answerID, answerText                       string
	confirm                                    *control
	draftHistory                               []string
	draftIndex                                 int
	retry                                      time.Duration
	coreHealth                                 map[string]any
	debug                                      bool
}
type tickMsg struct{}
type syncMsg struct {
	accepted   map[string]string
	authority  Authority
	history    History
	turns      []contract.InputTurn
	unresolved []string
	err        error
	initial    bool
}
type historyMsg struct {
	page History
	err  error
}
type runnerMsg struct{ online bool }
type sentMsg struct {
	accepted bool
	turnID   string
	id       string
	turn     contract.InputTurn
	err      error
}
type loadedPanelPage struct {
	cursor string
	page   Page
}
type panelMsg struct {
	name          string
	page          Page
	err           error
	append        bool
	generation    uint64
	pageIndex     int
	cursor        string
	invalid       bool
	targetID      string
	target        map[string]any
	targetMissing bool
}
type control struct {
	resource, id, verb, label, request string
	revision                           int
}
type controlMsg struct {
	target control
	err    error
}

func New(ctx context.Context, o Options) Model {
	if o.PollInterval <= 0 {
		o.PollInterval = time.Second
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = 5 * time.Second
	}
	if o.DataClass == "" {
		o.DataClass = "PERSONAL"
	}
	a := textarea.New()
	a.Placeholder = "输入给 Secretary 的消息…"
	a.CharLimit = 32768
	a.ShowLineNumbers = false
	a.SetHeight(5)
	a.Focus()
	a.KeyMap.Paste.SetEnabled(false)
	return Model{opts: o, ctx: ctx, input: a, history: viewport.New(80, 12), events: map[int]contract.ConversationEvent{}, turns: map[string]contract.InputTurn{}, pending: map[string]string{}, accepted: map[string]string{}, done: map[string]bool{}, envelopes: map[string]contract.InputEnvelope{}, coreStatus: "连接中", width: 80, height: 24, retry: o.PollInterval, draftIndex: -1}
}
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.sync(), m.runnerHealth())
}
func (m Model) tick() tea.Cmd { return tea.Tick(m.retry, func(time.Time) tea.Msg { return tickMsg{} }) }
func (m Model) runnerHealth() tea.Cmd {
	return func() tea.Msg {
		return runnerMsg{api(m.ctx, m.opts.Runner, m.opts.CallTimeout, "GET", "/v1/health", nil, nil) == nil}
	}
}
func (m Model) sync() tea.Cmd {
	initial := !m.initialized
	last := m.lastSequence
	ids := make([]string, 0, len(m.pending))
	for id := range m.pending {
		ids = append(ids, id)
	}
	return func() tea.Msg {
		out := syncMsg{initial: initial, accepted: map[string]string{}}
		out.authority, out.err = Resolve(m.ctx, m.opts.Core)
		if out.err != nil {
			return out
		}
		if initial {
			ids = loadPending(m.opts.RecoveryDir, out.authority.InstanceID)
		}
		path := "/v1/conversation/history?limit=50&after_sequence=" + fmt.Sprint(last)
		if initial {
			path = "/v1/conversation/history?limit=50&direction=backward"
		}
		out.err = api(m.ctx, m.opts.Core, m.opts.CallTimeout, "GET", path, nil, &out.history)
		if out.err != nil {
			return out
		}
		out.turns = append(out.turns, out.authority.PendingTurns...)
		for _, id := range ids {
			if len(out.unresolved) >= 100 {
				break
			}
			var receipt map[string]any
			e := api(m.ctx, m.opts.Core, m.opts.CallTimeout, "GET", "/v1/requests/"+url.PathEscape(id), nil, &receipt)
			if e != nil {
				out.unresolved = append(out.unresolved, id)
				continue
			}
			turnID, _ := receipt["turn_id"].(string)
			if !acceptedTurnID(turnID) {
				out.unresolved = append(out.unresolved, id)
				continue
			}
			out.accepted[id] = turnID
			var turn contract.InputTurn
			if api(m.ctx, m.opts.Core, m.opts.CallTimeout, "GET", "/v1/turns/"+url.PathEscape(turnID), nil, &turn) != nil {
				out.unresolved = append(out.unresolved, id)
				continue
			}
			out.turns = append(out.turns, turn)
		}
		return out
	}
}
func (m *Model) mergeHistory(h History) {
	atBottom := m.history.AtBottom()
	for _, ev := range h.Events {
		m.events[ev.Sequence] = ev
		if ev.Sequence > m.lastSequence {
			m.lastSequence = ev.Sequence
		}
		if m.firstSequence == 0 || ev.Sequence < m.firstSequence {
			m.firstSequence = ev.Sequence
		}
	}
	m.renderHistory()
	if atBottom {
		m.history.GotoBottom()
	}
}
func (m *Model) renderHistory() {
	seqs := make([]int, 0, len(m.events))
	for seq := range m.events {
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)
	var b strings.Builder
	for _, seq := range seqs {
		e := m.events[seq]
		who := "Master"
		if e.Role == "ASSISTANT" {
			who = "Secretary"
		}
		fmt.Fprintf(&b, "%s  ·  %d\n%s\n\n", who, seq, SafeText(e.Text))
		if m.debug {
			fmt.Fprintf(&b, "turn %s · %s\n", SafeText(e.TurnID), SafeText(e.DeliveryState))
		}
	}
	if len(seqs) == 0 {
		b.WriteString("尚无已同步原话。草稿只在本客户端；发送后由后端统一排队。")
	}
	m.history.SetContent(ansi.Wrap(b.String(), max(1, m.history.Width), ""))
}
func (m Model) loadOlder() tea.Cmd {
	before := m.firstSequence
	return func() tea.Msg {
		var h History
		e := api(m.ctx, m.opts.Core, m.opts.CallTimeout, "GET", fmt.Sprintf("/v1/conversation/history?direction=backward&before_sequence=%d&limit=50", before), nil, &h)
		return historyMsg{h, e}
	}
}
func (m *Model) send() tea.Cmd {
	text := m.input.Value()
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if len(text) > 32768 {
		m.notice = "草稿超过 32 KiB，请缩短后发送"
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(text), "/") {
		command := strings.TrimSpace(text)
		switch command {
		case "/quit":
			return tea.Quit
		case "/help", "/status", "/items", "/jobs", "/tasks", "/notifications":
			m.input.Reset()
			return m.showPanel(strings.TrimPrefix(command, "/"))
		case "/answer":
			m.input.Reset()
			return m.showPanel("questions")
		case "/history":
			m.input.Reset()
			return m.loadOlder()
		case "/reconnect":
			m.input.Reset()
			return m.sync()
		default:
			m.notice = "未知命令；/help 查看。没有新建或切换会话命令。"
			return nil
		}
	}
	if !m.initialized || m.coreStatus != "已连接" {
		m.notice = "未连接权威会话；草稿已保留，未提交"
		return nil
	}
	if len(m.pending) >= 100 {
		m.notice = "未决请求达到客户端上限 100；请先同步，草稿未发送"
		return nil
	}
	id := contract.NewID()
	in := contract.InputEnvelope{SchemaVersion: 1, RequestID: id, SessionID: m.authority.SessionID, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: text, AttachmentRefs: []contract.ObjectRef{}, DataClass: m.opts.DataClass, Extensions: map[string]any{}}
	if m.answerID != "" {
		answer := m.answerID
		in.AnswerToQuestionID = &answer
	}
	if e := savePending(m.opts.RecoveryDir, m.authority.InstanceID, id); e != nil {
		m.notice = e.Error() + "；草稿未发送"
		return nil
	}
	m.pending[id] = "提交中"
	m.envelopes[id] = in
	m.draftHistory = append(m.draftHistory, text)
	if len(m.draftHistory) > 100 {
		m.draftHistory = m.draftHistory[len(m.draftHistory)-100:]
	}
	m.draftIndex = len(m.draftHistory)
	m.input.Reset()
	m.answerID = ""
	m.answerText = ""
	m.notice = "发送中；受理结果未知时保留原 request_id"
	return m.post(in)
}
func acceptedTurnID(id string) bool {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}
	b, e := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	return e == nil && len(b) == 16
}
func (m Model) post(in contract.InputEnvelope) tea.Cmd {
	return func() tea.Msg {
		var receipt map[string]any
		status, e := apiStatus(m.ctx, m.opts.Core, m.opts.CallTimeout, "POST", "/v1/inputs", in, &receipt)
		if e != nil {
			return sentMsg{id: in.RequestID, err: e}
		}
		id, _ := receipt["turn_id"].(string)
		if status != 202 || !acceptedTurnID(id) {
			return sentMsg{id: in.RequestID, err: errors.New("SUBMISSION_UNCONFIRMED: 返回未满足受理回执契约，查询原 request")}
		}
		var turn contract.InputTurn
		e = api(m.ctx, m.opts.Core, m.opts.CallTimeout, "GET", "/v1/turns/"+url.PathEscape(id), nil, &turn)
		if e == nil && (turn.ID != id || turn.RequestID != in.RequestID) {
			e = errors.New("OBSERVATION_IDENTITY_MISMATCH")
		}
		return sentMsg{id: in.RequestID, accepted: true, turnID: id, turn: turn, err: e}
	}
}
func (m *Model) mergeTurn(turn contract.InputTurn) {
	if turn.ID == "" || turn.RequestID == "" || m.done[turn.RequestID] {
		return
	}
	m.accepted[turn.RequestID] = turn.ID
	m.turns[turn.ID] = turn
	if turn.State == "COMMITTED" || turn.State == "FAILED" {
		m.done[turn.RequestID] = true
		delete(m.pending, turn.RequestID)
		delete(m.envelopes, turn.RequestID)
		forgetPending(m.opts.RecoveryDir, turn.RequestID)
	} else {
		m.pending[turn.RequestID] = "已受理 · " + turn.State
	}
}
func (m Model) perform(c control) tea.Cmd {
	return func() tea.Msg {
		body := map[string]any{"schema_version": 1, "request_id": c.request, "expected_revision": c.revision}
		e := api(m.ctx, m.opts.Runner, m.opts.CallTimeout, "POST", "/v1/"+c.resource+"/"+url.PathEscape(c.id)+"/"+c.verb, body, nil)
		return controlMsg{c, e}
	}
}
func (m *Model) prepareControl() {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return
	}
	row := m.rows[m.selected]
	id, _ := row["id"].(string)
	verb := ""
	switch m.panel {
	case "tasks":
		verb = "cancel"
	case "jobs":
		verb = "pause"
		if row["enabled"] == false {
			verb = "resume"
		}
	case "notifications":
		verb = "ack"
	}
	if id == "" || verb == "" {
		return
	}
	rev, _ := row["revision"].(float64)
	m.confirm = &control{m.panel, id, verb, rowText(row), contract.NewID(), int(rev)}
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch v := message.(type) {
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.history, cmd = m.history.Update(message)
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = max(24, v.Width)
		m.height = max(12, v.Height)
		m.input.SetWidth(max(10, m.width-4))
		m.input.SetHeight(min(6, max(3, m.height/4)))
		m.history.Width = max(10, m.width-2)
		m.history.Height = max(3, m.height-m.input.Height()-7)
		m.renderHistory()
		return m, nil
	case tickMsg:
		if m.syncing {
			return m, m.tick()
		}
		m.syncing = true
		cmds := []tea.Cmd{m.sync(), m.runnerHealth()}
		if m.panel != "" && !m.panelBusy {
			cmds = append(cmds, m.openPanel(m.panel, false))
		}
		return m, tea.Batch(cmds...)
	case syncMsg:
		m.syncing = false
		if v.err != nil {
			m.coreStatus = "离线 / 重连中"
			m.notice = SafeText(v.err.Error())
			m.retry = min(15*time.Second, max(m.opts.PollInterval, m.retry*2))
			return m, m.tick()
		}
		if m.initialized && (m.authority.InstanceID != v.authority.InstanceID || m.authority.SessionID != v.authority.SessionID) {
			m.coreStatus = "实例已变化"
			m.notice = "拒绝将原实例草稿或未决请求发送到另一实例；请退出并核对配置"
			return m, m.tick()
		}
		m.authority = v.authority
		m.coreStatus = "已连接"
		m.initialized = true
		m.retry = m.opts.PollInterval
		m.mergeHistory(v.history)
		if v.initial {
			m.previousMore = v.history.HasMore
			m.history.GotoBottom()
		}
		for id, turnID := range v.accepted {
			if !m.done[id] {
				m.accepted[id] = turnID
				if _, ok := m.turns[turnID]; !ok {
					m.turns[turnID] = contract.InputTurn{ID: turnID, RequestID: id, State: "ACCEPTED", SessionID: m.authority.SessionID}
				}
			}
		}
		for _, id := range v.unresolved {
			if m.done[id] {
				continue
			}
			if m.accepted[id] != "" {
				m.pending[id] = "已受理，暂时无法读取结果；查询原 request"
			} else {
				m.pending[id] = "受理未确认：仅查询原 request，不自动重发"
			}
		}
		for _, turn := range v.turns {
			m.mergeTurn(turn)
		}
		if m.panel == "questions" {
			m.rows = nil
			for _, q := range m.authority.State.PendingQuestions {
				if q["resolved"] != true {
					m.rows = append(m.rows, q)
				}
			}
			m.selected = min(m.selected, max(0, len(m.rows)-1))
		}
		if v.history.HasMore && !v.initial {
			m.syncing = true
			return m, m.sync()
		}
		return m, m.tick()
	case runnerMsg:
		m.runnerOnline = v.online
		return m, nil
	case historyMsg:
		if v.err != nil {
			m.notice = SafeText(v.err.Error())
			return m, nil
		}
		m.previousMore = v.page.HasMore
		m.mergeHistory(v.page)
		m.history.GotoTop()
		return m, nil
	case sentMsg:
		if m.done[v.id] {
			return m, nil
		}
		if v.accepted {
			m.accepted[v.id] = v.turnID
			if _, ok := m.turns[v.turnID]; !ok {
				m.turns[v.turnID] = contract.InputTurn{ID: v.turnID, RequestID: v.id, State: "ACCEPTED", SessionID: m.authority.SessionID}
			}
		}
		if v.err != nil {
			if m.accepted[v.id] != "" {
				m.pending[v.id] = "已受理，暂时无法读取结果"
				m.notice = "已受理，暂时无法读取结果：" + SafeText(v.err.Error())
				var auth *APIError
				if errors.As(v.err, &auth) && (auth.Status == 401 || auth.Status == 403) {
					m.notice += "；请恢复认证，不会重新提交"
				}
				return m, nil
			}
			if !v.accepted && submissionRejected(v.err) {
				m.notice = "输入被后端拒绝：" + SafeText(v.err.Error())
				if in, ok := m.envelopes[v.id]; ok && m.input.Value() == "" {
					m.input.SetValue(in.Text)
				}
				delete(m.pending, v.id)
				delete(m.envelopes, v.id)
				forgetPending(m.opts.RecoveryDir, v.id)
				return m, nil
			}
			m.pending[v.id] = "受理未确认 / 查询原请求"
			m.notice = SafeText(v.err.Error()) + "；不更换 request_id 重发"
		} else if v.turn.ID != "" {
			m.mergeTurn(v.turn)
			m.notice = "输入已受理；决策提交不代表任务执行成功"
		}
		return m, nil
	case panelMsg:
		m.applyPanel(v)
		return m, nil
	case controlMsg:
		if v.err != nil {
			m.notice = "控制未确认：" + SafeText(v.err.Error()) + "；刷新目标后重新决定"
		} else {
			m.notice = "后端已确认 " + v.target.verb + "；以持久状态为准"
		}
		m.panelBusy = true
		return m, m.openPanel(m.panel, false)
	case tea.KeyMsg:
		// Bracketed paste is always one draft update, never a slash command or submit.
		if v.Paste {
			m.input.InsertString(SafeText(string(v.Runes)))
			return m, nil
		}
		switch v.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s", "alt+enter":
			if m.panel == "" {
				return m, m.send()
			}
			return m, nil
		case "ctrl+d":
			m.debug = !m.debug
			m.renderHistory()
			return m, nil
		case "pgup":
			if m.panel != "" {
				m.panelOffset = max(0, m.panelOffset-max(1, m.history.Height/2))
			} else {
				m.history.HalfViewUp()
			}
			return m, nil
		case "pgdown":
			if m.panel != "" {
				m.panelOffset += max(1, m.history.Height/2)
			} else {
				m.history.HalfViewDown()
			}
			return m, nil
		case "f1":
			return m, m.showPanel("help")
		case "f2":
			return m, m.showPanel("questions")
		case "f3":
			return m, m.showPanel("tasks")
		case "f4":
			return m, m.showPanel("jobs")
		case "f5":
			return m, m.showPanel("items")
		case "f6":
			return m, m.showPanel("notifications")
		}
		if m.confirm != nil {
			switch v.String() {
			case "y", "Y":
				c := *m.confirm
				m.confirm = nil
				m.notice = "提交控制…"
				return m, m.perform(c)
			case "n", "N", "esc":
				m.confirm = nil
			}
			return m, nil
		}
		if m.panel != "" {
			switch v.String() {
			case "esc", "tab":
				m.panel = ""
				m.input.Focus()
				return m, textarea.Blink
			case "up", "k":
				m.selected = max(0, m.selected-1)
			case "down", "j":
				m.selected = min(len(m.rows)-1, m.selected+1)
			case "n":
				if m.panelInvalid {
					m.notice = "分页快照已失效；按 r 明确重置到第一页"
					return m, nil
				}
				if m.panelCursor != nil {
					m.panelBusy = true
					return m, m.openPanel(m.panel, true)
				}
			case "r":
				if m.panelInvalid {
					return m, m.resetPanel()
				}
				m.panelBusy = true
				return m, m.openPanel(m.panel, false)
			case "enter":
				if m.panel == "questions" && m.selected >= 0 && m.selected < len(m.rows) {
					row := m.rows[m.selected]
					m.answerID, _ = row["id"].(string)
					m.answerText, _ = row["text"].(string)
					if m.answerID != "" {
						m.panel = ""
						m.input.Focus()
						m.notice = "回答已绑定后端问题，Ctrl+S发送"
						return m, textarea.Blink
					}
				} else {
					m.prepareControl()
				}
			}
			return m, nil
		}
		switch v.String() {
		case "tab":
			return m, m.showPanel("questions")
		case "ctrl+p":
			if len(m.draftHistory) > 0 {
				m.draftIndex = max(0, m.draftIndex-1)
				m.input.SetValue(m.draftHistory[m.draftIndex])
			}
			return m, nil
		case "ctrl+n":
			if len(m.draftHistory) > 0 {
				m.draftIndex = min(len(m.draftHistory), m.draftIndex+1)
				if m.draftIndex == len(m.draftHistory) {
					m.input.Reset()
				} else {
					m.input.SetValue(m.draftHistory[m.draftIndex])
				}
			}
			return m, nil
		case "esc":
			m.answerID = ""
			m.answerText = ""
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(message)
	return m, cmd
}

const helpText = "Enter 换行 · Ctrl+S / Alt+Enter 发送 · Ctrl+C 退出（后台继续）\nPgUp/PgDn 滚动 · Ctrl+P/N 输入历史 · Ctrl+D 展开诊断 ID\nF1 帮助 · F2 待答问题 · F3 委托 · F4 计划 · F5 事项 · F6 通知\n面板 ↑↓ 选择 · Enter 回答/控制确认 · n 下一页 · r 刷新 · Esc/Tab 返回草稿\n/help /status /history /items /jobs /tasks /notifications /answer /reconnect /quit\n客户端只有一个权威会话。退出不会取消执行；首版不支持撤销已受理 turn 或中止模型。\n通知只在明确确认后 ack；停止观察不等于业务取消。\n事项分页快照失效时 n 停用，r 明确重载第一页；原目标不在页内则取消选择。"

func (m Model) panelView() string {
	if m.panel == "help" {
		return helpText
	}
	if m.panel == "status" {
		pending := ""
		ids := make([]string, 0, len(m.pending))
		for id := range m.pending {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			pending += "\n" + SafeText(id) + " · " + SafeText(m.pending[id])
		}
		return fmt.Sprintf("实例：%s\n权威会话：%s\n同步：%d / %d · 摘要覆盖 %d\n焦点：%s\n摘要：%s\nCore %s · Runner %t\n配置模型：%s · 最近实际调用：未知（配置不证明在线）", SafeText(m.authority.InstanceID), SafeText(m.authority.SessionID), m.lastSequence, m.authority.HistorySequence, m.authority.SummaryThroughSequence, SafeText(strings.Join(m.authority.State.FocusEntityIDs, ", ")), SafeText(m.authority.State.Summary), m.coreStatus, m.runnerOnline, SafeText(m.opts.Config.Model.Profile+" / "+m.opts.Config.Model.Model)) + pending
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s  ·  ↑↓ 选择 / Enter 操作 / n 下一页 / Esc 返回\n", m.panel)
	if m.panelBusy {
		b.WriteString("正在查询，草稿保留…\n")
	}
	if len(m.rows) == 0 {
		b.WriteString("没有已同步条目。\n")
	}
	start := max(0, m.selected-max(2, m.history.Height-4))
	end := min(len(m.rows), start+max(3, m.history.Height-2))
	for i := start; i < end; i++ {
		prefix := "  "
		if i == m.selected {
			prefix = "› "
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, ansi.Truncate(rowText(m.rows[i]), max(1, m.width-4), "…"))
		if m.debug {
			fmt.Fprintf(&b, "    %v · revision %v\n", m.rows[i]["id"], m.rows[i]["revision"])
		}
	}
	return b.String()
}
func (m Model) View() string {
	runner := "离线"
	if m.runnerOnline {
		runner = "在线"
	}
	instance := m.authority.InstanceID
	if len(instance) > 8 {
		instance = instance[:8]
	}
	title := fmt.Sprintf("Secretary · %s · %s · 同步 %d/%d · %s", SafeText(instance), m.coreStatus, m.lastSequence, m.authority.HistorySequence, m.opts.DataClass)
	content := m.history.View()
	if m.panel != "" {
		panel := viewport.New(max(1, m.width-1), m.history.Height)
		panel.SetContent(ansi.Wrap(SafeText(m.panelView()), max(1, m.width-1), ""))
		panel.SetYOffset(m.panelOffset)
		content = panel.View()
	}
	if m.confirm != nil {
		c := m.confirm
		content += fmt.Sprintf("\n确认 %s %s\n目标 %s · revision %d\n[y] 确认 / [n] 放弃", c.verb, SafeText(c.label), SafeText(c.id), c.revision)
	}
	pending := ""
	if len(m.pending) > 0 {
		pending = fmt.Sprintf(" · %d 个未决请求（观察超时不取消）", len(m.pending))
	}
	answer := ""
	if m.answerID != "" {
		answer = "回答：" + SafeText(m.answerText) + " · Esc 解除选择\n"
	}
	status := fmt.Sprintf("Core %s · Runner %s · 配置 %s · 模型调用状态未知%s", m.coreStatus, runner, SafeText(m.opts.Config.Model.Profile+" / "+m.opts.Config.Model.Model), pending)
	return ansi.Truncate(title, m.width-1, "…") + "\n" + strings.Repeat("─", max(1, m.width-2)) + "\n" + content + "\n" + ansi.Truncate(status, m.width-1, "…") + "\n" + ansi.Truncate(SafeText(m.notice), m.width-1, "…") + "\n" + answer + m.input.View() + "\nCtrl+S / Alt+Enter 发送 · Enter 换行 · F1 帮助 · Ctrl+C 退出"
}
func Run(ctx context.Context, o Options) error {
	m := New(ctx, o)
	_, e := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return e
}
