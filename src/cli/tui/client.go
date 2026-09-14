// Package tui is a thin authenticated terminal client. It never opens a database.
package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/transport"
	"strings"
	"time"
)

type Caller interface {
	Call(context.Context, string, string, any) (transport.Envelope, int, error)
}
type Options struct {
	Config                    config.Config
	Core, Runner              Caller
	DataClass, RecoveryDir    string
	PollInterval, CallTimeout time.Duration
}
type Authority struct {
	InstanceID             string                     `json:"instance_id"`
	SessionID              string                     `json:"session_id"`
	Revision               int                        `json:"revision"`
	HistorySequence        int                        `json:"history_sequence"`
	SummaryThroughSequence int                        `json:"summary_through_sequence"`
	State                  contract.ConversationState `json:"state"`
	PendingTurns           []contract.InputTurn       `json:"pending_turns"`
}
type History struct {
	SessionID        string                       `json:"session_id"`
	Events           []contract.ConversationEvent `json:"events"`
	NextSequence     int                          `json:"next_sequence"`
	PreviousSequence int                          `json:"previous_sequence"`
	HistorySequence  int                          `json:"history_sequence"`
	HasMore          bool                         `json:"has_more"`
}
type Page struct {
	Items      []map[string]any `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

func decode(v any, to any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, to)
}

type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string { return e.Code }

func api(ctx context.Context, c Caller, timeout time.Duration, method, path string, body, to any) error {
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	v, status, e := c.Call(bounded, method, path, body)
	if e != nil {
		return errors.New("DEPENDENCY_UNAVAILABLE: 检查 daemon 和配置，客户端会重连")
	}
	if status >= 400 {
		code := "REQUEST_REJECTED"
		if m, ok := v.Error.(map[string]any); ok {
			if s, ok := m["code"].(string); ok {
				code = s
			}
		}
		if status == 404 {
			code = "NOT_FOUND"
		}
		return &APIError{status, SafeText(code)}
	}
	if to != nil {
		return decode(v.Result, to)
	}
	return nil
}
func Resolve(ctx context.Context, c Caller) (Authority, error) {
	var a Authority
	e := api(ctx, c, 5*time.Second, "GET", "/v1/conversation", nil, &a)
	if e == nil && (a.InstanceID == "" || a.SessionID == "") {
		e = errors.New("AUTHORITY_UNAVAILABLE: 后端尚未登记唯一会话")
	}
	return a, e
}

// SafeText removes terminal control sequences rather than interpreting model output.
// ESC sequences (including OSC clipboard/title) and C1 controls are discarded.
func SafeText(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c == 0x1b {
			if i+1 >= len(r) {
				continue
			}
			i++
			switch r[i] {
			case '[':
				for i+1 < len(r) {
					i++
					if r[i] >= 0x40 && r[i] <= 0x7e {
						break
					}
				}
			case ']', 'P', '^', '_':
				for i+1 < len(r) {
					i++
					if r[i] == 7 || r[i] == 0x9c {
						break
					}
					if r[i] == 0x1b && i+1 < len(r) && r[i+1] == '\\' {
						i++
						break
					}
				}
			}
			continue
		}
		if c == 0x9b {
			for i+1 < len(r) {
				i++
				if r[i] >= 0x40 && r[i] <= 0x7e {
					break
				}
			}
			continue
		}
		if c == 0x9d || c == 0x90 {
			for i+1 < len(r) {
				i++
				if r[i] == 7 || r[i] == 0x9c {
					break
				}
			}
			continue
		}
		if c == '\n' || c == '\t' || (c >= 0x20 && c != 0x7f && (c < 0x80 || c > 0x9f)) {
			b.WriteRune(c)
		}
	}
	return b.String()
}

type pendingID struct {
	InstanceID string `json:"instance_id"`
	RequestID  string `json:"request_id"`
}

func savePending(dir, instance, request string) error {
	if dir == "" {
		return errors.New("RECOVERY_DIRECTORY_REQUIRED")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return errors.New("RECOVERY_UNAVAILABLE")
	}
	data, _ := json.Marshal(pendingID{instance, request})
	f, e := os.CreateTemp(dir, ".pending-*")
	if e != nil {
		return errors.New("RECOVERY_UNAVAILABLE")
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return errors.New("RECOVERY_UNAVAILABLE")
	}
	if e = os.Rename(temp, filepath.Join(dir, request+".json")); e != nil {
		return errors.New("RECOVERY_UNAVAILABLE")
	}
	d, e := os.Open(dir)
	if e != nil {
		return errors.New("RECOVERY_UNAVAILABLE")
	}
	e = d.Sync()
	d.Close()
	if e != nil {
		return errors.New("RECOVERY_UNAVAILABLE")
	}

	return nil
}
func loadPending(dir, instance string) []string {
	out := []string{}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if len(out) >= 100 {
			break
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		f, e := os.Open(filepath.Join(dir, entry.Name()))
		if e != nil {
			continue
		}
		b, e := io.ReadAll(io.LimitReader(f, 1025))
		f.Close()
		if e != nil || len(b) > 1024 {
			continue
		}
		var id pendingID
		if json.Unmarshal(b, &id) == nil && id.InstanceID == instance && entry.Name() == id.RequestID+".json" {
			out = append(out, id.RequestID)
		}
	}
	return out
}
func forgetPending(dir, request string) {
	if filepath.Base(request) != request {
		return
	}
	_ = os.Remove(filepath.Join(dir, request+".json"))
}
func stateText(t contract.InputTurn) string {
	switch t.State {
	case "COMMITTED":
		if t.Reply != nil {
			if v, ok := (*t.Reply)["text"].(string); ok {
				return SafeText(v)
			}
		}
		return "决策已提交（不代表业务执行成功）"
	case "FAILED":
		return "决策失败 / 拒绝"
	default:
		return "已受理，等待后端：" + t.State
	}
}
func rowText(v map[string]any) string {
	label := ""
	for _, k := range []string{"title", "goal", "text", "name", "id"} {
		if s, ok := v[k].(string); ok && s != "" {
			label = s
			break
		}
	}
	state := v["state"]
	if state == nil {
		state = v["status"]
	}
	if enabled, ok := v["enabled"].(bool); ok {
		state = fmt.Sprintf("enabled=%t", enabled)
	}
	return SafeText(fmt.Sprintf("%s  [%v]", label, state))
}
