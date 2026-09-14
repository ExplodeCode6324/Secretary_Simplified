package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/contract"
	"time"
)

type MemorySnapshot struct {
	ItemOmitted   int
	FactOmitted   int
	TaskOmitted   int
	DataClasses   []string
	DeltaOmitted  int
	RecentOmitted int
	Seq           int
	Items         []contract.Item
	Facts         []contract.WorldFact
	Tasks         []contract.Task
	Sources       []contract.SourceState
	Observations  []contract.Observation
	Consciousness *contract.ConsciousnessState
	Conversation  contract.ConversationState
	Recent        []contract.ConversationEvent
	Deltas        []contract.ChangeEvent
}

func (s *Store) Snapshot(ctx context.Context, session string) (MemorySnapshot, error) {
	v := MemorySnapshot{Items: []contract.Item{}, Facts: []contract.WorldFact{}, Tasks: []contract.Task{}, Sources: []contract.SourceState{}, Observations: []contract.Observation{}, Recent: []contract.ConversationEvent{}, Deltas: []contract.ChangeEvent{}, Conversation: emptyConversation(session)}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, "SELECT coalesce(max(seq),0) FROM change_event").Scan(&v.Seq); e != nil {
		return v, e
	}
	classes := map[string]bool{}
	var collect func(any)
	collect = func(raw any) {
		switch x := raw.(type) {
		case map[string]any:
			if c, ok := x["data_class"].(string); ok {
				classes[c] = true
			}
			for _, v := range x {
				collect(v)
			}
		case []any:
			for _, v := range x {
				collect(v)
			}
		}
	}
	collectBytes := func(b []byte) {
		var raw any
		if json.Unmarshal(b, &raw) == nil {
			collect(raw)
		}
	}
	read := func(query, name string, appendValue func([]byte) error) error {
		rows, e := tx.QueryContext(ctx, query)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var b []byte
			if e = rows.Scan(&b); e != nil {
				return e
			}
			if name == "Item" || name == "Task" || name == "WorldFact" {
				if _, e = classificationBytes(b); e != nil {
					return e
				}
			}
			collectBytes(b)
			if e = contract.Validate(name, json.RawMessage(b)); e != nil {
				return e
			}
			if e = appendValue(b); e != nil {
				return e
			}
		}
		return rows.Err()
	}
	if e = read("SELECT payload_json FROM item ORDER BY created_at,id", "Item", func(b []byte) error {
		var x contract.Item
		e := json.Unmarshal(b, &x)
		v.Items = append(v.Items, x)
		return e
	}); e != nil {
		return v, e
	}
	if e = read("SELECT v.payload_json FROM world_fact_version v JOIN world_fact_head h ON v.fact_id=h.fact_id AND v.revision=h.revision ORDER BY v.fact_id", "WorldFact", func(b []byte) error {
		var x contract.WorldFact
		e := json.Unmarshal(b, &x)
		v.Facts = append(v.Facts, x)
		return e
	}); e != nil {
		return v, e
	}
	if e = read("SELECT payload_json FROM task WHERE state NOT IN ('SUCCEEDED','FAILED','CANCELLED') ORDER BY id", "Task", func(b []byte) error {
		var x contract.Task
		e := json.Unmarshal(b, &x)
		v.Tasks = append(v.Tasks, x)
		return e
	}); e != nil {
		return v, e
	}
	if e = read("SELECT payload_json FROM source_state ORDER BY id", "SourceState", func(b []byte) error {
		var x contract.SourceState
		e := json.Unmarshal(b, &x)
		v.Sources = append(v.Sources, x)
		return e
	}); e != nil {
		return v, e
	}
	if e = read("SELECT payload_json FROM observation ORDER BY observed_at DESC LIMIT 100", "Observation", func(b []byte) error {
		var x contract.Observation
		e := json.Unmarshal(b, &x)
		v.Observations = append(v.Observations, x)
		return e
	}); e != nil {
		return v, e
	}
	var b []byte
	e = tx.QueryRowContext(ctx, "SELECT payload_json FROM consciousness_snapshot ORDER BY slot DESC LIMIT 1").Scan(&b)
	if e == nil {
		var x contract.ConsciousnessState
		if _, e = classificationBytes(b); e != nil {
			return v, e
		}
		if e = contract.Decode("ConsciousnessState", b, &x); e != nil {
			return v, e
		}
		collectBytes(b)
		v.Consciousness = &x
	} else if !errors.Is(e, sql.ErrNoRows) {
		return v, e
	}
	e = tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", session).Scan(&b)
	if e == nil {
		var stateBody map[string]any
		if json.Unmarshal(b, &stateBody) != nil {
			return v, contract.ErrOutputClassUnknown
		}
		pending, _ := stateBody["pending_questions"].([]any)
		ext, _ := stateBody["extensions"].(map[string]any)
		if stateBody["summary"] != "" && stateBody["summary"] != nil || len(pending) > 0 || ext[contract.ClassificationKey] != nil {
			if _, e = classificationBytes(b); e != nil {
				return v, e
			}
		}
		if e = contract.Decode("ConversationState", b, &v.Conversation); e != nil {
			return v, e
		}
		if v.Conversation.Summary != "" || len(v.Conversation.PendingQuestions) > 0 {
			if _, e = contract.ReadClassification(v.Conversation.Extensions); e != nil {
				return v, e
			}
		}
		collectBytes(b)
	} else if !errors.Is(e, sql.ErrNoRows) {
		return v, e
	}
	rows, e := tx.QueryContext(ctx, "SELECT payload_json FROM conversation_event WHERE session_id=? ORDER BY sequence DESC LIMIT 40", session)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var x contract.ConversationEvent
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return v, e
		}
		var eventBody map[string]any
		if json.Unmarshal(b, &eventBody) != nil {
			return v, contract.ErrOutputClassUnknown
		}
		if eventBody["role"] != "MASTER" {
			if _, e = classificationBytes(b); e != nil {
				rows.Close()
				return v, e
			}
		}
		if e = contract.Decode("ConversationEvent", b, &x); e != nil {
			rows.Close()
			return v, e
		}
		if x.Role != "MASTER" {
			if _, e = contract.ReadClassification(x.Extensions); e != nil {
				rows.Close()
				return v, e
			}
			collectBytes(b)
		}
		v.Recent = append([]contract.ConversationEvent{x}, v.Recent...)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	after := 0
	if v.Consciousness != nil {
		after = v.Consciousness.SnapshotSeq
	}
	rows, e = tx.QueryContext(ctx, "SELECT payload_json FROM change_event WHERE seq>? ORDER BY seq LIMIT 1000", after)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var x contract.ChangeEvent
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return v, e
		}
		if e = contract.Decode("ChangeEvent", b, &x); e != nil {
			rows.Close()
			return v, e
		}
		if x.EntityType == "Item" || x.EntityType == "Task" || x.EntityType == "WorldFact" || x.EntityType == "ScheduledJob" {
			for _, key := range []string{"before", "after"} {
				if value := x.Change[key]; value != nil {
					raw, _ := json.Marshal(value)
					if _, e = classificationBytes(raw); e != nil {
						rows.Close()
						return v, e
					}
				}
			}
		}
		collectBytes(b)
		v.Deltas = append(v.Deltas, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	rows, e = tx.QueryContext(ctx, "SELECT payload_json FROM input_turn WHERE session_id=?", session)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return v, e
		}
		collectBytes(b)
	}
	e = rows.Err()
	rows.Close()
	var totalRecent, totalDelta int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM conversation_event WHERE session_id=?", session).Scan(&totalRecent); e != nil {
		return v, e
	}
	v.RecentOmitted = totalRecent - len(v.Recent)
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM change_event WHERE seq>?", after).Scan(&totalDelta); e != nil {
		return v, e
	}
	v.DeltaOmitted = totalDelta - len(v.Deltas)
	for c := range classes {
		v.DataClasses = append(v.DataClasses, c)
	}
	return v, e
}
func (s *Store) SaveConsciousness(ctx context.Context, v contract.ConsciousnessState) error {
	if _, e := contract.ReadClassification(v.Extensions); e != nil {
		return e
	}
	if e := contract.Validate("ConsciousnessState", v); e != nil {
		return e
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		var oldID string
		var oldSlot int
		e := tx.QueryRowContext(ctx, "SELECT id,slot FROM consciousness_snapshot ORDER BY slot DESC LIMIT 1").Scan(&oldID, &oldSlot)
		if e == nil {
			if v.Slot <= oldSlot {
				return errors.New("CONFLICT")
			}
			if v.PreviousSnapshotID == nil || *v.PreviousSnapshotID != oldID {
				return errors.New("CONFLICT")
			}
		} else if !errors.Is(e, sql.ErrNoRows) {
			return e
		} else if v.PreviousSnapshotID != nil {
			return errors.New("CONFLICT")
		}
		b, _ := json.Marshal(v)
		if _, e = tx.ExecContext(ctx, "INSERT INTO consciousness_snapshot VALUES(?,?,?,?,?,?)", v.ID, v.Slot, v.Revision, v.SnapshotSeq, v.CreatedAt, string(b)); e != nil {
			return e
		}
		return AppendEvent(ctx, tx, &contract.ChangeEvent{SchemaVersion: 1, ID: contract.NewID(), RootID: v.ID, EntityType: "memory", EntityID: v.ID, EntityRevision: v.Revision, EventType: "memory.refreshed", Origin: "memory.refresh", CreatedAt: v.CreatedAt, Change: map[string]any{"before": nil, "after": v, "evidence": []any{}}, Extensions: map[string]any{}})
	})
}
func (s *Store) SaveConversation(ctx context.Context, v contract.ConversationState, expected, previousThrough int) error {
	class, e := contract.ReadClassification(v.Extensions)
	if e != nil {
		return e
	}
	if e := contract.Validate("ConversationState", v); e != nil {
		return e
	}
	if v.Revision != expected+1 || v.ThroughSequence < previousThrough || v.SummaryFromSequence != 1 {
		return errors.New("CONFLICT")
	}
	return s.Write(ctx, func(tx *sql.Tx) error {
		var oldRaw []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM conversation_session WHERE id=?", v.ID).Scan(&oldRaw); e != nil {
			return e
		}
		var old contract.ConversationState
		if e := contract.Decode("ConversationState", oldRaw, &old); e != nil {
			return e
		}
		if old.Summary != "" || len(old.PendingQuestions) > 0 {
			prior, e := contract.ReadClassification(old.Extensions)
			if e != nil {
				return e
			}
			class, e = contract.JoinClass(class, prior)
			if e != nil {
				return e
			}
		}
		var e error
		v.Extensions, e = contract.ClassifyExtensions(v.Extensions, class)
		if e != nil {
			return e
		}
		var count int
		if e := tx.QueryRowContext(ctx, "SELECT count(*) FROM conversation_event WHERE session_id=? AND sequence>? AND sequence<=?", v.ID, previousThrough, v.ThroughSequence).Scan(&count); e != nil {
			return e
		}
		if count != v.ThroughSequence-previousThrough {
			return errors.New("SUMMARY_GAP")
		}
		b, _ := json.Marshal(v)
		r, e := tx.ExecContext(ctx, "UPDATE conversation_session SET revision=?,through_sequence=?,payload_json=? WHERE id=? AND revision=? AND through_sequence=?", v.Revision, v.ThroughSequence, string(b), v.ID, expected, previousThrough)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return errors.New("CONFLICT")
		}
		return nil
	})
}
func (s *Store) SearchMemory(ctx context.Context, query string, limit int) ([]contract.ConversationEvent, error) {
	if limit < 1 || limit > 10 {
		limit = 10
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT payload_json FROM conversation_event WHERE instr(json_extract(payload_json,'$.text'),?)>0 ORDER BY created_at DESC LIMIT ?", query, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []contract.ConversationEvent{}
	for rows.Next() {
		var b []byte
		var x contract.ConversationEvent
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = contract.Decode("ConversationEvent", b, &x); e != nil {
			return nil, e
		}
		if len(x.Text) > 2048 {
			x.Text = "[OMITTED: evidence exceeds page byte limit]"
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func WorldProjection(s MemorySnapshot, now time.Time) contract.WorldModelInput {
	conflicts := []string{}
	for _, f := range s.Facts {
		if f.ConflictGroup != nil {
			conflicts = append(conflicts, *f.ConflictGroup)
		}
	}
	return contract.WorldModelInput{SchemaVersion: 1, SnapshotSeq: s.Seq, AsOf: contract.Timestamp(now), Facts: s.Facts, UnresolvedConflictIDs: conflicts, OmittedCount: s.FactOmitted, MissingReasons: []string{}, Extensions: map[string]any{}}
}
func LiveProjection(s MemorySnapshot, now time.Time) contract.LiveWorldStateInput {
	refs := []contract.VersionRef{}
	for _, t := range s.Tasks {
		refs = append(refs, contract.VersionRef{ID: t.ID, Revision: t.Revision})
	}
	fresh := []map[string]any{}
	seen := map[string]bool{}
	omitted := s.ItemOmitted + s.TaskOmitted
	reasons := []string{}
	add := func(id, state, reason string) {
		if len(fresh) >= 100 {
			omitted++
			return
		}
		fresh = append(fresh, map[string]any{"entity_id": id, "state": state, "as_of": contract.Timestamp(now), "reason": reason})
		seen[id] = true
	}
	for _, source := range s.Sources {
		state, reason := "UNKNOWN", "source has no successful sync"
		if !source.Enabled {
			reason = "source disabled"
		} else if source.LastSuccessAt != nil {
			t, e := time.Parse(time.RFC3339Nano, *source.LastSuccessAt)
			if e == nil {
				state = "CURRENT"
				reason = "last successful sync=" + *source.LastSuccessAt
				if now.Before(t) {
					state = "UNKNOWN"
					reason = "successful sync timestamp is in the future"
				} else if !now.Before(t.Add(time.Duration(source.StaleAfterSeconds) * time.Second)) {
					state = "STALE"
					reason = "source sync freshness deadline expired; " + reason
				}
			}
		}
		add(source.ID, state, reason)
	}
	for _, ob := range s.Observations {
		if seen[ob.EntityID] {
			continue
		}
		state, reason := "UNKNOWN", "point-in-time observation has no current validity deadline; observed_at="+ob.ObservedAt
		observed, e := time.Parse(time.RFC3339Nano, ob.ObservedAt)
		if e == nil && !now.Before(observed) && ob.ValidUntil != nil {
			until, e := time.Parse(time.RFC3339Nano, *ob.ValidUntil)
			if e == nil {
				state = "CURRENT"
				reason = "observed_at=" + ob.ObservedAt + "; valid_until=" + *ob.ValidUntil
				if !now.Before(until) {
					state = "STALE"
				}
			}
		}
		add(ob.EntityID, state, reason)
	}
	if omitted > 0 {
		reasons = append(reasons, fmt.Sprintf("OMITTED: %d unselected items/tasks/freshness entries; omission does not establish absence", omitted))
	}
	return contract.LiveWorldStateInput{SchemaVersion: 1, SnapshotSeq: s.Seq, AsOf: contract.Timestamp(now), Items: s.Items, TaskRefs: refs, Observations: s.Observations, SourceStates: s.Sources, Freshness: fresh, OmittedCount: omitted, MissingReasons: reasons, Extensions: map[string]any{}}
}
func (s *Store) ConversationHistory(ctx context.Context, session string, after, limit int) ([]contract.ConversationEvent, error) {
	if limit < 1 || limit > 40 {
		limit = 40
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT payload_json FROM conversation_event WHERE session_id=? AND sequence>? ORDER BY sequence LIMIT ?", session, after, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []contract.ConversationEvent{}
	for rows.Next() {
		var raw []byte
		var v contract.ConversationEvent
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = contract.Decode("ConversationEvent", raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ConsciousnessAt(ctx context.Context, slot int) (*contract.ConsciousnessState, error) {
	var b []byte
	if e := s.DB.QueryRowContext(ctx, "SELECT payload_json FROM consciousness_snapshot WHERE slot=?", slot).Scan(&b); e != nil {
		return nil, e
	}
	var v contract.ConsciousnessState
	if e := contract.Decode("ConsciousnessState", b, &v); e != nil {
		return nil, e
	}
	if v.Slot != slot {
		return nil, errors.New("STORAGE_CORRUPTION: consciousness slot")
	}
	return &v, nil
}
func (s *Store) ReserveSummaryAttempt(ctx context.Context, root string, now time.Time, outputTokens int) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		if e := ensureBudget(ctx, tx, root); e != nil {
			return e
		}
		var raw []byte
		if e := tx.QueryRowContext(ctx, "SELECT payload_json FROM root_budget WHERE root_id=?", root).Scan(&raw); e != nil {
			return e
		}
		var b contract.RootBudget
		if e := contract.Decode("RootBudget", raw, &b); e != nil {
			return e
		}
		if b.ModelCallsUsed >= 3 {
			return errors.New("SUMMARY_ATTEMPTS_EXHAUSTED")
		}
		if b.CooldownUntil != nil {
			until, e := time.Parse(time.RFC3339Nano, *b.CooldownUntil)
			if e != nil {
				return e
			}
			if now.Before(until) {
				return errors.New("SUMMARY_RETRY_NOT_DUE")
			}
		}
		b.ModelCallsUsed++
		b.OutputTokensUsed += outputTokens
		b.Revision++
		b.Limits["model_calls"] = 3
		delay := 5 * time.Minute
		if b.ModelCallsUsed >= 2 {
			delay = 30 * time.Minute
		}
		next := contract.Timestamp(now.Add(delay))
		b.CooldownUntil = &next
		if e := contract.Validate("RootBudget", b); e != nil {
			return e
		}
		raw, _ = json.Marshal(b)
		_, e := tx.ExecContext(ctx, "UPDATE root_budget SET revision=?,payload_json=? WHERE root_id=?", b.Revision, string(raw), root)
		return e
	})
}

func classificationBytes(raw []byte) (string, error) {
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil {
		return "", contract.ErrOutputClassUnknown
	}
	ext, _ := v["extensions"].(map[string]any)
	return contract.ReadClassification(ext)
}
