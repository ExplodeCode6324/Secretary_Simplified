package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/diagnostics"
	"secretarysimplified/store"
	"secretarysimplified/transport"
	"strings"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		code := 2
		msg := e.Error()
		switch {
		case strings.Contains(msg, "CONFLICT") || strings.Contains(msg, "STALE_FENCE"):
			code = 3
		case strings.Contains(msg, "PERMISSION") || strings.Contains(msg, "UNAUTHENTICATED") || strings.Contains(msg, "DISCLOSURE"):
			code = 4
		case strings.Contains(msg, "UNAVAILABLE") || strings.Contains(msg, "DB_BUSY") || strings.Contains(msg, "BACKPRESSURE"):
			code = 5
		case strings.Contains(msg, "RESULT_UNKNOWN") || strings.Contains(msg, "NEEDS_ATTENTION"):
			code = 6
		}
		os.Exit(code)
	}
}
func flags(args []string) ([]string, map[string]string) {
	pos := []string{}
	m := map[string]string{}
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			k := strings.TrimPrefix(args[i], "--")
			v := "true"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++
				v = args[i]
			}
			m[k] = v
		} else {
			pos = append(pos, args[i])
		}
	}
	return pos, m
}

var jsonOutput bool

func output(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	if !jsonOutput {
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if state, ok := m["status"].(string); ok {
			fmt.Println("状态：" + state)
			if state == "ACCEPTED" {
				fmt.Println("输入已受理，执行尚未完成；请按 turn_id 查询。")
			}
		} else {
			fmt.Println("Secretary 查询结果：")
		}
	}
	fmt.Println(string(b))
}
func run() error {
	pos, f := flags(os.Args[1:])
	jsonOutput = f["json"] == "true"
	if len(pos) == 0 {
		fmt.Println("secretary init|input|chat|items|jobs|tasks|runs|alarm|world|memory|doctor|backup|verify --config PATH [--json] [--data-class PERSONAL|SYNTHETIC|SENSITIVE|SECRET] (default PERSONAL)")
		return nil
	}
	dir := f["data-dir"]
	if dir == "" {
		dir = "."
	}
	dir, _ = filepath.Abs(dir)
	if pos[0] == "init" {
		return initData(dir)
	}
	path := f["config"]
	if path == "" {
		path = filepath.Join(dir, "config.json")
	}
	c, e := config.Load(path)
	if e != nil {
		return e
	}
	if pos[0] == "verify" && f["suite"] != "" {
		v, e := diagnostics.VerifySuite(context.Background(), f["suite"], f["report"])
		output(v)
		return e
	}
	if pos[0] == "doctor" || pos[0] == "backup" || pos[0] == "restore" || pos[0] == "verify" {
		db, e := store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
		if e != nil {
			return e
		}
		defer db.Close()
		switch pos[0] {
		case "doctor":
			epoch, parseErr := time.Parse(time.RFC3339Nano, c.Epoch)
			if parseErr != nil {
				return parseErr
			}
			v, e := diagnostics.DoctorWithEpoch(context.Background(), db, c.DataDir, epoch)
			output(v)
			return e
		case "backup":
			if f["output"] == "" {
				return errors.New("--output required")
			}
			v, e := db.Backup(context.Background(), f["output"])
			output(v)
			return e
		case "restore":
			if f["backup"] == "" || f["target"] == "" {
				return errors.New("--backup and --target required")
			}
			v, e := store.Restore(context.Background(), f["backup"], f["target"])
			if e != nil {
				return e
			}
			defer v.Close()
			target, _ := filepath.Abs(f["target"])
			if e = provisionRestored(v, target); e != nil {
				return e
			}
			restored := config.Default(target)
			restored.Frozen = true
			return restored.Save(filepath.Join(target, "config.json"))
		case "verify":
			if f["backup"] != "" {
				m, e := store.VerifyBackup(context.Background(), f["backup"])
				output(m)
				return e
			}
			h, e := db.Health(context.Background())
			output(h)
			return e
		}
	}
	token, e := os.ReadFile(filepath.Join(c.DataDir, "run", "client.token"))
	if e != nil {
		return e
	}
	client := transport.Client{Socket: filepath.Join(c.DataDir, "run", "core.sock"), Token: strings.TrimSpace(string(token))}
	if pos[0] == "runs" || pos[0] == "alarm" || pos[0] == "tasks" || pos[0] == "notifications" || (pos[0] == "jobs" && len(pos) > 1 && pos[1] != "create") {
		client.Socket = filepath.Join(c.DataDir, "run", "runner.sock")
	}
	call := func(method, path string, body any) error {
		if method == "GET" {
			q := url.Values{}
			for _, k := range []string{"cursor", "limit", "domain", "status", "entity", "predicate", "after_seq"} {
				if f[k] != "" {
					q.Set(k, f[k])
				}
			}
			if len(q) > 0 {
				path += "?" + q.Encode()
			}
		}
		v, status, e := client.Call(context.Background(), method, path, body)
		if e != nil {
			return e
		}
		output(v)
		if status >= 400 {
			if em, ok := v.Error.(map[string]any); ok {
				return fmt.Errorf("%v", em["code"])
			}
			return fmt.Errorf("request failed (%d)", status)
		}
		return nil
	}
	dataClass := "PERSONAL"
	if value, present := f["data-class"]; present {
		dataClass = value
	}
	if _, err := contract.JoinClass(dataClass); err != nil {
		return errors.New("invalid --data-class")
	}
	session := f["session"]
	if session == "" {
		session = "00000000-0000-4000-8000-000000000001"
	}
	request := f["request-id"]
	if request == "" {
		request = contract.NewID()
	}
	mutation := func(resource, id, verb string, extra map[string]any) error {
		body := map[string]any{"schema_version": 1, "request_id": request}
		for k, v := range extra {
			body[k] = v
		}
		if verb == "cancel" || verb == "pause" || verb == "resume" || verb == "trigger" {
			if f["expected-revision"] != "" {
				var n int
				if _, e := fmt.Sscanf(f["expected-revision"], "%d", &n); e != nil {
					return e
				}
				body["expected_revision"] = n
			} else {
				v, status, e := client.Call(context.Background(), "GET", "/v1/"+resource+"/"+id, nil)
				if e != nil || status >= 400 {
					return errors.New("cannot read expected revision")
				}
				m, _ := v.Result.(map[string]any)
				if resource == "runs" {
					tid, _ := m["task_id"].(string)
					v, _, e = client.Call(context.Background(), "GET", "/v1/tasks/"+tid, nil)
					if e != nil {
						return e
					}
					m, _ = v.Result.(map[string]any)
				}
				body["expected_revision"] = m["revision"]
			}
		}
		return call("POST", "/v1/"+resource+"/"+id+"/"+verb, body)
	}
	action := func(a contract.ActionProposal) error {
		return call("POST", "/v1/actions", map[string]any{"schema_version": 1, "request_id": request, "session_id": session, "actions": []contract.ActionProposal{a}, "data_class": dataClass})
	}
	switch pos[0] {
	case "input":
		in := contract.InputEnvelope{SchemaVersion: 1, RequestID: request, SessionID: session, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: f["text"], AttachmentRefs: []contract.ObjectRef{}, DataClass: dataClass, Extensions: map[string]any{}}
		if answer, present := f["answer-to"]; present {
			in.AnswerToQuestionID = &answer
		}
		return call("POST", "/v1/inputs", in)
	case "chat":
		scan := bufio.NewScanner(os.Stdin)
		scan.Buffer(make([]byte, 4096), 32768)
		fmt.Println("Secretary chat; /quit exits. Accepted turns execute asynchronously. Answer a registered question with: secretary input --session <session-id> --answer-to <question-id> --text <answer>.")
		for {
			fmt.Print("> ")
			if !scan.Scan() {
				return scan.Err()
			}
			if scan.Text() == "/quit" {
				return nil
			}
			in := contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: session, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: scan.Text(), AttachmentRefs: []contract.ObjectRef{}, DataClass: dataClass, Extensions: map[string]any{}}
			v, code, e := client.Call(context.Background(), "POST", "/v1/inputs", in)
			if e != nil {
				return e
			}
			if code >= 400 {
				output(v)
				continue
			}
			m, _ := v.Result.(map[string]any)
			id, _ := m["turn_id"].(string)
			deadline := time.Now().Add(65 * time.Second)
			for time.Now().Before(deadline) {
				res, _, err := client.Call(context.Background(), "GET", "/v1/turns/"+id, nil)
				if err != nil {
					return err
				}
				x, _ := res.Result.(map[string]any)
				if x["state"] == "COMMITTED" || x["state"] == "FAILED" {
					output(res)
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
	case "actions":
		return actionFile(f, action)
	case "items":
		if len(pos) < 2 {
			return errors.New("items list|show|create|update")
		}
		switch pos[1] {
		case "list":
			return call("GET", "/v1/items", nil)
		case "show":
			if len(pos) < 3 {
				return errors.New("item id required")
			}
			return call("GET", "/v1/items/"+pos[2], nil)
		case "create":
			domain := f["domain"]
			if domain == "" {
				domain = "project"
			}
			var due any
			if f["due-at"] != "" {
				due = f["due-at"]
			}
			return action(contract.ActionProposal{OperationKey: "create_item", Kind: "CREATE_ITEM", Payload: map[string]any{"domain": domain, "kind": "TASK", "title": f["title"], "due_at": due, "timezone": c.Timezone, "priority": 1}})
		case "update":
			return actionFile(f, action)
		}
	case "jobs":
		if len(pos) < 2 {
			return errors.New("jobs list|create|pause|resume|trigger")
		}
		if pos[1] == "create" {
			return actionFile(f, action)
		}
		if pos[1] == "trigger" || pos[1] == "pause" || pos[1] == "resume" {
			if len(pos) < 3 {
				return errors.New("job id required")
			}
			return mutation("jobs", pos[2], pos[1], nil)
		}
		return call("GET", "/v1/jobs", nil)
	case "world":
		if len(pos) > 1 && pos[1] != "list" {
			return actionFile(f, action)
		}
		return call("GET", "/v1/world", nil)
	case "memory":
		if len(pos) > 1 && pos[1] == "search" {
			return call("POST", "/v1/memory/search", map[string]any{"schema_version": 1, "query": f["query"], "cursor": optionalString(f["cursor"])})
		}
		return call("GET", "/v1/memory/consciousness", nil)
	case "alarm":
		if len(pos) < 3 {
			return errors.New("alarm stop|snooze id required")
		}
		body := map[string]any{"request_id": request, "schema_version": 1}
		if pos[1] == "snooze" {
			var seconds int
			if _, e := fmt.Sscanf(f["seconds"], "%d", &seconds); e != nil {
				return errors.New("--seconds required")
			}
			body["delay_seconds"] = seconds
		}
		return call("POST", "/v1/alarms/"+pos[2]+"/"+pos[1], body)
	case "notifications":
		if len(pos) > 2 && pos[1] == "ack" {
			return call("POST", "/v1/notifications/"+pos[2]+"/ack", map[string]any{"schema_version": 1, "request_id": request})
		}
		return call("GET", "/v1/notifications", nil)
	case "context":
		if len(pos) < 3 || pos[1] != "show" {
			return errors.New("context show id required")
		}
		return call("GET", "/v1/context/"+pos[2], nil)
	case "events":
		return call("GET", "/v1/events", nil)
	case "turns":
		if len(pos) < 2 {
			return errors.New("turn id required")
		}
		return call("GET", "/v1/turns/"+pos[1], nil)
	case "requests":
		if len(pos) < 2 {
			return errors.New("request id required")
		}
		return call("GET", "/v1/requests/"+pos[1], nil)
	case "tasks", "runs":
		if len(pos) < 3 {
			return errors.New("show|cancel id required")
		}
		if pos[1] == "cancel" {
			return mutation(pos[0], pos[2], "cancel", nil)
		}
		return call("GET", "/v1/"+pos[0]+"/"+pos[2], nil)
	}
	return errors.New("unsupported command or missing parameters")
}
func actionFile(f map[string]string, submit func(contract.ActionProposal) error) error {
	b, e := os.ReadFile(f["file"])
	if e != nil {
		return e
	}
	var a contract.ActionProposal
	if e = contract.Decode("ActionProposal", b, &a); e != nil {
		return e
	}
	return submit(a)
}
func initData(dir string) error {
	for _, p := range []string{"state", "objects", "run", "logs", "reports"} {
		if e := os.MkdirAll(filepath.Join(dir, p), 0700); e != nil {
			return e
		}
	}
	if _, e := os.Stat(filepath.Join(dir, "config.json")); e == nil {
		return errors.New("already initialized")
	}
	s, e := store.Init(filepath.Join(dir, "state", "secretary.sqlite"), filepath.Join(dir, "objects"))
	if e != nil {
		return e
	}
	defer s.Close()
	for _, name := range []string{"client.token", "internal.token"} {
		b := make([]byte, 32)
		if _, e = rand.Read(b); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, "run", name), []byte(hex.EncodeToString(b)), 0600); e != nil {
			return e
		}
	}
	c := config.Default(dir)
	grant := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"notify.local", "alarm.play", "alarm.stop", "alarm.snooze", "artifact.write", "memory.refresh", "briefing.build", "memory.search", "world.update", "source.sync"}, Scope: map[string]any{"entity_ids": c.TestEntityIDs, "predicates": []string{"master.preference", "project.background", "entity.relation"}, "operations": []string{"ASSERT", "CORRECT", "RETRACT"}, "source_ids": []string{}, "path_roots": []string{filepath.Join(dir, "objects", "artifacts")}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().AddDate(1, 0, 0)), Extensions: map[string]any{}}
	if e = s.PutGrant(context.Background(), grant); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "run", "grant.id"), []byte(grant.ID), 0600); e != nil {
		return e
	}
	if e = c.Save(filepath.Join(dir, "config.json")); e != nil {
		return e
	}
	output(map[string]any{"status": "initialized", "data_dir": dir, "profile": "fixture", "grant_id": grant.ID})
	return nil
}

// Restored services can start with fresh local credentials while effects stay frozen.
func provisionRestored(s *store.Store, dir string) error {
	for _, sub := range []string{"run", "logs", "reports"} {
		if e := os.MkdirAll(filepath.Join(dir, sub), 0700); e != nil {
			return e
		}
	}
	for _, name := range []string{"client.token", "internal.token"} {
		b := make([]byte, 32)
		if _, e := rand.Read(b); e != nil {
			return e
		}
		if e := os.WriteFile(filepath.Join(dir, "run", name), []byte(hex.EncodeToString(b)), 0600); e != nil {
			return e
		}
	}
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().AddDate(1, 0, 0)), Extensions: map[string]any{}}
	if e := s.PutGrant(context.Background(), g); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "run", "grant.id"), []byte(g.ID), 0600)
}

func optionalString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
