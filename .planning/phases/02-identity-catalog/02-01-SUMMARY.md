---
phase: 02-identity-catalog
plan: 01
subsystem: auth
tags: [jwt, pier-ids, httpx, gateway, trust-boundary, seaweedfs, package-legitimacy]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: pkg/auth JWT issue/verify, pkg/httpx claim-forwarding trust boundary (RequireInternal, ForwardClaims), gateway BFF (D-27, D-29, D-30)
provides:
  - Signed pier_ids claim on access/refresh tokens, round-tripping Issue -> Verify
  - httpx.HeaderPierIDs (X-Pier-Ids), httpx.Claims.PierIDs, httpx.ForwardClaims (moved from gateway into pkg/httpx)
  - RequireInternal validates every X-Pier-Ids part as a uuid, 401s on any invalid part
  - GET /api/v1/whoami returns pier_ids as a JSON array through Kong
  - Role constants (auth.RoleCustomer/RoleStaff/RolePierAdmin/RoleSuperAdmin) for later plans to import
  - devtoken token -pier-ids flag
  - Dev object-storage decision (SeaweedFS) and package-legitimacy verdicts recorded for later plans' preconditions
affects: [02-02, 02-03, 02-04, 02-05, 02-08, 02-11]

# Actuals (#2632)
actuals:
  tokens: 6200
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "httpx.ForwardClaims is the single place trusted headers are deleted-then-set; new services never write their own copy"
    - "Comma-joined uuid list is the wire format for multi-value trusted claim headers (X-Pier-Ids) — same single-value-header convention as the other claim headers (research Assumption A4)"
    - "RequireInternal fails closed on any malformed trusted-header value (uuid.Parse) rather than passing raw strings toward SQL"

key-files:
  created: []
  modified:
    - pkg/auth/auth.go
    - pkg/auth/auth_test.go
    - pkg/auth/cmd/devtoken/main.go
    - pkg/httpx/claims.go
    - pkg/httpx/claims_test.go
    - services/gateway/internal/adapters/http/bff.go
    - services/gateway/internal/adapters/http/bff_test.go
    - services/gateway/CLAUDE.md
    - deploy/kong/roundtrip.sh

key-decisions:
  - "Dev object storage for pier photos (D-19 implementation detail): SeaweedFS S3 gateway, image chrislusf/seaweedfs:4.47 (pinned, never latest) — MinIO is unpullable from Docker Hub (404, archived Apr 2026)"
  - "go-redis v9.22.0, minio-go v7.3.0, maplibre-gl 6.11.2 approved by the developer at the Task 2 blocking-human legitimacy checkpoint"
  - "ForwardClaims moved from the gateway package into pkg/httpx so every future internal caller (not just the gateway) shares one implementation"

patterns-established:
  - "Role constants live once in pkg/auth (RoleCustomer, RoleStaff, RolePierAdmin, RoleSuperAdmin) — later plans import them instead of repeating string literals"

requirements-completed: [AUTH-02, AUTH-03, AUTH-05]

coverage:
  - id: D1
    description: "Dev object-storage server for pier photos decided: SeaweedFS chrislusf/seaweedfs:4.47"
    verification: []
    human_judgment: true
    rationale: "Human checkpoint decision (Task 1, gate=blocking) — no automated test applies to a human's storage-vendor choice."
  - id: D2
    description: "Package legitimacy verdicts recorded for go-redis v9.22.0, minio-go v7.3.0, maplibre-gl 6.11.2 (developer replied \"approved\" for all)"
    verification: []
    human_judgment: true
    rationale: "Human checkpoint decision (Task 2, gate=blocking-human) — package legitimacy is inherently a human judgment call, not something a test can assert."
  - id: D3
    description: "pier_ids is a signed JWT claim that round-trips Issue -> Verify; nil PierIDs verifies to an empty (non-nil) slice; role constants defined once in pkg/auth"
    requirement: AUTH-02
    verification:
      - kind: unit
        ref: "pkg/auth/auth_test.go#TestIssueVerifyRoundTripWithPierIDs"
        status: pass
      - kind: unit
        ref: "pkg/auth/auth_test.go#TestIssueNilPierIDsVerifiesToEmptySlice"
        status: pass
      - kind: unit
        ref: "pkg/auth/auth_test.go#TestRoleConstants"
        status: pass
    human_judgment: false
  - id: D4
    description: "httpx.RequireInternal parses/validates X-Pier-Ids (uuid.Parse per part, 401 on any invalid part, next never called); httpx.ForwardClaims (moved from gateway) deletes all five trusted headers first and sets X-Pier-Ids only when non-empty"
    requirement: AUTH-03
    verification:
      - kind: unit
        ref: "pkg/httpx/claims_test.go#TestRequireInternalValidPierIDsHeader"
        status: pass
      - kind: unit
        ref: "pkg/httpx/claims_test.go#TestRequireInternalInvalidPierIDsHeaderRejects"
        status: pass
      - kind: unit
        ref: "pkg/httpx/claims_test.go#TestForwardClaimsSetsVerifiedValues"
        status: pass
      - kind: unit
        ref: "pkg/httpx/claims_test.go#TestForwardClaimsEmptyPierIDsSetsNoHeader"
        status: pass
      - kind: unit
        ref: "pkg/httpx/claims_test.go#TestForwardClaimsOverwritesSpoofedHeaders"
        status: pass
    human_judgment: false
  - id: D5
    description: "GET /api/v1/whoami returns pier_ids as a JSON array end-to-end through Kong, for a devtoken minted with -pier-ids"
    requirement: AUTH-05
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestWhoamiReturnsPierIDs"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/bff_test.go#TestWhoamiReturnsEmptyPierIDsArray"
        status: pass
      - kind: e2e
        ref: "deploy/kong/roundtrip.sh check whoami-pier-ids (make kong-roundtrip)"
        status: pass
    human_judgment: false

# Metrics
duration: 11min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 1: pier_ids Claim End-to-End Summary

**Signed `pier_ids` JWT claim survives Kong -> gateway re-verify -> `X-Pier-Ids` header -> `httpx.RequireInternal` as validated `httpx.Claims.PierIDs`, with `ForwardClaims` consolidated into `pkg/httpx`; SeaweedFS and three package legitimacy verdicts recorded as the phase's two human decisions.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-09-26T16:11:17Z
- **Completed:** 2026-09-26T16:22:00Z
- **Tasks:** 3 (2 checkpoints resolved by the developer before dispatch, 1 tracer/TDD task executed)
- **Files modified:** 9

## Accomplishments

- `pkg/auth.Claims`/`jwtClaims` carry `PierIDs []string` (`json:"pier_ids,omitempty"`); `Issue` copies it in, `Verify` returns a non-nil empty slice when the claim is absent so callers can `len()` safely
- `auth.RoleCustomer`/`RoleStaff`/`RolePierAdmin`/`RoleSuperAdmin` constants defined once, ready for later plans to import instead of repeating string literals
- `pkg/httpx.HeaderPierIDs = "X-Pier-Ids"` and `httpx.Claims.PierIDs`; `RequireInternal` splits the header on `,`, trims, drops empty parts, and rejects (401, `next` never called) any part that fails `uuid.Parse`
- `ForwardClaims` moved from the gateway package into `pkg/httpx` (single implementation for every future internal caller): deletes all five trusted headers first, then sets user/operator/role/internal-token and `X-Pier-Ids` (comma-joined) only when `PierIDs` is non-empty
- Gateway `whoami` returns `pier_ids` as a JSON array (`writeJSON` now takes `any` instead of `map[string]string`); `deploy/kong/roundtrip.sh` gained a `whoami-pier-ids` check, verified green through the live Kong -> gateway stack
- `devtoken token -pier-ids <uuid,uuid>` flag for minting scoped test tokens
- Two human decisions recorded before any dependent plan runs: SeaweedFS for dev object storage, and legitimacy verdicts for go-redis/minio-go/maplibre-gl

## Task Commits

Each task was committed atomically (RED -> GREEN, no REFACTOR commit needed — implementation was minimal on first pass):

1. **Task 1 & 2 (checkpoints):** No code changes — decisions recorded in this SUMMARY (see Decisions Made below). Resolved by the developer with the orchestrator before this executor was dispatched.
2. **Task 3 (tracer, tdd): pier_ids end-to-end** —
   - `1833c0b` `test(02-01): add failing tests for pier_ids claim end-to-end (D-06)` (RED — build failure on undefined `PierIDs`/`HeaderPierIDs`/`ForwardClaims`/`Role*`, the target new symbols)
   - `e51cb75` `feat(02-01): pier_ids claim end-to-end through Kong -> gateway -> httpx.Claims (D-06)` (GREEN — all tests pass, `make kong-roundtrip` green)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `pkg/auth/auth.go` — `Claims.PierIDs`, `jwtClaims.PierIDs` (`pier_ids`), role constants
- `pkg/auth/auth_test.go` — PierIDs round-trip tests, role constant test; fixed `TestIssueVerifyRoundTrip`'s struct-equality comparison (broken by the new slice field)
- `pkg/auth/cmd/devtoken/main.go` — `-pier-ids` flag
- `pkg/httpx/claims.go` — `HeaderPierIDs`, `Claims.PierIDs`, `parsePierIDs` (uuid validation), `RequireInternal` pier-id parsing, `ForwardClaims` (moved from gateway)
- `pkg/httpx/claims_test.go` — new RequireInternal/ForwardClaims pier-id tests; fixed `TestRequireInternalCorrectTokenWithClaims`'s struct-equality comparison
- `services/gateway/internal/adapters/http/bff.go` — deleted local `ForwardClaims`, calls `httpx.ForwardClaims`; `whoamiHandler` returns `pier_ids`; `writeJSON` takes `any`
- `services/gateway/internal/adapters/http/bff_test.go` — new whoami pier_ids tests; moved `TestForwardClaims*` tests out (now in `pkg/httpx/claims_test.go`); fixed `TestWhoamiUnchanged`'s decode target (`map[string]string` can't hold an array field)
- `services/gateway/CLAUDE.md` — Trust Rules updated to list `X-Pier-Ids` and the `httpx.ForwardClaims` move
- `deploy/kong/roundtrip.sh` — new `whoami-pier-ids` check (renumbered the trailing rate-limit check from `(k)` to `(l)`)

## Decisions Made

- **Task 1 (checkpoint:decision, resolved before dispatch):** Dev object-storage server = **SeaweedFS**, image `chrislusf/seaweedfs:4.47` (pinned, never `latest`). MinIO was rejected — Docker Hub returns 404 for `minio/minio` (repository archived), so it cannot be pulled on a clean machine or CI agent. This choice affects plan 02-11 (which reads it) and any later plan wiring a storage container.
- **Task 2 (checkpoint:human-verify, gate=blocking-human, resolved before dispatch):** Developer replied "approved" for all three flagged packages:

| Package | Ecosystem | Version | Installed by | Verdict |
|---|---|---|---|---|
| github.com/redis/go-redis/v9 | Go | v9.22.0 | plan 02-02 (identity) | approved |
| github.com/minio/minio-go/v7 | Go | v7.3.0 | plan 02-08 (catalog) | approved |
| maplibre-gl | npm | 6.11.2 | plan 02-11 (apps/admin) | approved |

  OK verdicts (no action needed): input-otp, sonner, next-themes. Not installed at all (planner discretion, removed from scope): resend-go, @tanstack/react-table, testcontainers valkey/mailpit/minio modules.

- **Task 3:** `ForwardClaims` moved from the gateway package into `pkg/httpx` rather than kept local and called from a new location — every future internal caller shares one implementation instead of each service re-implementing the delete-then-set pattern.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed two pre-existing tests broken by the `PierIDs []string` field addition**
- **Found during:** Task 3 GREEN phase
- **Issue:** Adding a slice field to `auth.Claims` and `httpx.Claims` broke their existing `got != want` struct-equality comparisons in `TestIssueVerifyRoundTrip` (pkg/auth) and `TestRequireInternalCorrectTokenWithClaims` (pkg/httpx) — Go slices are not comparable with `==`/`!=`, so this is a compile error, not a runtime failure.
- **Fix:** Replaced whole-struct equality with explicit per-field comparisons (plus a `len(PierIDs) == 0` assertion) in both tests.
- **Files modified:** pkg/auth/auth_test.go, pkg/httpx/claims_test.go
- **Verification:** `go test ./pkg/auth/... ./pkg/httpx/...` green
- **Committed in:** e51cb75 (Task 3 GREEN commit)

**2. [Rule 1 - Bug] Fixed `TestWhoamiUnchanged` decode target after `whoami` gained an array field**
- **Found during:** Task 3 GREEN phase
- **Issue:** `whoamiHandler` now returns `pier_ids` as a JSON array; the existing test decoded the whole response into `map[string]string`, which fails on any non-string value (`json: cannot unmarshal array into Go value of type string`).
- **Fix:** Decode into a small anonymous struct with typed fields instead.
- **Files modified:** services/gateway/internal/adapters/http/bff_test.go
- **Verification:** `go test ./services/gateway/...` green
- **Committed in:** e51cb75 (Task 3 GREEN commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — compile/runtime breaks directly caused by this task's own field addition, not pre-existing unrelated issues)
**Impact on plan:** Both fixes were necessary to keep the build/test suite green after the intentional `PierIDs` field addition. No scope creep — no unrelated code touched.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. `make dev-keys && make up` rebuilt the gateway/catalog/schedule images with the new code and all services report healthy.

## Next Phase Readiness

- `pier_ids` is now a trusted, validated claim reaching every downstream service via `httpx.Claims.PierIDs` — plan 02-02 (identity), 02-03 (catalog scope helper), and 02-04 (gateway proxy expansion) can build on this shape without further trust-boundary changes.
- Both phase-blocking human decisions (dev storage, package legitimacy) are recorded here; plans 02-02, 02-08, 02-11 can check their preconditions against this SUMMARY instead of re-asking.
- No blockers. `go test` is green across every module (pkg, services/_template, services/catalog, services/gateway, services/schedule) and `make kong-roundtrip` passes all 11 checks including the new `whoami-pier-ids`.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-26*

## Self-Check: PASSED

- All 9 modified files verified present on disk (`[ -f ]`).
- Both commits (`1833c0b` RED, `e51cb75` GREEN) verified present in `git log --oneline --all`.
- All Task 3 acceptance criteria re-run and confirmed passing (field/const presence via grep, `func ForwardClaims` count = 0 in gateway, invalid-uuid 401 test present, `make kong-roundtrip` prints `PASS whoami-pier-ids` with no `FAIL` line).
- `go test -count=1 ./...` green across every go.work module: pkg, services/_template, services/catalog, services/gateway, services/schedule.
