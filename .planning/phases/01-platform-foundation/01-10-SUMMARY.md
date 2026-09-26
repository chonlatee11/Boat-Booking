---
phase: 01-platform-foundation
plan: 10
subsystem: infra
tags: [make, scaffold, connect-go, otelconnect, sqlc, outbox, catalog, boats]

requires:
  - phase: 01-platform-foundation (plans 01, 02, 04, 07, 09)
    provides: "pkg/httpx.RequireInternal/RequireClaims/FromContext (01-01), catalogv1/catalogv1connect generated Go (01-02), pkg/outbox.Insert + pkg/pgx.WithTx/NewPool (01-04), pkg/kafka.Consumer.Handle idempotent apply (01-07), services/_template one-binary runtime + health/readiness/ordered-shutdown (01-09)"
provides:
  - "make new-service name=<svc>: validates ^[a-z][a-z0-9]*$, refuses an existing services/<name> or a name already in deploy/services.txt, copies services/_template, renames __NAME__, wires go.work + deploy/services.txt — every guard exits 1 before touching the filesystem"
  - "make template-smoke: proves a freshly scaffolded service builds into a container image reporting Docker health status healthy, then removes every trace (container, dir, go.work entry, services.txt line) via an EXIT trap"
  - "services/catalog: the real first scaffolded service — CatalogService.UpsertBoat (connect-go) writes a boats row + catalog.BoatUpserted outbox row in one tx, operator_id from trusted claims only, tenant-scoped so a cross-operator boat_id reuse returns NotFound with the stored row unchanged; CatalogService.ListBoats returns boats ordered by name then id, internal-token only"
  - "pkg/httpx.ConnectOtel(): otelconnect interceptor connect.Option usable on both connect-go handlers and clients (D-50)"
affects: [01-11, 01-12-e2e-walking-skeleton, 01-13, phase-02-identity-catalog]

actuals:
  tokens: 24494
  tasks: 2
  commits: 2
  plan_head_before: e04feff67cd5857db86cdd86d201f588e51b1205

tech-stack:
  added:
    - "connectrpc.com/otelconnect v0.10.0 — connect-go OTel tracing/metrics interceptor (D-50)"
  patterns:
    - "make new-service's guards (missing name, invalid regex, existing services/<name>, name already in deploy/services.txt) all run before the first filesystem-mutating command, in separate recipe lines that each exit 1 on failure — GNU make aborts the whole target the instant one fails, so no guard's failure can leave a partial cp/sed/go-work-use behind"
    - "make template-smoke's cleanup lives in a single `trap ... EXIT` so container/dir/go.work/services.txt cleanup runs on both the success and failure paths of the same shell invocation"
    - "Every module whose go.mod pulls in pkg (directly or via replace) needs its own `GOWORK=off go mod tidy` whenever pkg gains a new dependency — go.work.sum covers workspace-mode builds but the root Dockerfile's GOWORK=off build only trusts each module's own go.sum (same class of fix as 01-09's services/gateway deviation, now also applied to services/_template and services/catalog)"
    - "Proto status<->domain status conversion is deliberately duplicated as two tiny private switches (app.statusToProto for the outgoing event, http.statusFromProto/statusToProto for request/response) rather than shared, because internal/domain must stay proto-free (D-14) and each side has different validation needs (the HTTP side rejects UNSPECIFIED as InvalidArgument; the app side only ever sees an already-validated domain.Status)"

key-files:
  created:
    - services/_template/CLAUDE.md
    - services/catalog/CLAUDE.md
    - services/catalog/go.mod
    - services/catalog/go.sum
    - services/catalog/cmd/main.go
    - services/catalog/cmd/main_integration_test.go
    - services/catalog/internal/domain/errors.go
    - services/catalog/internal/domain/boat.go
    - services/catalog/internal/app/boat.go
    - services/catalog/internal/adapters/postgres/sqlc.yaml
    - services/catalog/internal/adapters/postgres/queries/boats.sql
    - services/catalog/internal/adapters/postgres/db.go
    - services/catalog/internal/adapters/postgres/models.go
    - services/catalog/internal/adapters/postgres/boats.sql.go
    - services/catalog/internal/adapters/http/routes.go
    - services/catalog/internal/adapters/kafka/handlers.go
    - services/catalog/migrations/00001_platform.sql
    - services/catalog/migrations/00002_boats.sql
    - deploy/services.txt
    - pkg/httpx/connect.go
  modified:
    - Makefile
    - go.work
    - pkg/go.mod
    - pkg/go.sum
    - services/_template/go.mod
    - services/_template/go.sum
    - services/gateway/go.mod
    - services/gateway/go.sum

key-decisions:
  - "otelconnect pinned to v0.10.0 exactly as the plan's context specified (verified go.mod requires go 1.25.0, compatible with the repo's go1.25.7 pin) — no version substitution needed this time."
  - "Kafka-side proof for TestUpsertBoatPublishesBoatUpserted reuses pkg/kafka.Consumer (a throwaway uniquely-named consumer group, which franz-go defaults to reading from the earliest offset) instead of inventing new raw-consumer test infrastructure — the producer always sets the Kafka record key to the outbox row's aggregate_id, so asserting env.AggregateId == boatID after decode is equivalent proof of 'published to catalog.events keyed by boat_id' without needing raw kgo.Record access in the test."
  - "GOWORK=off go mod tidy re-run for pkg, services/_template, services/catalog, and services/gateway after adding otelconnect to pkg — workspace-mode go.work.sum absorbed the new checksums but each module's own go.sum (what the root Dockerfile's GOWORK=off build actually reads) did not, exactly the class of issue 01-09 hit with services/gateway; this time it also reached the two other modules that import pkg."

patterns-established:
  - "Guard-then-mutate Makefile targets: every precondition check is its own recipe line ending in `exit 1` on failure, placed before any command that touches the filesystem — a reusable shape for any future `make <verb>-service`-style scaffolding target."

requirements-completed: [PLAT-01, PLAT-05, PLAT-02]

coverage:
  - id: D1
    description: "make new-service name=<svc> scaffolds a working service from services/_template: renames every __NAME__ token, wires go.work + deploy/services.txt, and the result builds and its own template-inherited tests run"
    requirement: PLAT-01
    verification:
      - kind: integration
        ref: "make template-smoke (scaffolds services/tsmoke, go build succeeds) — PASS template-smoke printed"
        status: pass
    human_judgment: false
  - id: D2
    description: "make new-service guards reject a missing name, a name not matching ^[a-z][a-z0-9]*$, and an existing service name, each exiting non-zero with the filesystem left untouched (git status --porcelain unchanged)"
    requirement: PLAT-01
    verification:
      - kind: integration
        ref: "make template-smoke's guard loop (no name, Bad_Name, 9x, catalog) — all 4 rejected, tree unchanged each time"
        status: pass
    human_judgment: false
  - id: D3
    description: "make template-smoke builds a freshly scaffolded service into a container image reporting Docker health status healthy, then removes every trace (container, dir, go.work entry, services.txt line)"
    requirement: PLAT-01
    verification:
      - kind: integration
        ref: "make template-smoke (manual run, this plan) — PASS template-smoke; post-run test ! -e services/tsmoke, no tsmoke in go.work/deploy/services.txt, no bb-tsmoke container"
        status: pass
    human_judgment: false
  - id: D4
    description: "CatalogService.UpsertBoat (connect-go) writes the boats row and a catalog.BoatUpserted outbox row in one tx, operator_id from trusted claims only; the relay publishes it to catalog.events keyed by boat_id"
    requirement: PLAT-05
    verification:
      - kind: integration
        ref: "services/catalog/cmd/main_integration_test.go#TestUpsertBoatPublishesBoatUpserted"
        status: pass
    human_judgment: false
  - id: D5
    description: "UpsertBoat rejects missing claims (Unauthenticated) and invalid name/capacity/status (InvalidArgument); reusing an existing boat_id under a different operator returns NotFound and leaves the stored boat unchanged"
    requirement: PLAT-05
    verification:
      - kind: integration
        ref: "services/catalog/cmd/main_integration_test.go#TestUpsertBoatValidationAndTenancy"
        status: pass
    human_judgment: false
  - id: D6
    description: "ListBoats returns every boat ordered by name then id (stable order) and needs only the internal token, no claims"
    requirement: PLAT-02
    verification:
      - kind: integration
        ref: "services/catalog/cmd/main_integration_test.go#TestListBoatsOrdered"
        status: pass
    human_judgment: false
  - id: D7
    description: "make test and make lint stay green repo-wide with catalog wired into go.work, including the standalone (GOWORK=off) build every module needs for the root Dockerfile"
    verification:
      - kind: other
        ref: "make test (all ok), make lint (0 issues across pkg/gateway/_template/catalog, buf lint, PII check), and GOWORK=off go build ./... for pkg, services/_template, services/catalog, services/gateway — all re-run clean at the end of this plan"
        status: pass
    human_judgment: false

duration: ~27min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 10: Service Scaffolding + First Real Service (catalog) Summary

**`make new-service` scaffolds a guarded, proven-healthy service from the template in one command, and `services/catalog` is the first real one — its `UpsertBoat` connect-go RPC writes a tenant-scoped boats row and a `catalog.BoatUpserted` outbox row in one transaction, which the relay publishes to `catalog.events` keyed by `boat_id`.**

## Performance

- **Duration:** ~27 min
- **Completed:** 2026-09-26T06:10:58Z
- **Tasks:** 2 completed
- **Files:** 20 created, 8 modified

## Accomplishments

- `make new-service name=<svc>` (D-03): every guard (missing name, invalid `^[a-z][a-z0-9]*$` name, existing `services/<name>`, name already in `deploy/services.txt`) exits 1 before any filesystem mutation; on success it copies `services/_template`, renames every `__NAME__` token, and wires `go.work` + `deploy/services.txt` in one command
- `make template-smoke` (D-37): proves the guard rails (4 rejected calls, tree unchanged each time) and that a freshly scaffolded service builds into a container image the Docker healthcheck reports `healthy` — then removes every trace via an `EXIT` trap, success or failure
- `services/catalog` — the real first scaffolded service (D-01): `CatalogService.UpsertBoat` validates input, assigns a fresh uuid v7 on create, and writes the `boats` row + its `catalog.BoatUpserted` outbox row in the exact same Postgres transaction, with `operator_id` taken only from trusted gateway claims; the tenancy-scoped `ON CONFLICT ... WHERE boats.operator_id = excluded.operator_id` upsert means reusing another operator's `boat_id` returns `NotFound` with the stored row provably unchanged
- `CatalogService.ListBoats` returns every boat ordered by `name` then `id`, gated only by the internal token (no claims required — a catalog-wide read)
- `pkg/httpx.ConnectOtel()`: the otelconnect interceptor option every connect-go handler/client in the codebase can now reuse for OTel tracing/metrics (D-50)
- Four green integration tests prove the whole slice end-to-end: template boot/shutdown (inherited), the write-side proof event through a real Kafka consume, validation/auth/tenancy, and list ordering

## Task Commits

1. **Task 1: make new-service catalog -> UpsertBoat (connect) -> boats row + catalog.BoatUpserted outbox row in one tx -> relay -> catalog.events** - `e2ff718` (feat)
2. **Task 2: new-service guard rails, template-smoke image healthcheck, catalog CLAUDE.md, validation + tenancy tests** - `0249d94` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

_Task 1 is `type="tracer"` — the tracer feedback gate (re-running both of Task 1's `<verify>` commands: the two named integration tests, plus `make test && make lint` with the `__NAME__`/services.txt/go.work checks) was re-executed after commit and passed before starting Task 2, per the `human_verify_mode: end-of-phase` + automated-only `<verify>` rule (no checkpoint needed on success)._

## Files Created/Modified

- `Makefile` — `new-service` and `template-smoke` targets
- `deploy/services.txt` — the shared services list (D-24); currently `catalog`
- `go.work` — added `./services/catalog`
- `pkg/httpx/connect.go` — `ConnectOtel()`
- `pkg/go.mod`, `pkg/go.sum` — `connectrpc.com/otelconnect` dependency
- `services/_template/CLAUDE.md` — the per-service agent guide copied into every new service
- `services/_template/go.mod`, `go.sum` — re-tidied for GOWORK=off (see Deviations)
- `services/catalog/CLAUDE.md` — catalog's own Owns/Publishes/Consumes/Sync API doc
- `services/catalog/go.mod`, `go.sum` — new module in `go.work`
- `services/catalog/cmd/main.go` — the template's `run(ctx)`, renamed to `catalog`
- `services/catalog/cmd/main_integration_test.go` — `TestTemplateReadyAndGracefulShutdown` (renamed), `TestUpsertBoatPublishesBoatUpserted`, `TestUpsertBoatValidationAndTenancy`, `TestListBoatsOrdered`
- `services/catalog/internal/domain/{errors,boat}.go` — `Boat`, `Status`, `Validate`
- `services/catalog/internal/app/boat.go` — `EventBoatUpserted`, `UpsertBoat`, `ListBoats`
- `services/catalog/internal/adapters/http/routes.go` — the `CatalogService` connect handler
- `services/catalog/internal/adapters/kafka/handlers.go` — empty `Register` (catalog consumes nothing yet)
- `services/catalog/internal/adapters/postgres/{sqlc.yaml,queries/boats.sql,db.go,models.go,boats.sql.go}` — sqlc-generated Postgres layer
- `services/catalog/migrations/{00001_platform,00002_boats}.sql` — outbox/processed_events (copied) + `boats` table
- `services/gateway/go.mod`, `go.sum` — re-tidied for GOWORK=off (see Deviations)

## Decisions Made

See `key-decisions` in frontmatter: otelconnect pinned at v0.10.0 as specified, the Kafka-consume proof reusing `pkg/kafka.Consumer` instead of new raw-consumer test infrastructure, and the `GOWORK=off go mod tidy` re-tidy across `pkg`/`services/_template`/`services/catalog`/`services/gateway`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `pkg/go.mod`'s `otelconnect` requirement stayed `// indirect`, and downstream modules' go.sum lacked its transitive checksums for GOWORK=off builds**
- **Found during:** Task 2, first `make template-smoke` run — the root Dockerfile's `GOWORK=off go build` failed with `missing go.sum entry for module providing package connectrpc.com/otelconnect`.
- **Issue:** `go get connectrpc.com/otelconnect@v0.10.0` was run in workspace mode, which resolves/verifies against the shared `go.work.sum` rather than `pkg`'s own `go.sum`, and didn't re-derive the direct/indirect marker from `pkg/httpx/connect.go`'s actual import. `services/_template`, `services/catalog`, and `services/gateway` all import `pkg` (directly or via `replace`), so their own standalone (non-workspace) builds needed the new transitive checksums too — the exact class of issue 01-09 hit with `services/gateway` alone, now also reaching two more modules because `pkg` itself changed this time.
- **Fix:** Ran `GOWORK=off go mod tidy` in `pkg`, `services/_template`, `services/catalog`, and `services/gateway`.
- **Files modified:** `pkg/go.mod`, `services/_template/go.mod`, `services/_template/go.sum`, `services/catalog/go.mod`, `services/catalog/go.sum`, `services/gateway/go.mod`, `services/gateway/go.sum`
- **Verification:** `GOWORK=off go build ./...` succeeds standalone in all four modules; `make template-smoke` builds and reports `healthy`; workspace-mode `make test`/`make lint` unaffected (still green).
- **Committed in:** `0249d94` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (blocking dependency-checksum issue, same class as a prior plan's).
**Impact on plan:** Necessary for `make template-smoke`'s own Docker build (a Task 2 deliverable) to succeed at all — no production behavior changed beyond `go.mod`/`go.sum` bookkeeping. No scope creep.

### Task Boundary Note (not a deviation, disclosed for transparency)

The plan assigns `TestUpsertBoatValidationAndTenancy` and `TestListBoatsOrdered` to Task 2 as "tests-first" (`tdd="true"`), but the validation (`domain.Boat.Validate`) and tenancy enforcement (the `ON CONFLICT ... WHERE boats.operator_id = excluded.operator_id` upsert, the HTTP handler's claims check) were necessarily built as part of Task 1's own `UpsertBoat` — they are Rule 2 correctness/security requirements of a working upsert, and this plan's own threat register assigns `T-10-01` (Elevation of Privilege, high severity) and `T-10-04` (injection, medium severity) `mitigate` dispositions to exactly this code, both explicitly required from Task 1's first commit, not deferrable to Task 2. Both new test functions were therefore written and verified passing together with Task 1's implementation (in the same file rewrite, committed under Task 1's commit `e2ff718`), rather than as an isolated Task 2 RED-then-GREEN cycle — a true RED phase was not possible without shipping a real (if temporary) tenant-isolation gap. `workflow.tdd_mode` is `false` in this project's config, so the plan-level TDD gate (`gsd_run check tdd-red-evidence`) does not apply; this note is disclosed for auditability, not a gate violation.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `make new-service` is real and proven (`make template-smoke` green): any future service (schedule, identity, booking, payment, notification) can be scaffolded with one command and inherits every platform guarantee for free.
- `services/catalog` is the first real, `go.work`-registered, CI-tested service beyond the template and gateway — `catalog.BoatUpserted` is now a real event on `catalog.events`, ready for `services/schedule` (plan 01-11 or later) to consume.
- `pkg/httpx.ConnectOtel()` is available for any future connect-go service or client that needs OTel tracing wired in.
- No blockers for 01-11 onward.

## Self-Check: PASSED

- All 20 created files confirmed present on disk; `git log --oneline` confirms both commits (`e2ff718`, `0249d94`).
- Plan-level `<verification>` re-run clean at the end of this plan: all 4 catalog integration tests pass (`TestTemplateReadyAndGracefulShutdown`, `TestUpsertBoatPublishesBoatUpserted`, `TestUpsertBoatValidationAndTenancy`, `TestListBoatsOrdered`); `make template-smoke` prints `PASS template-smoke` and leaves no trace; `make test` and `make lint` (0 issues across `pkg`/`gateway`/`_template`/`catalog`, `buf lint`, PII check) both green.

## Self-Check: PASSED

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
