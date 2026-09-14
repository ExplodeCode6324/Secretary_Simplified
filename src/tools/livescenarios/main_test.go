package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"testing"
	"time"
)

func TestFrozenFixtureAndAdmission(t *testing.T) {
	b, e := os.ReadFile("../../../reports/fixtures/live-scenarios/scenarios.json")
	if e != nil {
		t.Fatal(e)
	}
	var f Fixture
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.sqlite"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	for _, v := range f.Items {
		// This new state is authored by the explicitly synthetic fixture, not migrated from a legacy DB.
		v.Extensions, e = contract.ClassifyExtensions(v.Extensions, "SYNTHETIC")
		if e != nil {
			t.Fatal(e)
		}
		if e = s.PutItem(ctx, v, 0); e != nil {
			t.Fatal(e)
		}
	}
	g := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{}, Scope: map[string]any{"predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}, "entity_ids": []string{"00000000-0000-4000-8000-000000000100"}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if e = s.PutGrant(ctx, g); e != nil {
		t.Fatal(e)
	}
	if len(f.Steps) != 7 || f.Steps[0].MinCalls < 2 || f.Steps[5].MinCalls < 2 {
		t.Fatal("missing conflict/retrieval oracles")
	}
	for _, step := range f.Steps {
		for id := range step.Expected {
			if _, e = s.GetItem(ctx, id); e != nil {
				t.Fatal("oracle references unknown item", e)
			}
		}
	}
}

func TestWorldFixturePermitsAndFrozenStates(t *testing.T) {
	b, e := os.ReadFile("../../../reports/fixtures/live-scenarios/world.json")
	if e != nil {
		t.Fatal(e)
	}
	var f Fixture
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	d := t.TempDir()
	s, e := store.Init(filepath.Join(d, "state.sqlite"), filepath.Join(d, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	for _, step := range f.Steps {
		for _, v := range step.WorldBefore {
			if e = seedWorld(ctx, s, v); e != nil {
				t.Fatal(e)
			}
		}
		snap, e := s.Snapshot(ctx, contract.NewID())
		if e != nil {
			t.Fatal(e)
		}
		actual := map[string]string{}
		for _, v := range snap.Facts {
			actual[v.ID] = v.Status
		}
		for id, want := range step.ExpectedWorld {
			if actual[id] != want {
				t.Fatal(id, actual[id], want)
			}
		}
	}
}
