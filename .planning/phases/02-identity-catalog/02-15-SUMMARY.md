---
phase: 02-identity-catalog
plan: 15
subsystem: catalog
tags: [go, connect-go, postgres, testcontainers, concurrency, error-handling, gap-closure]

# Dependency graph
requires:
  - phase: 02-identity-catalog
    provides: "app.Scope + http.scopeFrom/toConnectErr scoping rule, UpsertPier (D-07/D-08), UpsertBoat home-pier scoping and FOR UPDATE lock (D-07) from earlier 02-xx plans"
provides:
  - "createPier's archived-operator rejection wraps domain.ErrFailedPrecondition with a human-readable reason (\"operator is archived\"), wire code unchanged (CodeFailedPrecondition)"
  - "TestConcurrentUpsertBoatSameID -- a testcontainers-backed integration proof that 10 concurrent UpsertBoat calls on one boat_id serialize safely via the FOR UPDATE row lock, with no torn writes and correct outbox fact count"
affects: [02-16, 02-17]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 1242
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Domain sentinel errors get a reason via fmt.Errorf(\"%w: reason\", sentinel) -- toConnectErr's errors.Is switch is unaffected, so wire codes never change when a message gains detail"

key-files:
  created: []
  modified:
    - services/catalog/internal/app/pier.go
    - services/catalog/cmd/operators_piers_integration_test.go
    - services/catalog/cmd/boats_photo_integration_test.go

key-decisions:
  - "TestConcurrentUpsertBoatSameID lives in boats_photo_integration_test.go, reusing every existing helper (setCatalogEnv, runService, mustCreateOperator, mustCreatePier, countOutboxEvents, app.EventBoatUpserted) -- no new helper file, per the plan's explicit instruction"
  - "Used a //nolint:gosec (G115 int->int32) on the loop-index capacity conversion, matching the codebase's established //nolint-with-reason convention (e.g. services/catalog/cmd/main.go) rather than restructuring the loop"

patterns-established: []

requirements-completed: [CAT-02, CAT-04]

coverage:
  - id: D1
    description: "Creating a pier for an archived operator returns FailedPrecondition with a message naming the reason (\"operator is archived\") instead of the bare sentinel text (G-02-8 backend half)"
    requirement: "CAT-02"
    verification:
      - kind: integration
        ref: "services/catalog/cmd#TestArchiveOperatorBlockedByPiers"
        status: pass
      - kind: integration
        ref: "services/catalog/cmd#TestArchiveOperatorRaceWithPierCreate"
        status: pass
    human_judgment: false
  - id: D2
    description: "10 concurrent UpsertBoat calls on the same boat_id are safely serialized by the existing FOR UPDATE row lock: all succeed, exactly 1+10 BoatUpserted outbox rows are written, and the pier ends with exactly one boat whose name/capacity come from a single request (no torn write) (G-02-18)"
    requirement: "CAT-04"
    verification:
      - kind: integration
        ref: "services/catalog/cmd#TestConcurrentUpsertBoatSameID"
        status: pass
      - kind: integration
        ref: "services/catalog/cmd#TestBoatsScopedByHomePier"
        status: pass
    human_judgment: false

# Metrics
duration: 18min
completed: 2026-09-28
status: complete
---

# Phase 02 Plan 15: Catalog Gap Closure (G-02-8, G-02-18) Summary

**Archived-operator pier-create rejection now says why ("operator is archived"), and a new testcontainers integration test proves 10 concurrent UpsertBoat calls on one boat_id serialize safely through the existing FOR UPDATE lock with zero torn writes.**

## Performance

- **Duration:** ~18 min
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- `createPier`'s archived-operator branch wraps `domain.ErrFailedPrecondition` with `%w: operator is archived` instead of the bare sentinel — `toConnectErr`'s `errors.Is` switch still maps it to `CodeFailedPrecondition` unchanged, only the message gained detail
- `TestArchiveOperatorBlockedByPiers` extended to assert the new message text
- New `TestConcurrentUpsertBoatSameID`: 10 goroutines call `UpsertBoat` on the same `boat_id` concurrently; all 10 succeed, exactly 11 `BoatUpserted` outbox rows exist (1 create + 10 updates), and the pier ends with exactly one boat whose name/capacity match a single request's values — proving no torn write from the `GetBoatForUpdateScoped` `FOR UPDATE` lock (D-07)

## Task Commits

Each task was committed atomically:

1. **Task 1: Give the archived-operator pier-create rejection a specific message** - `3f9fbc8` (fix)
2. **Task 2: Integration test for concurrent UpsertBoat on the same boat_id** - `2908240` (test)

**Plan metadata:** commit created below for SUMMARY.md/STATE.md/ROADMAP.md/REQUIREMENTS.md.

## Files Created/Modified
- `services/catalog/internal/app/pier.go` — `createPier`'s archived-operator branch now returns `fmt.Errorf("%w: operator is archived", domain.ErrFailedPrecondition)`
- `services/catalog/cmd/operators_piers_integration_test.go` — added `strings` import; `TestArchiveOperatorBlockedByPiers` now asserts the rejection message contains "operator is archived"
- `services/catalog/cmd/boats_photo_integration_test.go` — added `TestConcurrentUpsertBoatSameID` (10-goroutine concurrency proof); added `fmt`/`sync` imports

## Decisions Made
- Reused `boats_photo_integration_test.go`'s existing helpers for the new concurrency test rather than adding a new test file or helper — matches the plan's explicit "add no new helper files" instruction and keeps one setup pattern per test file
- `//nolint:gosec` on the bounded loop-index `int32` conversion (G115), following the codebase's established `//nolint:<linter> // reason` convention rather than restructuring the loop to avoid the conversion

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

One self-caught syntax slip while writing the new test (a missing closing paren on the goroutine's `UpsertBoat(...)` call) — caught immediately by `go vet -tags=integration` before any test run, fixed, and re-verified. Not a deviation from the plan's design, just a typo during authoring.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both catalog (backend) halves of G-02-8 and G-02-18 are closed. G-02-8's admin-UI half (filtering archived operators from the select, Thai error message) remains in plan 02-16 as documented in this plan's objective.
- `go test -count=1 ./services/catalog/...` (unit) and `go test -tags=integration -count=1 -timeout 15m -run 'TestArchiveOperatorBlockedByPiers|TestConcurrentUpsertBoatSameID|TestBoatsScopedByHomePier|TestArchiveOperatorRaceWithPierCreate|TestPiersScoping' ./services/catalog/...` all pass against real testcontainers Postgres 17 + Redpanda.
- `~/go/bin/golangci-lint run ./...` inside `services/catalog` reports 0 issues (equivalent scope to the plan's "make lint stays green for services/catalog" — the repo-wide `make lint` target also runs `apps/web`/`apps/admin` npm lint/typecheck, out of scope for a catalog-only change).
- No blockers for plans 02-16/02-17.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-28*

## Self-Check: PASSED

- FOUND: services/catalog/internal/app/pier.go
- FOUND: services/catalog/cmd/operators_piers_integration_test.go
- FOUND: services/catalog/cmd/boats_photo_integration_test.go
- FOUND: commit 3f9fbc8 (fix)
- FOUND: commit 2908240 (test)
- FOUND: `operator is archived` in pier.go
- FOUND: `TestConcurrentUpsertBoatSameID` in boats_photo_integration_test.go
