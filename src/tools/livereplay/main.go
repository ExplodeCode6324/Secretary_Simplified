// livereplay compares production Core outputs with separately frozen oracles.
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
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: livereplay CONFIG FIXTURE_ROOT REPORT_DIR")
	}
	c, e := config.Load(os.Args[1])
	if e != nil {
		return e
	}
	root := os.Args[2]
	report := os.Args[3]
	if e = os.MkdirAll(report, 0700); e != nil {
		return e
	}
	s, e := store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
	if e != nil {
		return e
	}
	defer s.Close()
	grant, e := os.ReadFile(filepath.Join(c.DataDir, "run", "grant.id"))
	if e != nil {
		return e
	}
	client := &diagnostics.RecordingModel{Inner: model.New(c), Store: s, Config: c, Dir: filepath.Join(report, "model_calls")}
	svc := core.Service{Store: s, Model: client, Config: c, GrantID: string(grant)}
	ctx := context.Background()
	session := contract.NewID()
	results := []map[string]any{}
	failures := 0
	for i := 0; i < 90; i++ {
		var in struct {
			Index, Day, Point int
			AsOf              string `json:"as_of"`
			Text              string
		}
		b, e := os.ReadFile(filepath.Join(root, "input", fmt.Sprintf("%03d.json", i)))
		if e != nil {
			return e
		}
		if e = json.Unmarshal(b, &in); e != nil {
			return e
		}
		env := contract.InputEnvelope{SchemaVersion: 1, RequestID: contract.NewID(), SessionID: session, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: in.AsOf, Text: in.Text, AttachmentRefs: []contract.ObjectRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}
		turn, e := s.AcceptInput(ctx, env, 100)
		if e == nil {
			e = svc.Process(ctx, turn)
		}
		entry := map[string]any{"index": i, "status": "PASS", "request_id": env.RequestID}
		if e != nil {
			entry["error"] = e.Error()
		}
		var oracle struct{ Items map[string]map[string]any }
		b, e2 := os.ReadFile(filepath.Join(root, "oracle", fmt.Sprintf("%03d.json", i)))
		if e2 != nil {
			return e2
		}
		json.Unmarshal(b, &oracle)
		items, e2 := s.ListItems(ctx)
		if e2 != nil {
			return e2
		}
		actual := map[string]map[string]any{}
		for _, it := range items {
			var due any
			if it.DueAt != nil {
				due = *it.DueAt
			}
			actual[it.Title] = map[string]any{"domain": it.Domain, "status": it.Status, "due_at": due, "time_state": it.TimeState}
		}
		ab, _ := json.Marshal(actual)
		ob, _ := json.Marshal(oracle.Items)
		if e != nil || string(ab) != string(ob) {
			entry["status"] = "FAIL"
			entry["actual"] = actual
			entry["expected"] = oracle.Items
			failures++
		}
		if in.Point == 2 {
			now, _ := time.Parse(time.RFC3339Nano, in.AsOf)
			epoch := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
			m := memory.Service{Store: s, Model: client, Epoch: epoch, Config: c}
			snap, err := m.Refresh(ctx, now)
			if err != nil {
				entry["consciousness_status"] = "FAIL"
				entry["consciousness_error"] = err.Error()
				failures++
			} else {
				entry["consciousness_status"] = "PASS"
				entry["snapshot_id"] = snap.ID
			}
		}
		results = append(results, entry)
		raw, _ := json.MarshalIndent(map[string]any{"suite": "LIVE_MODEL-month-items-v1", "failures": failures, "results": results, "scope": "90 item changes plus 30 generated consciousness snapshots; does not alone cover all A23 world/conflict/source semantics"}, "", "  ")
		if e = os.WriteFile(filepath.Join(report, "report.json"), raw, 0600); e != nil {
			return e
		}
		fmt.Printf("checkpoint %02d: %v (cumulative failures=%d)\n", i, entry["status"], failures)
		if failures > 5 {
			return fmt.Errorf("replay stopped after %d failures; preserve evidence and fix", failures)
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d failed checks", failures)
	}
	return nil
}
