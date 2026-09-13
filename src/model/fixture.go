package model

import (
	"encoding/json"
	"fmt"
	"secretarysimplified/contract"
)

// Fixture is deliberately labelled. It provides deterministic offline plumbing,
// not a claim of language understanding or a replacement for live evaluation.
func Fixture(r Request) (json.RawMessage, error) {
	b, _ := json.Marshal(r.Input)
	var m map[string]any
	json.Unmarshal(b, &m)
	var v any
	switch r.OutputType {
	case "DecisionEnvelope":
		id, _ := m["context_id"].(string)
		v = contract.DecisionEnvelope{SchemaVersion: 1, ContextID: id, Reply: map[string]any{"text": "Fixture profile: input recorded. Use typed commands or the configured live model for actions.", "evidence": []any{}}, Actions: []contract.ActionProposal{}, Controls: []contract.Control{}, Extensions: map[string]any{}}
	case "ConsciousnessDraft":
		var in contract.ConsciousnessStateInput
		json.Unmarshal(b, &in)
		entries := []contract.FocusEntry{}
		for _, it := range in.Live.Items {
			if it.Status != "DONE" && it.Status != "CANCELLED" && len(entries) < 20 {
				entries = append(entries, contract.FocusEntry{Entity: contract.ReadRef{EntityType: "Item", ID: it.ID, Revision: it.Revision}, Reason: it.Title, Evidence: it.Evidence})
			}
		}
		v = map[string]any{"schema_version": 1, "focal_goals": entries, "priority_items": entries, "open_loops": entries, "important_changes": []any{}, "uncertainties": []string{"Deterministic fixture synthesis; not a live semantic evaluation."}, "brief_summary": fmt.Sprintf("%d current open items in fixture projection.", len(entries)), "extensions": map[string]any{}}
	case "ConversationSummaryDraft":
		v = map[string]any{"schema_version": 1, "summary": "Fixture summary; authoritative commitments remain in Items and original events.", "focus_entity_ids": []string{}, "pending_question_ids": []string{}, "commitment_item_ids": []string{}, "extensions": map[string]any{}}
	default:
		return nil, fmt.Errorf("fixture output unsupported: %s", r.OutputType)
	}
	out, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	return out, contract.Validate(r.OutputType, json.RawMessage(out))
}
