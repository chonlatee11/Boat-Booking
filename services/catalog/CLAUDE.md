# catalog — Agent Guide

Scaffolded from `services/_template` by `make new-service name=catalog`
(D-03) — see `services/_template/CLAUDE.md` for the shared template rules
and layout this service still follows.

## Owns

- `boats` (Phase 1). `operators` and `piers` (Phase 2, this plan). Routes
  and prices arrive in a later Phase 2 plan.

## Publishes

- `catalog.BoatUpserted` on the `catalog.events` topic, keyed by `boat_id` —
  emitted whenever `CatalogService.UpsertBoat` creates or updates a boat
  (D-01).
- `catalog.OperatorUpserted`, keyed by `operator_id` — emitted whenever
  `UpsertOperator`/`ArchiveOperator` creates, renames, or archives an
  operator.
- `catalog.PierUpserted`, keyed by `pier_id` — emitted whenever `UpsertPier`
  creates or edits a pier. Deliberately carries no `address` field — a
  physical address is PII-shaped and `proto/pii-check.sh` rejects it
  (Pitfall 5, D-45); the public `ListPiers` sync call serves address
  instead.

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
- `CatalogService.UpsertOperator`/`ListOperators`/`ArchiveOperator` —
  require verified claims; only `super_admin` may create, rename, or
  archive an operator (D-08) — every other role gets `PermissionDenied`, no
  claims gets `Unauthenticated`. `ArchiveOperator` is rejected with
  `FailedPrecondition` while the operator still owns a non-archived pier
  (D-15, no cascade); archiving an already-archived operator is a no-op.
- `CatalogService.UpsertPier` — requires verified claims. Create (empty
  `pier_id`) is `super_admin` only, targeting any non-archived operator
  named in the request. Update (non-empty `pier_id`) requires
  `super_admin` or `pier_admin` (`staff` is read-only); the pier must
  already be in the caller's scope — `super_admin` sees everything,
  `pier_admin`/`staff` only piers in `(operator_id, pier_ids)` from their
  claims (AUTH-05) — an out-of-scope or missing `pier_id` returns
  `NotFound`, never `PermissionDenied`, never revealing the row. The stored
  `operator_id` is always kept on update; the request body's `operator_id`
  is ignored (D-30).
- `CatalogService.ListPiers` — **no claims at all is the public projection**
  (CAT-06): every non-archived pier, ids/names/coordinates/address/hours,
  needs only the internal token. With claims, the same `app.Scope` rule as
  `UpsertPier` applies; the request's `operator_id` filter is honoured only
  for `super_admin`.
- The one scoping rule every operator/pier RPC shares lives in
  `internal/app/scope.go` (`Scope.All`/`CanWrite`/`PierIDArray`) and
  `internal/adapters/http/scope.go` (`scopeFrom`/`toConnectErr`) — new
  entities (routes, boats' pier scoping) reuse these, they are not
  reimplemented per entity.

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
- `internal/domain/{boat,operator,pier}.go` — types and `Validate()` rules
  only, no persistence or transport concerns.
- `internal/app/{boat,operator,pier}.go` — `Upsert*`/`List*`/`ArchiveOperator`
  use-case functions taking `pgx.Tx`/`*postgres.Queries` directly. No
  repository interfaces, no mocks.
- `internal/app/scope.go` — `Scope{Role, OperatorID, PierIDs}`, the one
  operator/pier scoping rule (`All`/`CanWrite`/`PierIDArray`) every entity's
  use-case functions apply.
- `internal/adapters/http/routes.go` — the `CatalogService` connect handler
  struct, mounted behind the internal-token trust boundary set up in
  `cmd/main.go`. `operators.go`/`piers.go` hold the per-entity RPC methods
  on the same `*server` type.
- `internal/adapters/http/scope.go` — `scopeFrom` (claims → `app.Scope`,
  the one place role checking happens before any handler runs) and
  `toConnectErr` (the one domain-sentinel → Connect-code switch).
- `internal/adapters/postgres/` — sqlc-generated code from
  `queries/{boats,operators,piers}.sql`.
- `internal/adapters/kafka/handlers.go` — `Register` (empty; catalog
  consumes nothing yet).
- `migrations/` — `00001_platform.sql` (outbox + processed_events, copied
  verbatim from the template), `00002_boats.sql`, `00003_operators.sql`,
  `00004_piers.sql`.

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
