// livescenarios runs frozen synthetic semantic oracles against the real provider.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/diagnostics"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"strings"
	"time"
)

type Step struct {
	Name          string                    `json:"name"`
	Text          string                    `json:"text"`
	Session       string                    `json:"session"`
	ConflictID    string                    `json:"conflict_id"`
	Expected      map[string]map[string]any `json:"expected"`
	ReplyContains string                    `json:"reply_contains"`
	MinCalls      int                       `json:"min_calls"`
}
type Fixture struct {
	Items []contract.Item `json:"items"`
	Steps []Step          `json:"steps"`
	Scope string          `json:"scope"`
}
type capture struct {
	model.Client
	S          *store.Store
	Dir        string
	Calls      int
	ConflictID string
}

func (c *capture) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	c.Calls++
	n := c.Calls
	save(filepath.Join(c.Dir, fmt.Sprintf("call-%02d.context.json", n)), r.Input)
	out, e := c.Client.Generate(ctx, r)
	save(filepath.Join(c.Dir, fmt.Sprintf("call-%02d.result.json", n)), map[string]any{"output": json.RawMessage(out.Output), "raw_response": string(out.RawResponse), "validation_issues": out.ValidationIssues, "error": fmt.Sprint(e)})
	if c.ConflictID != "" {
		v, x := c.S.GetItem(ctx, c.ConflictID)
		if x != nil {
			return out, x
		}
		old := v.Revision
		v.Revision++
		v.Priority = 2
		v.UpdatedAt = contract.Now()
		if x = c.S.PutItem(ctx, v, old); x != nil {
			return out, x
		}
		c.ConflictID = ""
	}
	return out, e
}
func save(p string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(p, b, 0600)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: livescenarios CONFIG FIXTURE_JSON NEW_REPORT_DIR")
	}
	c, e := config.Load(os.Args[1])
	if e != nil {
		return e
	}
	if c.Model.Profile != "opencode-go" {
		return fmt.Errorf("real opencode-go profile required")
	}
	var f Fixture
	b, e := os.ReadFile(os.Args[2])
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, &f); e != nil {
		return e
	}
	report, e := filepath.Abs(os.Args[3])
	if e != nil {
		return e
	}
	if e = os.Mkdir(report, 0700); e != nil {
		return e
	}
	save(filepath.Join(report, "fixture.json"), f)
	c.DataDir = filepath.Join(report, "data")
	s, e := store.Init(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
	if e != nil {
		return e
	}
	defer s.Close()
	ctx := context.Background()
	grant := contract.NewID()
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: grant, PrincipalID: "master", Revision: 1, CapabilityIDs: []string{}, Scope: map[string]any{"predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}, "entity_ids": c.TestEntityIDs}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e = s.PutGrant(ctx, g); e != nil {
		return e
	}
	for _, v := range f.Items {
		if e = s.PutItem(ctx, v, 0); e != nil {
			return e
		}
	}
	at := "2000-01-01T00:00:00.000Z"
	if e = s.EnsureSource(ctx, contract.SourceState{SchemaVersion: 1, ID: "00000000-0000-4000-8000-000000000900", Kind: "FIXTURE", Revision: 1, LastSuccessAt: &at, StaleAfterSeconds: 60, Enabled: true, DataClass: "SYNTHETIC", Extensions: map[string]any{}}); e != nil {
		return e
	}
	sessions := map[string]string{}
	results := []map[string]any{}
	failed := 0
	for i, step := range f.Steps {
		before, err := s.ListItems(ctx)
		if err != nil {
			return err
		}
		dir := filepath.Join(report, fmt.Sprintf("%02d-%s", i, step.Name))
		os.Mkdir(dir, 0700)
		save(filepath.Join(dir, "oracle.json"), step)
		if sessions[step.Session] == "" {
			sessions[step.Session] = contract.NewID()
		}
		in := contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: sessions[step.Session], PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: step.Text, AttachmentRefs: []contract.ObjectRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}
		save(filepath.Join(dir, "input.json"), in)
		rec := &diagnostics.RecordingModel{Inner: model.New(c), Store: s, Config: c, Dir: filepath.Join(dir, "model_calls")}
		client := &capture{Client: rec, S: s, Dir: dir, ConflictID: step.ConflictID}
		svc := core.Service{Store: s, Model: client, Config: c, GrantID: grant}
		turn, err := s.AcceptInput(ctx, in, 100)
		if err == nil {
			err = svc.Process(ctx, turn)
		}
		turn, _ = s.GetTurn(ctx, turn.ID)
		items, x := s.ListItems(ctx)
		if x != nil {
			return x
		}
		actual := map[string]map[string]any{}
		for _, v := range items {
			raw, _ := json.Marshal(v)
			var m map[string]any
			json.Unmarshal(raw, &m)
			actual[v.ID] = m
		}
		mismatches := []string{}
		if len(actual) != len(before) {
			mismatches = append(mismatches, "unexpected item create/delete")
		}
		for _, old := range before {
			raw, _ := json.Marshal(old)
			var prior map[string]any
			json.Unmarshal(raw, &prior)
			for key, want := range prior {
				if key == "updated_at" || key == "revision" {
					continue
				}
				if _, allowed := step.Expected[old.ID][key]; allowed {
					continue
				}
				av, _ := json.Marshal(actual[old.ID][key])
				wv, _ := json.Marshal(want)
				if string(av) != string(wv) {
					mismatches = append(mismatches, old.ID+"."+key+": unauthorized change")
				}
			}
		}
		for id, fields := range step.Expected {
			for key, want := range fields {
				av, _ := json.Marshal(actual[id][key])
				wv, _ := json.Marshal(want)
				if string(av) != string(wv) {
					mismatches = append(mismatches, id+"."+key+": "+string(av)+" != "+string(wv))
				}
			}
		}
		reply := ""
		if turn.Reply != nil {
			reply, _ = (*turn.Reply)["text"].(string)
		}
		if step.ReplyContains != "" && !strings.Contains(reply, step.ReplyContains) {
			mismatches = append(mismatches, "missing reply oracle token")
		}
		if client.Calls < step.MinCalls {
			mismatches = append(mismatches, "insufficient calls for required retrieval/conflict rebuild")
		}
		entry := map[string]any{"name": step.Name, "status": "PASS", "request_id": in.RequestID, "calls": client.Calls, "error": fmt.Sprint(err), "mismatches": mismatches, "reply": reply}
		if err != nil || len(mismatches) > 0 {
			entry["status"] = "FAIL"
			failed++
		}
		save(filepath.Join(dir, "actual-items.json"), actual)
		save(filepath.Join(dir, "turn.json"), turn)
		rows, x := s.DB.QueryContext(ctx, "SELECT payload_json FROM context_manifest WHERE intent_id=?", turn.IntentID)
		if x != nil {
			return x
		}
		manifests := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			rows.Scan(&raw)
			manifests = append(manifests, json.RawMessage(raw))
		}
		rows.Close()
		save(filepath.Join(dir, "manifests.json"), manifests)
		results = append(results, entry)
		save(filepath.Join(report, "report.json"), map[string]any{"scope": f.Scope, "failures": failed, "results": results, "evidence": "input/oracle/final contexts/raw model outputs/manifests plus synthetic database and object files retained"})
		fmt.Printf("%s: %s\n", step.Name, entry["status"])
	}
	if failed > 0 {
		return fmt.Errorf("%d semantic checks failed; evidence preserved", failed)
	}
	return nil
}
