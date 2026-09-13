package memory

import (
	"encoding/json"
	"secretarysimplified/contract"
	"secretarysimplified/model"
	"secretarysimplified/store"
)

func refreshManifest(req model.Request, in contract.ConsciousnessStateInput, snap store.MemorySnapshot, totalChanges int, wire []byte, profile string) contract.ContextManifest {
	refs := []contract.ReadRef{}
	for _, v := range snap.Items {
		refs = append(refs, contract.ReadRef{EntityType: "Item", ID: v.ID, Revision: v.Revision})
	}
	for _, v := range snap.Facts {
		refs = append(refs, contract.ReadRef{EntityType: "WorldFact", ID: v.ID, Revision: v.Revision})
	}
	for _, v := range snap.Tasks {
		refs = append(refs, contract.ReadRef{EntityType: "Task", ID: v.ID, Revision: v.Revision})
	}
	for _, v := range snap.Sources {
		refs = append(refs, contract.ReadRef{EntityType: "SourceState", ID: v.ID, Revision: v.Revision})
	}
	if snap.Consciousness != nil {
		v := snap.Consciousness
		refs = append(refs, contract.ReadRef{EntityType: "ConsciousnessState", ID: v.ID, Revision: v.Revision})
	}
	sections := []map[string]any{}
	add := func(name string, n, total int, v any, reason string) {
		b, _ := json.Marshal(v)
		sections = append(sections, map[string]any{"name": name, "selected_count": n, "omitted_count": total - n, "bytes": len(b), "reason": reason})
	}
	add("items", len(snap.Items), len(snap.Items)+snap.ItemOmitted, snap.Items, "current authoritative items")
	add("facts", len(snap.Facts), len(snap.Facts)+snap.FactOmitted, snap.Facts, "current facts including uncertainty")
	add("tasks", len(in.Tasks), len(snap.Tasks)+snap.TaskOmitted, in.Tasks, "current tasks and immutable criteria")
	add("delta_events", len(in.RecentChanges), totalChanges, in.RecentChanges, "bounded snapshot, schema maximum 100, then request byte budget")
	n := 0
	if in.PreviousSnapshot != nil {
		n = 1
	}
	add("consciousness", n, n, in.PreviousSnapshot, "previous derived snapshot")
	return contract.ContextManifest{SchemaVersion: 1, ID: req.ContextID, IntentID: req.RootID, SnapshotSeq: in.SnapshotSeq, AsOf: in.AsOf, ReadSet: refs, RequestHash: contract.Hash(wire), PolicyRevision: 1, OutputSchemaID: "ConsciousnessDraft", OutputSchemaHash: contract.Hash(contract.Schema("ConsciousnessDraft")), ModelProfile: profile, InputBytes: len(wire), InputTokens: len(wire), TokenCountMode: "CONSERVATIVE_ESTIMATE", Sections: sections, Extensions: map[string]any{}}
}
