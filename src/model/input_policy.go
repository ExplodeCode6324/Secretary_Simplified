package model

import (
	"encoding/json"
	"errors"
	"secretarysimplified/config"
	"secretarysimplified/contract"
)

// ValidateInputPolicy is shared by the provider and recording wrapper; arbitrary
// untyped maps cannot masquerade as a reviewed model Context.
func ValidateInputPolicy(c config.Config, r Request) error {
	var name string
	switch r.OutputType {
	case "DecisionEnvelope":
		name = "Context"
	case "ConsciousnessDraft":
		name = "ConsciousnessStateInput"
	case "ConversationSummaryDraft":
		name = ""
	default:
		return errors.New("UNKNOWN_MODEL_ROLE")
	}
	raw, e := json.Marshal(r.Input)
	if e != nil {
		return e
	}
	v, e := contract.ParseJSON(raw)
	if e != nil {
		return e
	}
	if e := validateDerivedInput(r.OutputType, v); e != nil {
		return e
	}
	if name != "" {
		if e := contract.Validate(name, r.Input); e != nil {
			return errors.New("INVALID_MODEL_INPUT")
		}
	}
	if r.OutputType == "ConversationSummaryDraft" {
		m, ok := v.(map[string]any)
		if !ok || len(m) != 3 {
			return errors.New("INVALID_MODEL_INPUT")
		}
		if e = contract.Validate("ConversationState", m["previous"]); e != nil {
			return errors.New("INVALID_MODEL_INPUT")
		}
		for field, typ := range map[string]string{"events": "ConversationEvent", "items": "Item"} {
			a, ok := m[field].([]any)
			if !ok {
				return errors.New("INVALID_MODEL_INPUT")
			}
			for _, x := range a {
				if e = contract.Validate(typ, x); e != nil {
					return errors.New("INVALID_MODEL_INPUT")
				}
			}
		}
	}
	if r.OutputType == "DecisionEnvelope" {
		m := v.(map[string]any)
		got, _ := json.Marshal(m["output_contract"])
		if string(got) != string(contract.Schema("DecisionEnvelope")) {
			return errors.New("OUTPUT_CONTRACT_MISMATCH")
		}
		if id, ok := m["context_id"].(string); !ok || id != r.ContextID {
			return errors.New("CONTEXT_ID_MISMATCH")
		}
	}
	rank := map[string]int{"SYNTHETIC": 0, "PERSONAL": 1, "SENSITIVE": 2, "SECRET": 3}
	declared, known := rank[r.DataClass]
	if !known || !c.Allows(r.DataClass) {
		return errors.New("DISCLOSURE_DENIED")
	}
	var scan func(any) error
	scan = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			if class, ok := x["data_class"].(string); ok {
				level, known := rank[class]
				if !known || !c.Allows(class) || level > declared {
					return errors.New("DISCLOSURE_DENIED")
				}
			}
			for key, value := range x {
				if key == "output_contract" {
					continue
				}
				if e := scan(value); e != nil {
					return e
				}
			}
		case []any:
			for _, value := range x {
				if e := scan(value); e != nil {
					return e
				}
			}
		}
		return nil
	}
	return scan(v)
}

// The role schemas identify derived carriers. Missing marks on those carriers
// cannot be justified by their surviving evidence or by the current input label.
func validateDerivedInput(role string, value any) error {
	m, ok := value.(map[string]any)
	if !ok {
		return errors.New("INVALID_MODEL_INPUT")
	}
	check := func(v any) error {
		if v == nil {
			return nil
		}
		_, e := contract.RequireDerivedClass(v)
		return e
	}
	array := func(v any) error {
		for _, x := range asArray(v) {
			if e := check(x); e != nil {
				return e
			}
		}
		return nil
	}
	state := func(v any) error {
		x, ok := v.(map[string]any)
		if !ok {
			return errors.New("INVALID_MODEL_INPUT")
		}
		if x["summary"] == "" && len(asArray(x["pending_questions"])) == 0 {
			return nil
		}
		return check(v)
	}
	changes := func(v any) error {
		for _, raw := range asArray(v) {
			ev, _ := raw.(map[string]any)
			typ, _ := ev["event_type"].(string)
			switch typ {
			case "item.created", "item.updated", "task.updated", "world.updated", "run.updated", "memory.refreshed", "scheduled_job.updated", "scheduled_job.skipped":
				change, _ := ev["change"].(map[string]any)
				for _, k := range []string{"before", "after"} {
					if e := check(change[k]); e != nil {
						return e
					}
				}
			}
		}
		return nil
	}
	if role == "ConversationSummaryDraft" {
		if e := state(m["previous"]); e != nil {
			return e
		}
		if e := array(m["events"]); e != nil {
			return e
		}
		return array(m["items"])
	}
	world, _ := m["world"].(map[string]any)
	live, _ := m["live"].(map[string]any)
	for _, v := range []any{world["facts"], live["items"], m["tasks"]} {
		if e := array(v); e != nil {
			return e
		}
	}
	if role == "ConsciousnessDraft" {
		if e := check(m["previous_snapshot"]); e != nil {
			return e
		}
		return changes(m["recent_changes"])
	}
	if e := state(m["conversation"]); e != nil {
		return e
	}
	if e := check(m["consciousness"]); e != nil {
		return e
	}
	if e := array(m["recent_events"]); e != nil {
		return e
	}
	ext, _ := m["extensions"].(map[string]any)
	retrieval, _ := ext["context.retrieval"].(map[string]any)
	if e := array(retrieval["events"]); e != nil {
		return e
	}
	return changes(m["delta_events"])
}
func asArray(v any) []any { a, _ := v.([]any); return a }
