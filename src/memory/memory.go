package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/diagnostics"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"time"
)

type Service struct {
	Store  *store.Store
	Model  model.Client
	Epoch  time.Time
	Config config.Config
}

func (s *Service) Refresh(ctx context.Context, now time.Time) (*contract.ConsciousnessState, error) {
	slot := int(now.Sub(s.Epoch) / (24 * time.Hour))
	if slot < 0 {
		slot = 0
	}
	return s.RefreshSlot(ctx, slot, now)
}

// RefreshSlot executes the immutable command target. It never substitutes the
// current wall-clock slot for the queued command's expected result.
func (s *Service) RefreshSlot(ctx context.Context, slot int, now time.Time) (state *contract.ConsciousnessState, resultErr error) {
	if slot < 0 {
		return nil, errors.New("INVALID_SLOT")
	}
	if e := s.Store.PinEpoch(ctx, s.Epoch); e != nil {
		return nil, e
	}
	if existing, e := s.Store.ConsciousnessAt(ctx, slot); e == nil {
		class, err := contract.ReadClassification(existing.Extensions)
		if err != nil {
			return nil, err
		}
		cfg := s.Config
		if len(cfg.Policy.AllowedClasses) == 0 {
			cfg.Policy.AllowedClasses = []string{"SYNTHETIC"}
		}
		if !cfg.Allows(class) {
			return nil, errors.New("DISCLOSURE_DENIED")
		}
		return existing, nil
	} else if e != sql.ErrNoRows {
		return nil, e
	}
	snap, e := s.Store.Snapshot(ctx, "00000000-0000-4000-8000-000000000000")
	if e != nil {
		return nil, e
	}
	cfg := s.Config
	if len(cfg.Policy.AllowedClasses) == 0 {
		cfg.Policy.AllowedClasses = []string{"SYNTHETIC"}
	}
	if e = ctxbuild.CheckDisclosure(cfg, snap, contract.InputEnvelope{}); e != nil {
		return nil, e
	}
	if snap.Consciousness != nil && snap.Consciousness.Slot > slot {
		return nil, errors.New("SUPERSEDED_MEMORY_SLOT")
	}
	in := contract.ConsciousnessStateInput{SchemaVersion: 1, Slot: slot, SnapshotSeq: snap.Seq, AsOf: contract.Timestamp(now), World: store.WorldProjection(snap, now), Live: store.LiveProjection(snap, now), OpenTaskRefs: []contract.VersionRef{}, RecentChanges: snap.Deltas, PreviousSnapshot: snap.Consciousness, Tasks: snap.Tasks, Extensions: map[string]any{}}
	if snap.Consciousness != nil {
		in.PreviousSnapshotID = &snap.Consciousness.ID
		in.ChangesFromSeq = snap.Consciousness.SnapshotSeq
		in.MissedSlots = slot - snap.Consciousness.Slot - 1
	}
	for _, t := range snap.Tasks {
		in.OpenTaskRefs = append(in.OpenTaskRefs, contract.VersionRef{ID: t.ID, Revision: t.Revision})
	}
	totalChanges := len(in.RecentChanges) + snap.DeltaOmitted
	if len(in.RecentChanges) > 100 {
		in.RecentChanges = in.RecentChanges[len(in.RecentChanges)-100:]
	}
	baseMissing := append([]string{}, in.Live.MissingReasons...)
	markOmitted := func() {
		in.Live.MissingReasons = append([]string{}, baseMissing...)
		if omitted := totalChanges - len(in.RecentChanges); omitted > 0 {
			in.Live.MissingReasons = append(in.Live.MissingReasons, fmt.Sprintf("OMITTED: %d change events excluded by bounded snapshot/schema/request budget; current authority retained", omitted))
		}
	}
	markOmitted()
	if e = contract.Validate("ConsciousnessStateInput", in); e != nil {
		return nil, e
	}
	root := contract.DeriveID("secretary.memory.slot.v1:" + contract.Timestamp(s.Epoch) + ":" + fmt.Sprint(slot))
	if e = s.Store.ChargeBudget(ctx, root, "model_calls", 1); e != nil {
		return nil, e
	}
	if e = s.Store.ChargeBudget(ctx, root, "output_tokens", 2000); e != nil {
		return nil, e
	}
	req := model.Request{RootID: root, ContextID: contract.NewID(), SessionID: "consciousness-" + s.Epoch.Format(time.RFC3339), OutputType: "ConsciousnessDraft", Input: in, DataClass: ctxbuild.EffectiveDataClass(snap, contract.InputEnvelope{}), Background: true}
	_, encodeErr := s.Model.Encode(req)
	for encodeErr != nil && len(in.RecentChanges) > 0 {
		in.RecentChanges = in.RecentChanges[1:]
		markOmitted()
		req.Input = in
		_, encodeErr = s.Model.Encode(req)
	}
	if encodeErr != nil {
		return nil, encodeErr
	}
	if e = contract.Validate("ConsciousnessStateInput", in); e != nil {
		return nil, e
	}
	wire, e := s.Model.Encode(req)
	if e != nil {
		return nil, e
	}
	manifest := refreshManifest(req, in, snap, totalChanges, wire, cfg.Model.Profile)
	manifest.Extensions, e = contract.ClassifyExtensions(manifest.Extensions, req.DataClass)
	if e != nil {
		return nil, e
	}
	if e = s.Store.SaveManifest(ctx, manifest); e != nil {
		return nil, e
	}
	r, e := diagnostics.Recorded(s.Model, s.Store, cfg).Generate(ctx, req)
	providerSucceeded := e == nil
	defer func() {
		recordMemoryResult(cfg, s.Store, r.CallID, req.ContextID, root, "consciousness", providerSucceeded, resultErr, req.DataClass)
	}()
	if e != nil {
		return nil, e
	}
	var draft contract.ConsciousnessDraft
	if e = contract.Decode("ConsciousnessDraft", r.Output, &draft); e != nil {
		return nil, e
	}
	known := map[string]int{}
	for _, x := range snap.Items {
		known["Item:"+x.ID] = x.Revision
	}
	for _, x := range snap.Facts {
		known["WorldFact:"+x.ID] = x.Revision
	}
	for _, x := range snap.Tasks {
		known["Task:"+x.ID] = x.Revision
	}
	for _, entries := range [][]contract.FocusEntry{draft.FocalGoals, draft.PriorityItems, draft.OpenLoops, draft.ImportantChanges} {
		for _, entry := range entries {
			if known[entry.Entity.EntityType+":"+entry.Entity.ID] != entry.Entity.Revision {
				return nil, errors.New("INVALID_REFERENCE")
			}
		}
	}
	b, _ := json.Marshal(in)
	v := contract.ConsciousnessState{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Slot: slot, SnapshotSeq: snap.Seq, PreviousSnapshotID: in.PreviousSnapshotID, InputHash: contract.Hash(b), CreatedAt: contract.Timestamp(now), FocalGoals: draft.FocalGoals, PriorityItems: draft.PriorityItems, OpenLoops: draft.OpenLoops, ImportantChanges: draft.ImportantChanges, Uncertainties: draft.Uncertainties, BriefSummary: draft.BriefSummary, MissedSlots: in.MissedSlots, Extensions: map[string]any{}}
	v.Extensions, e = contract.ClassifyExtensions(v.Extensions, req.DataClass)
	if e != nil {
		return nil, e
	}
	e = s.Store.SaveConsciousness(ctx, v)
	return &v, e
}

// Summarize incrementally covers a contiguous original-event range and CASes
// the prior coverage watermark; failed synthesis leaves all original rows.
func (s *Service) Summarize(ctx context.Context, session string) (state contract.ConversationState, resultErr error) {
	resultErr = s.Store.WithAuthorityConsumer(ctx, func(held context.Context) error {
		var e error
		state, e = s.summarizeAuthority(held, session)
		return e
	})
	return
}
func (s *Service) summarizeAuthority(ctx context.Context, session string) (state contract.ConversationState, resultErr error) {
	var prefixErr error
	ctx, prefixErr = s.Store.SummaryContext(ctx, session)
	if prefixErr != nil {
		return state, prefixErr
	}

	snap, e := s.Store.Snapshot(ctx, session)
	if e != nil {
		return contract.ConversationState{}, e
	}
	cfg := s.Config
	if len(cfg.Policy.AllowedClasses) == 0 {
		cfg.Policy.AllowedClasses = []string{"SYNTHETIC"}
	}
	if e = ctxbuild.CheckDisclosure(cfg, snap, contract.InputEnvelope{}); e != nil {
		return snap.Conversation, e
	}
	old := snap.Conversation
	events, e := s.Store.ConversationHistory(ctx, session, old.ThroughSequence, 40)
	if e != nil || len(events) == 0 {
		return old, e
	}
	root := contract.DeriveID(fmt.Sprintf("secretary.summary.v1:%s:%d", session, old.ThroughSequence))
	if e = s.Store.ReserveSummaryAttempt(ctx, root, time.Now(), 2000); e != nil {
		return old, e
	}
	req := model.Request{RootID: root, ContextID: contract.DeriveID(root + ":context"), SessionID: session, OutputType: "ConversationSummaryDraft", Input: map[string]any{"previous": old, "events": events, "items": snap.Items}, DataClass: ctxbuild.EffectiveDataClass(snap, contract.InputEnvelope{}), Background: true}
	r, e := diagnostics.Recorded(s.Model, s.Store, cfg).Generate(ctx, req)
	providerSucceeded := e == nil
	defer func() {
		recordMemoryResult(cfg, s.Store, r.CallID, contract.DeriveID(root+":context"), root, "conversation_summary", providerSucceeded, resultErr, req.DataClass)
	}()
	if e != nil {
		return old, e
	}
	var d contract.ConversationSummaryDraft
	if e = contract.Decode("ConversationSummaryDraft", r.Output, &d); e != nil {
		return old, e
	}
	known := map[string]bool{}
	for _, it := range snap.Items {
		known[it.ID] = true
	}
	for _, f := range snap.Facts {
		known[f.EntityID] = true
	}
	for _, id := range append(append([]string{}, d.FocusEntityIDs...), d.CommitmentItemIDs...) {
		if !known[id] {
			return old, errors.New("INVALID_REFERENCE")
		}
	}
	knownQuestions := map[string]map[string]any{}
	for _, q := range old.PendingQuestions {
		if id, ok := q["id"].(string); ok {
			knownQuestions[id] = q
		}
	}
	for _, id := range d.PendingQuestionIDs {
		if _, ok := knownQuestions[id]; !ok {
			return old, errors.New("UNKNOWN_PENDING_QUESTION")
		}
	}
	v := old
	v.Revision++
	v.SummaryFromSequence = 1
	v.ThroughSequence = events[len(events)-1].Sequence
	v.Summary = d.Summary
	v.FocusEntityIDs = d.FocusEntityIDs
	v.CommitmentItemIDs = d.CommitmentItemIDs
	v.RecentEventIDs = []string{}
	for _, ev := range events {
		v.RecentEventIDs = append(v.RecentEventIDs, ev.ID)
	}
	v.Extensions, e = contract.ClassifyExtensions(v.Extensions, req.DataClass)
	if e != nil {
		return old, e
	}
	if e = s.Store.SaveConversation(ctx, v, old.Revision, old.ThroughSequence); e != nil {
		return old, e
	}
	return v, nil
}

// Fixed codes avoid leaking provider bodies, source text, or local file paths.
func recordMemoryResult(c config.Config, st *store.Store, callID, contextID, rootID, role string, providerSucceeded bool, err error, dataClass string) {
	status, reason := "COMMITTED", ""
	if err != nil {
		status, reason = "SEMANTIC_REJECTED", "MEMORY_RESULT_REJECTED"
		if !providerSucceeded {
			status = "PROVIDER_FAILED"
			reason = "MODEL_CALL_FAILED"
		}
		for _, code := range []string{"OUTPUT_CLASS_UNKNOWN", "CLASSIFICATION_INJECTION", "INVALID_REFERENCE", "UNKNOWN_PENDING_QUESTION", "CONFLICT", "DISCLOSURE_DENIED", "BUDGET_EXHAUSTED", "CONTEXT_REQUIRED_OVERFLOW", "MODEL_HTTP_429", "MODEL_HTTP_401", "MODEL_SECRET_UNAVAILABLE"} {
			if err.Error() == code {
				reason = code
				break
			}
		}
	}
	_ = diagnostics.RecordMemoryOutcome(c, st, callID, contextID, rootID, role, status, reason, dataClass)
}
