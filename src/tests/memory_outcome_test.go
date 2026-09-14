package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"testing"
	"time"
)

type invalidMemoryResult struct{ model.Client }

func (p invalidMemoryResult) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	raw, e := model.Fixture(r)
	if e != nil {
		return model.Result{}, e
	}
	switch r.OutputType {
	case "ConsciousnessDraft":
		var d contract.ConsciousnessDraft
		json.Unmarshal(raw, &d)
		d.OpenLoops = []contract.FocusEntry{{Entity: contract.ReadRef{EntityType: "Item", ID: contract.NewID(), Revision: 1}, Reason: "synthetic unknown reference", Evidence: []contract.EvidenceRef{}}}
		raw, e = json.Marshal(d)
	case "ConversationSummaryDraft":
		var d contract.ConversationSummaryDraft
		json.Unmarshal(raw, &d)
		d.PendingQuestionIDs = []string{contract.NewID()}
		raw, e = json.Marshal(d)
	}
	return model.Result{Output: raw}, e
}
func TestMemoryProviderSuccessDoesNotHideSemanticRejection(t *testing.T) {
	for _, role := range []string{"consciousness", "conversation_summary"} {
		t.Run(role, func(t *testing.T) {
			s, c, p := setup(t)
			ctx := context.Background()
			epoch := time.Now().UTC()
			svc := memory.Service{Store: s, Config: c, Epoch: epoch, Model: invalidMemoryResult{p}}
			want := "INVALID_REFERENCE"
			if role == "consciousness" {
				if _, e := svc.RefreshSlot(ctx, 0, epoch); e == nil || e.Error() != want {
					t.Fatal(e)
				}
			} else {
				want = "UNKNOWN_PENDING_QUESTION"
				turn, e := s.AcceptInput(ctx, input(contract.NewID()), 100)
				if e != nil {
					t.Fatal(e)
				}
				if e = s.FinishTurn(ctx, turn.ID, map[string]any{"text": "synthetic reply", "evidence": []any{}}, []string{}, nil); e != nil {
					t.Fatal(e)
				}
				if _, e = svc.Summarize(ctx, turn.SessionID); e == nil || e.Error() != want {
					t.Fatal(e)
				}
				snap, e := s.Snapshot(ctx, turn.SessionID)
				if e != nil || snap.Conversation.ThroughSequence != 0 {
					t.Fatal("failed summary advanced watermark", e)
				}
			}
			files, e := filepath.Glob(filepath.Join(c.DataDir, "reports", "memory_attempts", "*.json"))
			if e != nil || len(files) != 1 {
				t.Fatal("missing durable memory outcome", files, e)
			}
			var outcome map[string]any
			raw, _ := os.ReadFile(files[0])
			if e = json.Unmarshal(raw, &outcome); e != nil {
				t.Fatal(e)
			}
			if outcome["status"] != "SEMANTIC_REJECTED" || outcome["reason"] != want || outcome["role"] != role {
				t.Fatal(outcome)
			}
			callID, ok := outcome["call_id"].(string)
			if !ok || callID == "" {
				t.Fatal("no model-call link")
			}
			var provider map[string]any
			raw, e = os.ReadFile(filepath.Join(c.DataDir, "reports", "model_calls", callID+".json"))
			if e != nil {
				t.Fatal(e)
			}
			json.Unmarshal(raw, &provider)
			if provider["status"] != "SUCCEEDED" {
				t.Fatal("provider outcome conflated with program outcome", provider["status"])
			}
			var n int
			s.DB.QueryRow("SELECT count(*) FROM consciousness_snapshot").Scan(&n)
			if n != 0 {
				t.Fatal("invalid memory was committed")
			}
		})
	}
}
