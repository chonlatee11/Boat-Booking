---
phase: 02-identity-catalog
fixed_at: 2026-09-27T08:35:00Z
review_path: .planning/phases/02-identity-catalog/02-REVIEW.md
iteration: 1
findings_in_scope: 20
fixed: 17
skipped: 3
status: partial
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-09-27T08:35:00Z
**Source review:** .planning/phases/02-identity-catalog/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 20 (CR-01..02, WR-01..09, IN-01..09 — `fix_scope: all`)
- Fixed: 17
- Skipped: 3 (WR-04, IN-03, IN-06 — each requires a design/product decision, not a bug with one correct answer)

**Verification:** every Go fix was built (`go build ./...`), gofmt-checked, `golangci-lint run ./...` (0 issues) on each touched module (catalog, identity, gateway), and re-verified against the module's existing `go test ./...` plus `go test -tags integration ./cmd/...` (real Postgres+Redpanda testcontainers) — all passing, including new regression tests added for CR-01, CR-02, WR-01, WR-02, WR-03, WR-06, WR-09. Every TypeScript fix was checked with `npm run typecheck` and `npm run lint` in the affected app (`apps/admin`, `apps/web`) — both clean. `deploy/docker-compose.yml`'s shell change was verified with `sh -n`.

## Fixed Issues

### CR-01: OTP attempt limit is check-then-increment, so concurrent guesses bypass the 5-attempt lockout

**Files modified:** `services/identity/internal/app/otp.go`, `services/identity/cmd/main_integration_test.go`
**Commit:** `c0077d8`
**Applied fix:** `VerifyOtp` now runs `HGetAll` and `HIncrBy` in one Redis transaction pipeline, so every verify attempt (right or wrong) consumes an attempt before the code is compared — a request past the 5-attempt cap is rejected without ever reaching the hash comparison, closing the concurrent-guess race. Also fixes IN-01 (a wrong guess recreating an expired key with no TTL) as the same `stored["h"] == ""` check covers both. Added `TestOtpAttemptsLockoutConcurrent`: 20 parallel wrong guesses against one code, asserting no more than 5 ever get compared and the code stays locked afterward.

### CR-02: UpdateBoat moves `home_pier_id` without updating `operator_id`, breaking tenant ownership

**Files modified:** `services/catalog/internal/adapters/postgres/queries/boats.sql`, `services/catalog/internal/adapters/postgres/boats.sql.go` (regenerated via `make sqlc-gen`), `services/catalog/internal/app/boat.go`, `services/catalog/cmd/main_integration_test.go`
**Commit:** `ddaf8b5`
**Applied fix:** `UpdateBoat` now also writes `operator_id` (already correctly derived from the new home pier by `UpsertBoat` before it calls `updateBoat`). Added `TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID`: a super_admin move updates `operator_id`, the boat leaves the old operator's `ListBoats` and appears in the new operator's, who can then edit it.

### WR-01: Login CSRF — `/api/v1/auth/otp/verify` accepts any Content-Type

**Files modified:** `services/gateway/internal/adapters/http/auth.go`, `services/gateway/internal/adapters/http/auth_test.go`
**Commit:** `b3401cf`
**Applied fix:** `decodeAuthBody` now requires `Content-Type: application/json` (same check `adminProxy` already had), rejecting anything else before the body is even read. Added `TestOtpVerifyRejectsNonJSONContentType` reproducing the review's exact `text/plain` attack shape.

### WR-02: UpsertRoute update checks `pier_from` from the request but ignores it

**Files modified:** `services/catalog/internal/app/route.go`, `services/catalog/cmd/routes_prices_integration_test.go`, `apps/admin/src/app/(admin)/routes/route-sheet.tsx`
**Commit:** `98da5af`
**Applied fix:** `updateRoute` now rejects a request whose `pier_from_id` differs from the stored route's with `InvalidArgument`, before ever reaching `UpdateRoute`. The admin route-sheet's origin `<select>` is now disabled in edit mode. Added `TestUpdateRouteRejectsChangedPierFrom` covering both the silent-no-op and the unhandled-500 (self-loop) shapes from the review.

### WR-03: Editing a route without a cancellation policy silently resets it to the default

**Files modified:** `services/catalog/internal/app/route.go`, `services/catalog/cmd/routes_prices_integration_test.go`
**Commit:** `5273e8d`
**Applied fix:** The D-13 default now applies only on create. On update, an empty policy in the request means "keep the stored schedule" — `updateRoute` falls back to the stored row's raw jsonb bytes when the request's policy is empty. Added `TestUpdateRouteWithoutPolicyKeepsStoredPolicy`.

### WR-05: Gateway clears session cookies on any identity error, including transient outages

**Files modified:** `services/gateway/internal/adapters/http/auth.go`, `services/gateway/internal/adapters/http/auth_test.go`
**Commit:** `5e593e2`
**Applied fix:** `refreshHandler` now clears cookies only when `connect.CodeOf(err) == connect.CodeUnauthenticated`; every other error (Unavailable, DeadlineExceeded, Internal) is returned with the cookies left untouched. Added `TestRefreshTransientIdentityErrorKeepsCookies`.

### WR-06: A pier can be created under an operator that is being archived at the same moment

**Files modified:** `services/catalog/internal/adapters/postgres/queries/operators.sql`, `services/catalog/internal/adapters/postgres/operators.sql.go` (regenerated), `services/catalog/internal/app/operator.go`, `services/catalog/internal/app/pier.go`, `services/catalog/cmd/operators_piers_integration_test.go`
**Commit:** `a5b0928`
**Applied fix:** Added `GetOperatorForShare`/`GetOperatorForUpdate` queries, mirroring the existing FOR SHARE/FOR UPDATE pattern for piers/routes. `createPier` takes FOR SHARE on the operator; `ArchiveOperator` takes FOR UPDATE before counting active piers — the two now serialize instead of racing under READ COMMITTED. Added `TestArchiveOperatorRaceWithPierCreate` (verified across 5 repeated runs: one side always loses, never both succeed).

### WR-07: Two spellings of one Thai number become two destinations

**Files modified:** `services/identity/internal/domain/destination.go`, `services/identity/internal/domain/destination_test.go`
**Commit:** `de11490`
**Applied fix:** `normalizePhone`'s `+` branch now collapses a `+660` trunk-zero prefix to `+66` before extracting digits, so `"0812345678"`, `"+66812345678"`, and `"+660812345678"` all normalise to the same destination. Added table tests for the trunk-zero spelling with and without formatting characters.

### WR-08: The staff edit dialog cannot be saved once an assigned pier has been archived

**Files modified:** `apps/admin/src/app/(admin)/staff/staff-dialog.tsx`
**Commit:** `eaa3176`
**Applied fix:** A derived `selectedPierIds` (raw `pierIds` intersected with the operator's current non-archived piers) is used for validation, checkbox rendering, and submission instead of the raw state — computed fresh each render, not synced via an effect, so a legitimate pier can't flash unchecked while its own fetch is in flight. No server-side change needed: `validatePiers` already only checks the ids actually submitted.

### WR-09: Internal error details leak to browsers through the gateway proxies

**Files modified:** `services/catalog/internal/adapters/http/scope.go`, `services/catalog/internal/adapters/http/scope_test.go` (new), `services/catalog/cmd/main.go`, `services/identity/internal/adapters/http/routes.go`, `services/identity/internal/adapters/http/routes_test.go` (new), `services/identity/internal/adapters/http/users.go`, `services/identity/internal/adapters/http/users_test.go` (new), `services/identity/cmd/main.go`
**Commit:** `3d8d641`
**Applied fix:** Redacted at the source per the review's primary suggestion: `toConnectErr`/`mapAuthError`/`mapUserError`'s `default` branch now logs the real error and returns `connect.NewError(CodeInternal, errors.New("internal error"))` instead of wrapping the original error — the gateway's proxies then have nothing sensitive left to forward, no proxy change needed. Each service's `main.go` now calls `slog.SetDefault(log)` so that log line goes through the project's JSON+trace_id logger (D-52/D-53) instead of stdlib's bare default. Added one unit test per mapping function proving an unmapped error's message never reaches the returned `connect.Error`.

### IN-02: ListRoutePrices takes a row lock on a read-only path

**Files modified:** `services/catalog/internal/adapters/postgres/queries/routes.sql`, `services/catalog/internal/adapters/postgres/routes.sql.go` (regenerated), `services/catalog/internal/app/price.go`
**Commit:** `087f38e`
**Applied fix:** Added `GetRouteScoped` (the same scope check without `FOR UPDATE`) and switched `ListRoutePrices` to it.

### IN-04: OTP login UIs show misleading error messages (partial)

**Files modified:** `apps/admin/src/components/otp-login.tsx`, `apps/web/src/components/otp-login.tsx`, `apps/web/messages/en.json`, `apps/web/messages/th.json`
**Commit:** `776371d`
**Applied fix:** Both apps now branch on `attemptsLeft !== undefined` instead of defaulting a missing value to 0, so a malformed destination on the request step shows a distinct invalid-destination message instead of "wrong code (0 attempts left)".
**Not fixed (documented, not silently dropped):** the 429/hourly-cap-vs-cooldown-vs-Kong-per-IP-limit distinction and the failed_precondition code-expired-vs-delivery-unavailable distinction both need a new distinguishing signal on the wire (a design decision the review flags but doesn't specify — "have the gateway pass through a distinguishing code"). Left as-is pending that decision; see Skipped Issues note style below (recorded here rather than in Skipped Issues because part of this finding *was* fixed).

### IN-05: identity dereferences a nil pool when DATABASE_URL is unset

**Files modified:** `services/identity/cmd/main.go`
**Commit:** `41ed612`
**Applied fix:** `databaseURL` now comes from `httpx.MustEnv("DATABASE_URL")` (matching the existing `INTERNAL_TOKEN` pattern in the same function), failing fast at startup with a clear message instead of a nil-pointer panic inside `WithTx`.

### IN-07: The pier Sheet enables Save for inputs the server will reject

**Files modified:** `apps/admin/src/app/(admin)/piers/pier-sheet.tsx`
**Commit:** `91fa398`
**Applied fix:** `isValid` now also requires `operatorId` when a super_admin is creating (`operatorValid`) and `opensAt < closesAt` when both are set (`hoursOutOfOrder`), each with its own inline `FieldError`, matching the existing name-field validation pattern in the same Sheet.

### IN-08: `eval` on a string containing secrets in storage-init

**Files modified:** `deploy/docker-compose.yml`
**Commit:** `8392f5c`
**Applied fix:** Replaced the `configure_cmd`-string-then-`eval` pattern with a plain shell function (`configure()`) the retry loop calls directly, exactly as the review suggested. Dev-only container; verified with `sh -n`.

### IN-09: The re-enable failure toast says "disable failed"

**Files modified:** `apps/admin/src/app/(admin)/staff/page.tsx`
**Commit:** `a9abcf7`
**Applied fix:** `handleSetDisabled`'s fallback error text now picks from the `disabled` argument actually passed, instead of always showing the disable-failure text.

## Skipped Issues

### WR-04: Refresh-token reuse detection logs users out on a normal multi-tab refresh race

**File:** `services/identity/internal/app/session.go:98-106`
**Reason:** The review's suggested fix (a ~30s grace window before treating a revoked-token replay as a compromise signal) is a genuine security/UX tradeoff, not a bug with one correct answer. A pure timestamp-based grace window cannot distinguish "an attacker replaying a stolen token 1ms after legitimate rotation" from "the browser's own second tab racing 1ms after its sibling tab's rotation" — both look identical to the server (same token, revoked moments ago). Implementing it would directly conflict with the existing `TestRefreshRotationAndReuseDetection` integration test, which deliberately asserts that even an *immediate* replay of a just-rotated token revokes the whole session family (T-02-05-02's documented threat model). Loosening that guarantee for UX is a product/security policy decision that needs an explicit call, not a fixer-agent judgment call. The review's own alternative (client-side cross-tab coordination via `BroadcastChannel`/`navigator.locks`) is also explicitly incomplete per the review itself ("does not cover web and admin together").

### IN-03: `photo_key` is not tied to the caller or to an actual upload

**File:** `services/catalog/internal/domain/pier.go:62-64`; `services/catalog/internal/app/photo.go:48-77`
**Reason:** The review's own Fix section states "This is acceptable for v1. If needed, record issued keys per operator..." — explicitly deferred by the reviewer, not a defect requiring action now.

### IN-06: No event is published for user promotion, update or disable

**File:** `services/identity/internal/app/users.go:122-137, 171-257`
**Reason:** Matches the fixer's explicit skip criterion from the dispatch instructions almost verbatim ("IN-06 adding new events may need proto changes — do it only if small and consistent with existing outbox pattern; otherwise skip with reason"). The review's own Fix section defers it too: "When a consumer needs it, add `identity.UserUpdated`/`UserDisabled`..." There is no consumer today (identity's own `CLAUDE.md` says "Nothing yet" for every downstream service), so adding unused event types and outbox wiring now would be speculative (YAGNI) ahead of an actual proto-schema decision about event shape/versioning.

---

_Fixed: 2026-09-27T08:35:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
