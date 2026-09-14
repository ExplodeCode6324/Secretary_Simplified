package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/contract"
	"secretarysimplified/core"
	"secretarysimplified/executor"
	"secretarysimplified/store"
	"testing"
	"time"
)

// Current run is the latest admitted run; historical evidence is never reused.
func TestD08ReplanCurrentRunEventuallyVerifies(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	g := probeGrant(t, s, []string{"alarm.play"}, []string{})
	svc := &core.Service{Store: s, Config: c, Model: p, GrantID: g}
	cmd := d08Command(t, s, "alarm.play")
	run := d08Typed(t, svc, cmd)
	if ok, e := s.VerifyTask(ctx, run.TaskID, ""); e != nil || ok {
		t.Fatal("missing evidence", ok, e)
	}
	task, e := s.RuntimeTask(ctx, run.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	control := contract.Control{Kind: "REPLAN", TaskID: &task.ID, Payload: map[string]any{"reason": "retry after absent evidence", "command": cmd}}
	if e = s.Write(ctx, func(tx *sql.Tx) error { return s.ApplyControlTx(ctx, tx, control, task.RootID, s.ObjectsDir) }); e != nil {
		t.Fatal(e)
	}
	runner := executor.New(s, nil)
	if worked, e := runner.Step(ctx); e != nil || !worked {
		t.Fatal(worked, e)
	}
	if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || !ok {
		t.Fatalf("new REPLAN run has valid session but cannot verify: passed=%v err=%v", ok, e)
	}
}

func TestD08CurrentRunLineageAcrossReopen(t *testing.T) {
	for _, cap := range []string{"alarm.play", "briefing.build", "source.sync"} {
		t.Run(cap, func(t *testing.T) {
			s, c, p := setup(t)
			ctx := context.Background()
			g := probeGrant(t, s, []string{cap}, []string{})
			cmd := d08Command(t, s, cap)
			if cap == "source.sync" {
				id := cmd.Arguments["source_id"].(string)
				path := filepath.Join(c.DataDir, "fixture.json")
				os.WriteFile(path, []byte(`[]`), 0600)
				c.SourceConfigs = []config.SourceConfig{{ID: id, FixturePath: path}}
				var raw string
				s.DB.QueryRow(`SELECT payload_json FROM authorization_grant WHERE id=?`, g).Scan(&raw)
				var grant contract.AuthorizationGrant
				contract.Decode("AuthorizationGrant", []byte(raw), &grant)
				grant.Revision++
				grant.Scope["source_ids"] = []string{id}
				if e := s.PutGrant(ctx, grant); e != nil {
					t.Fatal(e)
				}
			}
			svc := &core.Service{Store: s, Config: c, Model: p, GrantID: g}
			original := d08Typed(t, svc, cmd)
			claimed, e := s.ClaimRun(ctx, "lineage", time.Now().Add(time.Second))
			if e != nil {
				t.Fatal(e)
			}
			permit, e := s.DispatchRun(ctx, claimed, "lineage", time.Now().Add(time.Second))
			if e != nil {
				t.Fatal(e)
			}
			var receipt contract.ExecutorReceipt
			if cap == "alarm.play" {
				if e = s.MutedAlarm(ctx, claimed); e != nil {
					t.Fatal(e)
				}
				receipt = contract.ExecutorReceipt{SchemaVersion: 1, ID: contract.NewID(), RunID: claimed.ID, AttemptNo: claimed.AttemptNo, FencingToken: claimed.FencingToken, ReceiptKey: "first-proof", Status: "SUCCEEDED", EffectObserved: true, Artifacts: []contract.ObjectRef{}, Evidence: []contract.EvidenceRef{}, ReceivedAt: contract.Now(), Extensions: map[string]any{}}
			} else {
				receipt, e = (d08Bridge{svc.InternalHandler()}).Execute(ctx, claimed, permit)
				if e != nil {
					t.Fatal(e)
				}
			}
			if e = s.RecordReceipt(ctx, receipt); e != nil {
				t.Fatal(e)
			}
			task, _ := s.RuntimeTask(ctx, original.TaskID)
			replan := func() error {
				return s.Write(ctx, func(tx *sql.Tx) error {
					return s.ApplyControlTx(ctx, tx, contract.Control{Kind: "REPLAN", TaskID: &task.ID, Payload: map[string]any{"reason": "lineage fixture", "command": cmd}}, task.RootID, s.ObjectsDir)
				})
			}
			// REPLAN wins before verification; both successive generations must ignore old proof.
			for i := 0; i < 2; i++ {
				if e = replan(); e != nil {
					t.Fatal(e)
				}
				if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || ok {
					t.Fatal("historical proof leaked", i, ok, e)
				}
			}
			s.Close()
			s, e = store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			svc.Store = s
			if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || ok {
				t.Fatal("restart restored obsolete proof", ok, e)
			}
			runner := executor.New(s, &executor.Options{Core: d08Bridge{svc.InternalHandler()}})
			if worked, e := runner.Step(ctx); e != nil || !worked {
				t.Fatal(worked, e)
			}
			if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || !ok {
				t.Fatal("current proof missing", ok, e)
			}
			// Verification wins first: a subsequent REPLAN cannot reopen the terminal Task.
			if e = replan(); e == nil {
				t.Fatal("verified terminal task reopened")
			}
			var current string
			s.DB.QueryRow(`SELECT id FROM job_run WHERE task_id=? ORDER BY rowid DESC LIMIT 1`, task.ID).Scan(&current)
			deleteProof := func(id string) {
				switch cap {
				case "alarm.play":
					_, e = s.DB.Exec(`DELETE FROM alarm_session WHERE run_id=?`, id)
				case "briefing.build":
					_, e = s.DB.Exec(`DELETE FROM executor_receipt WHERE run_id=?`, id)
				case "source.sync":
					_, e = s.DB.Exec(`DELETE FROM change_event WHERE event_type='source.synced' AND json_extract(payload_json,'$.extensions."runtime.source_sync".run_id')=?`, id)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
			old, e := s.GetRun(ctx, original.ID)
			if e != nil {
				t.Fatal(e)
			}
			oldState := old.State
			old.State = "RUNNING"
			oldJSON, _ := json.Marshal(old)
			if _, e = s.DB.Exec(`UPDATE job_run SET state=?,payload_json=? WHERE id=?`, old.State, string(oldJSON), old.ID); e != nil {
				t.Fatal(e)
			}
			if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || ok {
				t.Fatal("older unresolved run ignored", ok, e)
			}
			old.State = oldState
			oldJSON, _ = json.Marshal(old)
			s.DB.Exec(`UPDATE job_run SET state=?,payload_json=? WHERE id=?`, old.State, string(oldJSON), old.ID)
			deleteProof(original.ID)
			if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || !ok {
				t.Fatal("old proof required", ok, e)
			}
			if cap == "source.sync" {
				var raw string
				s.DB.QueryRow(`SELECT payload_json FROM change_event WHERE event_type='source.synced'`).Scan(&raw)
				var event contract.ChangeEvent
				contract.Decode("ChangeEvent", []byte(raw), &event)
				prov := event.Extensions["runtime.source_sync"].(map[string]any)
				prov["fencing_token"] = prov["fencing_token"].(float64) + 1
				b, _ := json.Marshal(event)
				s.DB.Exec(`UPDATE change_event SET payload_json=? WHERE id=?`, string(b), event.ID)
				if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || ok {
					t.Fatal("wrong current fence provenance passed", ok, e)
				}
			}
			deleteProof(current)
			if ok, e := s.VerifyTask(ctx, task.ID, ""); e != nil || ok {
				t.Fatal("missing current evidence passed", ok, e)
			}
		})
	}
}
