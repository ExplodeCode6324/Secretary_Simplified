package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/executor"
	"secretarysimplified/store"
	"strings"
	"testing"
	"time"
)

func d12Ext(class string) map[string]any {
	return map[string]any{contract.ClassificationKey: map[string]any{"data_class": class}}
}
func d12Class(t *testing.T, ext map[string]any, want string) {
	t.Helper()
	got, e := contract.ReadClassification(ext)
	if e != nil || got != want {
		t.Fatalf("class %s expected %s: %v", got, want, e)
	}
}
func TestD12RuntimeJobRunReplanClassification(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	cmd := runtimeCommand()
	cmd.Extensions = d12Ext("PERSONAL")
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	at := contract.Now()
	job := contract.ScheduledJob{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, RootID: contract.NewID(), Enabled: true, Schedule: contract.Schedule{Kind: "once", At: &at, Timezone: "UTC", Weekdays: []int{}}, Command: cmd, TaskTemplate: map[string]any{"goal": "synthetic canary with PERSONAL label", "criteria": criteria, "item_id": nil, "item_operation_key": nil}, Misfire: "FIRE_ONCE_WITHIN_GRACE", GraceSeconds: 300, Overlap: "SKIP", MaxAttempts: 3, UpdatedAt: at, Extensions: d12Ext("SYNTHETIC")}
	if e = s.RegisterJob(ctx, job, g); e != nil {
		t.Fatal(e)
	}
	run, e := s.TriggerJob(ctx, job.ID, contract.NewID(), time.Now(), 1)
	if e != nil {
		t.Fatal(e)
	}
	d12Class(t, run.Extensions, "PERSONAL")
	d12Class(t, run.Command.Extensions, "PERSONAL")
	task, e := s.RuntimeTask(ctx, run.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	d12Class(t, task.Extensions, "PERSONAL")
	fixed := task.CriterionHash
	cmd.Extensions = map[string]any{} // This is a fresh model command; only the trusted request class may stamp it.
	control := contract.Control{Kind: "REPLAN", TaskID: &task.ID, Payload: map[string]any{"command": cmd, "reason": "synthetic classification replan"}}
	if e = s.Write(ctx, func(tx *sql.Tx) error { return s.ApplyControlTx(ctx, tx, control, task.RootID, dir, "SENSITIVE") }); e != nil {
		t.Fatal(e)
	}
	task, e = s.RuntimeTask(ctx, task.ID)
	if e != nil {
		t.Fatal(e)
	}
	d12Class(t, task.Extensions, "SENSITIVE")
	if task.CriterionHash != fixed {
		t.Fatal("criterion changed")
	}
	var raw string
	if e = s.DB.QueryRow(`SELECT payload_json FROM job_run WHERE task_id=? ORDER BY rowid DESC LIMIT 1`, task.ID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal([]byte(raw), &run); e != nil {
		t.Fatal(e)
	}
	d12Class(t, run.Extensions, "SENSITIVE")
	d12Class(t, run.Command.Extensions, "SENSITIVE")
	if run.Extensions["runtime.authorization"] == nil {
		t.Fatal("authorization lost")
	}
	runner := executor.New(s, nil)
	if _, e = runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"execution_attempt", "executor_receipt", "notification"} {
		if e = s.DB.QueryRow("SELECT payload_json FROM " + table + " ORDER BY rowid DESC LIMIT 1").Scan(&raw); e != nil {
			t.Fatal(table, e)
		}
		var v struct {
			Extensions map[string]any `json:"extensions"`
		}
		if e = json.Unmarshal([]byte(raw), &v); e != nil {
			t.Fatal(e)
		}
		d12Class(t, v.Extensions, "SENSITIVE")
	}
}
func TestD12UnknownCommandAndLegacyTaskAreNotSynthetic(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	g := runtimeGrant(t, s)
	cmd := runtimeCommand()
	cmd.Extensions = map[string]any{}
	criteria, _ := store.DeriveCriteria(cmd)
	if _, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g); e == nil || !strings.Contains(e.Error(), "OUTPUT_CLASS_UNKNOWN") {
		t.Fatal(e)
	}
	run := runtimeRegister(t, s, g)
	if _, e := s.DB.Exec(`UPDATE task SET payload_json=json_remove(payload_json,'$.extensions."security.classification"') WHERE id=?`, run.TaskID); e != nil {
		t.Fatal(e)
	}
	control := contract.Control{Kind: "REPLAN", TaskID: &run.TaskID, Payload: map[string]any{"command": runtimeCommand(), "reason": "synthetic unknown classification check"}}
	if e := s.Write(ctx, func(tx *sql.Tx) error { return s.ApplyControlTx(ctx, tx, control, contract.NewID(), dir, "SYNTHETIC") }); e == nil || !strings.Contains(e.Error(), "OUTPUT_CLASS_UNKNOWN") {
		t.Fatal(e)
	}
}
func TestD12ArtifactPersistentClassCannotBeForged(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	run := runtimeRegister(t, s, runtimeGrant(t, s))
	obj, e := s.PutObject(ctx, []byte("synthetic secret-labeled canary"), "text/plain", "SECRET")
	if e != nil {
		t.Fatal(e)
	}
	obj.DataClass = "SYNTHETIC"
	control := contract.Control{Kind: "SUBMIT_ARTIFACT", TaskID: &run.TaskID, Payload: map[string]any{"artifact": obj}}
	if e = s.Write(ctx, func(tx *sql.Tx) error { return s.ApplyControlTx(ctx, tx, control, contract.NewID(), dir, "SYNTHETIC") }); e == nil || e.Error() != "ARTIFACT_REFERENCE_MISMATCH" {
		t.Fatal(e)
	}
	obj.DataClass = "SECRET"
	control.Payload["artifact"] = obj
	if e = s.Write(ctx, func(tx *sql.Tx) error { return s.ApplyControlTx(ctx, tx, control, contract.NewID(), dir, "SYNTHETIC") }); e != nil {
		t.Fatal(e)
	}
	task, e := s.RuntimeTask(ctx, run.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	d12Class(t, task.Extensions, "SECRET")
}
func TestD12WorldProposalFactVersionCannotDowngrade(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	obj, e := s.PutObject(ctx, []byte("Synthetic classification-only fixture"), "text/plain", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	value := map[string]any{"key": "fixture", "value": "a"}
	p := contract.WorldUpdateProposal{SchemaVersion: 1, ID: contract.NewID(), RequestID: contract.NewID(), EntityID: contract.NewID(), Predicate: "master.preference", Operation: "ASSERT", FactID: contract.NewID(), Value: &value, Evidence: []contract.EvidenceRef{{ObjectID: obj.ID, SHA256: obj.SHA256, OriginID: obj.ID, Locator: "full", DataClass: obj.DataClass}}, Basis: "MASTER_EXPLICIT", PolicyRevision: 1, Reason: "classification fixture", Extensions: d12Ext("PERSONAL")}
	commit := func() contract.WorldFact {
		t.Helper()
		if e = s.Write(ctx, func(tx *sql.Tx) error { return s.PutProposalTx(ctx, tx, p) }); e != nil {
			t.Fatal(e)
		}
		v, e := s.CommitWorld(ctx, p, func(*sql.Tx) error { return nil })
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	first := commit()
	d12Class(t, first.Extensions, "PERSONAL")
	p.ID = contract.NewID()
	p.RequestID = contract.NewID()
	p.ExpectedRevision = 1
	p.Operation = "CORRECT"
	p.Extensions = d12Ext("SYNTHETIC")
	value["value"] = "b"
	second := commit()
	d12Class(t, second.Extensions, "PERSONAL")
}

func TestD12ArtifactWriteCarriesClassIntoStoredObject(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	g := probeGrant(t, s, []string{"artifact.write"}, []string{dir})
	cmd := runtimeCommand()
	cmd.Capability = "artifact.write"
	cmd.Arguments = map[string]any{"relative_path": "canary.txt", "content": "Synthetic canary; no private data"}
	cmd.Extensions = d12Ext("SENSITIVE")
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	runner := executor.New(s, &executor.Options{ArtifactDir: dir})
	if _, e = runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	var raw string
	if e = s.DB.QueryRow(`SELECT payload_json FROM executor_receipt WHERE run_id=?`, run.ID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var receipt contract.ExecutorReceipt
	if e = json.Unmarshal([]byte(raw), &receipt); e != nil {
		t.Fatal(e)
	}
	if len(receipt.Artifacts) != 1 || receipt.Artifacts[0].DataClass != "SENSITIVE" || !receipt.EffectObserved {
		t.Fatal(receipt)
	}
	d12Class(t, receipt.Extensions, "SENSITIVE")
	var class string
	if e = s.DB.QueryRow(`SELECT data_class FROM object_ref WHERE id=?`, receipt.Artifacts[0].ID).Scan(&class); e != nil || class != "SENSITIVE" {
		t.Fatal(class, e)
	}
}

func TestD12NotificationRejectsCallerClassificationDowngrade(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	cmd := runtimeCommand()
	cmd.Extensions = d12Ext("PERSONAL")
	criteria, _ := store.DeriveCriteria(cmd)
	_, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, runtimeGrant(t, s))
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.ClaimRun(ctx, "fixture", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.DispatchRun(ctx, run, "fixture", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	run.Extensions = d12Ext("SYNTHETIC")
	if e = s.RecordNotification(ctx, run); e != nil {
		t.Fatal(e)
	}
	notes, e := s.Notifications(ctx, "")
	if e != nil || len(notes) != 1 {
		t.Fatal(notes, e)
	}
	d12Class(t, notes[0].Extensions, "PERSONAL")
}

func TestD12ArtifactMetadataFailureRetainsCommittedEffect(t *testing.T) {
	s, dir := runtimeDB(t)
	ctx := context.Background()
	g := probeGrant(t, s, []string{"artifact.write"}, []string{dir})
	cmd := runtimeCommand()
	cmd.Capability = "artifact.write"
	cmd.Arguments = map[string]any{"relative_path": "committed.txt", "content": "Synthetic write before metadata fault"}
	cmd.Extensions = d12Ext("PERSONAL")
	criteria, _ := store.DeriveCriteria(cmd)
	run, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(`CREATE TRIGGER d12_object_fault BEFORE INSERT ON object_ref BEGIN SELECT RAISE(ABORT,'synthetic metadata fault'); END`); e != nil {
		t.Fatal(e)
	}
	runner := executor.New(s, &executor.Options{ArtifactDir: dir})
	if _, e = runner.Step(ctx); e == nil {
		t.Fatal("metadata fault not surfaced")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "committed.txt"))
	if e != nil || string(raw) != cmd.Arguments["content"] {
		t.Fatal("file effect missing", e)
	}
	var receiptRaw string
	if e = s.DB.QueryRow(`SELECT payload_json FROM executor_receipt WHERE run_id=?`, run.ID).Scan(&receiptRaw); e != nil {
		t.Fatal(e)
	}
	var receipt contract.ExecutorReceipt
	if e = json.Unmarshal([]byte(receiptRaw), &receipt); e != nil {
		t.Fatal(e)
	}
	if receipt.Status != "RESULT_UNKNOWN" || !receipt.EffectObserved {
		t.Fatal(receipt)
	}
	if worked, e := runner.Step(ctx); e != nil || worked {
		t.Fatal("blind retry", worked, e)
	}
}

func TestD12MutedAlarmSnoozeInheritsClass(t *testing.T) {
	s, _ := runtimeDB(t)
	ctx := context.Background()
	cmd := d08Command(t, s, "alarm.play")
	cmd.Extensions = d12Ext("PERSONAL")
	criteria, _ := store.DeriveCriteria(cmd)
	g := probeGrant(t, s, []string{"alarm.play"}, []string{})
	_, e := s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	runner := executor.New(s, nil)
	if _, e = runner.Step(ctx); e != nil {
		t.Fatal(e)
	}
	alarms, e := s.RuntimeAlarms(ctx)
	if e != nil || len(alarms) != 1 {
		t.Fatal(alarms, e)
	}
	d12Class(t, alarms[0].Extensions, "PERSONAL")
	if e = s.AlarmControl(ctx, alarms[0].ID, contract.NewID(), 60, time.Now()); e != nil {
		t.Fatal(e)
	}
	jobs, e := s.RuntimeJobs(ctx)
	if e != nil || len(jobs) != 1 {
		t.Fatal(jobs, e)
	}
	d12Class(t, jobs[0].Extensions, "PERSONAL")
	d12Class(t, jobs[0].Command.Extensions, "PERSONAL")
}
