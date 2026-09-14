// Package ingest accepts explicit synthetic fixture snapshots. Absence never means deletion.
package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"sync"
)

type FixtureRecord struct {
	ExternalID string          `json:"external_id"`
	Version    string          `json:"version"`
	Deleted    bool            `json:"deleted"`
	Value      json.RawMessage `json:"value"`
}
type Result struct {
	Inserted    int `json:"inserted"`
	Duplicates  int `json:"duplicates"`
	Quarantined int `json:"quarantined"`
}
type Service struct {
	Store         *store.Store
	QuarantineDir string
}

// Ingest is a Core-owned writer. Serialize independent Service values in the
// process so lookup/object/record admission cannot race another fixture sync.
var fixtureMu sync.Mutex

func (s *Service) Sync(ctx context.Context, sourceID string, records []FixtureRecord, provenance ...store.SourceSyncProvenance) (Result, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	out := Result{}
	if e := s.Store.EnsureSource(ctx, contract.SourceState{SchemaVersion: 1, ID: sourceID, Kind: "FIXTURE", Revision: 1, StaleAfterSeconds: 86400, Enabled: true, DataClass: "SYNTHETIC", Extensions: map[string]any{}}); e != nil {
		return out, e
	}
	for _, r := range records {
		raw, e := json.Marshal(r)
		if e != nil {
			return out, e
		}
		hash := contract.Hash(raw)
		version := r.Version
		if version == "" {
			version = hash
		}
		previous, e := s.Store.LookupSourceVersion(ctx, sourceID, r.ExternalID, version)
		if e != nil {
			return out, e
		}
		if previous == hash {
			out.Duplicates++
			continue
		}
		if previous != "" {
			if e = s.quarantine(sourceID, version, raw, "SOURCE_VERSION_CONFLICT"); e != nil {
				return out, e
			}
			out.Quarantined++
			continue
		}
		var val contract.FixtureItemValue
		var validation error
		if r.ExternalID == "" {
			validation = fmt.Errorf("external_id is required")
		} else if !r.Deleted {
			validation = contract.Decode("FixtureItemValue", r.Value, &val)
		}
		if validation != nil {
			if e = s.quarantine(sourceID, version, raw, "INVALID_SOURCE_RECORD"); e != nil {
				return out, e
			}
			out.Quarantined++
			continue
		}
		obj, e := s.Store.PutObject(ctx, raw, "application/json", "SYNTHETIC")
		if e != nil {
			return out, e
		}
		v := contract.SourceRecord{SchemaVersion: 1, ID: contract.NewID(), SourceID: sourceID, ExternalID: r.ExternalID, SourceVersion: version, ContentHash: hash, RawRef: obj, ObservedAt: contract.Now(), Deleted: r.Deleted, NormalizedType: "fixture.item", ValidationStatus: "VALID", DataClass: "SYNTHETIC", Extensions: map[string]any{}}
		if !r.Deleted {
			v.Normalized = &val
		}
		dup, e := s.Store.SaveSourceRecord(ctx, v)
		if e != nil {
			return out, e
		}
		if dup {
			out.Duplicates++
		} else {
			out.Inserted++
		}
	}
	return out, s.Store.SourceSynced(ctx, sourceID, len(records), provenance...)
}
func (s *Service) quarantine(sourceID, version string, raw []byte, reason string) error {
	if s.QuarantineDir == "" {
		return fmt.Errorf("quarantine directory required")
	}
	if e := os.MkdirAll(s.QuarantineDir, 0700); e != nil {
		return e
	}
	key := contract.Hash(append([]byte(sourceID+"\n"+version+"\n"), raw...))
	b, _ := json.MarshalIndent(map[string]any{"schema_version": 1, "source_id": sourceID, "source_version": version, "reason": reason, "raw_sha256": contract.Hash(raw), "raw": json.RawMessage(raw)}, "", "  ")
	f, e := os.OpenFile(filepath.Join(s.QuarantineDir, key+".json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
