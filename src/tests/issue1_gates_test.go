package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/model"
	"strings"
	"testing"
)

// All canaries are invented. The real provider serialization and policy path
// runs, but the HTTP transport cannot reach an external service.
func TestIssue1GatePersonalAuthorizationAndDirectoryIsolation(t *testing.T) {
	ctx := context.Background()
	local, localConfig, localModel := setup(t)
	localService := core.Service{Store: local, Config: localConfig, Model: localModel}
	secretAction := createAction()
	secretAction.Payload["title"] = "INVENTED_LOCAL_ONLY_SECRET_ISSUE1"
	if _, err := localService.TypedClass(ctx, contract.NewID(), contract.NewID(), []contract.ActionProposal{secretAction}, "SECRET"); err != nil {
		t.Fatal(err)
	}
	for _, authorized := range []bool{false, true} {
		t.Run(map[bool]string{false: "unapproved", true: "explicitly_approved"}[authorized], func(t *testing.T) {
			s, c, _ := setup(t)
			if c.DataDir == localConfig.DataDir {
				t.Fatal("directories overlap")
			}
			if authorized {
				c.Policy.AllowedClasses = []string{"SYNTHETIC", "PERSONAL"}
			}
			c.Model.Profile = "opencode-go"
			c.Model.SecretRef = filepath.Join(t.TempDir(), "fake.key")
			if err := os.WriteFile(c.Model.SecretRef, []byte("invented-test-key"), 0600); err != nil {
				t.Fatal(err)
			}
			p := model.New(c)
			calls := 0
			p.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				wire, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(wire, []byte("INVENTED_LOCAL_ONLY_SECRET_ISSUE1")) {
					t.Fatal("other directory disclosed")
				}
				if !bytes.Contains(wire, []byte("INVENTED_APPROVED_PERSONAL_ISSUE1")) {
					t.Fatal("positive control input absent")
				}
				var body struct {
					Input []struct {
						Content string `json:"content"`
					} `json:"input"`
				}
				if err = json.Unmarshal(wire, &body); err != nil {
					t.Fatal(err)
				}
				var projection map[string]any
				if err = json.Unmarshal([]byte(body.Input[1].Content), &projection); err != nil {
					t.Fatal(err)
				}
				out, err := model.Fixture(model.Request{ContextID: projection["context_id"].(string), OutputType: "DecisionEnvelope", Input: projection})
				if err != nil {
					t.Fatal(err)
				}
				response, _ := json.Marshal(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(out)}}}}})
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(response))}, nil
			})}
			svc := core.Service{Store: s, Config: c, Model: p}
			in := input(contract.NewID())
			in.Text = "INVENTED_APPROVED_PERSONAL_ISSUE1"
			raw, _ := json.Marshal(in)
			var body map[string]any
			json.Unmarshal(raw, &body)
			delete(body, "data_class") // Exercise the public default, not a test-assigned label.
			raw, _ = json.Marshal(body)
			w := httptest.NewRecorder()
			svc.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/v1/inputs", bytes.NewReader(raw)))
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			if err := s.DB.QueryRow("SELECT payload_json FROM input_turn WHERE request_id=?", in.RequestID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var turn contract.InputTurn
			if err := json.Unmarshal(raw, &turn); err != nil {
				t.Fatal(err)
			}
			if turn.Input.DataClass != "PERSONAL" {
				t.Fatal(turn.Input.DataClass)
			}
			if err := svc.Process(ctx, turn); err != nil {
				t.Fatal(err)
			}
			final, err := s.GetTurn(ctx, turn.ID)
			if err != nil || final.Reply == nil {
				t.Fatal(err, final)
			}
			text := (*final.Reply)["text"].(string)
			if authorized {
				if calls != 1 || !strings.Contains(text, "Fixture profile") {
					t.Fatal(calls, text)
				}
			} else if calls != 0 || !strings.Contains(text, "DISCLOSURE_DENIED") {
				t.Fatal(calls, text)
			}
			// The supported isolation does not claim mixed-store availability.
			if authorized {
				if _, err := svc.TypedClass(ctx, contract.NewID(), contract.NewID(), []contract.ActionProposal{secretAction}, "SECRET"); err != nil {
					t.Fatal(err)
				}
				blocked := input(contract.NewID())
				blocked.DataClass = "PERSONAL"
				bt, err := s.AcceptInput(ctx, blocked, 100)
				if err != nil {
					t.Fatal(err)
				}
				if err = svc.Process(ctx, bt); err != nil {
					t.Fatal(err)
				}
				bf, err := s.GetTurn(ctx, bt.ID)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 1 || !strings.Contains((*bf.Reply)["text"].(string), "DISCLOSURE_DENIED") {
					t.Fatal("mixed store unexpectedly disclosed", calls, bf.Reply)
				}
			}
			// Model diagnostic archives must not contain either private-class canary.
			if err := filepath.WalkDir(filepath.Join(c.DataDir, "reports"), func(path string, entry os.DirEntry, walkErr error) error {
				if os.IsNotExist(walkErr) {
					return nil
				}
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if bytes.Contains(b, []byte("INVENTED_LOCAL_ONLY_SECRET_ISSUE1")) || bytes.Contains(b, []byte("INVENTED_APPROVED_PERSONAL_ISSUE1")) {
					t.Fatal("private-class text archived in diagnostics", filepath.Base(path))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
