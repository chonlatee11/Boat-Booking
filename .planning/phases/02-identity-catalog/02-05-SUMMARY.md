---
phase: 02-identity-catalog
plan: 05
subsystem: auth
tags: [cookies, connect-go, jwt, kong, refresh-rotation, otp, super-admin]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-02)
    provides: identity AuthService.RequestOtp/VerifyOtp, pkg/auth Issuer/Verifier/Cookie, notify.Sender
  - phase: 02-identity-catalog (plan 02-04)
    provides: gateway Routes/proxyClient wiring, httpx.NewHTTPClient, httpx.ForwardClaims
provides:
  - "Browser-facing cookie session: POST /api/v1/auth/{otp/request,otp/verify,refresh,logout} on the gateway, backed by a dedicated unauthenticated + rate-limited Kong route (api-auth)"
  - "identity AuthService.Refresh/Logout RPCs: single-use refresh-token rotation with whole-family reuse detection, D-10 claim re-read every refresh, idempotent logout"
  - "identity startup bootstrap: EnsureSuperAdmin makes SUPER_ADMIN_EMAIL a non-disabled super_admin on every boot (D-09), no other path can mint one"
affects: [02-11]

# Actuals (#2632)
actuals:
  tokens: 24851
  tasks: 3
  commits: 4
plan_head_before: b8bfb550f89729372cf542afa587235d70e126d2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Gateway auth routes build the outbound connect.Request from scratch and set only X-Internal-Token — never copy any inbound header/cookie — the same claim-less pattern proxy.go's publicHandler already established (D-29)"
    - "Session-invalidating errors (reuse, expiry, disabled) all collapse to one domain.ErrSessionInvalid -> Unauthenticated mapping; the gateway reacts identically (clear both cookies) regardless of which internal reason caused it"
    - "RS256 (PKCS1v15) signing is deterministic for identical claims+iat-second — two access tokens issued within the same wall-clock second with unchanged claims are byte-identical; rotation is proven by the refresh token (always fresh random bytes), not by asserting access-token inequality"

key-files:
  created:
    - services/gateway/internal/adapters/http/auth.go
    - services/gateway/internal/adapters/http/auth_test.go
    - services/identity/internal/app/bootstrap.go
    - services/identity/cmd/sessions_integration_test.go
    - services/identity/cmd/bootstrap_integration_test.go
    - deploy/auth-roundtrip.sh
  modified:
    - pkg/httpx/errors.go, pkg/httpx/errors_test.go
    - proto/services/identity/v1/auth.proto, gen/go/identity/v1/**, gen/ts/services/identity/v1/auth_pb.ts
    - services/identity/internal/app/session.go
    - services/identity/internal/adapters/http/routes.go
    - services/identity/internal/adapters/postgres/queries/{sessions,users}.sql + generated code
    - services/identity/internal/domain/errors.go
    - services/identity/cmd/main.go, services/identity/cmd/main_integration_test.go
    - services/gateway/cmd/main.go, services/gateway/go.mod
    - deploy/kong/kong.yml.tmpl, deploy/docker-compose.yml, .env.example, Makefile
    - services/identity/CLAUDE.md, services/gateway/CLAUDE.md

key-decisions:
  - "Kong's global cors plugin origins list gained http://localhost:3002 directly (not a second per-route plugin) — one shared allow-list for both the customer and admin apps, matching the plan's D-18 requirement with the smallest diff"
  - "Task 2's TDD RED phase included the proto regen + generated Go/TS bindings alongside the failing tests — the interface contract, not behavior, so it doesn't blur the test/feat commit-scope boundary; sqlc queries and the domain.ErrSessionInvalid sentinel were deferred to the GREEN commit since nothing in RED referenced them"
  - "EnsureSuperAdmin's promote-existing-row path (xmax != 0) intentionally builds no identity.UserCreated event — only a brand-new insert announces the identity; a promotion's next refresh re-reads the new role via D-10, so nothing downstream needs the event"

requirements-completed: [AUTH-03]  # AUTH-01/AUTH-02 also declared by sibling plans (02-07, 02-09, 02-10, 02-13) without a SUMMARY yet — requirements.ready-ids reported 1/3 ready (AUTH-03 only); the last plan to finish AUTH-01/02 marks them

coverage:
  - id: D1
    description: "Browser OTP login through Kong's api-auth route (no JWT plugin, 20/min rate limit, CORS from localhost:3001/3002): otp/request -> otp/verify with the real code read from Mailpit sets httpOnly/Secure/SameSite=Lax access_token+refresh_token cookies and returns {userId, role, operatorId, pierIds} with no token in the body"
    requirement: AUTH-01
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestOtpVerifySetsCookiesAndBodyHasNoTokens"
        status: pass
      - kind: e2e
        ref: "deploy/auth-roundtrip.sh checks otp-request-200, mailpit-code, verify-200-cookies, whoami-customer (make auth-roundtrip)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Wrong-code, cooldown/hourly, and unmapped-code error shapes: 400 {code:invalid_argument, attemptsLeft:n} on a wrong code, 429 on cooldown/hourly, pkg/httpx maps ResourceExhausted->429 and FailedPrecondition->400 with the message preserved"
    requirement: AUTH-01
    verification:
      - kind: unit
        ref: "pkg/httpx/errors_test.go#TestWriteErrorMapsResourceExhaustedTo429"
        status: pass
      - kind: unit
        ref: "pkg/httpx/errors_test.go#TestWriteErrorMapsFailedPreconditionTo400"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestOtpVerifyWrongCodeReturnsAttemptsLeft"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestOtpRequestResourceExhaustedReturns429"
        status: pass
      - kind: e2e
        ref: "deploy/auth-roundtrip.sh checks otp-cooldown-429, verify-wrong-400 (make auth-roundtrip)"
        status: pass
    human_judgment: false
  - id: D3
    description: "No client-supplied claim header ever reaches identity through the auth routes, and an oversized body is rejected before any upstream contact"
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestOtpRequestSpoofedClaimHeaderNeverReachesIdentity"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestOtpRequestBodyTooLargeReturns400"
        status: pass
    human_judgment: false
  - id: D4
    description: "Refresh(A) rotates to a fresh refresh token, revokes A, and re-reads the user's current role/operator_id/pier_ids on every call (D-10); a role/pier change reaches the very next access token"
    requirement: AUTH-01
    verification:
      - kind: integration
        ref: "services/identity/cmd/sessions_integration_test.go#TestRefreshRotationAndReuseDetection"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/sessions_integration_test.go#TestRefreshRereadsClaimsAndDisabled"
        status: pass
      - kind: e2e
        ref: "deploy/auth-roundtrip.sh check refresh-rotates (make auth-roundtrip)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Replaying an already-rotated refresh token revokes the whole session family (both the replayed token and the one issued after it are rejected); an expired or unknown token is rejected; a disabled user's refresh is rejected"
    requirement: AUTH-01
    verification:
      - kind: integration
        ref: "services/identity/cmd/sessions_integration_test.go#TestRefreshRotationAndReuseDetection"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/sessions_integration_test.go#TestRefreshExpired"
        status: pass
      - kind: integration
        ref: "services/identity/cmd/sessions_integration_test.go#TestRefreshRereadsClaimsAndDisabled"
        status: pass
      - kind: e2e
        ref: "deploy/auth-roundtrip.sh checks refresh-reuse-401, refresh-after-reuse-401 (make auth-roundtrip)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Gateway refresh/logout cookie shaping: missing/invalid refresh cookie or any identity error clears both cookies; logout always returns 204 and clears cookies even when identity errors"
    requirement: AUTH-01
    verification:
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestRefreshMissingCookieReturns401AndClearsCookies"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestRefreshIdentityErrorClearsCookies"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestLogoutClearsCookiesAndReturns204"
        status: pass
      - kind: unit
        ref: "services/gateway/internal/adapters/http/auth_test.go#TestLogoutClearsCookiesEvenOnIdentityError"
        status: pass
      - kind: e2e
        ref: "deploy/auth-roundtrip.sh checks logout-204, refresh-after-logout-401 (make auth-roundtrip)"
        status: pass
    human_judgment: false
  - id: D7
    description: "identity startup bootstrap (D-09): SUPER_ADMIN_EMAIL becomes a non-disabled super_admin idempotently — a repeat call, a pre-existing non-super-admin row, and a disabled super_admin all converge to the same state with at most one identity.UserCreated ever published"
    requirement: AUTH-02
    verification:
      - kind: integration
        ref: "services/identity/cmd/bootstrap_integration_test.go#TestEnsureSuperAdminIdempotent"
        status: pass
    human_judgment: false
  - id: D8
    description: "Logging in through Kong with SUPER_ADMIN_EMAIL yields a whoami with role super_admin and empty pier_ids"
    requirement: AUTH-02
    verification:
      - kind: integration
        ref: "services/identity/cmd/bootstrap_integration_test.go#TestSuperAdminLoginViaOtp"
        status: pass
      - kind: e2e
        ref: "deploy/auth-roundtrip.sh check super-admin-login (make auth-roundtrip)"
        status: pass
    human_judgment: false

# Metrics
duration: 30min
completed: 2026-09-27
status: complete
---

# Phase 2 Plan 5: Browser Cookie Sessions — OTP Login, Refresh Rotation, Super-Admin Bootstrap Summary

**Gateway cookie routes (`otp/request`, `otp/verify`, `refresh`, `logout`) behind a dedicated unauthenticated + rate-limited Kong route, identity's `Refresh`/`Logout` RPCs with single-use rotation + whole-family reuse detection + D-10 claim re-read, and a `SUPER_ADMIN_EMAIL` startup bootstrap — all proven end-to-end by a script reading real OTP codes from Mailpit.**

## Performance

- **Duration:** 30 min
- **Started:** 2026-09-27T00:55:00+07:00
- **Completed:** 2026-09-27T01:22:00+07:00
- **Tasks:** 3 (1 tracer, 1 TDD, 1 auto)
- **Files modified:** 27 (6 created, 21 modified)

## Accomplishments

- `services/gateway/internal/adapters/http/auth.go`: `AuthRoutes` registers `POST /api/v1/auth/{otp/request,otp/verify,refresh,logout}`. Every outbound call to identity carries only `X-Internal-Token`, built from scratch (never copies an inbound header/cookie). Success sets `access_token`/`refresh_token` as httpOnly/Secure/SameSite=Lax cookies and returns `{userId, role, operatorId, pierIds}` — never a token in the body. A wrong-code `InvalidArgument` surfaces its `Attempts-Left` connect metadata as `attemptsLeft` in the JSON error.
- `deploy/kong/kong.yml.tmpl`: new `api-auth` route (`paths: ["/api/v1/auth"]`, no `jwt` plugin, route-level `rate-limiting` 20/min overriding the global 120/min for these paths only); the global `cors` plugin's origin allow-list now includes `http://localhost:3002` (admin app) alongside `3001`.
- `pkg/httpx.WriteError` maps `ResourceExhausted`→429 and `FailedPrecondition`→400, message preserved.
- identity `AuthService.Refresh`/`Logout` RPCs (new proto messages, regenerated Go/TS bindings): `Refresh` locks the presented token's row (`for update`), revokes it, re-reads the user's current role/operator_id/pier_ids/disabled state every time (D-10), and on replay of an already-rotated token revokes every refresh token for that user inside the same transaction before returning the invalidation — a stolen-then-replayed token kills the whole session family. `Logout` is an idempotent revoke-by-hash.
- identity `EnsureSuperAdmin` (D-09): called from `cmd/main.go` after the pool is created and before the HTTP server starts; idempotently makes `SUPER_ADMIN_EMAIL` a non-disabled `super_admin` — insert (+ `identity.UserCreated`), or promote/re-enable an existing row with no duplicate event. No API or CLI path can mint a `super_admin` any other way.
- `deploy/auth-roundtrip.sh` + `make auth-roundtrip`: 13 PASS/FAIL checks proving the whole flow live through Kong — CORS preflight from the admin origin, OTP request/cooldown, the real code from Mailpit, a wrong-code `attemptsLeft`, cookie shape, refresh rotation + reuse detection (both directions), logout, and the `SUPER_ADMIN_EMAIL` login.

## Task Commits

Each task was committed atomically (Task 2's TDD cycle produced RED then GREEN):

1. **Task 1 (tracer): Browser login through the edge** - `5d29d6c` (feat)
2. **Task 2 (tdd) RED: failing tests for refresh rotation, reuse detection, logout** - `aa5128a` (test)
2. **Task 2 (tdd) GREEN: refresh rotation, D-10 re-read, logout implementation** - `cdf1320` (feat)
3. **Task 3 (auto): super_admin bootstrap (D-09)** - `2c2a887` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `services/gateway/internal/adapters/http/auth.go` / `auth_test.go` — `AuthRoutes`, cookie shaping, error mapping
- `pkg/httpx/errors.go` / `errors_test.go` — ResourceExhausted/FailedPrecondition status mapping
- `proto/services/identity/v1/auth.proto` + `gen/go/identity/v1/**` + `gen/ts/services/identity/v1/auth_pb.ts` — Refresh/Logout RPCs
- `services/identity/internal/app/session.go` — `Auth.Refresh`, `Auth.Logout`
- `services/identity/internal/app/bootstrap.go` — `EnsureSuperAdmin`
- `services/identity/internal/adapters/http/routes.go` — Refresh/Logout handlers, `ErrSessionInvalid` mapping
- `services/identity/internal/adapters/postgres/queries/{sessions,users}.sql` + generated code — `GetRefreshTokenForUpdate`, `RevokeRefreshToken`, `RevokeAllRefreshTokens`, `RevokeRefreshTokenByHash`, `UpsertSuperAdmin`
- `services/identity/internal/domain/errors.go` — `ErrSessionInvalid`
- `services/identity/cmd/sessions_integration_test.go`, `bootstrap_integration_test.go` — new integration suites
- `services/identity/cmd/main.go` — `EnsureSuperAdmin` call before `ListenAndServe`
- `services/gateway/cmd/main.go` — identity `AuthServiceClient` wiring
- `deploy/kong/kong.yml.tmpl`, `deploy/docker-compose.yml`, `.env.example`, `Makefile` — `api-auth` route, `SUPER_ADMIN_EMAIL`, `auth-roundtrip` target
- `deploy/auth-roundtrip.sh` — end-to-end proof script
- `services/identity/CLAUDE.md`, `services/gateway/CLAUDE.md` — docs

## Decisions Made

- Kong's global `cors` plugin gained `http://localhost:3002` directly rather than a second route-scoped CORS plugin — one shared allow-list, smallest diff satisfying D-18.
- Task 2's RED commit carries the proto regen + generated bindings alongside the failing tests (interface contract, not behavior); the sqlc queries and `domain.ErrSessionInvalid` sentinel moved to the GREEN commit since RED never referenced them.
- `EnsureSuperAdmin`'s promote-existing-row path deliberately builds no `identity.UserCreated` event — only a fresh insert announces the identity; a promotion's next refresh re-reads the new role via D-10.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test assertion required byte-inequality of two RS256 access tokens issued in the same wall-clock second**
- **Found during:** Task 2 GREEN verification (`TestRefreshRotationAndReuseDetection`)
- **Issue:** The test asserted `refreshed.AccessToken != initial.AccessToken`. RS256 (PKCS1v15) signing is deterministic for an identical header+payload, and both tokens' claims (plus `iat` truncated to the second) were identical — the two tokens were legitimately byte-identical, not a session.go bug.
- **Fix:** Removed the over-strict access-token-inequality assertion; kept the refresh-token-inequality assertion (the actual rotation proof) and a comment explaining why. `TestRefreshRereadsClaimsAndDisabled` already proves the access token DOES change once claims change.
- **Files modified:** `services/identity/cmd/sessions_integration_test.go`
- **Verification:** `TestRefreshRotationAndReuseDetection` passes
- **Committed in:** `cdf1320` (Task 2 GREEN commit)

**2. [Rule 1 - Bug] Pre-existing test's global outbox count broke once the D-09 bootstrap started publishing its own event**
- **Found during:** Task 3 full-suite regression run
- **Issue:** `TestExistingStaffLoginGetsClaims` (from plan 02-02) asserted `count(*) from outbox where event_type = 'identity.UserCreated'` is 0 after an existing user logs in. Since `cmd/main.go` now bootstraps `SUPER_ADMIN_EMAIL` at every startup — including this test's own fresh DB — that bootstrap legitimately publishes one `identity.UserCreated`, making the global count 1.
- **Fix:** Rescoped the query to `and aggregate_id = $2` (the seeded staff user's own id) — the test's actual intent ("this login didn't publish") survives; the bootstrap's own unrelated event no longer collides with it.
- **Files modified:** `services/identity/cmd/main_integration_test.go`
- **Verification:** Full `services/identity/cmd` integration suite (14 tests) passes
- **Committed in:** `2c2a887` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 bugs, both in test code, no production logic changed)
**Impact on plan:** Both fixes correct test assertions that were either overly strict (deterministic RS256 signing) or made stale by this same plan's own new behavior (the D-09 bootstrap). No scope creep.

## Issues Encountered

- The Kong container did not pick up the rendered `kong.yml`'s new `api-auth` route on the first `make auth-roundtrip` run after editing `kong.yml.tmpl`, because the running `kong-1` container (left over from an earlier session) was not recreated by `docker compose up` — only its declarative config file changed on the bind mount, which Kong (no admin API, DB-less) only re-reads at process start. A manual `docker restart boatbooking-kong-1` picked up the new route immediately. Not a code defect — a fresh `make up` from a stopped stack would have created the container with the correct config from the start; this only surfaced because of iterative same-session testing.

## User Setup Required

None — no external service configuration required. `make up` rebuilt the `gateway` and `identity` images across all three tasks; all services report healthy.

## Next Phase Readiness

- AUTH-01 and AUTH-02 are NOT yet marked complete in REQUIREMENTS.md — `requirements.ready-ids` reported 1/3 ready (AUTH-03 only, now marked); sibling plans 02-07, 02-09, 02-10, 02-13 also declare AUTH-01/02 and haven't produced a SUMMARY yet. The last one to finish marks them.
- The full cookie-session lifecycle (login, refresh, logout, super-admin bootstrap) is complete and independently verified — 02-11 (any UI needing session issuance) can build directly on these routes with no further gateway/identity trust-boundary changes.
- No blockers. `go build`/`go test` are green across the whole workspace (workspace mode and standalone `GOWORK=off` for gateway/identity), `make proto-check`/`make sqlc-gen` are idempotent, the full identity integration suite (14 tests) and gateway unit suite pass live, and `make auth-roundtrip` (13/13) + `make kong-roundtrip` (12/12) both pass against the live stack.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 6 created files verified present on disk (`[ -f ]`): `auth.go`, `auth_test.go`, `bootstrap.go`, `sessions_integration_test.go`, `bootstrap_integration_test.go`, `deploy/auth-roundtrip.sh`.
- All 4 task commits (`5d29d6c`, `aa5128a`, `cdf1320`, `2c2a887`) verified present in `git log --oneline --all`.
- All three tasks' acceptance criteria re-run and confirmed: `kong.yml.tmpl` contains `name: api-auth`, `paths: ["/api/v1/auth"]`, `http://localhost:3002`; `pkg/httpx/errors.go` contains `connect.CodeResourceExhausted`/`connect.CodeFailedPrecondition`; `session.go` contains `RevokeAllRefreshTokens`/`clock.Now()`; `users.sql` contains `(xmax = 0) as inserted`; `.env.example` contains `SUPER_ADMIN_EMAIL=admin@boatbooking.local`.
- Plan-level `<verification>` re-run clean: `go test -count=1` green across `pkg/httpx` and `services/gateway`; `go test -tags=integration` green across all 14 `services/identity/cmd` tests; `make auth-roundtrip` prints 13/13 PASS (no FAIL); `make kong-roundtrip` prints 12/12 PASS (no regression).
