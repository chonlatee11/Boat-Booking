# catalog — Agent Guide

Scaffolded from `services/_template` by `make new-service name=catalog`
(D-03) — see `services/_template/CLAUDE.md` for the shared template rules
and layout this service still follows.

## Owns

- `boats` (Phase 1). `operators`, `piers`, `routes`, and `route_prices`
  (Phase 2).

## Publishes

- `catalog.BoatUpserted` on the `catalog.events` topic, keyed by `boat_id` —
  emitted whenever `CatalogService.UpsertBoat`/`ArchiveBoat` creates,
  updates, or archives a boat (D-01). `home_pier_id` and `archived` are
  additive fields (D-07) — schedule's existing consumer ignores them and
  keeps applying the event unchanged.
- `catalog.OperatorUpserted`, keyed by `operator_id` — emitted whenever
  `UpsertOperator`/`ArchiveOperator` creates, renames, or archives an
  operator.
- `catalog.PierUpserted`, keyed by `pier_id` — emitted whenever
  `UpsertPier`/`ArchivePier` creates, edits, or archives a pier.
  Deliberately carries no `address` field — a physical address is
  PII-shaped and `proto/pii-check.sh` rejects it (Pitfall 5, D-45); the
  public `ListPiers` sync call serves address instead.
- `catalog.RouteUpserted`, keyed by `route_id` — emitted whenever
  `UpsertRoute`/`ArchiveRoute` creates, edits, or archives a one-way route
  (D-11).
- `catalog.PriceChanged`, keyed by `route_id` — emitted whenever
  `AddRoutePrice` adds or replaces an effective-dated ticket price (D-14).

## Consumes

- Nothing yet.

## Sync API

- `CatalogService.UpsertBoat` — requires verified claims and
  `scope.CanWrite()` (`pier_admin`/`super_admin`; `staff`/`customer` get
  `PermissionDenied`). Every boat is scoped to a `home_pier_id` (D-07):
  the home pier must be in the caller's scope (same
  `GetPierForShareScoped` rule as pier writes) and non-archived —
  missing/out-of-scope is `NotFound`, archived is `FailedPrecondition`, no
  `home_pier_id` at all is `InvalidArgument`. `operator_id` is always
  derived from the home pier's stored operator, never the request body
  (D-30). An empty `boat_id` creates a new boat; a non-empty `boat_id`
  updates one, but only when its home pier is in the caller's scope
  (cross-scope reuse of an existing `boat_id` returns `NotFound`, and the
  stored row is left unchanged); editing an already-archived boat is
  `FailedPrecondition`.
- `CatalogService.ListBoats` — no claims returns the public projection
  (every non-archived boat, needs only the internal token); with claims the
  same `Scope` rule as `ListPiers` applies (AUTH-05, CAT-04) — a
  `pier_admin` with no assigned piers gets an empty list, `super_admin` sees
  every boat. Ordered by `name`, then `id`.
- `CatalogService.ArchiveBoat` — requires verified claims and
  `scope.CanWrite()`; same home-pier scope rule as `UpsertBoat`. Idempotent:
  archiving an already-archived boat is a successful no-op that publishes
  no new event.
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
  is ignored (D-30). `photo_key` must be either empty or exactly the shape
  `piers/<uuidv7>.(jpg|png|webp)` (D-19) — anything else is
  `InvalidArgument`; a client can only ever supply a key it previously got
  back from `PresignPierPhoto`, never an arbitrary path.
- `CatalogService.ListPiers` — **no claims at all is the public projection**
  (CAT-06): every non-archived pier, ids/names/coordinates/address/hours/
  `photo_url`, needs only the internal token. With claims, the same
  `app.Scope` rule as `UpsertPier` applies; the request's `operator_id`
  filter is honoured only for `super_admin`. Every returned `Pier` carries
  `photo_key` (stored key or `""`) and `photo_url` (`PHOTO_PUBLIC_BASE_URL`
  `+ "/" + photo_key`, or `""` when no photo/no base URL configured, D-19).
- `CatalogService.PresignPierPhoto` — requires verified claims and
  `scope.CanWrite()` (`pier_admin`/`super_admin`); `staff`/`customer` get
  `PermissionDenied`. Validates `content_type` against the
  `image/jpeg|image/png|image/webp` allow-list and `size_bytes` against
  `1..5,242,880` (5 MB) before minting anything (`InvalidArgument`
  otherwise); returns `FailedPrecondition` when no object storage is
  configured (`S3_PUBLIC_ENDPOINT` unset) so catalog still starts without
  it. The object key is always server-generated
  (`piers/<uuidv7>.<jpg|png|webp>`) — the browser never chooses it
  (T-02-08-03) — and the returned PUT URL is valid for 10 minutes with its
  signature covering `Content-Type` and `Content-Length`, so a mismatched
  upload is rejected by storage itself (T-02-08-02). **catalog never
  proxies file bytes** — the browser PUTs directly to object storage with
  this URL; catalog only issues it and later stores the resulting key via
  `UpsertPier`.
- The one scoping rule every operator/pier/route RPC shares lives in
  `internal/app/scope.go` (`Scope.All`/`CanWrite`/`PierIDArray`) and
  `internal/adapters/http/scope.go` (`scopeFrom`/`toConnectErr`) — new
  entities (boats' pier scoping) reuse these, they are not reimplemented
  per entity.
- `CatalogService.UpsertRoute` — requires verified claims. Routes are
  one-way (D-11): `pier_from_id` must be in the caller's scope (via
  `GetPierForShareScoped`, same rule as pier writes); `pier_to_id` may be
  any non-archived pier of any operator (D-12). `route.operator_id` is
  always derived from `pier_from_id`'s stored operator, never the request
  (D-30). An empty `cancellation_policy` on create uses the default
  `[{24,100},{2,50},{0,0}]` schedule (D-13); a second active route for the
  same `(pier_from_id, pier_to_id)` pair returns `AlreadyExists`.
- `CatalogService.ListRoutes` — no claims returns the public projection
  (non-archived routes between non-archived piers, CAT-06); with claims the
  same `Scope` rule applies. Every route carries `current_prices` — each
  ticket type's price in effect for today's Asia/Bangkok date (D-14), empty
  (never zero) when no price has been set.
- `CatalogService.ArchiveRoute`/`ArchivePier` — require verified claims and
  the same scope rule as the entity's write RPC. `ArchivePier` is rejected
  with `FailedPrecondition` while any non-archived route uses the pier as
  `pier_from` or `pier_to`; the message names only routes in the caller's
  scope and counts the rest as `"+N routes of other operators"` — never
  another operator's names or ids (T-02-06-03). Archiving an
  already-archived route/pier is a successful no-op (D-15).
- `CatalogService.AddRoutePrice`/`ListRoutePrices` — require verified
  claims; the route must be in the caller's scope (`AddRoutePrice` also
  requires `CanWrite`; `ListRoutePrices` allows `staff` to read).
  `effective_from` may never be before today (Asia/Bangkok, D-14); the
  price in effect on date D is the row with the latest
  `effective_from <= D` (inclusive); re-adding the same
  `(route, ticket_type, effective_from)` replaces the amount in one row.

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
- `internal/domain/{boat,operator,pier,route,price}.go` — types and
  `Validate()` rules only, no persistence or transport concerns.
  `route.go` also holds `CancellationTier`/`DefaultCancellationPolicy`/
  `ValidateCancellationPolicy` (D-13). `pier.go`'s `photoKeyPattern` is the
  one regexp `photo_key` must match (D-19).
- `internal/app/{boat,operator,pier,route,price}.go` —
  `Upsert*`/`List*`/`Archive*` use-case functions taking
  `pgx.Tx`/`*postgres.Queries` directly. No repository interfaces, no
  mocks. `route.go`'s `ListRoutes` calls `price.go`'s `attachCurrentPrices`
  to populate `CurrentPrices` (D-14). `boat.go`'s `UpsertBoat`/`ListBoats`/
  `ArchiveBoat` apply the home-pier scope rule (D-07).
- `internal/app/photo.go` — `Photos{Client, Bucket, PublicBaseURL}`
  (`PresignPierPhoto`, `URL`, D-19); a nil `Client` (no
  `S3_PUBLIC_ENDPOINT` configured) makes `PresignPierPhoto` return
  `FailedPrecondition` instead of panicking.
- `internal/app/scope.go` — `Scope{Role, OperatorID, PierIDs}`, the one
  operator/pier/route scoping rule (`All`/`CanWrite`/`PierIDArray`) every
  entity's use-case functions apply.
- `internal/adapters/http/routes.go` — **not** the Route RPC handlers; this
  is the `CatalogService` connect handler struct registration file,
  mounted behind the internal-token trust boundary set up in `cmd/main.go`.
  `operators.go`/`piers.go`/`route_handlers.go`/`prices.go` hold the
  per-entity RPC methods on the same `*server` type (Route handlers live in
  `route_handlers.go` to avoid a name clash with this file).
- `internal/adapters/http/scope.go` — `scopeFrom` (claims → `app.Scope`,
  the one place role checking happens before any handler runs) and
  `toConnectErr` (the one domain-sentinel → Connect-code switch, including
  `ErrAlreadyExists` → `CodeAlreadyExists`).
- `internal/adapters/postgres/` — sqlc-generated code from
  `queries/{boats,operators,piers,routes,route_prices}.sql`.
- `internal/adapters/kafka/handlers.go` — `Register` (empty; catalog
  consumes nothing yet).
- `migrations/` — `00001_platform.sql` (outbox + processed_events, copied
  verbatim from the template), `00002_boats.sql`, `00003_operators.sql`,
  `00004_piers.sql`, `00005_routes.sql`, `00006_route_prices.sql`,
  `00007_boats_home_pier.sql` (D-07), `00008_pier_photo.sql` (D-19).

## Environment (D-19)

Object storage for pier photos is optional — catalog starts without it and
`PresignPierPhoto` returns `FailedPrecondition` until it's configured:

- `S3_PUBLIC_ENDPOINT` — host:port the **browser** uses (e.g.
  `localhost:8333`); presigning is a local computation with a fixed
  `S3_REGION`, so catalog itself never needs to reach storage.
- `S3_REGION` (default `us-east-1`), `S3_BUCKET` (default `pier-photos`),
  `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_USE_SSL` (default `false`) — never
  logged.
- `PHOTO_PUBLIC_BASE_URL` (e.g. `http://localhost:8333/pier-photos`) — used
  to build `Pier.photo_url`; empty when unset, even if `photo_key` is set.

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
