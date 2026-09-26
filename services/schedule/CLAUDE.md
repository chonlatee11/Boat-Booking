# schedule — Agent Guide

Scaffolded from `services/_template` by `make new-service name=schedule`
(D-03) — see `services/_template/CLAUDE.md` for the shared template rules
and layout this service still follows.

## Owns

- `boats` (Phase 1) — its own projection of catalog boats (default
  capacity, status), kept current by the `catalog.BoatUpserted` consumer.
  schedule never reads catalog's database (database-per-service).
  Departures/holds (Phase 3) arrive later.

## Publishes

- Nothing yet. `schedule.Departure*` events arrive in Phase 3.

## Consumes

- `catalog.BoatUpserted` (from `catalog.events`) — applied exactly once via
  `kafka.Consumer.Handle`'s built-in idempotency check
  (`processed_events`); a later event for the same `boat_id` overwrites the
  projection so it always holds the latest capacity/status (D-01, PLAT-05).

## Sync API

- Nothing yet — schedule's sync API (departure search/holds) arrives in
  Phase 3.

## Rules

- Never read another service's database — every service owns its own
  Postgres database (database-per-service, enforced at the DB level).
- Publish only via `outbox.Insert` in the same `pgx.Tx` as the state change it
  describes — app/adapter code never calls `pkg/kafka.Producer` directly.
- Consume only via `kafka.Consumer.Handle` — the idempotency check
  (`processed_events`) is built into the wrapper; there is no way to bypass
  it from service code.
- Use `pkg/clock.Now()`, never `time.Now()` directly (lint-enforced), and
  `pkg/money.Satang` for any money field, never a float.
- Trust claims only via `httpx.FromContext(ctx)` — never trust a
  client-supplied operator/user id. Every operator-scoped query must filter
  by `operator_id` taken from claims.
- No personal data (PII) in events, logs, or span attributes — ids and
  non-personal business fields only (proto review checklist, D-45).

## Layout

- `cmd/` — the one binary: HTTP (chi), outbox relay (unused today — no
  events published yet), and the Kafka consumer running as `errgroup`
  goroutines with ordered shutdown.
- `internal/domain/boat.go` — `Boat`, `Status` — types only, no persistence
  or transport concerns.
- `internal/app/boat.go` — `EventCatalogBoatUpserted`, `ApplyBoatUpserted`
  use-case function taking `pgx.Tx` directly. No repository interfaces, no
  mocks.
- `internal/adapters/http/routes.go` — registers no routes yet.
- `internal/adapters/postgres/` — sqlc-generated code from
  `queries/boats.sql`.
- `internal/adapters/kafka/handlers.go` — `Register` wiring
  `catalog.BoatUpserted` to `app.ApplyBoatUpserted`.
- `migrations/` — `00001_platform.sql` (outbox + processed_events, copied
  verbatim from the template) and `00002_boats.sql` (the `boats` projection
  table).

## Commands

- `make run-schedule` — run this service locally against the infra stack
  (`go run ./cmd`, DB/Kafka hosts overridden to localhost).
- `make migrate-schedule` — run this service's goose migrations.
- `make sqlc-gen` — regenerate every service's `internal/adapters/postgres/*.go`
  from `queries/*.sql`.
- `make test-integration` — run every module's `//go:build integration`
  tests against real Postgres + Redpanda testcontainers.

## Commit Scope

Commits touching only this service use `schedule` as the conventional-commit
scope (D-49), e.g. `feat(schedule): add departure search`.
