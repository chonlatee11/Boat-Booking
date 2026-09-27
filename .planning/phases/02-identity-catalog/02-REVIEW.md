---
phase: 02-identity-catalog
reviewed: 2026-09-27T09:00:00Z
depth: standard
files_reviewed: 34
files_reviewed_list:
  - apps/admin/src/app/(admin)/piers/pier-sheet.tsx
  - apps/admin/src/app/(admin)/routes/route-sheet.tsx
  - apps/admin/src/app/(admin)/staff/page.tsx
  - apps/admin/src/app/(admin)/staff/staff-dialog.tsx
  - apps/admin/src/components/otp-login.tsx
  - apps/web/messages/en.json
  - apps/web/messages/th.json
  - apps/web/src/components/otp-login.tsx
  - deploy/docker-compose.yml
  - services/catalog/cmd/main.go
  - services/catalog/cmd/main_integration_test.go
  - services/catalog/cmd/operators_piers_integration_test.go
  - services/catalog/cmd/routes_prices_integration_test.go
  - services/catalog/internal/adapters/http/scope.go
  - services/catalog/internal/adapters/http/scope_test.go
  - services/catalog/internal/adapters/postgres/boats.sql.go
  - services/catalog/internal/adapters/postgres/operators.sql.go
  - services/catalog/internal/adapters/postgres/queries/boats.sql
  - services/catalog/internal/adapters/postgres/queries/operators.sql
  - services/catalog/internal/adapters/postgres/queries/routes.sql
  - services/catalog/internal/adapters/postgres/routes.sql.go
  - services/catalog/internal/app/boat.go
  - services/catalog/internal/app/operator.go
  - services/catalog/internal/app/pier.go
  - services/catalog/internal/app/price.go
  - services/catalog/internal/app/route.go
  - services/gateway/internal/adapters/http/auth.go
  - services/gateway/internal/adapters/http/auth_test.go
  - services/identity/cmd/main.go
  - services/identity/cmd/main_integration_test.go
  - services/identity/internal/adapters/http/routes.go
  - services/identity/internal/adapters/http/routes_test.go
  - services/identity/internal/adapters/http/users.go
  - services/identity/internal/adapters/http/users_test.go
  - services/identity/internal/app/otp.go
  - services/identity/internal/domain/destination.go
  - services/identity/internal/domain/destination_test.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 02: Code Review Report (re-review after fix pass)

**Reviewed:** 2026-09-27T09:00:00Z
**Depth:** standard
**Files Reviewed:** 34
**Status:** issues_found

## Summary

This is a re-review of the fix pass applied to the 20 findings from the prior `02-REVIEW.md` (commits `c0077d8`..`a9abcf7`, see `02-REVIEW-FIX.md`). Scope is exactly the files touched by that fix pass. For every fix I read the diff against `ee4c14b`, then re-read the resulting file in full (not just the hunk) to trace the logic end to end and check for regressions.

**17 of 17 attempted fixes verified correct**, each traced against its stated failure mode:

- **CR-01** (OTP lockout race): `VerifyOtp` now does `HGetAll`+`HIncrBy` inside one `TxPipelined` (MULTI/EXEC), so concurrent guesses are serialized by Redis itself — attempt N's increment is visible to attempt N+1 before either compares a hash. Traced the boundary cases (expired-key stub recreation, exactly-5th-attempt correct guess, post-lockout correct guess) — all land where the fix report claims. The new `TestOtpAttemptsLockoutConcurrent` (20 parallel wrong guesses) is a real concurrency test, not a mock.
- **CR-02** (boat operator_id not following home_pier move): `UpdateBoat` now writes `operator_id` from `b.OperatorID`, which `UpsertBoat` derives from the pier's *stored* operator before either create or update path runs (`boat.go:67`) — confirmed the derivation happens on both create and update, and that only `super_admin` can actually trigger a cross-operator move (`pier_admin`'s scope is fixed to one operator's piers via `GetPierForShareScoped`). `TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID` proves the old operator loses visibility and the new one gains it.
- **WR-01/WR-05** (gateway CSRF / cookie-clear-on-transient-error): `decodeAuthBody` now mirrors `adminProxy`'s `application/json`-only check; `refreshHandler` clears cookies only on `connect.CodeOf(err) == CodeUnauthenticated`. Cross-checked every path in `session.go.Refresh` (missing token, expired, reused, disabled) — all consistently return `domain.ErrSessionInvalid` → `CodeUnauthenticated`, so the narrowed clear-cookie condition still fires for every real invalidation case, only sparing `Unavailable`/`DeadlineExceeded`.
- **WR-02/WR-03** (route pier_from mutability / cancellation policy reset): `updateRoute` now rejects a changed `pier_from_id` before it ever reaches `UpdateRoute`, and falls back to the stored raw jsonb bytes when the request's policy is empty. Traced both the silent-no-op and self-loop-500 cases from the original finding — both now return `InvalidArgument`.
- **WR-06** (operator archive vs. pier create race): `GetOperatorForShare`/`GetOperatorForUpdate` on the same operator row, used respectively by `createPier` and `ArchiveOperator` — `FOR SHARE` and `FOR UPDATE` on the same single row serialize under READ COMMITTED with no deadlock risk (one lock target, no ordering issue).
- **WR-07** (phone trunk-zero double destination): the `+660` → `+66` collapse only fires inside the `+` branch and only for the Thailand country code, which can never legitimately be followed by a literal subscriber `0` in E.164. Table-tested both with and without formatting characters.
- **WR-08** (staff dialog stuck on archived pier): `selectedPierIds` is derived fresh every render (intersection of `pierIds` state and the operator's current non-archived piers), not synced via effect, so it can't flash unchecked while `usePiersForOperator` is still loading. Used consistently for validation, checkbox `checked`, and submission.
- **WR-09** (unmapped errors leaking through proxies): `toConnectErr`/`mapAuthError`/`mapUserError`'s default branches now return `errors.New("internal error")` instead of the original error. Confirmed this is sufficient even for the admin/public proxies' raw byte-for-byte passthrough (`httputil.ReverseProxy`, `io.Copy`) since the redaction happens at the source before the body is ever built — see WR-01 (new) below for a side effect of *how* this was implemented.
- **IN-02/IN-04/IN-05/IN-07/IN-08/IN-09**: each independently verified against its stated repro (non-locking `GetRouteScoped` for reads, `attemptsLeft !== undefined` branching client-side matching the server only ever setting `Attempts-Left` for `CodeMismatchError`, `httpx.MustEnv("DATABASE_URL")` panicking at startup instead of a nil-pool panic in `WithTx`, pier Sheet's `operatorValid`/`hoursOutOfOrder` matching the backend's own `Validate()` rules exactly (including rejecting equal open/close, not just "both empty or reversed"), the `eval`-free `configure()` shell function, and the disable/enable toast picking its fallback text from the actual direction requested).

**One new issue found in this fix pass**, described below — the WR-09 redaction fix drops trace correlation from exactly the log lines it added.

## Warnings

### WR-01 (new): WR-09's redaction logging drops trace_id/span_id — the exact log line meant to diagnose a redacted 500 can't be correlated to its request

**File:** `services/identity/internal/adapters/http/routes.go:119`, `services/identity/internal/adapters/http/users.go:146`, `services/catalog/internal/adapters/http/scope.go:73`

**Issue:** All three new log calls use the package-level `slog.Error("...", "error", err)`, not `slog.ErrorContext(ctx, "...", "error", err)`. `mapAuthError`, `mapUserError`, and `toConnectErr` all take only `err error` — no `context.Context` parameter — even though every call site (`RequestOtp`, `VerifyOtp`, `Refresh`, `Logout`, and catalog's operator/pier/route/price/boat handlers) has a live request `ctx` available and passes it to nothing but the underlying `app.*` call.

`pkg/httpx/logger.go`'s `traceHandler.Handle(ctx, r)` attaches `trace_id`/`span_id` by reading `trace.SpanContextFromContext(ctx)` — it only works when the `ctx` actually carries the span. The package-level `slog.Error(...)` (as opposed to `Logger.ErrorContext`) calls the default logger with `context.Background()` internally (stdlib `log/slog` behavior), so `trace.SpanContextFromContext` always returns an invalid span context here, and the `trace_id`/`span_id` attributes are silently never attached. The same applies to the second fanout branch, `otelslog.NewHandler(service)`, whose entire purpose is span-correlated log export.

Every RPC handler's `ctx` here *does* carry a real span — `httpx.ConnectOtel()` wires `otelconnect.NewInterceptor()`, which starts a span per RPC before the handler runs — so this isn't a hypothetical; in production every one of these "internal error" log lines will print with no `trace_id`, which is precisely the field an operator needs to go from "a browser saw code=internal" to "which request, which trace, what upstream call failed." This directly undercuts the fix's own stated purpose (`main.go`'s comment: "route that log through the same JSON+trace_id logger") and the project's Phase-0 observability requirement (`trace_id` correlation across every hop).

Not classified as a security/correctness bug (the redaction itself — the actual security fix — works correctly), but it is a real regression in the fix pass: these are new log lines, and they ship already missing the one attribute that makes them useful for on-call debugging.

**Fix:** Thread `ctx` through the three mapping functions and use `ErrorContext`:
```go
func mapAuthError(ctx context.Context, err error) error {
    ...
    default:
        slog.ErrorContext(ctx, "identity: unmapped auth error", "error", err)
        return connect.NewError(connect.CodeInternal, errors.New("internal error"))
    }
}
```
and update every call site (`mapAuthError(ctx, err)`, `mapUserError(ctx, err)`, `toConnectErr(ctx, err)`) — `ctx` is already in scope at all of them.

## Info

### IN-01 (new): CR-01's atomic-attempt fix creates and immediately deletes a throwaway Valkey key on every verify against a never-issued/already-consumed code

**File:** `services/identity/internal/app/otp.go:153-167`

**Issue:** When `key` doesn't exist (wrong destination, code already consumed, or genuinely expired), `HIncrBy` inside the `TxPipelined` still creates a hash with only field `a` (no `h`, no TTL) as a side effect of incrementing a nonexistent key. The very next line detects `stored["h"] == ""` and issues a `Del` to clean it up. Functionally harmless (out of scope for this review per the performance carve-out) and already the same trade-off the fix report's own IN-01 note acknowledges for the "recreated stub" case — flagging only because it's an extra round trip on a path that's now hit on every wrong-destination/expired-code verify, not just the concurrent-lockout edge case the original IN-01 was about.

**Fix:** Not required for v1; if it matters later, check `EXISTS` before the pipeline, or accept a `WATCH`-free read: `HGetAll` first, and only take the `TxPipelined` increment path when the key exists.

---

_Reviewed: 2026-09-27T09:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
