---
phase: 02-identity-catalog
reviewed: 2026-09-26T21:39:42Z
depth: standard
files_reviewed: 139
files_reviewed_list:
  - .env.example
  - Jenkinsfile
  - Makefile
  - apps/admin/.env.example
  - apps/admin/next.config.ts
  - "apps/admin/src/app/(admin)/boats/boat-dialog.tsx"
  - "apps/admin/src/app/(admin)/boats/page.tsx"
  - "apps/admin/src/app/(admin)/boats/queries.ts"
  - "apps/admin/src/app/(admin)/layout.tsx"
  - "apps/admin/src/app/(admin)/operators/page.tsx"
  - "apps/admin/src/app/(admin)/page.tsx"
  - "apps/admin/src/app/(admin)/piers/page.tsx"
  - "apps/admin/src/app/(admin)/piers/pier-sheet.tsx"
  - "apps/admin/src/app/(admin)/piers/queries.ts"
  - "apps/admin/src/app/(admin)/routes/money.ts"
  - "apps/admin/src/app/(admin)/routes/page.tsx"
  - "apps/admin/src/app/(admin)/routes/policy-editor.tsx"
  - "apps/admin/src/app/(admin)/routes/price-section.tsx"
  - "apps/admin/src/app/(admin)/routes/queries.ts"
  - "apps/admin/src/app/(admin)/routes/route-sheet.tsx"
  - "apps/admin/src/app/(admin)/staff/page.tsx"
  - "apps/admin/src/app/(admin)/staff/queries.ts"
  - "apps/admin/src/app/(admin)/staff/staff-dialog.tsx"
  - apps/admin/src/app/layout.tsx
  - apps/admin/src/app/login/page.tsx
  - apps/admin/src/app/providers.tsx
  - apps/admin/src/components/admin-shell.tsx
  - apps/admin/src/components/archive-dialog.tsx
  - apps/admin/src/components/data-table.tsx
  - apps/admin/src/components/map-picker.tsx
  - apps/admin/src/components/operator-dialog.tsx
  - apps/admin/src/components/otp-login.tsx
  - apps/admin/src/components/photo-upload.tsx
  - apps/admin/src/lib/api.ts
  - apps/admin/src/lib/queries.ts
  - apps/admin/src/lib/utils.ts
  - "apps/web/src/app/[locale]/login/page.tsx"
  - "apps/web/src/app/[locale]/page.tsx"
  - apps/web/src/components/account-status.tsx
  - apps/web/src/components/otp-login.tsx
  - apps/web/src/lib/api.ts
  - deploy/auth-roundtrip.sh
  - deploy/docker-compose.yml
  - deploy/kong/kong.yml.tmpl
  - deploy/kong/roundtrip.sh
  - deploy/photo-roundtrip.sh
  - deploy/proof.sh
  - deploy/storage/s3-config.json
  - pkg/auth/auth.go
  - pkg/auth/auth_test.go
  - pkg/auth/cmd/devtoken/main.go
  - pkg/httpx/claims.go
  - pkg/httpx/claims_test.go
  - pkg/httpx/errors.go
  - pkg/httpx/errors_test.go
  - pkg/httpx/otel.go
  - pkg/testenv/images_test.go
  - pkg/testenv/testenv.go
  - proto/events/catalog/v1/boat.proto
  - proto/events/catalog/v1/operator.proto
  - proto/events/catalog/v1/pier.proto
  - proto/events/catalog/v1/price.proto
  - proto/events/catalog/v1/route.proto
  - proto/events/identity/v1/user.proto
  - proto/services/catalog/v1/catalog.proto
  - proto/services/identity/v1/auth.proto
  - proto/services/identity/v1/users.proto
  - services/catalog/cmd/boats_photo_integration_test.go
  - services/catalog/cmd/main.go
  - services/catalog/cmd/main_integration_test.go
  - services/catalog/cmd/operators_piers_integration_test.go
  - services/catalog/cmd/routes_prices_integration_test.go
  - services/catalog/internal/adapters/http/operators.go
  - services/catalog/internal/adapters/http/piers.go
  - services/catalog/internal/adapters/http/prices.go
  - services/catalog/internal/adapters/http/route_handlers.go
  - services/catalog/internal/adapters/http/routes.go
  - services/catalog/internal/adapters/http/scope.go
  - services/catalog/internal/adapters/postgres/queries/boats.sql
  - services/catalog/internal/adapters/postgres/queries/operators.sql
  - services/catalog/internal/adapters/postgres/queries/piers.sql
  - services/catalog/internal/adapters/postgres/queries/route_prices.sql
  - services/catalog/internal/adapters/postgres/queries/routes.sql
  - services/catalog/internal/app/boat.go
  - services/catalog/internal/app/operator.go
  - services/catalog/internal/app/photo.go
  - services/catalog/internal/app/photo_test.go
  - services/catalog/internal/app/pier.go
  - services/catalog/internal/app/price.go
  - services/catalog/internal/app/route.go
  - services/catalog/internal/app/scope.go
  - services/catalog/internal/domain/boat.go
  - services/catalog/internal/domain/errors.go
  - services/catalog/internal/domain/operator.go
  - services/catalog/internal/domain/pier.go
  - services/catalog/internal/domain/pier_test.go
  - services/catalog/internal/domain/price.go
  - services/catalog/internal/domain/price_test.go
  - services/catalog/internal/domain/route.go
  - services/catalog/internal/domain/route_test.go
  - services/catalog/migrations/00003_operators.sql
  - services/catalog/migrations/00004_piers.sql
  - services/catalog/migrations/00005_routes.sql
  - services/catalog/migrations/00006_route_prices.sql
  - services/catalog/migrations/00007_boats_home_pier.sql
  - services/catalog/migrations/00008_pier_photo.sql
  - services/gateway/cmd/main.go
  - services/gateway/internal/adapters/http/auth.go
  - services/gateway/internal/adapters/http/auth_test.go
  - services/gateway/internal/adapters/http/bff.go
  - services/gateway/internal/adapters/http/bff_test.go
  - services/gateway/internal/adapters/http/proxy.go
  - services/gateway/internal/adapters/http/proxy_test.go
  - services/identity/cmd/bootstrap_integration_test.go
  - services/identity/cmd/main.go
  - services/identity/cmd/main_integration_test.go
  - services/identity/cmd/sessions_integration_test.go
  - services/identity/cmd/users_integration_test.go
  - services/identity/internal/adapters/http/routes.go
  - services/identity/internal/adapters/http/users.go
  - services/identity/internal/adapters/kafka/handlers.go
  - services/identity/internal/adapters/notify/notify.go
  - services/identity/internal/adapters/notify/notify_test.go
  - services/identity/internal/adapters/postgres/queries/sessions.sql
  - services/identity/internal/adapters/postgres/queries/users.sql
  - services/identity/internal/adapters/postgres/sqlc.yaml
  - services/identity/internal/app/bootstrap.go
  - services/identity/internal/app/convert.go
  - services/identity/internal/app/otp.go
  - services/identity/internal/app/session.go
  - services/identity/internal/app/users.go
  - services/identity/internal/domain/destination.go
  - services/identity/internal/domain/destination_test.go
  - services/identity/internal/domain/errors.go
  - services/identity/internal/domain/user.go
  - services/identity/internal/domain/user_test.go
  - services/identity/migrations/00001_platform.sql
  - services/identity/migrations/00002_users.sql
  - services/identity/migrations/00003_refresh_tokens.sql
findings:
  critical: 2
  warning: 9
  info: 9
  total: 20
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-09-26T21:39:42Z
**Depth:** standard
**Files Reviewed:** 139
**Status:** issues_found

## Summary

Scope: identity (OTP login, sessions, staff users, bootstrap), gateway (auth cookie routes, admin/public proxies), catalog (operators/piers/routes/prices/boats/photos), shared `pkg/auth` + `pkg/httpx`, the Kong/compose wiring, and the admin/web Next.js clients.

Things that hold up:
- The trusted-header boundary works. `ForwardClaims` deletes every spoofable header, including `X-Pier-Ids`. `RequireInternal` checks the token in constant time.
- The admin proxy requires `application/json`, allow-lists services, and checks method shape.
- Refresh tokens are stored as sha256 hashes, rotated under `FOR UPDATE`, and reuse revokes the whole family.
- Scoped SQL always ANDs `operator_id` with `pier_ids`.
- The photo key regexp is enforced in both the domain layer and a DB check.
- Money uses int64 satang end to end, and the admin UI converts with BigInt.

The problems:
- **The OTP lockout can be beaten with concurrent requests.** The code is compared first and the attempt is counted afterwards, so parallel guesses all land before the 5-attempt cap applies.
- **A boat moved to another operator's pier keeps its old `operator_id`.** This breaks tenant ownership and puts a stale `operator_id` in `BoatUpserted`.
- **Warnings:**
  - The public auth endpoints accept a login CSRF.
  - The route edit path accepts a changed `pier_from` but ignores it, and resets the cancellation policy when none is sent.
  - Refresh-token reuse detection logs users out on a normal multi-tab refresh race.
  - An operator can be archived at the same moment a pier is created under it.
  - Two spellings of the same Thai phone number become two destinations.
  - Internal error text leaks through the gateway proxies.

## Critical Issues

### CR-01: OTP attempt limit is check-then-increment, so concurrent guesses bypass the 5-attempt lockout

**File:** `services/identity/internal/app/otp.go:145-173`
**Issue:** `VerifyOtp` does `HGetAll` → compares the hash → only on mismatch runs `HIncrBy "a"` → deletes the key once `attempts >= 5`. The comparison happens before any attempt is counted.

An attacker can fire N concurrent `VerifyOtp` calls for one destination. Every request that ran `HGetAll` before the 5th wrong `HIncrBy` deleted the key still gets its code compared. The correct guess wins whenever its `Del` (line 166) runs before the 5th wrong guess's `Del` (line 160).

The real number of guesses per code is therefore limited by how many requests are in flight at once, not by `otpMaxAttempts`. Kong's `rate-limiting` on `api-auth` is only 20/min **per IP** (`deploy/kong/kong.yml.tmpl:22-26`), so a small botnet multiplies this. D-03's "5 wrong attempts then lock" is not enforced under concurrency.

**Fix:** Count the attempt atomically before comparing. Every verify, right or wrong, consumes one attempt:
```go
var (
    attemptsCmd *redis.IntCmd
    getCmd      *redis.MapStringStringCmd
)
if _, err := a.RDB.TxPipelined(ctx, func(p redis.Pipeliner) error {
    getCmd = p.HGetAll(ctx, key)
    attemptsCmd = p.HIncrBy(ctx, key, "a", 1)
    return nil
}); err != nil {
    return Session{}, fmt.Errorf("app: check otp: %w", err)
}
stored, attempts := getCmd.Val(), attemptsCmd.Val()
if len(stored) == 0 || stored["h"] == "" { // expired (HIncrBy just recreated a TTL-less stub)
    a.RDB.Del(ctx, key)
    return Session{}, domain.ErrCodeExpired
}
if attempts > otpMaxAttempts {
    a.RDB.Del(ctx, key)
    return Session{}, domain.ErrCodeExpired
}
if !hmac.Equal([]byte(stored["h"]), []byte(hashCode(a.Pepper, dh, code))) {
    if attempts == otpMaxAttempts { a.RDB.Del(ctx, key); return Session{}, domain.ErrCodeExpired }
    return Session{}, &domain.CodeMismatchError{AttemptsLeft: int(otpMaxAttempts - attempts)}
}
// then the existing Del-wins single-use consumption
```
Add a concurrency test: 20 parallel wrong guesses plus 1 correct guess must never succeed after 5 attempts.

### CR-02: UpdateBoat moves `home_pier_id` without updating `operator_id`, breaking tenant ownership

**File:** `services/catalog/internal/app/boat.go:67, 116-126`; `services/catalog/internal/adapters/postgres/queries/boats.sql:6-14`
**Issue:** `UpsertBoat` sets `b.OperatorID` from the requested home pier (line 67). `UpdateBoat`, however, only writes `home_pier_id, name, default_capacity, status` and never writes `operator_id`.

When a super_admin moves a boat to a pier owned by another operator, the row ends up with `operator_id = old operator` and `home_pier_id = pier of new operator`. This breaks the documented invariant "OperatorID is always its home pier's operator" (`domain/boat.go:18-21`). The consequences:
- Neither operator's pier_admin or staff can see or edit the boat. `ListBoatsAdmin` and `GetBoatForUpdateScoped` require both `operator_id = X` and `home_pier_id = any(X's piers)`.
- `publishBoatUpserted(boatFromRow(row))` publishes the stale `operator_id` in `catalog.BoatUpserted`. Downstream consumers such as schedule-service attribute the boat to the wrong tenant.

**Fix:** Persist the derived operator on update:
```sql
-- name: UpdateBoat :one
update boats
set operator_id = $2, home_pier_id = $3, name = $4, default_capacity = $5, status = $6, updated_at = now()
where id = $1
returning *;
```
```go
row, err := q.UpdateBoat(ctx, postgres.UpdateBoatParams{
    ID: toPgUUID(b.ID), OperatorID: toPgUUID(b.OperatorID), HomePierID: toPgUUID(b.HomePierID), ...
})
```
If cross-operator moves should not be allowed at all, reject with `ErrFailedPrecondition` when `pier.OperatorID != stored.OperatorID`.

## Warnings

### WR-01: Login CSRF — `/api/v1/auth/otp/verify` accepts any Content-Type

**File:** `services/gateway/internal/adapters/http/auth.go:148-160`
**Issue:** `decodeAuthBody` parses the body with `protojson` whatever the `Content-Type` is, unlike `adminProxy`, which insists on `application/json`. A cross-site auto-submitting `<form method=POST enctype="text/plain">` can produce valid JSON.

Example: an input named `{"code":"123456","destination":"x` with value `y@attacker.com"}` serialises to `{"code":"123456","destination":"x=y@attacker.com"}\r\n`, and `=` is legal in an email local part. The attacker requests an OTP for their own address and then drives the victim's browser to verify it. The response's `Set-Cookie` is accepted on a top-level navigation, so the victim is now logged in as the attacker. Whatever the victim then books, pays for, or enters lands in the attacker's account.

**Fix:** In `decodeAuthBody`, reject anything that is not JSON, the same way the admin proxy does:
```go
if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
    httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("content-type must be application/json")))
    return false
}
```
A non-simple Content-Type forces a CORS preflight, and Kong's `cors` origin allow-list then blocks the attack.

### WR-02: UpsertRoute update checks `pier_from` from the request but ignores it; the self-loop check uses the wrong pier

**File:** `services/catalog/internal/app/route.go:51-94, 142-147`; `routes.sql:6-13`; `apps/admin/src/app/(admin)/routes/route-sheet.tsx:153-158`
**Issue:** On update, `r.PierFromID` comes from the request. It is checked for scope and non-archival, and `Validate()` compares it with `PierToID`. `UpdateRoute`, however, never writes `pier_from_id`, and the stored route's pier_from is never compared with the request.

The admin Sheet leaves the origin `<select>` enabled in edit mode, so:
- **Silent no-op.** A user changes the origin, gets a "saved" toast, and the change is dropped.
- **Unhandled 500.** The user changes the origin to C and sets the destination to the old origin A. `Validate` passes (C ≠ A), but the row becomes A→A. The `check (pier_from_id <> pier_to_id)` constraint rejects it with SQLSTATE 23514, which `isUniqueViolation` does not match, so the client gets a generic `CodeInternal`.

**Fix:** In `updateRoute`, require the request's pier_from to equal the stored one, or load the stored route first and validate against `stored.PierFromID`:
```go
if fromPgUUID(stored.PierFromID) != r.PierFromID {
    return domain.Route{}, fmt.Errorf("%w: pier_from_id is immutable", domain.ErrInvalidArgument)
}
```
Also disable the origin select in edit mode (`disabled={pending || isEdit}`).

### WR-03: Editing a route without a cancellation policy silently resets it to the default

**File:** `services/catalog/internal/app/route.go:79-81`
**Issue:** `if len(r.CancellationPolicy) == 0 { r.CancellationPolicy = domain.DefaultCancellationPolicy() }` runs for both create and update. The doc comment says it is create-only (D-13). An update that omits `cancellation_policy`, for example an API client changing only `duration_minutes`, overwrites a custom refund schedule with 100/50/0. That schedule drives refund amounts.

**Fix:** Apply the default only when `r.ID == uuid.Nil`. On update, either require a policy (`ErrInvalidArgument` if empty) or keep `stored.CancellationPolicy`.

### WR-04: Refresh-token reuse detection logs users out on a normal multi-tab or multi-app refresh race

**File:** `services/identity/internal/app/session.go:98-106`; `apps/admin/src/lib/api.ts:11-26`; `apps/web/src/lib/api.ts:11-26`
**Issue:** The `refreshPromise` de-duplication is per JS realm. Every open tab, and both the web app and the admin app (same cookie jar on the API host), gets its 401 at the same moment because all of them share one access cookie with a 15-min TTL. Each then POSTs `/auth/refresh` with the same refresh token.

The second request finds `revoked_at` set and runs `RevokeAllRefreshTokens`, so every session for that user is killed. This fires regularly for anyone with two tabs open. It also makes reuse detection noisy, which weakens its value as a compromise signal.

**Fix:** Add a short grace window. When the presented token was revoked less than ~30s ago *by rotation*, return `ErrSessionInvalid` without revoking the family. The losing tab then retries with the new cookie the winner already set. Record a `replaced_by`/`rotated_at` column to tell rotation apart from logout or disable revocation. Cross-tab `BroadcastChannel`/`navigator.locks` on the client helps too, but does not cover web and admin together.

### WR-05: Gateway clears session cookies on any identity error, including transient outages

**File:** `services/gateway/internal/adapters/http/auth.go:84-89`
**Issue:** `refreshHandler` calls `clearAuthCookies` for every error from `identity.Refresh`, including `Unavailable`, `DeadlineExceeded` and `Internal`. In those cases the refresh token was never rotated and is still valid, but the browser deletes it, so a short identity or DB blip logs out every user whose access token expires during it.

**Fix:** Clear cookies only when `connect.CodeOf(err) == connect.CodeUnauthenticated` (`ErrSessionInvalid`). For other errors, return the mapped error and keep the cookies.

### WR-06: A pier can be created under an operator that is being archived at the same time

**File:** `services/catalog/internal/app/operator.go:105-113`; `services/catalog/internal/app/pier.go:51-60`; `operators.sql:15-16, 25-26`
**Issue:** `createPier` reads the operator with a plain `GetOperator` (no lock) and then inserts. `ArchiveOperator` counts active piers and then updates `archived_at`.

The insert's FK check takes only `FOR KEY SHARE`, and an update of a non-key column takes `FOR NO KEY UPDATE`, so the two do not conflict. Under READ COMMITTED:
1. T1 reads the operator as active.
2. T2 counts 0 active piers, archives the operator, and commits.
3. T1 inserts the pier and commits.

The result is an active pier, and later routes and boats, under an archived operator. This breaks D-15 ("archiving is never a cascade" and archive is blocked while active piers exist).

**Fix:** Serialize on the operator row. Add `-- name: GetOperatorForUpdate :one select * from operators where id = $1 for update;` and call it first in `ArchiveOperator`. Make `createPier` use a `for share` variant (`GetOperatorForShare`), mirroring how piers and routes already use FOR UPDATE/FOR SHARE.

### WR-07: Two spellings of one Thai number become two destinations, splitting accounts and multiplying OTP send limits

**File:** `services/identity/internal/domain/destination.go:70-81`
**Issue:** `0812345678` normalises to `+66812345678`, but `+66 081 234 5678` normalises to `+660812345678`. The E.164 branch keeps the trunk `0` after `+66`. These are the same subscriber but get different `users.phone` rows, different `hashDestination` values, and separate cooldown and 5/hour counters.

A user who types the other spelling gets a second, empty customer account. An attacker gets a fresh 5/hour SMS budget for each spelling against one phone number, which matters once a real SMS provider is attached.

**Fix:** In the `+` branch, strip a single trunk `0` after the `+66` country code, and add table tests for `+660…` ≡ `0…` ≡ `+66…`:
```go
if strings.HasPrefix(stripped, "+660") { stripped = "+66" + stripped[4:] }
```

### WR-08: The staff edit dialog cannot be saved once an assigned pier has been archived

**File:** `apps/admin/src/app/(admin)/staff/staff-dialog.tsx:66, 83, 218-235`; `services/identity/internal/app/users.go:74-90`
**Issue:** `handleOpenChange` loads `user.pierIds`, which can include piers archived after assignment (archiving a pier does not touch identity's `pier_ids`). The checkbox list renders only non-archived piers, so the archived id stays hidden in state and cannot be unchecked. `validatePiers` rejects archived piers, so every save fails, including a simple rename. The super_admin cannot fix the user from the UI.

**Fix:** In the dialog, drop ids that are not in the non-archived pier list when opening, or render archived assigned piers as uncheckable-but-removable items. Server-side, consider allowing an update that only removes piers.

### WR-09: Internal error details leak to browsers through the gateway proxies

**File:** `services/gateway/internal/adapters/http/proxy.go:83-109, 145-154`; `services/catalog/internal/adapters/http/scope.go:66-67`; `services/identity/internal/adapters/http/users.go:139-140`
**Issue:** `httpx.WriteError` deliberately hides messages for 500s. The admin `ReverseProxy` and `publicHandler`, however, copy the upstream connect response through unchanged. connect-go puts `err.Error()` on the wire for every code, including `CodeInternal`.

So wrapped errors like `app: insert pier: ERROR: new row violates check constraint "..."` or `app: list public piers: failed to connect to host=postgres ...` reach admin users and, via `/api/v1/public/*`, anonymous internet clients. This exposes schema, constraint and host names.

**Fix:** Redact at the source: in `toConnectErr`/`mapUserError`/`mapAuthError`, log the full error and return `connect.NewError(connect.CodeInternal, errors.New("internal error"))` from the `default` branch. Alternatively, add a `ModifyResponse` in the proxies that rewrites 5xx bodies.

## Info

### IN-01: A wrong guess can re-create an expired OTP key with no TTL

**File:** `services/identity/internal/app/otp.go:155`
**Issue:** If the code key expires between `HGetAll` and `HIncrBy`, `HIncrBy` creates `{a:1}` with no TTL. The key then lives until 5 more wrong guesses or the next RequestOtp. The CR-01 fix covers this: delete when `h` is empty.
**Fix:** See CR-01.

### IN-02: ListRoutePrices takes a row lock on a read-only path

**File:** `services/catalog/internal/app/price.go:93-98`
**Issue:** `ListRoutePrices` reuses `GetRouteForUpdateScoped` (`FOR UPDATE`) on the pool outside a transaction. Read-only staff calls briefly take row locks that contend with `AddRoutePrice`/`ArchiveRoute`.
**Fix:** Add a non-locking `GetRouteScoped` query for reads.

### IN-03: `photo_key` is not tied to the caller or to an actual upload

**File:** `services/catalog/internal/domain/pier.go:62-64`; `services/catalog/internal/app/photo.go:48-77`
**Issue:** Any string matching `piers/<36 hex/dash>.(jpg|png|webp)` is accepted. That includes another operator's key, which is visible in the public `ListPiers` photo_url, and keys that were never uploaded. The presigned URL is also reusable until it expires, so the same object can be overwritten.
**Fix:** This is acceptable for v1. If needed, record issued keys per operator (a Valkey TTL set) and accept only keys issued to the caller's operator.

### IN-04: OTP login UIs show misleading error messages

**File:** `apps/admin/src/components/otp-login.tsx:19-31`; `apps/web/src/components/otp-login.tsx` (same mapping)
**Issue:**
- An invalid destination on the request step returns `invalid_argument` without `attemptsLeft`, so the UI shows "wrong code (0 attempts left)".
- Every 429 shows the hourly-cap text, including the 60s resend cooldown and Kong's per-IP limit.
- `failed_precondition` from `ErrDeliveryUnavailable` shows "code expired".
**Fix:** Branch on `attemptsLeft !== undefined` for the wrong-code message and add a generic invalid-destination message. Have the gateway pass through a distinguishing code for cooldown vs hourly cap.

### IN-05: identity dereferences a nil pool when `DATABASE_URL` is unset

**File:** `services/identity/cmd/main.go:92-98, 171`
**Issue:** `pool` stays nil when `DATABASE_URL` is empty, but `EnsureSuperAdmin` and `app.Auth`/`app.Users` require it. Startup panics inside `WithTx` instead of returning a clear config error.
**Fix:** `databaseURL := httpx.MustEnv("DATABASE_URL")` in identity.

### IN-06: No event is published for user promotion, update or disable

**File:** `services/identity/internal/app/users.go:122-137, 171-257`
**Issue:** Only `UserCreated` is emitted. Promoting a customer to staff, changing role/operator/piers, and disabling a user publish nothing. Any future consumer projecting users from identity events will keep a stale role and scope.
**Fix:** When a consumer needs it, add `identity.UserUpdated`/`UserDisabled` (ids, role and scope only, no PII) through the outbox in the same tx.

### IN-07: The pier Sheet enables Save for inputs the server will reject

**File:** `apps/admin/src/app/(admin)/piers/pier-sheet.tsx:84-86, 101`
**Issue:** When a super_admin creates a pier, `isValid` does not require `operatorId`, which is sent as `""` and the server answers NotFound. `hoursValid` does not check that opens is before closes. Both show only a generic failure message.
**Fix:** Add `(isEdit || !isSuperAdmin || Boolean(operatorId))` and `opensAt < closesAt` to `isValid`.

### IN-08: `eval` on a string containing secrets in storage-init

**File:** `deploy/docker-compose.yml:196-198`
**Issue:** `configure_cmd` interpolates `$S3_ACCESS_KEY`/`$S3_SECRET_KEY` into a string that is then passed to `eval`. A quote or `$(...)` in `.env` would run in the container. This is dev-only.
**Fix:** Use a shell function instead of `eval`, for example `configure() { printf '...' "$S3_ACCESS_KEY" "$S3_SECRET_KEY" | weed shell ...; }; until configure; do ...`.

### IN-09: The re-enable failure toast says "disable failed"

**File:** `apps/admin/src/app/(admin)/staff/page.tsx:167-175`
**Issue:** `handleSetDisabled` uses the disable-failure text for both directions.
**Fix:** Choose the message from `disabled`.

---

_Reviewed: 2026-09-26T21:39:42Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
