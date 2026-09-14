package contract

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRegisteredExtensionsEquivalentToOriginalInlineShape(t *testing.T) {
	var doc map[string]any
	if e := json.Unmarshal(SchemaJSON, &doc); e != nil {
		t.Fatal(e)
	}
	defs := doc["$defs"].(map[string]any)
	var original any
	// Exact inline shape before the purely structural D12 deduplication.
	if e := json.Unmarshal([]byte(`{"type":"object","propertyNames":{"pattern":"^[a-z][a-z0-9_]*\\.[a-z][a-z0-9_.]*$"},"additionalProperties":{"type":"object"},"description":"有命名空间的可选扩展","properties":{"security.classification":{"$ref":"#/$defs/SecurityClassification"}}}`), &original); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(defs["RegisteredExtensions"], original) {
		t.Fatal("shared shape changed validation semantics")
	}
	n := 0
	for _, v := range defs {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		p, _ := m["properties"].(map[string]any)
		if ext, ok := p["extensions"]; ok {
			n++
			if !reflect.DeepEqual(ext, map[string]any{"$ref": "#/$defs/RegisteredExtensions"}) {
				t.Fatal("carrier not using exact shared shape")
			}
		}
	}
	if n != 40 {
		t.Fatal("carrier set changed; audit expected set", n)
	}
	for _, bad := range []any{map[string]any{ClassificationKey: map[string]any{"data_class": "INVALID"}}, map[string]any{ClassificationKey: map[string]any{"data_class": "SYNTHETIC", "extra": true}}, map[string]any{ClassificationKey: nil}} {
		if Validate("RegisteredExtensions", bad) == nil {
			t.Fatal("bad class accepted", bad)
		}
	}
	if e := Validate("RegisteredExtensions", map[string]any{ClassificationKey: map[string]any{"data_class": "PERSONAL"}}); e != nil {
		t.Fatal(e)
	}
}
