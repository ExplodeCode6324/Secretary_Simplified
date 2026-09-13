package scheduler

import (
	"context"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"time"
)

type Scheduler struct{ Store *store.Store }

func New(s *store.Store) *Scheduler { return &Scheduler{s} }
func (s *Scheduler) Step(ctx context.Context, now time.Time) (int, error) {
	return s.Store.ScheduleStep(ctx, now)
}
func Next(schedule contract.Schedule, after time.Time) (*time.Time, error) {
	return store.NextOccurrence(schedule, after)
}
