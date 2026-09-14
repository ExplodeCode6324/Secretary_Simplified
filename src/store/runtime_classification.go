package store

import (
	"context"
	"database/sql"
	"errors"
	"secretarysimplified/contract"
)

// rtInherit stamps a new target from known authoritative sources. Missing source
// labels are unknown, never an implicit synthetic baseline.
func rtInherit(target runtimeObject, sources ...runtimeObject) error {
	classes := []string{}
	for _, source := range sources {
		c, e := contract.ReadClassification(rtObj(source["extensions"]))
		if e != nil {
			return e
		}
		classes = append(classes, c)
	}
	c, e := contract.JoinClass(classes...)
	if e != nil {
		return e
	}
	ext, e := contract.ClassifyExtensions(rtObj(target["extensions"]), c)
	if e != nil {
		return e
	}
	target["extensions"] = ext
	return nil
}
func rtClass(target runtimeObject, class string) error {
	ext, e := contract.ClassifyExtensions(rtObj(target["extensions"]), class)
	if e != nil {
		return e
	}
	target["extensions"] = ext
	return nil
}
func rtObjectClassTx(ctx context.Context, tx *sql.Tx, id, hash string) (string, error) {
	var actual, class string
	if e := tx.QueryRowContext(ctx, "SELECT sha256,data_class FROM object_ref WHERE id=?", id).Scan(&actual, &class); e != nil {
		return "", e
	}
	if actual != hash {
		return "", errors.New("ARTIFACT_REFERENCE_MISMATCH")
	}
	return contract.JoinClass(class)
}
