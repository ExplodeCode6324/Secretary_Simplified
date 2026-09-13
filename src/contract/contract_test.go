package contract

import (
	"encoding/json"
	"testing"
)

func TestStrictContract(t *testing.T) {
	v := map[string]any{"title": "test", "domain": "work", "due_at": nil, "status": "OPEN"}
	if e := ValidateRegistered("fixture.item", v); e != nil {
		t.Fatal(e)
	}
	v["unexpected"] = true
	if ValidateRegistered("fixture.item", v) == nil {
		t.Fatal("unknown field accepted")
	}
	delete(v, "unexpected")
	v["due_at"] = "tomorrow"
	if ValidateRegistered("fixture.item", v) == nil {
		t.Fatal("invalid datetime accepted")
	}
	if Validate("Unknown", v) == nil {
		t.Fatal("unknown type")
	}
}
func TestSchemaClosure(t *testing.T) {
	var v map[string]any
	if e := json.Unmarshal(Schema("ConversationSummaryDraft"), &v); e != nil {
		t.Fatal(e)
	}
	defs := v["$defs"].(map[string]any)
	if _, ok := defs["ExecutionPermit"]; ok {
		t.Fatal("unrelated definition leaked")
	}
}
func TestDuplicateKeysAndCanonicalGolden(t *testing.T) {
	if _, e := ParseJSON([]byte(`{"a":{"b":1,"b":2}}`)); e == nil {
		t.Fatal("duplicate key")
	}
	b, e := CanonicalJSON([]byte(`{"z":null,"a":"<中文>","v":1}`))
	if e != nil {
		t.Fatal(e)
	}
	if string(b) != `{"a":"<中文>","v":1,"z":null}` {
		t.Fatal(string(b))
	}
}
