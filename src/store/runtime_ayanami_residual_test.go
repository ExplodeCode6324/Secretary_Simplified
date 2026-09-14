package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"secretarysimplified/contract"
	"strings"
	"testing"
	"time"
)

func TestAyanamiResidualCriterionHashMutationMustReject(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(filepath.Join(dir, "test.db"), filepath.Join(dir, "objects"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	grant := contract.AuthorizationGrant{SchemaVersion: 1, ID: contract.NewID(), PrincipalID: "master", Revision: 1, CapabilityIDs: []string{"notify.local"}, Scope: map[string]any{"entity_ids": []string{}, "predicates": []string{}, "operations": []string{}, "source_ids": []string{}, "path_roots": []string{}}, PolicyRevision: 1, ExpiresAt: contract.Timestamp(time.Now().Add(time.Hour)), Extensions: map[string]any{}}
	if err = s.PutGrant(context.Background(), grant); err != nil {
		t.Fatal(err)
	}
	cmd := contract.Command{SchemaVersion: 1, OperationKey: "notify", Capability: "notify.local", CapabilityVersion: 1, Arguments: map[string]any{"text": "x", "notification_key": "residual"}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	criteria, err := DeriveCriteria(cmd)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.RegisterImmediate(context.Background(), contract.NewID(), contract.NewID(), cmd, criteria, grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Write(context.Background(), func(tx *sql.Tx) error {
		task, e := rtRead(context.Background(), tx, "task", run.TaskID)
		if e != nil {
			return e
		}
		task["criterion_hash"] = strings.Repeat("0", 64)
		return rtSaveTask(context.Background(), tx, task)
	})
	if err == nil {
		t.Fatal("criterion hash mutation was accepted")
	}
}
