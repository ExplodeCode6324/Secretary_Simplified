package store

import (
	"context"
	"database/sql"
	"errors"
	"secretarysimplified/contract"
	"testing"
)

func TestAcceptanceA04AtomicCommitBoundaries(t *testing.T) {
	for _, point := range []string{"state_write", "event_write", "receipt_write", "after_receipt_before_commit"} {
		t.Run(point, func(t *testing.T) {
			s := foundationDB(t)
			ctx := context.Background()
			item := fixtureItem()
			request := contract.NewID()
			table := map[string]string{"state_write": "item", "event_write": "change_event", "receipt_write": "request_receipt"}[point]
			// Persistent trigger affects every SQLite connection.
			if table != "" {
				if _, e := s.DB.Exec("CREATE TRIGGER fail_boundary BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(ABORT,'INJECTED_BOUNDARY'); END"); e != nil {
					t.Fatal(e)
				}
			}
			e := s.Write(ctx, func(tx *sql.Tx) error {
				if e := PutItemTx(ctx, tx, item, 0); e != nil {
					return e
				}
				if e := saveControlReceipt(ctx, tx, request, rtHash(item), item.ID); e != nil {
					return e
				}
				if point == "after_receipt_before_commit" {
					return errors.New("INJECTED_BEFORE_COMMIT")
				}
				return nil
			})
			if e == nil {
				t.Fatal("injection did not fire")
			}
			for _, table := range []string{"item", "change_event", "request_receipt"} {
				var n int
				if e = s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != 0 {
					t.Fatal("partial commit", point, table, n, e)
				}
			}
			if table != "" {
				if _, e = s.DB.Exec("DROP TRIGGER fail_boundary"); e != nil {
					t.Fatal(e)
				}
			}
			if e = s.Write(ctx, func(tx *sql.Tx) error {
				if e := PutItemTx(ctx, tx, item, 0); e != nil {
					return e
				}
				return saveControlReceipt(ctx, tx, request, rtHash(item), item.ID)
			}); e != nil {
				t.Fatal("retry", e)
			}
			for _, table := range []string{"item", "change_event", "request_receipt"} {
				var n int
				s.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
				if n != 1 {
					t.Fatal("retry count", table, n)
				}
			}
		})
	}
}
