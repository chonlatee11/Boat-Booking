---
phase: 02-identity-catalog
fixed_at: 2026-09-27T04:18:22Z
review_path: .planning/phases/02-identity-catalog/02-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 1
skipped: 1
status: partial
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-09-27T04:18:22Z
**Source review:** .planning/phases/02-identity-catalog/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2
- Fixed: 1
- Skipped: 1

## Fixed Issues

### WR-01 (new): WR-09's redaction logging drops trace_id/span_id

**Files modified:** `services/identity/internal/adapters/http/routes.go`, `services/identity/internal/adapters/http/routes_test.go`, `services/identity/internal/adapters/http/users.go`, `services/identity/internal/adapters/http/users_test.go`, `services/catalog/internal/adapters/http/scope.go`, `services/catalog/internal/adapters/http/scope_test.go`, `services/catalog/internal/adapters/http/operators.go`, `services/catalog/internal/adapters/http/piers.go`, `services/catalog/internal/adapters/http/routes.go`, `services/catalog/internal/adapters/http/route_handlers.go`, `services/catalog/internal/adapters/http/prices.go`
**Commit:** `8dda7a5`
**Applied fix:** Threaded `ctx context.Context` through `mapAuthError`, `mapUserError`, and `toConnectErr`, and switched their default-branch log calls from package-level `slog.Error` to `slog.ErrorContext(ctx, ...)`. Updated every call site (identity's `RequestOtp`/`VerifyOtp`/`Refresh`/`Logout` and `UpsertUser`/`ListUsers`/`SetUserDisabled`; catalog's operator/pier/route/price/boat handlers) to pass the request `ctx`, which already carries the OTel span started by `otelconnect.NewInterceptor()`. The three redaction unit tests (`TestMapAuthErrorRedactsUnmappedErrors`, `TestMapUserErrorRedactsUnmappedErrors`, `TestToConnectErrRedactsUnmappedErrors`) were updated to pass `context.Background()` — no behavioral assertions changed. `go build ./...` and `go test ./...` (unit only) pass clean for both `services/identity` and `services/catalog`.

## Skipped Issues

### IN-01 (new): CR-01's atomic-attempt fix creates and immediately deletes a throwaway Valkey key on wrong-destination/expired verify

**File:** `services/identity/internal/app/otp.go:153-167`
**Reason:** The review's own suggested fix (check `EXISTS`/`HGetAll` before the atomic `TxPipelined` block, only incrementing when the key exists) is not a small, isolated change — it adds a second Redis round trip to *every* `VerifyOtp` call, including correct guesses, not just the wrong-destination/expired edge case it targets. It also only partially closes the gap: a key deleted concurrently by another goroutine's winning guess between the pre-check and the atomic block would still hit the same throwaway-create path, so the added complexity doesn't fully eliminate the behavior it's meant to fix. The review itself classifies this as "Not required for v1" and "functionally harmless." Per the fix-scope instruction (only apply if small and safe without weakening CR-01's atomicity) and the project's YAGNI/ponytail convention, this is deferred rather than applied.
**Original issue:** When the Valkey `otp:code:{dh}` key doesn't exist (wrong destination, already-consumed code, or genuine expiry), `HIncrBy` inside the `TxPipelined` block still creates a hash with only field `a` as a side effect, which the next line detects and cleans up with `Del` — an extra round trip on a path now hit on every wrong-destination/expired-code verify, not just the concurrent-lockout edge case the original IN-01 was about.

---

_Fixed: 2026-09-27T04:18:22Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
