package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func probeGrant(t *testing.T, s *store.Store, caps []string, roots []string) string {
	t.Helper()
	g := contract.AuthorizationGrant{
		SchemaVersion:  1,
		ID:             contract.NewID(),
		PrincipalID:    "master",
		Revision:       1,
		CapabilityIDs:  caps,
		Scope:          map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": roots},
		PolicyRevision: 1,
		ExpiresAt:      contract.Timestamp(time.Now().Add(time.Hour)),
		Extensions:     map[string]any{},
	}
	if e := s.PutGrant(context.Background(), g); e != nil {
		t.Fatal(e)
	}
	return g.ID
}

func TestAyanamiProbeArtifactCancelMustDeny(t *testing.T) {
	s, dir := runtimeDB(t)
	g := probeGrant(t, s, []string{"artifact.write"}, []string{dir})
	cmd := contract.Command{SchemaVersion: 1, OperationKey: "write", Capability: "artifact.write", CapabilityVersion: 1, Arguments: map[string]any{"relative_path": "cancel.txt", "content": "x"}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	criteria, e := store.DeriveCriteria(cmd)
	if e != nil {
		t.Fatal(e)
	}
	run, e := s.RegisterImmediate(context.Background(), contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	claimed, e := s.ClaimRun(context.Background(), "worker", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	permit, e := s.DispatchRun(context.Background(), claimed, "worker", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CancelTask(context.Background(), run.TaskID); e != nil {
		t.Fatal(e)
	}
	if e = s.CheckArtifactScope(context.Background(), permit, filepath.Join(dir, "cancel.txt")); e == nil {
		t.Fatal("artifact scope still permits a cancelled dispatched task")
	}
}

func TestAyanamiProbeExpiredPermitMustDenyLocalEffect(t *testing.T) {
	s, _ := runtimeDB(t)
	g := probeGrant(t, s, []string{"notify.local"}, []string{})
	run := runtimeRegister(t, s, g)
	now := time.Now().Add(time.Second)
	claimed, e := s.ClaimRun(context.Background(), "worker", now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DispatchRun(context.Background(), claimed, "worker", now); e != nil {
		t.Fatal(e)
	}
	var permitID string
	if e = s.DB.QueryRow(`SELECT id FROM execution_permit WHERE run_id=?`, run.ID).Scan(&permitID); e != nil {
		t.Fatal(e)
	}
	past := contract.Timestamp(time.Now().Add(-time.Minute))
	var payload string
	if e = s.DB.QueryRow(`SELECT payload_json FROM execution_permit WHERE id=?`, permitID).Scan(&payload); e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	if e = json.Unmarshal([]byte(payload), &m); e != nil {
		t.Fatal(e)
	}
	m["expires_at"] = past
	b, _ := json.Marshal(m)
	if _, e = s.DB.Exec(`UPDATE execution_permit SET expires_at=?,payload_json=? WHERE id=?`, past, string(b), permitID); e != nil {
		t.Fatal(e)
	}
	if e = s.RecordNotification(context.Background(), claimed); e == nil {
		t.Fatal("expired permit still permitted local notification")
	}
}

func TestAyanamiProbeCancelledUnknownReconcileMustPreserveCancellation(t *testing.T) {
	s, dir := runtimeDB(t)
	g := probeGrant(t, s, []string{"notify.local"}, []string{})
	run := runtimeRegister(t, s, g)
	ctx := context.Background()
	now := time.Now().Add(time.Second)
	claimed, e := s.ClaimRun(ctx, "worker", now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DispatchRun(ctx, claimed, "worker", now); e != nil {
		t.Fatal(e)
	}
	if e = s.RecordNotification(ctx, claimed); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ClaimRun(ctx, "recovery", now.Add(31*time.Second)); e != sql.ErrNoRows {
		t.Fatal(e)
	}
	if e = s.CancelTask(ctx, run.TaskID); e != nil {
		t.Fatal(e)
	}
	if ok, e := s.ReconcileLocal(ctx, run.ID, dir); e != nil || !ok {
		t.Fatalf("reconcile: %v %v", ok, e)
	}
	var state string
	if e = s.DB.QueryRow(`SELECT state FROM task WHERE id=?`, run.TaskID).Scan(&state); e != nil {
		t.Fatal(e)
	}
	if state == "SUCCEEDED" {
		t.Fatal("reconciliation rewrote cancelled task to SUCCEEDED")
	}
}

func TestAyanamiProbeMutedAlarmPersistenceAndExpiry(t *testing.T) {
	s, _ := runtimeDB(t)
	g := probeGrant(t, s, []string{"alarm.play"}, []string{})
	audio := contract.ObjectRef{SchemaVersion: 1, ID: contract.NewID(), RelativePath: "fixture.wav", SHA256: "0000000000000000000000000000000000000000000000000000000000000000", MediaType: "audio/wav", ByteSize: 0, DataClass: "SYNTHETIC", CreatedAt: contract.Now(), Extensions: map[string]any{}}
	cmd := contract.Command{SchemaVersion: 1, OperationKey: "alarm", Capability: "alarm.play", CapabilityVersion: 1, Arguments: map[string]any{"audio_ref": audio, "device_id": "synthetic-muted", "max_duration_seconds": 1}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	criteria := []contract.Criterion{{ID: contract.NewID(), Kind: "notification_recorded", Expected: map[string]any{"notification_key": "alarm-proof"}, EvidencePolicy: "synthetic"}}
	run, e := s.RegisterImmediate(context.Background(), contract.NewID(), contract.NewID(), cmd, criteria, g)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().Add(time.Second)
	claimed, e := s.ClaimRun(context.Background(), "muted-runner", now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DispatchRun(context.Background(), claimed, "muted-runner", now); e != nil {
		t.Fatal(e)
	}
	if e = s.MutedAlarm(context.Background(), claimed); e != nil {
		t.Fatal(e)
	}
	var state string
	if e = s.DB.QueryRow(`SELECT state FROM alarm_session WHERE run_id=?`, run.ID).Scan(&state); e != nil {
		t.Fatal(e)
	}
	if state != "PLAYING" {
		t.Fatalf("alarm state %s", state)
	}
	if e = s.ExpireMutedAlarms(context.Background(), time.Now().Add(10*time.Second)); e != nil {
		t.Fatal(e)
	}
	if e = s.DB.QueryRow(`SELECT state FROM alarm_session WHERE run_id=?`, run.ID).Scan(&state); e != nil {
		t.Fatal(e)
	}
	if state != "STOPPED" {
		t.Fatalf("expired alarm state %s", state)
	}
}

func TestAyanamiProbeRejectedDispatchMustRefundActiveBudget(t *testing.T) {
	s, _ := runtimeDB(t)
	gID := probeGrant(t, s, []string{"notify.local"}, []string{})
	run := runtimeRegister(t, s, gID)
	claimed, e := s.ClaimRun(context.Background(), "worker", time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: gID, PrincipalID: "master", Revision: 2, CapabilityIDs: []string{"notify.local"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Revoked: true, Extensions: map[string]any{}}
	if e = s.PutGrant(context.Background(), g); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DispatchRun(context.Background(), claimed, "worker", time.Now().Add(time.Second)); e == nil {
		t.Fatal("revoked grant dispatched")
	}
	var root, raw string
	if e = s.DB.QueryRow(`SELECT root_id FROM task WHERE id=?`, run.TaskID).Scan(&root); e != nil {
		t.Fatal(e)
	}
	if e = s.DB.QueryRow(`SELECT payload_json FROM root_budget WHERE root_id=?`, root).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var budget map[string]any
	if e = json.Unmarshal([]byte(raw), &budget); e != nil {
		t.Fatal(e)
	}
	if used, _ := budget["active_ms_used"].(float64); used != 0 {
		t.Fatalf("active budget leaked: %v", used)
	}
}
