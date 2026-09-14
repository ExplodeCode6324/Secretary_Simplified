package tests

import (
	"secretarysimplified/contract"
	"secretarysimplified/scheduler"
	"testing"
	"time"
)

// Expected instants are hand-authored UTC oracles, never produced by Next.
func TestCalendarFixedUTCOracles(t *testing.T) {
	cases := []struct {
		name, kind, zone, local, after, want string
		week                                 []int
	}{
		{"HK next day", "daily", "Asia/Hong_Kong", "08:00", "2026-09-14T00:00:00Z", "2026-09-15T00:00:00Z", nil},
		{"HK before minute", "daily", "Asia/Hong_Kong", "08:00", "2026-09-13T23:59:59Z", "2026-09-14T00:00:00Z", nil},
		{"Monday Wednesday", "weekly", "Asia/Hong_Kong", "09:30", "2026-09-14T01:30:00Z", "2026-09-16T01:30:00Z", []int{1, 3}},
		{"Sunday ISO seven", "weekly", "UTC", "12:00", "2026-09-14T00:00:00Z", "2026-09-20T12:00:00Z", []int{7}},
		{"NY weekly gap", "weekly", "America/New_York", "02:30", "2026-03-08T00:00:00Z", "2026-03-15T06:30:00Z", []int{7}},
		{"NY fold first only", "weekly", "America/New_York", "01:30", "2026-11-01T05:30:00Z", "2026-11-08T06:30:00Z", []int{7}},
		{"Kathmandu quarter offset", "daily", "Asia/Kathmandu", "06:00", "2026-09-13T23:00:00Z", "2026-09-14T00:15:00Z", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			after, _ := time.Parse(time.RFC3339, c.after)
			s := contract.Schedule{Kind: c.kind, Timezone: c.zone, LocalTime: &c.local, Weekdays: c.week}
			got, e := scheduler.Next(s, after)
			if e != nil || got == nil || got.Format(time.RFC3339) != c.want {
				t.Fatalf("got %v err %v; want %s", got, e, c.want)
			}
		})
	}
	at := "2026-09-14T00:00:00Z"
	instant, _ := time.Parse(time.RFC3339, at)
	once := contract.Schedule{Kind: "once", At: &at, Timezone: "UTC", Weekdays: []int{}}
	if got, e := scheduler.Next(once, instant); e != nil || got != nil {
		t.Fatalf("once repeated %v %v", got, e)
	}
	interval := 60
	s := contract.Schedule{Kind: "interval", AnchorAt: &at, EverySeconds: &interval, Timezone: "UTC", Weekdays: []int{}}
	got, e := scheduler.Next(s, instant.Add(122*time.Second))
	if e != nil || got == nil || got.Format(time.RFC3339) != "2026-09-14T00:03:00Z" {
		t.Fatalf("interval lost anchor %v %v", got, e)
	}
}
