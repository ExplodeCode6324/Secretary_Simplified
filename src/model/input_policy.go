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
	if name != "" {
		if e := contract.Validate(name, r.Input); e != nil {
			return errors.New("INVALID_MODEL_INPUT")
		}
	}
	raw, e := json.Marshal(r.Input)
	if e != nil {
		return e
	}
	v, e := contract.ParseJSON(raw)
	if e != nil {
		return e
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
