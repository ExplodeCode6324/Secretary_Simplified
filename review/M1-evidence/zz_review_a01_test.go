package contract

import (
	"encoding/json"
	"testing"
)

func rvItemMap() map[string]any {
	return map[string]any{
		"schema_version": 1, "id": NewID(), "revision": 1, "domain": "work", "kind": "TASK",
		"title": "review", "status": "OPEN", "priority": 1, "due_at": nil, "timezone": "UTC",
		"time_state": "UNKNOWN", "parent_id": nil, "dependency_ids": []any{}, "evidence": []any{},
		"created_at": Now(), "updated_at": Now(), "extensions": map[string]any{},
	}
}

func TestReviewA01StrictContractRejections(t *testing.T) {
	if e := Validate("Item", rvItemMap()); e != nil {
		t.Fatalf("valid baseline rejected: %v", e)
	}
	cases := []struct {
		name   string
		mutate func(m map[string]any)
	}{
		{"unknown_major", func(m map[string]any) { m["schema_version"] = 2 }},
		{"extra_top_field", func(m map[string]any) { m["surprise"] = true }},
		{"unknown_enum_status", func(m map[string]any) { m["status"] = "NEW" }},
		{"unknown_enum_time_state", func(m map[string]any) { m["time_state"] = "LATER" }},
		{"priority_out_of_range", func(m map[string]any) { m["priority"] = 9 }},
		{"bad_uuid", func(m map[string]any) { m["id"] = "not-a-uuid" }},
		{"bad_timestamp", func(m map[string]any) { m["created_at"] = "not-a-time" }},
		{"wrong_type", func(m map[string]any) { m["title"] = 42 }},
		{"unregistered_extension", func(m map[string]any) { m["extensions"] = map[string]any{"app.unknown": map[string]any{}} }},
		{"misfire_negative", func(m map[string]any) {
			m["extensions"] = map[string]any{"runtime.misfire": map[string]any{"omitted_occurrences": -1}}
		}},
		{"misfire_fraction", func(m map[string]any) {
			m["extensions"] = map[string]any{"runtime.misfire": map[string]any{"omitted_occurrences": 1.5}}
		}},
		{"auth_bad_grant", func(m map[string]any) {
			m["extensions"] = map[string]any{"runtime.authorization": map[string]any{"grant_id": "nope"}}
		}},
		{"auth_extra_key", func(m map[string]any) {
			m["extensions"] = map[string]any{"runtime.authorization": map[string]any{"grant_id": NewID(), "x": 1}}
		}},
	}
	for _, c := range cases {
		m := rvItemMap()
		c.mutate(m)
		if e := Validate("Item", m); e == nil {
			t.Errorf("A01 %s: expected rejection, got nil", c.name)
		} else {
			t.Logf("A01 %s rejected: %v", c.name, e)
		}
	}
	ok1 := rvItemMap()
	ok1["extensions"] = map[string]any{"runtime.misfire": map[string]any{"omitted_occurrences": 3}}
	if e := Validate("Item", ok1); e != nil {
		t.Errorf("registered misfire rejected: %v", e)
	}
	ok2 := rvItemMap()
	ok2["extensions"] = map[string]any{"runtime.authorization": map[string]any{"grant_id": NewID()}}
	if e := Validate("Item", ok2); e != nil {
		t.Errorf("registered authorization rejected: %v", e)
	}
	if e := ValidateRegistered("unregistered.kind", map[string]any{}); e == nil {
		t.Error("unregistered kind accepted")
	}
}

func TestReviewA01DecodeNoPartialWriteAndDupKeys(t *testing.T) {
	out := Item{SchemaVersion: 1, Title: "sentinel"}
	if e := Decode("Item", []byte(`{"schema_version":2,"id":"x"}`), &out); e == nil {
		t.Fatal("invalid decode accepted")
	}
	if out.Title != "sentinel" {
		t.Fatalf("partial write into out: %q", out.Title)
	}
	full, _ := json.Marshal(rvItemMap())
	var got Item
	if e := Decode("Item", full, &got); e != nil {
		t.Fatalf("valid item decode failed: %v", e)
	}
	if _, e := ParseJSON([]byte(`{"a":1,"a":2}`)); e == nil {
		t.Fatal("duplicate key accepted")
	}
	if _, e := CanonicalJSON([]byte(`{"v":1.5}`)); e == nil {
		t.Fatal("non-integer semantic number accepted")
	}
	cb, e := CanonicalJSON([]byte(`{"b":2,"a":[3,{"z":null,"y":1}]}`))
	if e != nil || string(cb) != `{"a":[3,{"y":1,"z":null}],"b":2}` {
		t.Fatalf("canonical: %s err=%v", cb, e)
	}
}
