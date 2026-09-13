package diagnostics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"syscall"
	"time"
)

type Report struct {
	CheckedAt       string         `json:"checked_at"`
	Database        store.Health   `json:"database"`
	Heartbeats      map[string]any `json:"heartbeats"`
	DiskFreeBytes   *uint64        `json:"disk_free_bytes"`
	ExecutionFrozen bool           `json:"execution_frozen"`
	LastBackup      any            `json:"last_backup"`
	Status          string         `json:"status"`
	Limitations     []string       `json:"limitations"`
}

func Doctor(ctx context.Context, s *store.Store, dataDir string) (Report, error) {
	return DoctorWithEpoch(ctx, s, dataDir, time.Time{})
}
func DoctorWithEpoch(ctx context.Context, s *store.Store, dataDir string, epoch time.Time) (Report, error) {
	r := Report{CheckedAt: contract.Now(), Heartbeats: map[string]any{}, LastBackup: map[string]any{"state": "UNKNOWN", "at": nil}, Status: "OK", Limitations: []string{"Unrecorded process heartbeats, scan times, backup times and budget exhaustion are UNKNOWN; no healthy state is inferred."}}
	h, e := s.Health(ctx)
	if e != nil {
		r.Status = "ERROR"
		return r, e
	}
	if epoch.IsZero() {
		h.Consciousness["overdue_reason"] = "UNKNOWN: configured epoch was not supplied"
	} else {
		expected := int(time.Since(epoch) / (24 * time.Hour))
		h.Consciousness["expected_slot"] = expected
		actual, exists := h.Consciousness["slot"].(int)
		h.Consciousness["overdue"] = expected >= 0 && (!exists || actual < expected)
		h.Consciousness["overdue_reason"] = "computed from configured epoch and exact persisted slot"
	}
	r.Database = h
	if raw, readErr := os.ReadFile(filepath.Join(dataDir, "backup.latest.json")); readErr == nil {
		var history map[string]any
		if json.Unmarshal(raw, &history) == nil && history["state"] == "COMPLETE" {
			r.LastBackup = history
		}
	}
	if h.Integrity != "ok" || h.ForeignKeyViolations > 0 {
		r.Status = "ERROR"
	}
	for _, p := range []string{"core", "runner"} {
		entry := map[string]any{"state": "UNKNOWN", "at": nil, "last_scan_at": nil}
		if b, e := os.ReadFile(filepath.Join(dataDir, "run", p+".heartbeat.json")); e == nil {
			var v map[string]any
			if json.Unmarshal(b, &v) == nil {
				if at, ok := v["at"].(string); ok {
					if t, e := time.Parse(time.RFC3339Nano, at); e == nil {
						entry["at"] = at
						entry["state"] = "STALE"
						age := time.Since(t)
						if age >= 0 && age < 10*time.Second {
							entry["state"] = "RECENT"
						}
						entry["last_scan_at"] = v["last_scan_at"]
					}
				}
			}
		}
		r.Heartbeats[p] = entry
	}
	var stat syscall.Statfs_t
	if e = syscall.Statfs(dataDir, &stat); e == nil {
		n := stat.Bavail * uint64(stat.Bsize)
		r.DiskFreeBytes = &n
	}
	_, e = os.Stat(filepath.Join(dataDir, "execution_frozen"))
	r.ExecutionFrozen = e == nil
	if r.Status == "OK" {
		r.Status = "DEGRADED"
	}
	return r, nil
}
