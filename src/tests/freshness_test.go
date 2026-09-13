package tests

import (
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestFreshnessDistinguishesCurrentStaleUnknown(t *testing.T) {
	now := time.Now().UTC()
	at := contract.Timestamp(now.Add(-time.Minute))
	sourceID, unknownID, deviceID := contract.NewID(), contract.NewID(), contract.NewID()
	until := contract.Timestamp(now.Add(-time.Second))
	value := map[string]any{"available": true, "device_id": "synthetic"}
	snap := store.MemorySnapshot{Sources: []contract.SourceState{{SchemaVersion: 1, ID: sourceID, Kind: "FIXTURE", Revision: 1, LastSuccessAt: &at, StaleAfterSeconds: 120, Enabled: true, DataClass: "SYNTHETIC", Extensions: map[string]any{}}, {SchemaVersion: 1, ID: unknownID, Kind: "FIXTURE", Revision: 1, StaleAfterSeconds: 120, Enabled: true, DataClass: "SYNTHETIC", Extensions: map[string]any{}}}, Observations: []contract.Observation{{SchemaVersion: 1, ID: contract.NewID(), EntityID: deviceID, Kind: "device.availability", Value: &value, ObservedAt: at, ValidUntil: &until, Certainty: "OBSERVED", Evidence: []contract.EvidenceRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}}, Items: []contract.Item{}, Tasks: []contract.Task{}}
	p := store.LiveProjection(snap, now)
	states := map[string]string{}
	for _, f := range p.Freshness {
		states[f["entity_id"].(string)] = f["state"].(string)
	}
	if states[sourceID] != "CURRENT" || states[unknownID] != "UNKNOWN" || states[deviceID] != "STALE" {
		t.Fatal(states)
	}
	if e := contract.Validate("LiveWorldStateInput", p); e != nil {
		t.Fatal(e)
	}
}
