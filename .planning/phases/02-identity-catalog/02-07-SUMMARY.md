---
phase: 02-identity-catalog
plan: 07
subsystem: auth
tags: [connect-go, sqlc, postgres, outbox, catalog-sync-call, super-admin, staff-management]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-05)
    provides: identity AuthService.Refresh/Logout, EnsureSuperAdmin bootstrap, pkg/httpx.ForwardClaims moved to pkg/httpx
  - phase: 02-identity-catalog (plan 02-03)
    provides: catalog CatalogService.ListPiers (super_admin + operator_id filter, archived flag), app.Scope pattern
provides:
  - "identity UserService (ListUsers, UpsertUser, SetUserDisabled) — super_admin's staff-management API, reachable through the gateway admin proxy's existing boatbooking.identity.v1.UserService allow-list"
  - "app.Users.validatePiers — the one synchronous cross-service pier-ownership check (research Pattern 3): forwards caller claims + internal token to catalog.ListPiers, rejects unknown/archived/foreign piers by id"
  - "identity CATALOG_URL env var + catalogv1connect.CatalogServiceClient wiring in cmd/main.go — the first identity-to-catalog sync call"
affects: [02-13]

# Actuals (#2632)
actuals:
  tokens: 25583
  tasks: 2
  commits: 2
plan_head_before: a57b55c3a02c294ff934fa396496079d9b6c6f6e

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cross-service ownership check without a foreign key (database-per-service): app.Users.validatePiers builds a connect.Request, calls httpx.ForwardClaims(req.Header(), caller, internalToken) to carry the caller's role/operator/piers + internal token, then diffs the requested ids against catalog's response — the same ForwardClaims helper the gateway uses, reused for service-to-service calls"
    - "Idempotent create-or-promote insert: InsertStaffUser (ON CONFLICT DO NOTHING) falling back to PromoteCustomerToStaff (UPDATE ... WHERE role='customer') on ErrNoRows — a losing concurrent insert's promote attempt also finds nothing and reports ErrAlreadyExists, so the same two-query shape handles create-new, promote-existing-customer, and the 5-way concurrent-create race with no extra locking"
    - "sqlc CASE WHEN $n type-inference gotcha: `disabled_at = case when $2 then now() else null end` typed $2 as pgtype.Timestamptz instead of bool — fixed by forcing the type with `sqlc.arg(disabled)::bool`"

key-files:
  created:
    - proto/services/identity/v1/users.proto
    - gen/go/identity/v1/users.pb.go
    - gen/go/identity/v1/identityv1connect/users.connect.go
    - gen/ts/services/identity/v1/users_pb.ts
    - services/identity/internal/domain/user_test.go
    - services/identity/internal/app/users.go
    - services/identity/internal/adapters/http/users.go
    - services/identity/cmd/users_integration_test.go
  modified:
    - services/identity/internal/domain/user.go
    - services/identity/internal/domain/errors.go
    - services/identity/internal/adapters/postgres/queries/users.sql
    - services/identity/internal/adapters/postgres/users.sql.go
    - services/identity/internal/adapters/http/routes.go
    - services/identity/cmd/main.go
    - services/identity/CLAUDE.md

key-decisions:
  - "domain.StaffUserInput.Validate uses a pointer receiver so it normalises Email in place via NormalizeDestination (kind email) as part of validation, instead of duplicating normalization in the app layer"
  - "domain/errors.go gained ErrPermissionDenied/ErrAlreadyExists/ErrUnavailable/ErrFailedPrecondition in one edit (Task 1) even though only Task 1's sentinels were needed immediately — avoids touching the same small file twice across the two tasks"
  - "toPgUUIDs kept local to app/users.go (not extracted to convert.go) — only user code needs it today, matching catalog's own per-file placement of the identical helper (YAGNI)"
  - "SetUserDisabled checks self-disable before the target row's existence/role, so disabling one's own claimed user id returns FailedPrecondition even if that id has no matching row — matches the plan's literal guard ordering and needs no seeded row in tests"

requirements-completed: []  # AUTH-04/AUTH-02/AUTH-05 also declared by sibling plans (02-08, 02-09, 02-13) without a SUMMARY yet — requirements.ready-ids reported 0/3 ready; the last plan declaring each one marks it

coverage:
  - id: D1
    description: "Only super_admin can call UserService (ListUsers, UpsertUser, SetUserDisabled); pier_admin/staff get PermissionDenied, no claims gets Unauthenticated"
    requirement: AUTH-04
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestCreateStaffUserValidatesPiers/pier_admin_caller_gets_PermissionDenied"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestCreateStaffUserValidatesPiers/no_claims_gets_Unauthenticated"
        status: pass
    human_judgment: false
  - id: D2
    description: "UpsertUser only assigns role staff or pier_admin; customer and super_admin are rejected with InvalidArgument (super_admin only via SUPER_ADMIN_EMAIL, D-09)"
    requirement: AUTH-04
    verification:
      - kind: unit
        ref: "services/identity/internal/domain/user_test.go#TestStaffUserInputValidate/role_customer_rejected"
        status: pass
      - kind: unit
        ref: "services/identity/internal/domain/user_test.go#TestStaffUserInputValidate/role_super_admin_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestCreateStaffUserValidatesPiers/role_super_admin_rejected"
        status: pass
    human_judgment: false
  - id: D3
    description: "Before persisting, UpsertUser calls catalog ListPiers (forwarding caller claims + internal token, operator_id filter) and rejects with InvalidArgument naming any pier that is missing, archived, or owned by another operator"
    requirement: AUTH-04
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestCreateStaffUserValidatesPiers/pier_of_another_operator_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestCreateStaffUserValidatesPiers/archived_pier_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestCreateStaffUserValidatesPiers/valid_pier_creates_staff_and_publishes_UserCreated_with_no_PII"
        status: pass
    human_judgment: false
  - id: D4
    description: "A created staff user who signs in via OTP receives a JWT with that role, operator_id, and pier_ids"
    requirement: AUTH-02
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestStaffLoginCarriesAssignedClaims"
        status: pass
    human_judgment: false
  - id: D5
    description: "Creating a user whose email already belongs to staff/pier_admin/super_admin returns AlreadyExists and changes nothing; an existing customer email is promoted in place (same user id), with no second identity.UserCreated"
    requirement: AUTH-04
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpsertUserIdempotencyAndConcurrency/re-creating_the_same_email_is_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpsertUserIdempotencyAndConcurrency/existing_customer_email_is_promoted_in_place"
        status: pass
    human_judgment: false
  - id: D6
    description: "Five concurrent UpsertUser creates for the same new email yield exactly one success and four AlreadyExists, with one users row"
    requirement: AUTH-04
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpsertUserIdempotencyAndConcurrency/concurrent_creates_for_one_new_email_yield_exactly_one_winner"
        status: pass
    human_judgment: false
  - id: D7
    description: "SetUserDisabled(true) sets disabled_at and revokes all refresh tokens (refresh -> Unauthenticated, OTP login -> PermissionDenied); disabling yourself or a super_admin is FailedPrecondition; SetUserDisabled(false) re-enables login"
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/disable_revokes_refresh_and_blocks_login;_re-enable_restores_it"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/disabling_self_is_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/disabling_a_super_admin_is_rejected"
        status: pass
    human_judgment: false
  - id: D8
    description: "ListUsers returns non-customer users ordered by email then id, optionally filtered by operator_id; customers are never listed; updating by user_id changes name/role/operator/piers with catalog re-validation, email is immutable, and a customer/super_admin target row is FailedPrecondition"
    requirement: AUTH-04
    verification:
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/ListUsers_excludes_customers,_orders_by_email_then_id,_and_filters_by_operator"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/update_changes_name/role/operator/piers"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/changing_email_on_update_is_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/updating_a_customer_row_is_rejected"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/users_integration_test.go#TestUpdateAndDisableUser/updating_a_super_admin_row_is_rejected"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-09-27
status: complete
---

# Phase 2 Plan 7: Staff User Management via UserService + Synchronous Catalog Pier Validation Summary

**identity gains `UserService` (ListUsers/UpsertUser/SetUserDisabled): super_admin-only staff/pier_admin account management whose pier ownership is confirmed by a live call to catalog's `ListPiers` (no foreign key possible across service databases), with idempotent create-or-promote semantics and disable-time refresh-token revocation.**

## Performance

- **Duration:** 20 min
- **Started:** 2026-09-27T01:53:00+07:00
- **Completed:** 2026-09-27T02:10:34+07:00
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 15 (8 created, 7 modified)

## Accomplishments

- `proto/services/identity/v1/users.proto`: `UserService` (`ListUsers`, `UpsertUser`, `SetUserDisabled`) and `StaffUser` — reachable immediately through the gateway admin proxy's `boatbooking.identity.v1.UserService` allow-list wired in plan 02-04, no gateway change needed.
- `app.Users.validatePiers`: the only pier-ownership check that can exist across a database-per-service boundary — forwards the caller's claims plus the internal token to catalog's `ListPiers` (operator-filtered), then rejects any requested pier that catalog didn't return non-archived under that operator, naming the offending ids.
- `app.Users.UpsertUser` create path: `InsertStaffUser` (`ON CONFLICT (email) DO NOTHING`) falling back to `PromoteCustomerToStaff` (`UPDATE ... WHERE role = 'customer'`) on no rows — handles brand-new emails, promoting an existing customer in place (same user id, no duplicate `identity.UserCreated`), and a 5-way concurrent race for the same new email (exactly one winner, four `AlreadyExists`) with the same two queries and no extra locking.
- `app.Users.updateUser`: keeps `email` immutable, re-validates operator/piers via the same catalog call on every update, and rejects touching a `customer`/`super_admin` row with `FailedPrecondition`.
- `app.Users.SetUserDisabled`: sets `disabled_at` and revokes every refresh token for the user in the same transaction (D-10) — a disabled user's next refresh fails immediately and OTP login returns `PermissionDenied`; disabling the caller's own id or a `super_admin` row is rejected.
- `app.Users.ListUsers`: `staff`/`pier_admin`/`super_admin` only (never `customer`, PDPA minimal exposure), ordered by `email` then `id`, optional `operator_id` filter.
- identity `cmd/main.go`: new `CATALOG_URL` env var (default `http://catalog:8080`) wires a `catalogv1connect.CatalogServiceClient` into `app.Users` — identity's first synchronous call to another service.
- `cmd/users_integration_test.go`: a fake catalog (`httptest` server hosting a real `catalogv1connect.CatalogServiceHandler` behind `httpx.RequireInternal`) proves the whole loop live — `TestCreateStaffUserValidatesPiers`, `TestStaffLoginCarriesAssignedClaims`, `TestUpsertUserIdempotencyAndConcurrency`, `TestUpdateAndDisableUser`.

## Task Commits

1. **Task 1 (tracer): Staff user end-to-end — UpsertUser create path + catalog pier validation + staff OTP login** - `0e2ffae` (feat)
2. **Task 2 (auto, tdd): List/update/disable with guards** - `f9d58bb` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `proto/services/identity/v1/users.proto` + `gen/go/identity/v1/{users.pb.go,identityv1connect/users.connect.go}` + `gen/ts/services/identity/v1/users_pb.ts` — `UserService` contract
- `services/identity/internal/domain/user.go` — `StaffUserInput` + `Validate` (role/name/pier_ids/email invariants)
- `services/identity/internal/domain/user_test.go` — table tests for `Validate`, including the email-normalization side effect
- `services/identity/internal/domain/errors.go` — `ErrPermissionDenied`, `ErrAlreadyExists`, `ErrUnavailable`, `ErrFailedPrecondition`
- `services/identity/internal/app/users.go` — `Users{Pool,Catalog,InternalToken,Nudge}`, `UpsertUser`/`validatePiers`/`createUser`/`updateUser`/`SetUserDisabled`/`ListUsers`, `toPgUUIDs`
- `services/identity/internal/adapters/postgres/queries/users.sql` + generated code — `InsertStaffUser`, `PromoteCustomerToStaff`, `UpdateStaffUser`, `SetUserDisabledAt`, `ListStaffUsers`
- `services/identity/internal/adapters/http/users.go` — `UserService` handler (`UpsertUser`/`ListUsers`/`SetUserDisabled`), `toProtoStaffUser`, `mapUserError`
- `services/identity/internal/adapters/http/routes.go` — `Routes` now mounts both `AuthService` and `UserService`
- `services/identity/cmd/main.go` — `CATALOG_URL`, `catalogv1connect.CatalogServiceClient` wiring, `app.Users` construction
- `services/identity/cmd/users_integration_test.go` — fake catalog fixture + all four Task 1/2 integration tests
- `services/identity/CLAUDE.md` — `UserService` sync-API section, `CATALOG_URL` env var, updated layout notes

## Decisions Made

- `StaffUserInput.Validate` uses a pointer receiver so it normalises `Email` in place (via `NormalizeDestination`, kind email) as part of validation — the app layer never duplicates that normalization.
- `domain/errors.go` gained all four new sentinels (`ErrPermissionDenied`, `ErrAlreadyExists`, `ErrUnavailable`, `ErrFailedPrecondition`) in Task 1's commit, even though Task 2 was the only consumer of `ErrFailedPrecondition` — avoids a second small edit to the same file.
- `toPgUUIDs` stays local to `app/users.go` rather than a shared `convert.go` — matches catalog's own per-file placement of the identical helper (YAGNI); only user code needs it today.
- `SetUserDisabled` checks the self-disable guard before loading the target row, so disabling the caller's own claimed id returns `FailedPrecondition` even without a matching database row — matches the plan's literal guard order and needs no seeded fixture in tests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `domain/errors.go` needed new sentinels not listed in the plan's `files_modified`**
- **Found during:** Task 1 (writing `app.Users.UpsertUser`, which returns `domain.ErrPermissionDenied`)
- **Issue:** The plan's frontmatter `files_modified` list omits `services/identity/internal/domain/errors.go`, but the action text explicitly requires `ErrPermissionDenied` ("add sentinel") plus `ErrAlreadyExists`/`ErrUnavailable`, and Task 2 additionally needs `ErrFailedPrecondition` — none of the four existed in identity's domain package before this plan.
- **Fix:** Added all four sentinels to `domain/errors.go` in Task 1's commit.
- **Files modified:** `services/identity/internal/domain/errors.go`
- **Verification:** `go build`/`go vet` clean; both integration test files exercise every mapped connect code.
- **Committed in:** `0e2ffae` (Task 1 commit)

**2. [Rule 1 - Bug] sqlc mistyped the `SetUserDisabledAt` boolean parameter as `pgtype.Timestamptz`**
- **Found during:** Task 2, first `sqlc generate` for `SetUserDisabledAt`
- **Issue:** `disabled_at = case when $2 then now() else null end` made sqlc's Postgres-based type inference type `$2` from the `SET` target column (`timestamptz`) instead of the `CASE WHEN` boolean condition, producing a `SetUserDisabledAtParams{ID, DisabledAt pgtype.Timestamptz}` that could never be called with a plain `bool`.
- **Fix:** Forced the parameter type with `sqlc.arg(disabled)::bool` in the query; regenerated to a correct `SetUserDisabledAtParams{ID, Disabled bool}`.
- **Files modified:** `services/identity/internal/adapters/postgres/queries/users.sql`, `services/identity/internal/adapters/postgres/users.sql.go`
- **Verification:** `TestUpdateAndDisableUser` disable/re-enable subtest passes.
- **Committed in:** `f9d58bb` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking — missing sentinels, 1 Rule 1 bug — sqlc type inference)
**Impact on plan:** Both fixes were necessary for the plan's own described behavior to compile and work correctly. No scope creep.

## Issues Encountered

- `make lint` fails on a **pre-existing, out-of-scope** gosec finding (`G124: http.Cookie missing Secure/HttpOnly/SameSite`) in `services/gateway/internal/adapters/http/auth_test.go` (three `req.AddCookie` calls), introduced by plan 02-05 and already tracked in `.planning/WINDOWS.md` (`lint-warning`, phase 02) before this plan ran. identity's own `golangci-lint run ./...` is clean (0 issues), and the loop in `make lint` aborts on the first failing module (gateway, which precedes identity alphabetically) — so identity's lint was verified directly rather than via the full `make lint` run. Not fixed here per the deviation-rules scope boundary (pre-existing, unrelated file).

## User Setup Required

None — no external service configuration required. `CATALOG_URL` has a working default (`http://catalog:8080`) matching the existing Compose service name.

## Next Phase Readiness

- `AUTH-04`, `AUTH-02`, and `AUTH-05` are **NOT** yet marked complete in `REQUIREMENTS.md` — `requirements.ready-ids` reported 0/3 ready: `AUTH-04` is also declared by 02-13 (no SUMMARY yet), `AUTH-02` by 02-09/02-13, and `AUTH-05` by 02-08 — all still in flight. The last plan to finish each one marks it.
- `UserService` is complete and independently verified end-to-end (create → catalog validation → login claims → update → disable/re-enable → list) — plan 02-13's admin staff-page backend can build directly on it with no further trust-boundary or contract changes expected.
- No blockers. `go build`/`go vet`/`go test` are green across the whole workspace, `make proto-check` is clean (gen/ committed, no drift), `make sqlc-gen` is idempotent, and the full identity integration suite passes live against real Postgres + Redpanda + Valkey + Mailpit testcontainers (`go test -tags=integration -count=1 -timeout 15m ./services/identity/...`).

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 8 created files verified present on disk (`[ -f ]`): `users.proto`, `users.pb.go`, `users.connect.go`, `users_pb.ts`, `user_test.go`, `app/users.go`, `adapters/http/users.go`, `cmd/users_integration_test.go`.
- Both task commits (`0e2ffae`, `f9d58bb`) verified present in `git log --oneline --all`.
- All acceptance criteria re-run and confirmed: `proto/services/identity/v1/users.proto` contains `service UserService`; `app/users.go` contains `ForwardClaims(` and `ListPiersRequest`; `users.sql` contains `order by email, id` and `role <> 'customer'`; `app/users.go` calls `RevokeAllRefreshTokens` inside the disable tx.
- Plan-level `<verification>` re-run clean: `make proto-check` exits 0 with a clean `git status` on `gen/`; `go test -count=1 ./services/identity/...` and `go test -tags=integration -count=1 -timeout 15m ./services/identity/...` both green (all 4 new tests plus the full pre-existing suite); `golangci-lint run ./...` inside `services/identity` reports 0 issues (the full `make lint` fails only on a pre-existing, out-of-scope gosec finding in `services/gateway`, documented above).
