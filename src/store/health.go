package store

import (
	"context"
	"fmt"
)

type Health struct {
	SchemaVersion         int              `json:"schema_version"`
	Integrity             string           `json:"integrity"`
	ForeignKeyViolations  int              `json:"foreign_key_violations"`
	Queues                map[string]int   `json:"queues"`
	Sources               []map[string]any `json:"sources"`
	Consciousness         map[string]any   `json:"consciousness"`
	UnknownRuns           int              `json:"unknown_runs"`
	UnresolvedConflicts   int              `json:"unresolved_conflicts"`
	BudgetExhaustedReason string           `json:"budget_exhausted_reason"`
	BudgetExhausted       *int             `json:"budget_exhausted"`
}

func (s *Store) Health(ctx context.Context) (Health, error) {
	h := Health{BudgetExhaustedReason: "UNKNOWN: no durable exhaustion timestamp is recorded; current counters may be inspected separately", Queues: map[string]int{}, Sources: []map[string]any{}, Consciousness: map[string]any{"state": "UNKNOWN", "slot": nil, "created_at": nil, "overdue": nil}}
	if e := s.DB.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migration").Scan(&h.SchemaVersion); e != nil {
		return h, e
	}
	if e := s.DB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&h.Integrity); e != nil {
		return h, e
	}
	rows, e := s.DB.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		return h, e
	}
	for rows.Next() {
		h.ForeignKeyViolations++
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return h, e
	}
	for name, q := range map[string]string{"input_turn": "SELECT COUNT(*) FROM input_turn WHERE state IN ('PENDING','PROCESSING')", "core_work": "SELECT COUNT(*) FROM core_work WHERE state IN ('QUEUED','RUNNING')", "job_run": "SELECT COUNT(*) FROM job_run WHERE state IN ('QUEUED','CLAIMED','RUNNING')"} {
		var n int
		if e = s.DB.QueryRowContext(ctx, q).Scan(&n); e != nil {
			return h, fmt.Errorf("health %s: %w", name, e)
		}
		h.Queues[name] = n
	}
	if e = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM job_run WHERE state='RESULT_UNKNOWN'").Scan(&h.UnknownRuns); e != nil {
		return h, e
	}
	if e = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM world_fact_version v JOIN world_fact_head h ON v.fact_id=h.fact_id AND v.revision=h.revision WHERE v.status='CONTESTED'").Scan(&h.UnresolvedConflicts); e != nil {
		return h, e
	}
	rows, e = s.DB.QueryContext(ctx, "SELECT id,last_success_at FROM source_state ORDER BY id")
	if e != nil {
		return h, e
	}
	for rows.Next() {
		var id string
		var at *string
		if e = rows.Scan(&id, &at); e != nil {
			rows.Close()
			return h, e
		}
		h.Sources = append(h.Sources, map[string]any{"id": id, "last_success_at": at})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return h, e
	}
	var slot int
	var at string
	e = s.DB.QueryRowContext(ctx, "SELECT slot,created_at FROM consciousness_snapshot ORDER BY slot DESC LIMIT 1").Scan(&slot, &at)
	if e == nil {
		h.Consciousness = map[string]any{"state": "PRESENT", "slot": slot, "created_at": at, "overdue": nil}
	}
	return h, nil
}
