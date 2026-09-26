# Service Template — Agent Guide

Every real service starts life as `cp -r services/_template services/<name>` +
`sed` replacing `__NAME__` (`make new-service name=<name>`, D-03). This file is
copied verbatim into the new service directory — edit the copy in place once
the sample slice below is replaced with real business logic.

## Owns

_(fill in: this service's own Postgres tables — database-per-service, no
service reads another service's database)_

## Publishes

_(fill in: `<service>.<EventName>` events on the `<service>.events` topic, or
"Nothing yet")_

## Consumes

_(fill in: event types this service's Kafka consumer handles via
`kafka.Consumer.Handle`, or "Nothing yet")_

## Sync API

_(fill in: connect-go RPCs this service exposes, and whether each requires
only the internal token or verified claims too)_

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

- `cmd/` — the one binary: HTTP (chi + connect handlers), outbox relay, and
  Kafka consumer running as `errgroup` goroutines with ordered shutdown.
- `internal/domain/` — types and validation rules only, no persistence or
  transport concerns.
- `internal/app/` — use-case functions taking `pgx.Tx` directly. No
  repository interfaces, no mocks; tested against real Postgres under the
  `integration` build tag.
- `internal/adapters/{postgres,http,kafka}/` — sqlc-generated Postgres code,
  HTTP/connect route wiring, and Kafka consumer handler registration.
- `migrations/` — goose SQL migrations. `00001_platform.sql` (outbox +
  processed_events) is copied verbatim from the template and never edited.

## Replace the Sample

The scaffolded service starts with a `pings` sample slice proving the full
HTTP -> tx -> outbox -> Kafka -> consumer loop end to end. Delete it before
adding real business logic:

1. `rm internal/domain/ping.go internal/app/ping.go internal/adapters/postgres/queries/pings.sql internal/adapters/postgres/pings.sql.go migrations/00002_pings.sql`
2. Remove the `POST /v1/pings` route from `internal/adapters/http/routes.go`.
3. Remove the `PingRecorded` handler registration from
   `internal/adapters/kafka/handlers.go`.
4. Remove `TestPingRoundTrip` from `cmd/main_integration_test.go` — keep
   `TestTemplateReadyAndGracefulShutdown` (renamed by `sed` to this service).
5. Run `make sqlc-gen` to drop the generated `pings` code from
   `internal/adapters/postgres/{db,models}.go`.

## Commands

- `make run-<svc>` — run this service locally against the infra stack
  (`go run ./cmd`, DB/Kafka hosts overridden to localhost).
- `make migrate-<svc>` — run this service's goose migrations.
- `make sqlc-gen` — regenerate every service's `internal/adapters/postgres/*.go`
  from `queries/*.sql`.
- `make test-integration` — run every module's `//go:build integration`
  tests against real Postgres + Redpanda testcontainers.

## Commit Scope

Commits touching only this service use `<service>` as the conventional-commit
scope (D-49), e.g. `feat(catalog): add boat search filter`.
