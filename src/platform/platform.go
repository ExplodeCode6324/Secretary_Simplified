package platform

import (
	"os"
	"sync"
	"syscall"
	"time"
)

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type ManualClock struct {
	mu sync.Mutex
	T  time.Time
}

func (c *ManualClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.T.UTC() }
func (c *ManualClock) Advance(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.T = c.T.Add(d) }

type Lock struct{ f *os.File }

func AcquireLock(path string) (*Lock, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, e
	}
	return &Lock{f}, nil
}
func (l *Lock) Close() error { syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN); return l.f.Close() }
