---
phase: 02-identity-catalog
plan: 06
subsystem: catalog
tags: [connect-go, sqlc, postgres, outbox, scoping, cancellation-policy, effective-dated-pricing, auth-05, buf]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-03)
    provides: app.Scope{Role, OperatorID, PierIDs} (All/CanWrite/PierIDArray), http.scopeFrom/toConnectErr, operators+piers tables, GetPierForUpdateScoped
provides:
  - CatalogService.UpsertRoute/ListRoutes/ArchiveRoute/ArchivePier/AddRoutePrice/ListRoutePrices
  - routes + route_prices tables; domain.Route/CancellationTier/RoutePrice/TicketType and their Validate() rules
  - catalog.RouteUpserted / catalog.PriceChanged outbox events; TicketType enum
  - The D-14 price-lookup contract (ListCurrentPrices: latest effective_from <= on_date, distinct on route_id/ticket_type) that Phase 4's booking snapshot reuses
  - CAT-06 public route listing (pier ids, duration, current adult/child prices for today's Asia/Bangkok date)
affects: [02-11, 04]

# Actuals (#2632)
actuals:
  tokens: 52262
  tasks: 3
  commits: 3
plan_head_before: 94b7e8ef78440df8d3233300a764b2dce1683eed

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "route_from scope reuses the same GetPierForShareScoped/GetPierForUpdateScoped shape as pier writes (02-03's Scope rule) — pier_to is deliberately unscoped (GetPierForShare) since D-12 lets it belong to any operator"
    - "route.operator_id and price validation's 'today' are both re-derived server-side on every write (pier_from's stored operator_id; clock.LocalDate(clock.Now())) — never trusted from the request or a stale client clock"
    - "blocked-archive error messages build a caller-visible list from in-scope rows and a bare count for everything else — the same disclosure shape as the scope-check's NotFound-never-PermissionDenied rule, applied to error message content, not just HTTP codes"
    - "FOR UPDATE (ArchivePier) vs FOR SHARE (UpsertRoute's pier_from/pier_to reads) on the same piers rows is the sole concurrency control for the archive-vs-create race (T-02-06-07) — no application-level distributed lock"

key-files:
  created:
    - proto/events/catalog/v1/route.proto
    - proto/events/catalog/v1/price.proto
    - services/catalog/migrations/00005_routes.sql
    - services/catalog/migrations/00006_route_prices.sql
    - services/catalog/internal/adapters/postgres/queries/routes.sql
    - services/catalog/internal/adapters/postgres/queries/route_prices.sql
    - services/catalog/internal/domain/route.go
    - services/catalog/internal/domain/route_test.go
    - services/catalog/internal/domain/price.go
    - services/catalog/internal/domain/price_test.go
    - services/catalog/internal/app/route.go
    - services/catalog/internal/app/price.go
    - services/catalog/internal/adapters/http/route_handlers.go
    - services/catalog/internal/adapters/http/prices.go
    - services/catalog/cmd/routes_prices_integration_test.go
  modified:
    - proto/services/catalog/v1/catalog.proto
    - services/catalog/internal/adapters/postgres/models.go
    - services/catalog/internal/adapters/postgres/queries/piers.sql
    - services/catalog/internal/domain/errors.go
    - services/catalog/internal/adapters/http/scope.go
    - services/catalog/internal/adapters/http/piers.go
    - services/catalog/internal/app/pier.go
    - services/catalog/CLAUDE.md

key-decisions:
  - "All Task 1-3 proto (route.proto, price.proto, and catalog.proto's full RPC/message surface for UpsertRoute/ListRoutes/ArchiveRoute/ArchivePier/AddRoutePrice/ListRoutePrices) was added and regenerated in a single buf generate pass during Task 1, rather than incrementally per task — proto files are shared/coupled (price.proto's TicketType is imported by catalog.proto's Route.current_prices field added in Task 2), and a single generation pass keeps gen/ internally consistent. UnimplementedCatalogServiceHandler covered the not-yet-implemented RPCs between tasks; each task's own commit still adds only that task's Go implementation."
  - "AddRoutePrice/ListRoutePrices/ArchiveRoute all reuse GetRouteForUpdateScoped (FOR UPDATE) for their scope check rather than adding read-only variants — simpler (one query, one lock semantic) and the extra row lock is uncontended in practice for these low-frequency admin writes/reads."
  - "TestRoutePricesEffectiveDating exercises AddRoutePrice's archived-route rejection via a direct SQL UPDATE (archiveRouteDirectly helper) rather than the ArchiveRoute RPC, since ArchiveRoute is Task 3's deliverable and Task 2's test needed to stay self-contained without forward-depending on unfinished work."

patterns-established:
  - "Domain Validate(today time.Time) taking the current date as a parameter (not reading clock.Now() itself) — keeps domain pure/testable while app.AddRoutePrice supplies clock.LocalDate(clock.Now()) at the call site."

requirements-completed: [CAT-03, CAT-05, CAT-06, CAT-02, AUTH-05]

coverage:
  - id: D1
    description: "Routes end-to-end: pier_admin creates/edits a one-way route only when pier_from is in their (operator_id, pier_ids) scope; pier_to may be any non-archived pier of any operator; route.operator_id always derived from pier_from; a second active route for the same pair is AlreadyExists; update-by-route_id updates the same row"
    requirement: CAT-03
    verification:
      - kind: integration
        ref: "services/catalog/cmd/routes_prices_integration_test.go#TestRoutesScopingAndSharedPierTo"
        status: pass
      - kind: unit
        ref: "services/catalog/internal/domain/route_test.go#TestRouteValidate"
        status: pass
      - kind: unit
        ref: "services/catalog/internal/domain/route_test.go#TestValidateCancellationPolicy"
        status: pass
    human_judgment: false
  - id: D2
    description: "Effective-dated prices (D-14): AddRoutePrice validates amount/ticket_type/not-before-today, publishes catalog.PriceChanged, and re-adding the same (route, ticket_type, effective_from) replaces the amount in one row; ListRoutePrices orders effective_from desc then ticket_type"
    requirement: CAT-05
    verification:
      - kind: integration
        ref: "services/catalog/cmd/routes_prices_integration_test.go#TestRoutePricesEffectiveDating"
        status: pass
      - kind: unit
        ref: "services/catalog/internal/domain/price_test.go#TestRoutePriceValidate"
        status: pass
    human_judgment: false
  - id: D3
    description: "Public (no-claims) ListRoutes serves CAT-06: non-archived routes between non-archived piers, each with current_prices reflecting the price in effect for today's Asia/Bangkok date (inclusive effective_from boundary), absent (never zero) when no price is set"
    requirement: CAT-06
    verification:
      - kind: integration
        ref: "services/catalog/cmd/routes_prices_integration_test.go#TestRoutePricesEffectiveDating"
        status: pass
      - kind: integration
        ref: "services/catalog/cmd/routes_prices_integration_test.go#TestRoutesScopingAndSharedPierTo"
        status: pass
    human_judgment: false
  - id: D4
    description: "Archive rules (D-15): ArchivePier is rejected with FailedPrecondition while any non-archived route uses it as pier_from/pier_to, naming only in-scope routes and counting the rest as other operators; ArchiveRoute/ArchivePier are idempotent; archived piers/routes disappear from public lists and reject further writes"
    requirement: CAT-02
    verification:
      - kind: integration
        ref: "services/catalog/cmd/routes_prices_integration_test.go#TestArchivePierBlockedByRoutes"
        status: pass
    human_judgment: false

# Metrics
duration: 33min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 6: Catalog Routes, Effective-Dated Prices, and Archive Rules Summary

**CatalogService gains one-way routes with tiered cancellation policies, effective-dated adult/child prices with a D-14 latest-effective-from lookup, and archive rules for both routes and piers that reject in-use archives without ever naming another operator's data.**

## Performance

- **Duration:** 33 min
- **Started:** 2026-09-26T18:16:00Z
- **Completed:** 2026-09-26T18:49:00Z
- **Tasks:** 3 (1 tracer, 2 auto/tdd — `workflow.tdd_mode` is disabled for this project, so tests and implementation for each task were written and verified together in one pass, matching the process note already established in 02-03-SUMMARY.md)
- **Files modified:** 23 (15 created, 8 modified in `services/catalog`/`proto`; generated `gen/**` output tracked separately from the count above)

## Accomplishments

- `CatalogService.UpsertRoute`/`ListRoutes`: pier_from is loaded through the caller's scope (`GetPierForShareScoped`, reusing 02-03's `Scope` rule), pier_to through the unscoped `GetPierForShare` since D-12 lets it belong to any operator; `route.operator_id` is always derived from pier_from's stored row, never the request (D-30); an empty cancellation policy on create gets the default `[{24,100},{2,50},{0,0}]` schedule (D-13); a second active route for the same `(pier_from, pier_to)` pair is `AlreadyExists`
- `CatalogService.AddRoutePrice`/`ListRoutePrices`: effective-dated adult/child fares (`route_prices`, D-14) — amount 0-10,000,000 satang, `effective_from` never before today's Asia/Bangkok date, re-adding the same `(route, ticket_type, effective_from)` replaces the amount in one row; `AddRoutePrice` requires `CanWrite`, `ListRoutePrices` allows `staff` to read
- Every `ListRoutes` call (admin and public) attaches `current_prices` — the price in effect for today's local date per ticket type (`ListCurrentPrices`, `distinct on (route_id, ticket_type) ... order by effective_from desc`) — a route with no price set has an empty `current_prices`, never a zero-amount entry
- `CatalogService.ArchiveRoute`/`ArchivePier`: `ArchivePier` is rejected with `FailedPrecondition` while any non-archived route uses the pier as `pier_from` or `pier_to`; the message names only routes visible in the caller's scope and appends `"+N routes of other operators"` for the rest — never another operator's names or ids; both archive RPCs are idempotent (re-archiving is a successful no-op, no new event); the row lock (`FOR UPDATE`) on `ArchivePier` serializes against `UpsertRoute`'s `FOR SHARE` reads of pier_from/pier_to, so a route can never be created against a pier archived concurrently
- Archived routes/piers disappear from the public (no-claims) listings and reject further `UpsertPier`/`UpsertRoute` writes with `FailedPrecondition`
- `domain.ErrAlreadyExists` added to the shared sentinel-error set and mapped to `connect.CodeAlreadyExists` in `toConnectErr` — the first catalog entity to need this code

## Task Commits

1. **Task 1 (tracer): Routes end-to-end** — `814f878` (feat)
2. **Task 2 (auto, tdd): Effective-dated prices** — `c1353ca` (feat)
3. **Task 3 (auto): Archive rules for routes and piers** — `01f1dbe` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `proto/events/catalog/v1/{route,price}.proto` — `RouteUpserted`, `TicketType` enum, `PriceChanged`
- `proto/services/catalog/v1/catalog.proto` — `Route`, `CancellationTier`, `RoutePrice` messages and the six new RPCs
- `services/catalog/migrations/{00005_routes,00006_route_prices}.sql`
- `services/catalog/internal/adapters/postgres/queries/{routes,route_prices}.sql` (+ additions to `piers.sql`: `GetPierForShareScoped`, `GetPierForShare`, `ArchivePier`) + sqlc output
- `services/catalog/internal/domain/{route,price}.go` + their `_test.go` — `CancellationTier`, `DefaultCancellationPolicy`, `ValidateCancellationPolicy`, `Route.Validate`, `TicketType`, `RoutePrice.Validate`; `errors.go` gains `ErrAlreadyExists`
- `services/catalog/internal/app/{route,price}.go` — `UpsertRoute`/`ListRoutes`/`ArchiveRoute`, `AddRoutePrice`/`ListRoutePrices`/`attachCurrentPrices`; `pier.go` gains `ArchivePier`/`blockedArchiveError`
- `services/catalog/internal/adapters/http/{route_handlers,prices}.go` — the six new RPC handlers; `scope.go` gains the `ErrAlreadyExists` → `CodeAlreadyExists` mapping; `piers.go` gains the `ArchivePier` handler
- `services/catalog/cmd/routes_prices_integration_test.go` — `TestRoutesScopingAndSharedPierTo`, `TestRoutePricesEffectiveDating`, `TestArchivePierBlockedByRoutes`
- `services/catalog/CLAUDE.md` — Owns/Publishes/Sync API/Layout updated for routes, prices, and archive rules

## Decisions Made

- All Task 1-3 proto was generated in one `buf generate` pass during Task 1 (see key-decisions above) rather than incrementally per task, since `price.proto`'s `TicketType` is imported by `catalog.proto`'s `Route.current_prices` field (added in Task 2) — keeping `gen/` internally consistent from the start avoided a partial/inconsistent intermediate generation. Each task's commit still adds only that task's Go implementation; `UnimplementedCatalogServiceHandler` covers RPCs declared but not yet implemented.
- `AddRoutePrice`/`ListRoutePrices`/`ArchiveRoute` all reuse `GetRouteForUpdateScoped` (`FOR UPDATE`) for their scope check instead of adding read-only variants — one query, one lock semantic; the extra row lock is uncontended for these low-frequency admin operations.
- `TestRoutePricesEffectiveDating`'s archived-route assertion archives the route via a direct SQL `UPDATE` (`archiveRouteDirectly` helper) rather than the `ArchiveRoute` RPC, since `ArchiveRoute` is Task 3's deliverable — keeps Task 2's test self-contained without forward-depending on unfinished work.

## Deviations from Plan

None — plan executed as written. All `must_haves.truths`, `artifacts`, `key_links`, and `prohibitions` from the plan frontmatter are satisfied by the implementation and exercised by the three integration tests plus the two domain unit-test files.

## Issues Encountered

- The whole-workspace `make lint` target fails on 3 pre-existing `gosec` G124 findings (missing `Secure`/`HttpOnly`/`SameSite` on test-only `http.Cookie` literals) in `services/gateway/internal/adapters/http/{auth_test,proxy_test}.go`, introduced by plan 02-05 (`aa5128a`) — unrelated to this plan's catalog-only scope. `services/catalog`'s own `golangci-lint run ./...` is clean (0 issues). Logged to `.planning/phases/02-identity-catalog/deferred-items.md` and the `WINDOWS.md` ledger (kind: `lint-warning`) rather than fixed here, per the deviation-rules scope boundary (pre-existing issues in unrelated files are out of scope for this task).

## Known Stubs

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The D-14 `ListCurrentPrices(route_ids, on_date)` lookup is the exact contract Phase 4's booking-time price snapshot reuses (per the plan's own "Flagged Assumptions").
- `CAT-02`, `CAT-03`, `CAT-05`, `CAT-06`, `AUTH-05` are marked complete in this plan's frontmatter `requirements`; `AUTH-05`/`CAT-02`/`CAT-06` are also declared by sibling plans in this phase — `requirements.ready-ids` gates completion on that automatically.
- No blockers for continuing this phase's remaining plans. `go build`/`go test` are green across the whole workspace, `make proto-check` is idempotent (no drift after committing), and the full catalog integration suite (10 tests) passes live against real Postgres + Redpanda.
- One pre-existing, unrelated `make lint` failure is tracked in `deferred-items.md`/`WINDOWS.md` (see Issues Encountered) — not a blocker for this plan, but should be swept up before `/gsd-ship`.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-26*

## Self-Check: PASSED

- All 17 key created files verified present on disk (`[ -f ]`): route/price protos, migrations, sqlc queries, domain+app+http files, the integration test, this SUMMARY, and `deferred-items.md`.
- All three task commits (`814f878`, `c1353ca`, `01f1dbe`) verified present in `git log --oneline --all`.
- All acceptance criteria re-run and confirmed: `00005_routes.sql` contains `routes_active_pair_uq` and `check (pier_from_id <> pier_to_id)`; `route_test.go` contains default-policy and missing-0-hour-tier cases; `00006_route_prices.sql` contains `amount_satang bigint` and the composite primary key; `price.proto` contains `int64 amount_satang`; `pier.go` contains `ListActiveRoutesForPier` and the literal `routes of other operators`.
- Plan-level `<verification>` re-run: unit suite green (`go test -count=1 ./services/catalog/...`), full catalog integration suite green (10 tests, `go test -tags=integration -count=1 -timeout 15m ./services/catalog/...`), `TestRoutesScopingAndSharedPierTo`/`TestRoutePricesEffectiveDating`/`TestArchivePierBlockedByRoutes` each print `--- PASS`, `make proto-check` exits 0 with a clean `git status` on `gen/`, and `services/catalog`'s own `golangci-lint run ./...` reports 0 issues. The whole-workspace `make lint` fails only on the pre-existing, unrelated gateway test-file findings documented above.
