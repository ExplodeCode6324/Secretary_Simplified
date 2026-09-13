package tests

import (
	"context"
	"encoding/json"
	"secretarysimplified/contract"
	"secretarysimplified/memory"
	"secretarysimplified/model"
	"testing"
	"time"
)

type memoryCapture struct {
	model.Client
	Input contract.ConsciousnessStateInput
	Hash  string
	ID    string
}

func (c *memoryCapture) Generate(ctx context.Context, r model.Request) (model.Result, error) {
	c.Input = r.Input.(contract.ConsciousnessStateInput)
	b, e := c.Encode(r)
	if e != nil {
		return model.Result{}, e
	}
	c.Hash = contract.Hash(b)
	c.ID = r.ContextID
	return model.Result{Output: []byte(`{"schema_version":1,"focal_goals":[],"priority_items":[],"open_loops":[],"important_changes":[],"uncertainties":[],"brief_summary":"Bounded synthetic refresh","extensions":{}}`)}, nil
}
func TestMemoryMoreThanHundredDeltasPersistsExactManifest(t *testing.T) {
	s, c, p := setup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	it := contract.Item{SchemaVersion: 1, ID: contract.NewID(), Revision: 1, Domain: "work", Kind: "TASK", Title: "bounded refresh", Status: "OPEN", Priority: 1, Timezone: "UTC", TimeState: "UNKNOWN", DependencyIDs: []string{}, Evidence: []contract.EvidenceRef{}, CreatedAt: contract.Timestamp(now), UpdatedAt: contract.Timestamp(now), Extensions: map[string]any{}}
	for i := 0; i < 125; i++ {
		it.Revision = i + 1
		if e := s.PutItem(ctx, it, i); e != nil {
			t.Fatal(e)
		}
	}
	capture := &memoryCapture{Client: p}
	svc := memory.Service{Store: s, Model: capture, Config: c, Epoch: now}
	if _, e := svc.RefreshSlot(ctx, 0, now); e != nil {
		t.Fatal(e)
	}
	if len(capture.Input.RecentChanges) > 100 {
		t.Fatal("schema limit exceeded")
	}
	var b []byte
	if e := s.DB.QueryRow("SELECT payload_json FROM context_manifest WHERE id=?", capture.ID).Scan(&b); e != nil {
		t.Fatal(e)
	}
	var m contract.ContextManifest
	if e := contract.Decode("ContextManifest", b, &m); e != nil {
		t.Fatal(e)
	}
	if m.RequestHash != capture.Hash || m.OutputSchemaID != "ConsciousnessDraft" {
		t.Fatal("manifest not exact request")
	}
	found := false
	for _, section := range m.Sections {
		if section["name"] == "delta_events" {
			found = true
			n := int(section["selected_count"].(float64))
			o := int(section["omitted_count"].(float64))
			raw, _ := json.Marshal(capture.Input.RecentChanges)
			if n+o != 125 || n != len(capture.Input.RecentChanges) || int(section["bytes"].(float64)) != len(raw) {
				t.Fatal(section)
			}
		}
	}
	if !found || len(capture.Input.Live.MissingReasons) == 0 {
		t.Fatal("omission unreported")
	}
}
