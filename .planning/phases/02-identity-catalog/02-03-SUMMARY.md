---
phase: 02-identity-catalog
plan: 03
subsystem: catalog
tags: [connect-go, sqlc, postgres, outbox, scoping, auth-05, buf]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-01)
    provides: pier_ids signed claim, auth.Role* constants, httpx.Claims.PierIDs/HeaderPierIDs
provides:
  - operators and piers tables plus CatalogService.UpsertOperator/ListOperators/ArchiveOperator/UpsertPier/ListPiers
  - app.Scope{Role, OperatorID, PierIDs} (All/CanWrite/PierIDArray) — the one operator/pier scoping rule every later catalog entity (routes, boats' home_pier_id, prices) reuses
  - http.scopeFrom/toConnectErr — the one claims-to-Scope and domain-error-to-Connect-code mapping every catalog handler shares
  - catalog.OperatorUpserted / catalog.PierUpserted outbox events (PierUpserted carries no address, Pitfall 5)
  - CAT-06 public pier listing (no claims = non-archived-only projection)
affects: [02-06, 02-08, 02-11]

# Actuals (#2632)
actuals:
  tokens: 39776
  tasks: 2
  commits: 2
plan_head_before: 25800ed514a8780d713641ea38dcc41d213ecafb

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "app.Scope{Role, OperatorID, PierIDs} with All()/CanWrite()/PierIDArray() — written once in scope.go, every entity's list/get/write calls it instead of repeating the super_admin-bypass branch (Pitfall 6)"
    - "http.scopeFrom(ctx) is the single place claims turn into a Scope and role validity is checked; http.toConnectErr(err) is the single domain-sentinel-to-Connect-code switch — every new catalog RPC handler is claims-extract -> scopeFrom -> tx-wrapped app call -> toConnectErr, no per-handler error switches"
    - "Scoped update path always re-derives operator_id from the stored row, never the request body, even when the caller is otherwise authorized to write (D-30 applies to updates, not just creates)"
    - "Out-of-scope or missing ids answer NotFound uniformly (never PermissionDenied, never the row) so existence of another operator's data is never revealed"

key-files:
  created:
    - services/catalog/internal/app/scope.go
    - services/catalog/internal/app/operator.go
    - services/catalog/internal/app/pier.go
    - services/catalog/internal/domain/operator.go
    - services/catalog/internal/domain/pier.go
    - services/catalog/internal/domain/pier_test.go
    - services/catalog/internal/adapters/http/scope.go
    - services/catalog/internal/adapters/http/operators.go
    - services/catalog/internal/adapters/http/piers.go
    - services/catalog/internal/adapters/postgres/queries/operators.sql
    - services/catalog/internal/adapters/postgres/queries/piers.sql
    - services/catalog/migrations/00003_operators.sql
    - services/catalog/migrations/00004_piers.sql
    - proto/events/catalog/v1/operator.proto
    - proto/events/catalog/v1/pier.proto
  modified:
    - proto/services/catalog/v1/catalog.proto
    - services/catalog/internal/domain/errors.go
    - services/catalog/internal/adapters/postgres/models.go
    - services/catalog/cmd/main_integration_test.go
    - services/catalog/cmd/operators_piers_integration_test.go
    - services/catalog/CLAUDE.md
    - services/catalog/go.mod
    - services/catalog/go.sum

key-decisions:
  - "UpsertOperator keeps boat.go's single ON CONFLICT DO UPDATE ... WHERE archived_at is null upsert shape (not a separate create/update branch) — matches the plan's literal query spec and existing precedent; a client-supplied id that doesn't exist yet creates a new row with that id, same property boat.go already has"
  - "UpsertPier uses an explicit create-vs-update branch (unlike operators) because pier updates must be scope-checked against (operator_id, pier_ids) via GetPierForUpdateScoped, which an ON CONFLICT upsert can't express — matches the plan's own explicit action text for piers"
  - "toPgUUIDs/toPgTime/fromPgTime conversions kept local to app/pier.go rather than extracted to a shared file — only pier code needs them (YAGNI)"

patterns-established:
  - "Scope.PierIDArray() never returns nil — pgx must send an empty array literal, not NULL, or `= any($1)` degrades from 'matches nothing' to undefined NULL-comparison behavior"

requirements-completed: [CAT-01, CAT-02, CAT-06, AUTH-05]

coverage:
  - id: D1
    description: "Operators end-to-end: super_admin creates/renames via CatalogService.UpsertOperator, every other role (pier_admin, staff, customer) gets PermissionDenied, no claims gets Unauthenticated; ListOperators applies the Scope rule, ordered by name then id with an id tiebreak"
    requirement: CAT-01
    verification:
      - kind: integration
        ref: "services/catalog/cmd/operators_piers_integration_test.go#TestOperatorsSuperAdminOnly"
        status: pass
    human_judgment: false
  - id: D2
    description: "Piers scoped by (operator_id, pier_ids): super_admin unrestricted (optionally filtered by operator_id), pier_admin/staff scoped to their claims, an empty pier_ids scope sees nothing; updates can never move a pier to a different operator even when the request tries; out-of-scope/cross-operator ids answer NotFound, never PermissionDenied, never the row; PierUpserted carries no address field"
    requirement: CAT-02
    verification:
      - kind: integration
        ref: "services/catalog/cmd/operators_piers_integration_test.go#TestPiersScoping"
        status: pass
    human_judgment: false
  - id: D3
    description: "Public (no-claims) ListPiers returns only non-archived piers with full projection (ids, names, coordinates, address, hours) — CAT-06"
    requirement: CAT-06
    verification:
      - kind: integration
        ref: "services/catalog/cmd/operators_piers_integration_test.go#TestPiersScoping (public-projection assertion)"
        status: pass
    human_judgment: false
  - id: D4
    description: "ArchiveOperator is FailedPrecondition while the operator owns any non-archived pier (no cascade, D-15); archiving an operator with none succeeds and is idempotent; creating a pier for an archived operator fails FailedPrecondition"
    verification:
      - kind: integration
        ref: "services/catalog/cmd/operators_piers_integration_test.go#TestArchiveOperatorBlockedByPiers"
        status: pass
    human_judgment: false
  - id: D5
    description: "domain.Pier.Validate: Unicode-code-point name/address length limits (including combining marks), lat/lng range and (0,0) rejection, opens_at/closes_at both-empty-or-both-HH:MM-with-opens<closes"
    requirement: CAT-02
    verification:
      - kind: unit
        ref: "services/catalog/internal/domain/pier_test.go#TestPierValidate"
        status: pass
    human_judgment: false

# Metrics
duration: 23min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 3: Catalog Operators + Piers with Shared Scope Rule Summary

**CatalogService gains UpsertOperator/ListOperators/ArchiveOperator/UpsertPier/ListPiers backed by a single reusable `app.Scope` (super_admin bypass, operator+pier_ids filter for everyone else), with pier updates unable to move operator ownership and public ListPiers serving CAT-06's unauthenticated pier list.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-26T17:04:40Z
- **Completed:** 2026-09-26T17:28:45Z
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 32 (22 created, 10 modified — 6 of the created/modified are generated `gen/` output from `buf generate`)

## Accomplishments

- `app.Scope{Role, OperatorID, PierIDs}` (`All()`, `CanWrite()`, `PierIDArray()`) is the one pier/operator scoping rule — written once, reused by operators and piers today, ready for routes/boats/prices in later plans (Pitfall 6)
- `CatalogService.UpsertOperator`/`ListOperators`/`ArchiveOperator`: only `super_admin` creates, renames, or archives an operator; `ArchiveOperator` is blocked by any active pier (D-15, no cascade) and is idempotent once archived
- `CatalogService.UpsertPier`/`ListPiers`: create is `super_admin`-only against a non-archived operator; update requires the pier already be in the caller's scope — out-of-scope/cross-operator ids answer `NotFound`, never revealing the row; the stored `operator_id` always wins over the request body (D-30)
- `ListPiers` with no claims serves the CAT-06 public projection (every non-archived pier); with claims it's scope-filtered, and the request's `operator_id` filter is honoured only for `super_admin`
- `http.scopeFrom`/`toConnectErr` are the one claims-to-Scope and error-to-Connect-code mapping every catalog handler (present and future) shares
- `catalog.OperatorUpserted`/`catalog.PierUpserted` published to the outbox in the same tx as every write; `PierUpserted` deliberately carries no `address` field (Pitfall 5) — verified both via the proto's own field descriptors and against a real outbox row
- `domain.Pier.Validate` enforces name/address code-point limits (Thai combining marks counted correctly), lat/lng range plus a (0,0) "not set" rejection, and the opens/closes-hours invariant

## Task Commits

1. **Task 1 (tracer): Operators end-to-end with the shared Scope rule** — `158dd5e` (feat)
2. **Task 2 (auto, tdd): Piers with pier-level scoping, public projection, operator archive** — `cb1b37c` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `services/catalog/internal/app/scope.go` — `Scope` and its three methods
- `services/catalog/internal/app/operator.go` — `UpsertOperator`/`ListOperators`/`ArchiveOperator`
- `services/catalog/internal/app/pier.go` — `UpsertPier`/`ListPiers` + pgtype conversion helpers
- `services/catalog/internal/domain/{operator,pier}.go` + `pier_test.go` — validation rules and their table test
- `services/catalog/internal/adapters/http/scope.go` — `scopeFrom`/`toConnectErr`
- `services/catalog/internal/adapters/http/{operators,piers}.go` — the five new RPC handlers
- `services/catalog/internal/adapters/postgres/queries/{operators,piers}.sql` + sqlc output
- `services/catalog/migrations/{00003_operators,00004_piers}.sql`
- `proto/services/catalog/v1/catalog.proto`, `proto/events/catalog/v1/{operator,pier}.proto` + generated Go/TS
- `services/catalog/cmd/{main_integration_test,operators_piers_integration_test}.go` — `claimHeaders` helper, `TestOperatorsSuperAdminOnly`, `TestPiersScoping`, `TestArchiveOperatorBlockedByPiers`
- `services/catalog/CLAUDE.md` — Owns/Publishes/Sync API updated for operators and piers
- `services/catalog/go.mod`/`go.sum` — `GOWORK=off go mod tidy` after catalog's first `pkg/auth` import (role constants), matching identity's plan 02-02 fix for the same standalone-module gap

## Decisions Made

- Kept `UpsertOperator`'s single `ON CONFLICT ... WHERE archived_at is null` upsert shape (matches `boat.go` precedent and the plan's literal query text) rather than an explicit create/update branch — operators have no pier-scoped update path to justify the extra complexity.
- `UpsertPier` uses an explicit create-vs-update branch (unlike operators) because updates must be scope-checked via `GetPierForUpdateScoped`'s `(operator_id, pier_ids)` filter, which a single upsert statement can't express — matches the plan's own explicit action text.
- Kept `toPgUUIDs`/`toPgTime`/`fromPgTime` local to `app/pier.go` rather than extracting a shared conversions file — only pier code needs them today (YAGNI).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `services/catalog/go.mod`'s `go.sum` was incomplete for the Dockerfile's standalone (`GOWORK=off`) module build**
- **Found during:** Task 1, after adding catalog's first `pkg/auth` import (role constants) for `app/scope.go`
- **Issue:** Workspace-mode `go build` resolved fine, but the Dockerfile builds each service as an isolated module (`GOWORK=off`), which needs every transitive dependency's `go.sum` entry — including `github.com/golang-jwt/jwt/v5`, pulled in via `pkg/auth` (a new import edge for catalog). This is the same gap identity's plan 02-02 hit and fixed.
- **Fix:** `cd services/catalog && GOWORK=off go mod tidy`.
- **Files modified:** `services/catalog/go.mod`, `services/catalog/go.sum`
- **Verification:** `GOWORK=off go build ./...` succeeds in `services/catalog`; workspace-mode build and full test suite still green.
- **Committed in:** `158dd5e` (Task 1 commit)

### Process Note (not a Rule 1-4 deviation)

Task 2 carried `tdd="true"`, but `workflow.tdd_mode` is disabled for this project (this is an `execute`-type plan, not a `type: tdd` plan, so the strict RED-gate enforcement in `gsd-core/references/tdd.md` does not apply). Domain validation rules, the app/http implementation, and the integration tests were written and verified together in one pass rather than as a separately-committed failing-test step — reverting the implementation after the fact to manufacture a genuine RED commit would not have reflected an actual failing run at the time, so no RED commit was fabricated. Every acceptance criterion and `<verify>` command was re-run and confirmed green before committing.

---

**Total deviations:** 1 auto-fixed (Rule 3 blocking), 1 process note (TDD gate not applicable — `workflow.tdd_mode: false`)
**Impact on plan:** The go.sum fix was necessary for the service's real Docker build path; the TDD process note has no functional impact — all behavior is fully implemented and tested.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `app.Scope` and `http.scopeFrom`/`toConnectErr` are the established pattern plans 02-06 (routes, cancellation policy, prices) and 02-08 (boats gain `home_pier_id` pier scoping) build directly on, with no further trust-boundary or scoping-shape changes expected.
- `catalog.PierUpserted`'s no-address shape and the `ListPiersPublic`/`ListPiersAdmin` split are ready for 02-04's gateway public-route wiring (`/api/v1/public/piers`) and 02-11's admin UI.
- `CAT-01`, `CAT-02`, `CAT-06`, `AUTH-05` are marked complete in this plan's frontmatter `requirements`; `AUTH-05` may also be declared by sibling plans still in flight in this phase — `requirements.ready-ids` gates on that automatically.
- No blockers. `go build`/`go test` green across the whole workspace, `make proto-check` and `make sqlc-gen` are idempotent (no drift) after committing, and the full catalog integration suite (7 tests: 2 boat, 1 template, 2 operator/pier, 2 new piers/archive) passes live against real Postgres + Redpanda.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-26*

## Self-Check: PASSED

- All 16 key created files verified present on disk (`[ -f ]`): scope/operator/pier app+domain+http files, sqlc queries, migrations, event protos, and this SUMMARY.
- Both task commits (`158dd5e`, `cb1b37c`) verified present in `git log --oneline --all`.
- All acceptance criteria re-run and confirmed: `Scope`/`scopeFrom`/`toConnectErr` symbols present via build; `TestOperatorsSuperAdminOnly`, `TestPiersScoping`, `TestArchiveOperatorBlockedByPiers` print `--- PASS`; `proto/pii-check.sh proto/events` exits 0; `queries/piers.sql` contains `order by name_th, id` in both list queries; `00004_piers.sql` contains `references operators(id)` and the opens/closes check constraint.
- Plan-level `<verification>` re-run clean: `make proto-check` exits 0 with a clean `git status` on `gen/`, `make sqlc-gen` is idempotent (no diff), `go test -count=1` (unit) and `go test -tags=integration -count=1 -timeout 15m` (full catalog suite, 7 tests) both green.
