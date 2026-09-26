// Package clock is the single source of wall-clock time and Bangkok local-date
// bucketing for the whole codebase.
//
// The database always stores UTC. But a ferry departure's identity — which
// calendar day it belongs to for search, schedule generation, and reports —
// is a local-date concept, not an instant-in-time concept (PITFALLS.md #6).
// Any code answering "which calendar day is this" must convert to
// Asia/Bangkok first via LocalDate, never truncate a raw UTC timestamp.
//
// App code must call clock.Now() instead of time.Now() directly; this is
// lint-enforced everywhere except this package (see .golangci.yml forbidigo).
package clock

import (
	"time"

	// Embeds the IANA tzdata so time.LoadLocation("Asia/Bangkok") resolves
	// inside distroless images, which ship no system zoneinfo files.
	_ "time/tzdata"
)

// Bangkok is the Asia/Bangkok location (UTC+7, no DST).
var Bangkok = mustLoadLocation("Asia/Bangkok")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// tzdata is embedded at compile time, so a load failure here means the
		// binary itself is broken, not a runtime environment gap — panic at
		// package init rather than surface it lazily to a caller.
		panic(err)
	}
	return loc
}

// Now is the single overridable wall-clock source. Tests replace it with a
// fixed function and restore the original afterward, typically via
// t.Cleanup.
var Now = time.Now

// LocalDate returns the Asia/Bangkok calendar date of t, encoded as midnight
// UTC of that date (the pgx `date` column convention). Storage stays UTC;
// this is the one place a UTC instant is bucketed into a local calendar day.
func LocalDate(t time.Time) time.Time {
	y, m, d := t.In(Bangkok).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
