package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/platform"
	"syscall"
	"time"
)

//go:embed 002_authority.sql
var authorityDDL string

type Authority struct {
	InstanceID             string                     `json:"instance_id"`
	SessionID              string                     `json:"session_id"`
	Revision               int                        `json:"revision"`
	HistorySequence        int                        `json:"history_sequence"`
	SummaryThroughSequence int                        `json:"summary_through_sequence"`
	State                  contract.ConversationState `json:"state"`
	PendingTurns           []contract.InputTurn       `json:"pending_turns"`
	LegacySessions         []map[string]any           `json:"legacy_sessions"`
}

func validateMigrations(ctx context.Context, db *sql.DB, requireAuthority bool) error {
	rows, e := db.QueryContext(ctx, "SELECT version,checksum FROM schema_migration ORDER BY version")
	if e != nil {
		return e
	}
	defer rows.Close()
	versions := 0
	for rows.Next() {
		var v int
		var hash string
		if e = rows.Scan(&v, &hash); e != nil {
			return e
		}
		versions++
		expected := ""
		if v == 1 {
			expected = contract.Hash([]byte(baseline))
		}
		if v == 2 {
			expected = contract.Hash([]byte(authorityDDL))
		}
		if v != versions || expected == "" || hash != expected {
			return errors.New("MIGRATION_MISMATCH")
		}
	}
	if e = rows.Err(); e != nil {
		return e
	}
	if versions == 0 {
		return errors.New("MIGRATION_MISMATCH")
	}
	if requireAuthority && versions != 2 {
		return errors.New("AUTHORITY_MIGRATION_REQUIRED: stop Core and Runner, run migrate with an explicit legacy authority selection")
	}
	return nil
}
func installAuthority(ctx context.Context, s *Store, selected string) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		var count int
		if e := tx.QueryRowContext(ctx, "SELECT count(*) FROM schema_migration WHERE version=2").Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			if selected != "" {
				var current string
				if e := tx.QueryRowContext(ctx, "SELECT session_id FROM authority_registry WHERE singleton=1").Scan(&current); e != nil {
					return e
				}
				if selected != current {
					return errors.New("AUTHORITY_ALREADY_REGISTERED")
				}
			}
			return nil
		}
		var candidates int
		if e := tx.QueryRowContext(ctx, "SELECT count(DISTINCT session_id) FROM input_turn WHERE principal_id='master' AND json_extract(payload_json,'$.input.origin')='MASTER_CLI'").Scan(&candidates); e != nil {
			return e
		}
		reason := "FRESH_INSTANCE"
		if candidates > 0 && selected == "" {
			return errors.New("AUTHORITY_SELECTION_REQUIRED")
		}
		if selected != "" {
			var exists int
			if e := tx.QueryRowContext(ctx, "SELECT count(*) FROM input_turn WHERE session_id=? AND principal_id='master' AND json_extract(payload_json,'$.input.origin')='MASTER_CLI'", selected).Scan(&exists); e != nil {
				return e
			}
			if exists == 0 {
				return errors.New("AUTHORITY_SELECTION_INVALID")
			}
			reason = "EXPLICIT_LEGACY_MASTER"
		} else {
			selected = contract.NewID()
			state := emptyConversation(selected)
			raw, _ := json.Marshal(state)
			if _, e := tx.ExecContext(ctx, "INSERT INTO conversation_session VALUES(?,1,0,?)", selected, string(raw)); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, authorityDDL); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO authority_registry VALUES(1,?,?,?,?)", contract.NewID(), selected, contract.Now(), reason); e != nil {
			return e
		}
		// Existing selected turns retain immutable identity and original MASTER order.
		if _, e := tx.ExecContext(ctx, `INSERT INTO authority_turn(turn_id,accepted_seq) SELECT id,row_number() OVER (ORDER BY COALESCE((SELECT min(sequence) FROM conversation_event e WHERE json_extract(e.payload_json,'$.turn_id')=t.id AND e.role='MASTER'),9223372036854775807),t.rowid) FROM input_turn t WHERE session_id=?`, selected); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO schema_migration VALUES(2,?,?)", contract.Hash([]byte(authorityDDL)), contract.Now())
		return e
	})
}
func UpgradeAuthority(path, objects, selected string) (*Store, error) {
	run := filepath.Join(filepath.Dir(objects), "run")
	if e := os.MkdirAll(run, 0700); e != nil {
		return nil, e
	}
	var locks []*platform.Lock
	defer func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Close()
		}
	}()
	for _, p := range []string{filepath.Join(run, "core.lock"), filepath.Join(run, "runner.lock"), path + ".migration.lock"} {
		l, e := platform.AcquireLock(p)
		if e != nil {
			return nil, fmt.Errorf("AUTHORITY_MIGRATION_BUSY: stop both daemon roles: %w", e)
		}
		locks = append(locks, l)
	}
	if _, e := os.Stat(path); e != nil {
		return nil, e
	}
	s, e := connect(path, objects)
	if e != nil {
		return nil, e
	}
	if e = validateMigrations(context.Background(), s.DB, false); e == nil {
		e = installAuthority(context.Background(), s, selected)
	}
	if e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) AuthoritySession(ctx context.Context) (string, error) {
	var id string
	e := s.DB.QueryRowContext(ctx, "SELECT session_id FROM authority_registry WHERE singleton=1").Scan(&id)
	return id, e
}
func authoritySessionTx(ctx context.Context, tx *sql.Tx, session string) error {
	var id string
	if e := tx.QueryRowContext(ctx, "SELECT session_id FROM authority_registry WHERE singleton=1").Scan(&id); e != nil {
		return e
	}
	if id != session {
		return errors.New("AUTHORITY_SESSION_MISMATCH")
	}
	return nil
}

// Resolve only omitted IDs. An explicit foreign ID is never rewritten on replay.
func (s *Store) ResolveSession(ctx context.Context, principal, request, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	var session string
	e := s.DB.QueryRowContext(ctx, "SELECT session_id FROM input_turn WHERE principal_id=? AND request_id=?", principal, request).Scan(&session)
	if e == nil {
		return session, nil
	}
	if e != sql.ErrNoRows {
		return "", e
	}
	return s.AuthoritySession(ctx)
}

type consumerKey struct{}

func (s *Store) WithAuthorityConsumer(ctx context.Context, fn func(context.Context) error) error {
	if held, ok := ctx.Value(consumerKey{}).(*Store); ok && held == s {
		return fn(ctx)
	}
	f, e := os.OpenFile(filepath.Join(s.ObjectsDir, ".authority-consumer.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	for {
		if e = ctx.Err(); e != nil {
			return e
		}
		e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == nil {
			break
		}
		if e != syscall.EWOULDBLOCK && e != syscall.EAGAIN {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn(context.WithValue(ctx, consumerKey{}, s))
}

type TurnPrefix struct {
	TurnID      string
	AcceptedSeq int
	Sequence    int
	RowID       int64
	State       contract.ConversationState
}
type prefixKey struct{}

func WithTurnPrefix(ctx context.Context, p TurnPrefix) context.Context {
	return context.WithValue(ctx, prefixKey{}, p)
}
func turnPrefix(ctx context.Context) (TurnPrefix, bool) {
	p, ok := ctx.Value(prefixKey{}).(TurnPrefix)
	return p, ok
}

// This shared predicate defines eligibility for both event text and its classes.
func prefixPredicate(p TurnPrefix, alias string) (string, []any) {
	return alias + `.session_id=? AND ` + alias + `.sequence<=? AND EXISTS(SELECT 1 FROM authority_turn ap JOIN input_turn tp ON tp.id=ap.turn_id WHERE tp.id=json_extract(` + alias + `.payload_json,'$.turn_id') AND ap.accepted_seq<=? AND (tp.state IN ('COMMITTED','FAILED') OR tp.id=?))`, []any{p.State.ID, p.Sequence, p.AcceptedSeq, p.TurnID}
}
func (s *Store) FreezeTurn(ctx context.Context, t contract.InputTurn) (p TurnPrefix, err error) {
	err = s.Write(ctx, func(tx *sql.Tx) error {
		if e := authoritySessionTx(ctx, tx, t.SessionID); e != nil {
			return e
		}
		var head string
		if e := tx.QueryRowContext(ctx, "SELECT t.id FROM input_turn t JOIN authority_turn a ON a.turn_id=t.id WHERE t.session_id=? AND t.state IN ('PENDING','PROCESSING') ORDER BY a.accepted_seq LIMIT 1", t.SessionID).Scan(&head); e != nil {
			return e
		}
		if head != t.ID {
			return errors.New("AUTHORITY_TURN_NOT_HEAD")
		}
		p.TurnID = t.ID
		var seq sql.NullInt64
		var rowid sql.NullInt64
		var raw sql.NullString
		if e := tx.QueryRowContext(ctx, "SELECT accepted_seq,prefix_sequence,prefix_rowid,frozen_state_json FROM authority_turn WHERE turn_id=?", t.ID).Scan(&p.AcceptedSeq, &seq, &rowid, &raw); e != nil {
			return e
		}
		if raw.Valid {
			p.Sequence = int(seq.Int64)
			p.RowID = rowid.Int64
			return contract.Decode("ConversationState", []byte(raw.String), &p.State)
		}
		var state []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", t.SessionID).Scan(&state); e != nil {
			return e
		}
		if e := contract.Decode("ConversationState", state, &p.State); e != nil {
			return e
		}
		if e := tx.QueryRowContext(ctx, "SELECT coalesce(max(sequence),0),coalesce(max(rowid),0) FROM conversation_event WHERE session_id=?", t.SessionID).Scan(&p.Sequence, &p.RowID); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, "UPDATE authority_turn SET prefix_sequence=?,prefix_rowid=?,frozen_state_json=? WHERE turn_id=?", p.Sequence, p.RowID, string(state), t.ID)
		return e
	})
	return
}
func (s *Store) SnapshotForTurn(ctx context.Context, t contract.InputTurn) (MemorySnapshot, error) {
	p, ok := turnPrefix(ctx)
	if !ok || p.TurnID != t.ID {
		return MemorySnapshot{}, errors.New("AUTHORITY_PREFIX_REQUIRED")
	}
	return s.Snapshot(WithTurnPrefix(ctx, p), t.SessionID)
}
func (s *Store) AuthorityState(ctx context.Context, limit int) (out Authority, err error) {
	out.PendingTurns = []contract.InputTurn{}
	out.LegacySessions = []map[string]any{}
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, "SELECT instance_id,session_id FROM authority_registry WHERE singleton=1").Scan(&out.InstanceID, &out.SessionID); e != nil {
		return out, e
	}
	var raw []byte
	if e = tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", out.SessionID).Scan(&raw); e != nil {
		return out, e
	}
	if e = contract.Decode("ConversationState", raw, &out.State); e != nil {
		return out, e
	}
	out.Revision = out.State.Revision
	out.SummaryThroughSequence = out.State.ThroughSequence
	if e = tx.QueryRowContext(ctx, "SELECT coalesce(max(sequence),0) FROM conversation_event WHERE session_id=?", out.SessionID).Scan(&out.HistorySequence); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, "SELECT t.payload_json FROM input_turn t JOIN authority_turn a ON a.turn_id=t.id WHERE t.session_id=? AND t.state IN ('PENDING','PROCESSING') ORDER BY a.accepted_seq LIMIT ?", out.SessionID, limit)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var t contract.InputTurn
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return out, e
		}
		if e = contract.Decode("InputTurn", raw, &t); e != nil {
			rows.Close()
			return out, e
		}
		out.PendingTurns = append(out.PendingTurns, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = tx.QueryContext(ctx, "SELECT id FROM conversation_session WHERE id<>? ORDER BY id LIMIT 100", out.SessionID)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return out, e
		}
		out.LegacySessions = append(out.LegacySessions, map[string]any{"id": id, "mode": "READ_ONLY"})
	}
	return out, rows.Err()
}

type AuthorityHistory struct {
	SessionID        string                       `json:"session_id"`
	Events           []contract.ConversationEvent `json:"events"`
	NextSequence     int                          `json:"next_sequence"`
	PreviousSequence int                          `json:"previous_sequence"`
	HasMore          bool                         `json:"has_more"`
	HistorySequence  int                          `json:"history_sequence"`
}

func (s *Store) AuthorityHistory(ctx context.Context, after, before, limit int, backward bool) (out AuthorityHistory, err error) {
	out.Events = []contract.ConversationEvent{}
	if after < 0 || before < 0 || limit < 1 || limit > 100 {
		return out, errors.New("INVALID_QUERY")
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, "SELECT session_id FROM authority_registry WHERE singleton=1").Scan(&out.SessionID); e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT coalesce(max(sequence),0) FROM conversation_event WHERE session_id=?", out.SessionID).Scan(&out.HistorySequence); e != nil {
		return out, e
	}
	if before == 0 {
		before = out.HistorySequence + 1
	}
	order := "ASC"
	if backward {
		order = "DESC"
	}
	rows, e := tx.QueryContext(ctx, "SELECT payload_json FROM conversation_event WHERE session_id=? AND sequence>? AND sequence<? ORDER BY sequence "+order+" LIMIT ?", out.SessionID, after, before, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var v contract.ConversationEvent
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if e = contract.Decode("ConversationEvent", raw, &v); e != nil {
			return out, e
		}
		out.Events = append(out.Events, v)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Events) > limit {
		out.HasMore = true
		out.Events = out.Events[:limit]
	}
	if backward {
		for i, j := 0, len(out.Events)-1; i < j; i, j = i+1, j-1 {
			out.Events[i], out.Events[j] = out.Events[j], out.Events[i]
		}
	}
	out.NextSequence = after
	if len(out.Events) > 0 {
		out.PreviousSequence = out.Events[0].Sequence
		out.NextSequence = out.Events[len(out.Events)-1].Sequence
	}
	return out, nil
}

// SummaryPrefix stops at the first pending physical event; it never claims a gap.
func (s *Store) SummaryContext(ctx context.Context, session string) (context.Context, error) {
	p := TurnPrefix{}
	e := s.Write(ctx, func(tx *sql.Tx) error {
		if e := authoritySessionTx(ctx, tx, session); e != nil {
			return e
		}
		var raw []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", session).Scan(&raw); e != nil {
			return e
		}
		if e := contract.Decode("ConversationState", raw, &p.State); e != nil {
			return e
		}
		if e := tx.QueryRowContext(ctx, "SELECT coalesce(max(accepted_seq),0) FROM authority_turn").Scan(&p.AcceptedSeq); e != nil {
			return e
		}
		if e := tx.QueryRowContext(ctx, "SELECT coalesce(min(e.sequence)-1,(SELECT coalesce(max(sequence),0) FROM conversation_event WHERE session_id=?)) FROM conversation_event e JOIN input_turn t ON t.id=json_extract(e.payload_json,'$.turn_id') WHERE e.session_id=? AND t.state IN ('PENDING','PROCESSING')", session, session).Scan(&p.Sequence); e != nil {
			return e
		}
		return tx.QueryRowContext(ctx, "SELECT coalesce(max(rowid),0) FROM conversation_event WHERE session_id=? AND sequence<=?", session, p.Sequence).Scan(&p.RowID)
	})
	return WithTurnPrefix(ctx, p), e
}
