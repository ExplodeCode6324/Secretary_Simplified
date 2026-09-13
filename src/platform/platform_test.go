package platform

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLockAndClock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	a, e := AcquireLock(path)
	if e != nil {
		t.Fatal(e)
	}
	if b, e := AcquireLock(path); e == nil {
		b.Close()
		t.Fatal("second holder admitted")
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	b, e := AcquireLock(path)
	if e != nil {
		t.Fatal(e)
	}
	b.Close()
	c := ManualClock{T: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}
	c.Advance(25 * time.Hour)
	if c.Now().Day() != 15 || c.Now().Hour() != 1 {
		t.Fatal(c.Now())
	}
}
