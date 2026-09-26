// Package httpadapter wires services/schedule's HTTP routes. schedule
// registers no routes in Phase 1 — its sync API (departure search/holds)
// arrives in Phase 3; today it only consumes catalog.BoatUpserted.
package httpadapter

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Routes mounts this service's protected routes onto r. pool is the
// service's Postgres pool; nudge wakes the outbox relay immediately after a
// write instead of waiting for its next poll tick (a no-op when the relay is
// disabled). Neither is used yet — kept for signature parity with every
// other service scaffolded from the template.
func Routes(_ chi.Router, _ *pgxpool.Pool, _ func()) {}
