// Package httpadapter wires services/_template's HTTP routes. The sample
// ping slice (added in Task 2) is the template's executable example: it
// exercises the full domain -> app -> postgres -> outbox loop over one real
// endpoint. Real services delete this route and add their own — plan 10's
// CLAUDE.md lists the exact files to remove.
package httpadapter

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Routes mounts this service's protected routes onto r. pool is the
// service's Postgres pool; nudge wakes the outbox relay immediately after a
// write instead of waiting for its next poll tick (a no-op when the relay is
// disabled). The sample /v1/pings route is added in Task 2.
func Routes(r chi.Router, pool *pgxpool.Pool, nudge func()) {
}
