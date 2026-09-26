# catalog — Agent Guide

Scaffolded from `services/_template` by `make new-service name=catalog`
(D-03) — see `services/_template/CLAUDE.md` for the shared template rules
and layout this service still follows.

## Owns

- `boats` (Phase 1). Operators, piers, routes, and prices arrive in Phase 2.

## Publishes

- `catalog.BoatUpserted` on the `catalog.events` topic, keyed by `boat_id` —
  emitted whenever `CatalogService.UpsertBoat` creates or updates a boat
  (D-01).

## Consumes

- Nothing yet.

## Sync API

- `CatalogService.UpsertBoat` — requires verified claims; `operator_id`
  always comes from the claims, never the request body (D-30). An empty
  `boat_id` creates a new boat; a non-empty `boat_id` updates one, but only
  when it belongs to the calling operator (cross-operator reuse of an
  existing `boat_id` returns `NotFound`, and the stored row is left
  unchanged).
- `CatalogService.ListBoats` — needs only the internal token, no claims.
  Returns every boat ordered by `name`, then `id`.

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

- `cmd/` — the one binary: HTTP (chi + the CatalogService connect handler),
  outbox relay, and Kafka consumer running as `errgroup` goroutines with
  ordered shutdown.
- `internal/domain/boat.go` — `Boat`, `Status`, and `Validate()` — types and
  validation rules only, no persistence or transport concerns.
- `internal/app/boat.go` — `UpsertBoat`/`ListBoats` use-case functions taking
  `pgx.Tx`/`*postgres.Queries` directly. No repository interfaces, no mocks.
- `internal/adapters/http/routes.go` — the `CatalogService` connect handler,
  mounted behind the internal-token trust boundary set up in `cmd/main.go`.
- `internal/adapters/postgres/` — sqlc-generated code from
  `queries/boats.sql`.
- `internal/adapters/kafka/handlers.go` — `Register` (empty; catalog
  consumes nothing yet).
- `migrations/` — `00001_platform.sql` (outbox + processed_events, copied
  verbatim from the template) and `00002_boats.sql` (the `boats` table).

## Commands

- `make run-catalog` — run this service locally against the infra stack
  (`go run ./cmd`, DB/Kafka hosts overridden to localhost).
- `make migrate-catalog` — run this service's goose migrations.
- `make sqlc-gen` — regenerate every service's `internal/adapters/postgres/*.go`
  from `queries/*.sql`.
- `make test-integration` — run every module's `//go:build integration`
  tests against real Postgres + Redpanda testcontainers.

## Commit Scope

Commits touching only this service use `catalog` as the conventional-commit
scope (D-49), e.g. `feat(catalog): add boat search filter`.
