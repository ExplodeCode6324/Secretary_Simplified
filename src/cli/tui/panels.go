package tui

import (
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"net/url"
)

func (m Model) selectedID() string {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return ""
	}
	id, _ := m.rows[m.selected]["id"].(string)
	return id
}
func (m *Model) openPanel(name string, more bool) tea.Cmd {
	if name == "questions" || name == "help" || name == "status" {
		return nil
	}
	if more && m.panelNoProgress {
		m.notice = "追加页没有新 ID；已停止继续翻页，重新打开面板可重置"
		return nil
	}
	if more && m.panelInvalid {
		m.notice = "分页快照已失效；按 r 明确重置到第一页"
		return nil
	}
	cursor := ""
	index := 0
	selected := m.selectedID()
	if more {
		index = len(m.panelPages)
		if m.panelCursor != nil {
			cursor = *m.panelCursor
		}
	} else if len(m.panelPages) > 0 {
		index = len(m.panelPages) - 1
		for i, page := range m.panelPages {
			for _, row := range page.page.Items {
				if row["id"] == selected {
					index = i
				}
			}
		}
		cursor = m.panelPages[index].cursor
	}
	m.panelGeneration++
	generation := m.panelGeneration
	m.panelBusy = true
	opts, ctx := m.opts, m.ctx
	alreadyInvalid := m.panelInvalid
	return func() tea.Msg {
		out := panelMsg{name: name, append: more, generation: generation, pageIndex: index, cursor: cursor, targetID: selected}
		c := opts.Runner
		if name == "items" {
			c = opts.Core
		}
		if !alreadyInvalid {
			path := "/v1/" + name + "?limit=50"
			if cursor != "" {
				path += "&cursor=" + url.QueryEscape(cursor)
			}
			out.err = api(ctx, c, opts.CallTimeout, "GET", path, nil, &out.page)
		}
		var conflict *APIError
		if name == "items" && (alreadyInvalid || (errors.As(out.err, &conflict) && conflict.Status == 409)) {
			out.invalid = true
			if selected != "" {
				var target map[string]any
				lookup := api(ctx, c, opts.CallTimeout, "GET", "/v1/items/"+url.PathEscape(selected), nil, &target)
				var missing *APIError
				if lookup == nil && target["id"] == selected {
					out.target = target
				} else if errors.As(lookup, &missing) && missing.Status == 404 {
					out.targetMissing = true
				} else if lookup != nil {
					out.err = lookup
				}
			}
		}
		return out
	}
}
func (m *Model) showPanel(name string) tea.Cmd {
	m.panelGeneration++ // Also invalidate the previous visit when opening a non-query panel.
	m.panel = name
	m.panelOffset = 0
	m.selected = 0
	m.rows = nil
	m.panelCursor = nil
	m.panelPages = nil
	m.panelInvalid = false
	m.panelNoProgress = false
	m.panelSelection = ""
	m.input.Blur()
	if name == "questions" {
		m.panelBusy = false
		for _, q := range m.authority.State.PendingQuestions {
			if q["resolved"] != true {
				m.rows = append(m.rows, q)
			}
		}
		return nil
	}
	m.panelBusy = name != "help" && name != "status"
	return m.openPanel(name, false)
}
func (m *Model) resetPanel() tea.Cmd {
	selected := m.selectedID()
	if selected == "" {
		selected = m.panelSelection
	}
	cmd := m.showPanel(m.panel)
	m.panelSelection = selected
	if selected != "" {
		m.selected = -1
	}
	return cmd
}
func (m *Model) rebuildPanel(selected string, first bool) {
	rows := []map[string]any{}
	positions := map[string]int{}
	for _, page := range m.panelPages {
		for _, row := range page.page.Items {
			id, _ := row["id"].(string)
			if id == "" {
				continue
			}
			if pos, ok := positions[id]; ok {
				rows[pos] = row
			} else {
				positions[id] = len(rows)
				rows = append(rows, row)
			}
		}
	}
	m.rows = rows
	m.selected = -1
	if selected != "" {
		if pos, ok := positions[selected]; ok {
			m.selected = pos
		} else {
			m.notice = "原选中目标已离开当前范围；请选择目标后再操作"
		}
	} else if first && len(rows) > 0 {
		m.selected = 0
	}
	if len(m.panelPages) > 0 {
		m.panelCursor = m.panelPages[len(m.panelPages)-1].page.NextCursor
		if m.panelNoProgress {
			m.panelCursor = nil
		}
	}
}
func (m *Model) applyPanel(v panelMsg) {
	if v.name != m.panel || v.generation != m.panelGeneration || v.generation <= m.panelApplied {
		return
	}
	m.panelApplied = v.generation
	m.panelBusy = false
	selected := m.selectedID()
	if m.panelSelection != "" {
		selected = m.panelSelection
	}
	if v.invalid {
		m.panelInvalid = true
		m.notice = "分页快照已失效；仅刷新当前目标，按 r 明确重置到第一页"
		if v.target != nil || v.targetMissing {
			for i := range m.panelPages {
				items := []map[string]any{}
				for _, row := range m.panelPages[i].page.Items {
					if row["id"] == v.targetID {
						if v.targetMissing {
							continue
						}
						row = v.target
					}
					items = append(items, row)
				}
				m.panelPages[i].page.Items = items
			}
			m.rebuildPanel(selected, false)
		}
		if v.targetMissing {
			m.notice = "目标已不存在；分页快照失效，按 r 重置后重新选择"
		} else if v.target == nil && v.err != nil {
			m.notice += "；目标读取失败：" + SafeText(v.err.Error())
		}
		return
	}
	if v.err != nil {
		m.notice = SafeText(v.err.Error())
		return
	}
	first := len(m.panelPages) == 0 && selected == ""
	if v.append {
		if v.pageIndex != len(m.panelPages) {
			return
		}
		known := map[string]bool{}
		for _, row := range m.rows {
			if id, ok := row["id"].(string); ok {
				known[id] = true
			}
		}
		added := false
		for _, row := range v.page.Items {
			if id, ok := row["id"].(string); ok && id != "" && !known[id] {
				added = true
				break
			}
		}
		if !added {
			m.panelNoProgress = true
			m.notice = "追加页没有新 ID；已停止继续翻页，重新打开面板可重置"
		}
		m.panelPages = append(m.panelPages, loadedPanelPage{v.cursor, v.page})
	} else if v.pageIndex < len(m.panelPages) {
		m.panelPages[v.pageIndex] = loadedPanelPage{v.cursor, v.page}
	} else if v.pageIndex == 0 && len(m.panelPages) == 0 {
		m.panelPages = []loadedPanelPage{{v.cursor, v.page}}
	} else {
		return
	}
	m.panelSelection = ""
	m.rebuildPanel(selected, first)
}
