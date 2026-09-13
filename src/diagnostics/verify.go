package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"time"
)

// VerifySuite runs only local, bounded, synthetic checks. It never certifies
// live-model semantics or changes the caller's business database.
func VerifySuite(ctx context.Context, suite, reportDir string) (map[string]any, error) {
	if suite != "persistence" && suite != "smoke" {
		return nil, errors.New("INVALID_ARGUMENT: supported suites are smoke and persistence")
	}
	if reportDir == "" {
		return nil, errors.New("--report required")
	}
	if e := os.MkdirAll(reportDir, 0700); e != nil {
		return nil, e
	}
	tmp, e := os.MkdirTemp("", "secretary-verify-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(tmp)
	checks := []map[string]any{}
	add := func(id string, e error) {
		status := "PASS"
		var detail any
		if e != nil {
			status = "FAIL"
			detail = "LOCAL_CHECK_FAILED"
		}
		checks = append(checks, map[string]any{"test_id": id, "status": status, "error": detail})
	}
	s, e := store.Init(filepath.Join(tmp, "state.sqlite"), filepath.Join(tmp, "objects"))
	if e != nil {
		return nil, e
	}
	defer s.Close()
	h, e := s.Health(ctx)
	if e == nil && (h.Integrity != "ok" || h.ForeignKeyViolations != 0) {
		e = errors.New("integrity")
	}
	add("sqlite_integrity", e)
	ref, e := s.PutObject(ctx, []byte("fixed synthetic verification fixture\n"), "text/plain", "SYNTHETIC")
	if e == nil {
		var raw []byte
		raw, e = s.ReadObject(ctx, ref.ID)
		if e == nil && string(raw) != "fixed synthetic verification fixture\n" {
			e = errors.New("object mismatch")
		}
	}
	add("immutable_object_roundtrip", e)
	b, e := s.Backup(ctx, filepath.Join(tmp, "backup"))
	_ = b
	if e == nil {
		_, e = store.VerifyBackup(ctx, filepath.Join(tmp, "backup"))
	}
	add("backup_manifest_and_objects", e)
	if e == nil {
		_, e = store.Restore(ctx, filepath.Join(tmp, "backup"), filepath.Join(tmp, "restore"))
	}
	if e == nil {
		_, e = os.Stat(filepath.Join(tmp, "restore", "execution_frozen"))
	}
	add("restore_integrity_and_freeze", e)
	status := "PASS"
	for _, c := range checks {
		if c["status"] != "PASS" {
			status = "FAIL"
		}
	}
	build := "UNKNOWN"
	if exe, err := os.Executable(); err == nil {
		if raw, err := os.ReadFile(exe); err == nil {
			build = contract.Hash(raw)
		}
	}
	out := map[string]any{"suite": suite, "status": status, "build_id": build, "schema_version": 1, "policy_version": 1, "clock_mode": "REAL_LOCAL_BOUNDED", "provider_profile": "fixture", "real_model": false, "real_data": false, "completed_at": time.Now().UTC().Format(time.RFC3339Nano), "tests": checks, "scope": "Local SQLite, object and backup smoke only. Does not certify A01-A25, model semantics, or real-use readiness."}
	raw, _ := json.MarshalIndent(out, "", "  ")
	if e = os.WriteFile(filepath.Join(reportDir, "report.json"), raw, 0600); e != nil {
		return out, e
	}
	md := "# Local verification\n\nStatus: " + status + "\n\nThis bounded synthetic suite validates SQLite integrity, immutable object bytes and backup/restore. It does not replace project acceptance or live-model testing. See report.json for individual checks and binary hash.\n"
	if e = os.WriteFile(filepath.Join(reportDir, "report.md"), []byte(md), 0600); e != nil {
		return out, e
	}
	if status != "PASS" {
		return out, errors.New("VERIFICATION_FAILED")
	}
	return out, nil
}
