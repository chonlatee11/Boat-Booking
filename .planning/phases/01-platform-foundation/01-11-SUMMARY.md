---
phase: 01-platform-foundation
plan: 11
subsystem: api
tags: [make-new-service, kafka-consumer, connect-go, protojson, jwt, bff, sqlc]

requires:
  - phase: 01-platform-foundation (plans 01, 07, 09, 10)
    provides: "pkg/auth.Verifier/Issuer/Cookie (01-01), pkg/kafka.Consumer.Handle idempotent apply (01-07), services/_template one-binary runtime (01-09), make new-service + services/catalog CatalogService.UpsertBoat/ListBoats + pkg/httpx.ConnectOtel (01-10)"
provides:
  - "services/schedule: the consumer half of the catalog<->schedule proof event — app.ApplyBoatUpserted unmarshals catalog.BoatUpserted and upserts a boats projection (boat_id, operator_id, default_capacity, status), applied exactly once via kafka.Consumer.Handle's processed_events wrapper; a later event for the same boat overwrites the projection"
  - "services/gateway re-scaffolded from the template runtime (chi + errgroup + ordered shutdown + -healthcheck), no DATABASE_URL so relay/consumer resolve disabled by the template's own existing logic — no special-casing needed"
  - "Routes(r, v, catalog, internalToken): GET /api/v1/public/boats proxies ListBoats with only the internal token (empty list renders as []); POST /api/v1/boats verifies the access_token cookie, rejects unknown JSON fields, forwards verified claims via ForwardClaims (deleting any client-spoofed X-* headers first), and maps catalog connect errors through httpx.WriteError; GET /api/v1/whoami unchanged"
affects: ["01-12-e2e-walking-skeleton", "phase-02-identity-catalog", "phase-03-scheduling-inventory"]

actuals:
  tokens: 25700
  tasks: 2
  commits: 3
  plan_head_before: 64a13a865b8c1eb500b8c40c7fbd63c1cc9d51d5

tech-stack:
  added: []
  patterns:
    - "A DB-less service (gateway) reuses the template's run(ctx) byte-for-byte: relayEnabled/consumerEnabled already resolve false whenever DATABASE_URL is unset, so no gateway-specific branch was needed in main.go — only the route-mounting section differs (direct mount, no httpx.RequireInternal, plus verifier/catalog-client construction)"
    - "Trust-boundary origin services (the gateway) mount their routes directly on the chi router; every other service mounts behind httpx.RequireInternal — the one structural difference from the template's route-wiring section"
    - "protojson.Unmarshal's zero-value UnmarshalOptions already rejects unknown fields (DiscardUnknown defaults false) — no extra option needed to satisfy the 'reject unknown fields' requirement (T-11-03)"

key-files:
  created:
    - services/schedule/internal/domain/boat.go
    - services/schedule/internal/app/boat.go
    - services/schedule/internal/adapters/postgres/{sqlc.yaml,queries/boats.sql,db.go,models.go,boats.sql.go}
    - services/schedule/migrations/{00001_platform.sql,00002_boats.sql}
    - services/schedule/cmd/main_integration_test.go
    - services/schedule/CLAUDE.md
    - services/gateway/CLAUDE.md
    - services/gateway/internal/adapters/kafka/handlers.go
  modified:
    - go.work
    - deploy/services.txt
    - services/gateway/cmd/main.go
    - services/gateway/go.mod
    - services/gateway/go.sum
    - services/gateway/internal/adapters/http/bff.go
    - services/gateway/internal/adapters/http/bff_test.go

key-decisions:
  - "Kept the schedule projection deliberately minimal (boat_id, operator_id, default_capacity, status, updated_at) — Phase 3 departures need only these fields from catalog; anything else stays in catalog's own database (database-per-service)"
  - "Reused auth.Cookie(auth.KindAccess, tok) in gateway tests instead of a bare http.Cookie{} literal to satisfy gosec G124 (missing Secure/HttpOnly/SameSite) without a nolint escape hatch — the sanctioned constructor already sets those attributes"
  - "gateway's go.mod moved gen/go from a dangling replace (no require) to a real require once cmd/main.go started importing catalogv1connect — go mod tidy resolved this automatically, no manual edit needed"

patterns-established:
  - "TDD task boundary for a single-task plan step (not a whole type=tdd plan): RED commit is a test file targeting a not-yet-existing function signature, confirmed failing via `go vet` (a compile-time signature mismatch, not a runtime assertion) since workflow.tdd_mode is false and no strict RED-evidence gate applies at the task level"

requirements-completed: [PLAT-01, PLAT-05, PLAT-10]

coverage:
  - id: D1
    description: "schedule scaffolded via make new-service and boots/shuts down cleanly through the same template-inherited health/readiness/shutdown lifecycle as every other service"
    requirement: PLAT-01
    verification:
      - kind: integration
        ref: "services/schedule/cmd/main_integration_test.go#TestTemplateReadyAndGracefulShutdown"
        status: pass
    human_judgment: false
  - id: D2
    description: "schedule consumes catalog.BoatUpserted from catalog.events and upserts its own boats projection exactly once; a duplicate delivery of the same event_id is a no-op and a later event for the same boat overwrites the projection"
    requirement: PLAT-05
    verification:
      - kind: integration
        ref: "services/schedule/cmd/main_integration_test.go#TestBoatUpsertedAppliedOnce"
        status: pass
    human_judgment: false
  - id: D3
    description: "services/gateway is re-scaffolded from the template runtime (config, logging, OTel, readiness, ordered shutdown, -healthcheck) with no database and relay/consumer off"
    requirement: PLAT-01
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestWhoamiUnchanged (proves the runtime + verifier wiring boots correctly end-to-end at the HTTP layer)"
        status: pass
    human_judgment: true
    rationale: "No dedicated integration test exercises gateway's own /healthz, /readyz, or SIGTERM shutdown path (it has no DATABASE_URL/KAFKA_BROKERS in tests, so those goroutines never start) — the runtime shape is structurally identical to services/catalog's and services/schedule's, both of which DO have TestTemplateReadyAndGracefulShutdown, but gateway's own boot/shutdown is only proven by code reuse, not a direct test. Plan 12 wires gateway into the running docker-compose stack, which is the first point a human or `make kong-roundtrip` can observe it end-to-end."
  - id: D4
    description: "GET /api/v1/public/boats calls catalog ListBoats with only X-Internal-Token and returns protojson {\"boats\": [...]} (empty list rendered as [], not null)"
    requirement: PLAT-10
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestPublicBoatsListNoAuthRequired"
        status: pass
    human_judgment: false
  - id: D5
    description: "POST /api/v1/boats verifies the access_token cookie, strips any client-supplied X-User-Id/X-Operator-Id/X-Role/X-Internal-Token and sets them from verified claims + the internal token, and calls catalog UpsertBoat over connect-go; 201 on success, 401 without a valid access token (missing/expired/refresh-kind), and catalog's CodeInvalidArgument maps to HTTP 400 {\"code\":\"invalid_argument\"}"
    requirement: PLAT-10
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestUpsertBoatForwardsVerifiedClaimsAndStripsSpoofedHeaders"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestUpsertBoatRequiresValidAccessToken"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestUpsertBoatMapsCatalogErrorCodes"
        status: pass
    human_judgment: false
  - id: D6
    description: "make test, make test-integration, and make lint all stay green repo-wide with schedule and the re-scaffolded gateway in the workspace"
    verification:
      - kind: other
        ref: "make test (all ok), make test-integration (all ok, ~2min), make lint (0 issues across pkg/gateway/_template/catalog/schedule, buf lint, PII check) — all re-run clean at the end of this plan"
        status: pass
    human_judgment: false

duration: ~30min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 11: Schedule Consumer + Gateway BFF Re-scaffold Summary

**`services/schedule` applies `catalog.BoatUpserted` into its own `boats` projection exactly once via `pkg/kafka.Consumer.Handle`, and `services/gateway` is re-scaffolded onto the shared template runtime with real `GET /api/v1/public/boats` / `POST /api/v1/boats` routes that turn verified JWTs into trusted `connect-go` calls to catalog.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-09-26T06:04:00Z (approx.)
- **Completed:** 2026-09-26T06:30:00Z
- **Tasks:** 2 completed
- **Files:** 17 created, 8 modified

## Accomplishments

- `make new-service name=schedule` scaffolded the second real service; the sample `pings` slice was fully replaced with schedule's own `boats` projection (`boat_id`, `operator_id`, `default_capacity`, `status`) — schedule never reads catalog's database (database-per-service)
- `app.ApplyBoatUpserted` unmarshals `catalog.BoatUpserted`, maps the proto status enum to schedule's own domain status (an `UNSPECIFIED` status is an error so it retries then DLQs rather than writing a bad projection), and upserts the projection — the exactly-once guarantee comes entirely from `pkg/kafka.Consumer`'s existing `processed_events` wrapper, no extra idempotency code was needed
- `TestBoatUpsertedAppliedOnce`: a duplicate delivery of the same envelope applies once (`result=duplicate` logged, no error), and a later event for the same boat overwrites the projection (capacity 42 -> 50); `processed_events` holds exactly 2 rows at the end
- `services/gateway/cmd/main.go` now shares the exact one-binary runtime shape as every other service scaffolded from `services/_template` — the only differences are building an `auth.Verifier` + a `catalogv1connect.CatalogServiceClient`, and mounting routes directly instead of behind `httpx.RequireInternal` (the gateway is where that trust boundary originates)
- `bff.Routes` gained two real routes: `GET /api/v1/public/boats` (internal-token-only proxy to `ListBoats`, empty list renders as `[]`) and `POST /api/v1/boats` (verifies the `access_token` cookie, decodes+validates the body via `protojson` — rejecting unknown fields by default — forwards verified claims + the internal token via the existing `ForwardClaims`, and proxies to `UpsertBoat`); `GET /api/v1/whoami` is unchanged
- Five gateway behavior cases proven with a fake `catalogv1connect.CatalogServiceHandler` behind `httpx.RequireInternal` standing in for the real catalog service: verified-claims forwarding with spoofed-header stripping, three 401 cases (missing/expired/refresh-kind token) with the fake never called, public listing needing no auth, and catalog's `CodeInvalidArgument` mapping to HTTP 400

## Task Commits

1. **Task 1: make new-service schedule -> consume catalog.BoatUpserted -> boats projection applied exactly once (TestBoatUpsertedAppliedOnce)** - `cb3f1dd` (feat)
2. **Task 2, RED: failing test for gateway BFF routes** - `aa45c0c` (test)
2. **Task 2, GREEN: gateway re-scaffolded + BFF routes implemented** - `3f59d7b` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

_Task 1 is `type="tracer"` — the tracer feedback gate (re-running both of Task 1's `<verify>` commands: the two named integration tests, plus the sample-slice/services.txt/go.work checks plus `make test && make lint`) was re-executed after commit and passed before starting Task 2, per the `human_verify_mode: end-of-phase` + automated-only `<verify>` rule (no checkpoint needed on success)._

## Files Created/Modified

- `go.work`, `deploy/services.txt` — `./services/schedule` registered
- `services/schedule/internal/domain/boat.go` — `Boat`, `Status`
- `services/schedule/internal/app/boat.go` — `EventCatalogBoatUpserted`, `ApplyBoatUpserted`
- `services/schedule/internal/adapters/postgres/{sqlc.yaml,queries/boats.sql,db.go,models.go,boats.sql.go}` — sqlc-generated `boats` projection layer
- `services/schedule/internal/adapters/kafka/handlers.go` — `Register` wiring `catalog.BoatUpserted` to `app.ApplyBoatUpserted`
- `services/schedule/internal/adapters/http/routes.go` — registers no routes (Phase 3 work)
- `services/schedule/migrations/{00001_platform.sql,00002_boats.sql}` — outbox/processed_events (copied) + `boats` projection table
- `services/schedule/cmd/main_integration_test.go` — `TestTemplateReadyAndGracefulShutdown` (kept), `TestBoatUpsertedAppliedOnce` (new)
- `services/schedule/CLAUDE.md` — schedule's own Owns/Publishes/Consumes/Sync API doc
- `services/gateway/cmd/main.go` — re-scaffolded onto the template runtime + verifier/catalog-client construction
- `services/gateway/internal/adapters/http/bff.go` — `publicBoatsHandler`, `upsertBoatHandler`, `writeProtoJSON`; `Routes` signature gained `catalog`/`internalToken`
- `services/gateway/internal/adapters/http/bff_test.go` — full behavior suite (5 new tests + the 2 pre-existing `ForwardClaims` unit tests)
- `services/gateway/internal/adapters/kafka/handlers.go` — empty `Register` (template parity)
- `services/gateway/go.mod`, `go.sum` — `gen/go` now a real require, `pgx`/`errgroup` direct deps
- `services/gateway/CLAUDE.md` — gateway's own agent guide (owns no data, routes table, trust rules)

## Decisions Made

See `key-decisions` in frontmatter: minimal schedule projection scope, `auth.Cookie` over a bare `http.Cookie{}` literal in tests (gosec), and the `gen/go` require getting picked up automatically by `go mod tidy` once `cmd/main.go` imported `catalogv1connect`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Lint/Security] gosec G124 on three test-only `http.Cookie{}` literals**
- **Found during:** Task 2, first `make lint` run after implementing `bff.go`
- **Issue:** `golangci-lint`'s gosec linter flagged `&http.Cookie{Name: auth.AccessCookie, Value: tok}` in `bff_test.go` for missing `Secure`/`HttpOnly`/`SameSite` attributes (G124) — a real rule, just triggered on test code building an outbound request cookie.
- **Fix:** Replaced all three occurrences with `auth.Cookie(auth.KindAccess, tok)`, the project's own sanctioned cookie constructor, which already sets those attributes — no `nolint` annotation needed.
- **Files modified:** `services/gateway/internal/adapters/http/bff_test.go`
- **Verification:** `make lint` reports 0 issues; all gateway tests still pass.
- **Committed in:** `3f59d7b` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (lint/security, no scope creep).
**Impact on plan:** Zero behavior change — a test-only fix that also makes the test's cookie construction consistent with production code's own `auth.Cookie` usage.

## Issues Encountered

None beyond the deviation above. The Redpanda testcontainer failed to start once during schedule's integration test run (port-mapping timeout, a known flaky-startup mode already called out for this environment) — the immediate retry succeeded cleanly.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The consumer half of the `catalog.BoatUpserted` proof event is now real, tested code: `services/schedule` holds a correct, exactly-once projection of catalog boats.
- `services/gateway` now has the real BFF -> catalog hop (`POST /api/v1/boats`, `GET /api/v1/public/boats`) that PLAT-10's "curl -> Kong -> BFF -> stub service" walking skeleton needs — catalog is that service.
- Both are unit/integration tested in isolation; plan 12 wires them into the running `docker-compose` stack (env vars, Kong route config, `make kong-roundtrip`) for the first true end-to-end proof spanning HTTP -> outbox -> Kafka -> consumer as a single trace.
- No blockers for 01-12.

## Self-Check: PASSED

- All created files confirmed present on disk (`services/schedule/**`, `services/gateway/CLAUDE.md`, `services/gateway/internal/adapters/kafka/handlers.go`).
- `git log --oneline` confirms all three commits (`cb3f1dd`, `aa45c0c`, `3f59d7b`).
- Plan-level `<verification>` re-run clean at the end of this plan: `make test` (all green), `make test-integration` (all green, ~2min across pkg/kafka, pkg/outbox, `_template`, catalog, gateway, schedule), `make lint` (0 issues, buf lint clean, PII check clean).
- All plan `<acceptance_criteria>` re-verified via direct `grep`/`test` commands, all pass.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
