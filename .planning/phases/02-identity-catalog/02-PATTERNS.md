# Phase 2: Identity + Catalog - Pattern Map

**Mapped:** 2026-09-26
**Files analyzed:** ~40 (new service `identity` + catalog extensions + gateway extension + `apps/admin` scaffold + `apps/web` OTP screens)
**Analogs found:** all — every file class has a strong Phase-1 analog (this phase is explicitly "more instances of the same domain/app/postgres/http quadruplet")

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `services/identity/cmd/main.go` | config/bootstrap | request-response | `services/_template/cmd/main.go` | exact (copy verbatim, add Valkey client + email/SMS sender wiring) |
| `services/identity/internal/domain/user.go` | model | CRUD | `services/catalog/internal/domain/boat.go` | exact |
| `services/identity/internal/domain/otp.go` | model | CRUD | `services/catalog/internal/domain/boat.go` (struct+Validate shape) | role-match |
| `services/identity/internal/app/otp.go` | service | event-driven (rate-limit + Postgres write + email send) | `services/catalog/internal/app/boat.go` (UpsertBoat shape) + RESEARCH.md Pattern 1 (has full code sketch) | role-match |
| `services/identity/internal/app/user.go` (CreateStaffUser) | service | request-response + cross-service sync call | `services/catalog/internal/app/boat.go` + RESEARCH.md Pattern 3 (full code sketch, models the connect-go call itself) | role-match |
| `services/identity/internal/app/refresh.go` | service | CRUD (re-read + rotate) | `services/catalog/internal/app/boat.go` (tx-taking use-case shape); token mechanics from `pkg/auth/auth.go` | role-match |
| `services/identity/internal/adapters/http/routes.go` | controller | request-response | `services/catalog/internal/adapters/http/routes.go` | exact |
| `services/identity/internal/adapters/postgres/*.sql.go` | model/DAL | CRUD | `services/catalog/internal/adapters/postgres/boats.sql.go` (sqlc-generated) | exact |
| `services/identity/internal/adapters/kafka/handlers.go` | event-driven | event-driven | `services/catalog/internal/adapters/kafka/handlers.go` (empty `Register`, template parity) | exact |
| `services/identity/internal/adapters/email/{interface,smtp,resend}.go` | utility/adapter | request-response (external send) | none in codebase — first `EmailSender`-shaped adapter; RESEARCH.md Standard Stack + Pattern 1 give exact library/shape | no analog (see below) |
| `services/identity/internal/adapters/sms/{interface,log}.go` | utility/adapter | request-response | same as above — no analog, mirror the email adapter's interface shape once written | no analog |
| `services/identity/internal/adapters/valkey/*.go` | utility/adapter | CRUD (counters) | none — RESEARCH.md Pattern 1 has the exact `rdb.Incr`/`Expire`/`Exists` code to copy | no analog |
| `services/identity/migrations/00002_users.sql`, `00003_otps.sql` | migration | CRUD | `services/catalog/migrations/00002_boats.sql` | exact |
| `services/catalog/internal/domain/{operator,pier,route,route_price}.go` | model | CRUD | `services/catalog/internal/domain/boat.go` | exact |
| `services/catalog/internal/domain/boat.go` (add `HomePierID`) | model | CRUD | itself (extend in place) | exact |
| `services/catalog/internal/app/{operator,pier,route,route_price}.go` | service | CRUD | `services/catalog/internal/app/boat.go` | exact |
| `services/catalog/internal/app/boat.go` (extend `UpsertBoat` for `HomePierID` + pier scoping) | service | CRUD | itself (extend in place) + RESEARCH.md Pitfall 6 (shared scoping helper) | exact |
| `services/catalog/internal/app/upload.go` (PresignUpload) | service | file-I/O | none — RESEARCH.md "Don't Hand-Roll" names `minio-go`'s `PresignedPutObject`; shape it like `boat.go`'s use-case functions (tx-taking where it needs to persist the object key) | no analog |
| `services/catalog/internal/adapters/http/routes.go` (add Operator/Pier/Route/Boat/Price handlers + PresignUpload) | controller | request-response | itself (extend in place) — `UpsertBoat`/`ListBoats` handlers are the template for every new RPC | exact |
| `services/catalog/migrations/00003..00007_*.sql` | migration | CRUD | `services/catalog/migrations/00002_boats.sql` | exact |
| `proto/services/identity/v1/identity.proto` | config (proto contract) | request-response | `proto/services/catalog/v1/catalog.proto` | exact |
| `proto/events/identity/v1/user.proto` | config (proto contract) | event-driven | `proto/events/catalog/v1/boat.proto` | exact |
| `proto/services/catalog/v1/catalog.proto` (add Operator/Pier/Route/RoutePrice/PresignUpload/ValidatePiers RPCs+messages) | config | request-response | itself (extend in place) | exact |
| `proto/events/catalog/v1/{operator,pier,route,price}.proto` + `boat.proto` (add `home_pier_id` field) | config | event-driven | `proto/events/catalog/v1/boat.proto` | exact |
| `pkg/auth/auth.go` (`Claims` + `jwtClaims` gain `PierIDs`) | model | request-response | itself (extend in place) — RESEARCH.md Pattern 2 shows the exact diff | exact |
| `pkg/httpx/claims.go` (`Claims` gains `PierIDs`, new `HeaderPierIDs`, `RequireInternal` parses it) | middleware | request-response | itself (extend in place) | exact |
| `services/gateway/internal/adapters/http/bff.go` (extend `ForwardClaims` +5th header; add `/api/v1/auth/*`, `/api/v1/public/piers`, `/api/v1/public/routes`) | controller | request-response | itself (`publicBoatsHandler`/`upsertBoatHandler`/`ForwardClaims`) | exact |
| `deploy/kong/kong.yml` (add `/api/v1/auth/*` no-JWT route, admin CORS origin) | config | request-response | itself (`/api/v1/public` vs `/api` split already documented) | exact |
| `apps/web/src/app/[locale]/...` (OTP login/verify screens) | component | request-response | `apps/web/src/components/boat-list.tsx` (TanStack Query + apiFetch + loading/error/empty states) | role-match |
| `apps/admin/src/lib/api.ts` | utility | request-response | `apps/web/src/lib/api.ts` | exact (copy verbatim) |
| `apps/admin/src/app/.../providers.tsx` | provider | request-response | `apps/web/src/app/[locale]/providers.tsx` | exact (copy, drop next-intl) |
| `apps/admin/src/components/data-table.tsx` | component | CRUD (list) | none in codebase — new `@tanstack/react-table` v8 wrapper; RESEARCH.md names the exact library/version | no analog (external lib pattern) |
| `apps/admin/src/components/map-picker.tsx` | component | request-response | none in codebase — new `maplibre-gl` wrapper | no analog (external lib pattern) |
| `apps/admin/src/app/(routes)/{operators,piers,routes,boats,staff}/**` | component | CRUD | `apps/web/src/components/boat-list.tsx` (query/loading/error/empty shape) + UI-SPEC's `data-table`/`sheet`/`dialog` component list | role-match |

## Pattern Assignments

### `services/identity/*` (new service — controller/service/model/migration)

**Analog:** `services/catalog` end-to-end, scaffolded via `make new-service identity` (copies `services/_template`).

**Bootstrap pattern** — copy `services/_template/cmd/main.go` verbatim (lines 1-206 of that file, see excerpt below for the shape); identity's `run()` additionally constructs a `go-redis/v9` client from `VALKEY_ADDR` and wires the `EmailSender`/`SMSSender` adapters based on env (`SMTP_HOST` set → dev sender, else `RESEND_API_KEY` → prod sender), and calls a `pkg/auth` idempotent-super-admin-bootstrap function once before `g.Wait()` (D-09).

**Readiness pattern** (`services/_template/cmd/main.go` lines 118-124):
```go
checks := map[string]func(context.Context) error{}
if pool != nil {
	checks["db"] = pool.Ping
}
if producer != nil {
	checks["kafka"] = producer.Ping
}
ready := httpx.NewReadiness(checks)
```
Identity adds `checks["valkey"] = rdb.Ping` per D-03/Phase-1 D-39 (readyz includes every real dependency).

**Domain model pattern** — `services/catalog/internal/domain/boat.go` (full file, 45 lines): plain struct + a `Validate() error` method returning `fmt.Errorf("%w: ...", domain.ErrInvalidArgument)`. Copy this shape for `domain.User{ID, Role, OperatorID, PierIDs, Email, Phone, DisabledAt}` and `domain.OTP{DestinationHash, CodeHash, ExpiresAt, Attempts}`.

**Use-case (app layer) pattern** — `services/catalog/internal/app/boat.go` (full file, 127 lines). Concrete excerpt to copy structurally (imports + tx-taking signature + outbox insert):
```go
// imports (lines 1-20)
import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// core pattern (lines 34-81): validate -> assign uuid v7 if new -> sqlc
// upsert -> build event via events.New -> outbox.Insert in the SAME tx ->
// return the stored row. UpsertBoat itself is the template for
// UpsertOperator/UpsertPier/UpsertRoute/UpsertRoutePrice.
func UpsertBoat(ctx context.Context, tx pgx.Tx, operatorID uuid.UUID, b domain.Boat) (domain.Boat, error) {
	b.OperatorID = operatorID
	if err := b.Validate(); err != nil {
		return domain.Boat{}, err
	}
	// ... uuid.NewV7() when id is zero-value, postgres.New(tx).UpsertBoat(...),
	// events.New(EventBoatUpserted, ...), outbox.Insert(ctx, tx, env)
}
```
For **OTP request/verify**, use RESEARCH.md's Pattern 1 code block verbatim (lines 284-317 of 02-RESEARCH.md) — it already models the Valkey-counter + Postgres-row + `crypto/rand`/HMAC shape identity needs; do not re-derive it.

For **cross-service validation** (`CreateStaffUser` → catalog `ValidatePiers`), use RESEARCH.md's Pattern 3 code block verbatim (lines 341-353) — models the connect-go call with `httpx.HeaderInternalToken` set directly (no `ForwardClaims`, since this is a service-to-service call, not gateway-to-service).

**Error handling pattern** — every `app` function returns `fmt.Errorf("app: <verb>: %w", err)` wrapping either a `domain.Err*` sentinel or the underlying driver error; the HTTP layer maps sentinels to Connect codes (see below). Copy verbatim, no new error taxonomy.

**HTTP/controller pattern** — `services/catalog/internal/adapters/http/routes.go` (full file, 147 lines). Concrete excerpt (claims extraction + error mapping, lines 46-101):
```go
claims, ok := httpx.FromContext(ctx)
if !ok {
	return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
}
operatorID, err := uuid.Parse(claims.OperatorID)
// ... business call inside bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error { ... })
if err != nil {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrNotFound):
		return nil, connect.NewError(connect.CodeNotFound, err)
	default:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
}
s.nudge()
```
Identity's `IdentityService` handler (`RequestOTP`, `VerifyOTP`, `CreateStaffUser`, `Refresh`) copies this exact claims-extract → tx-wrapped-app-call → error-switch → nudge shape. `RequestOTP`/`VerifyOTP` are unauthenticated (no `httpx.FromContext` check — mirror `ListBoats`'s "needs only the internal token, no claims" shape instead).

**Migration pattern** — `services/catalog/migrations/00002_boats.sql` (full file, 15 lines): goose `-- +goose Up`/`Down`, uuid primary key, check constraints inline, one operator_id index. Copy for `users`/`otps` tables.

**Proto pattern** — `proto/services/catalog/v1/catalog.proto` (full file, 33 lines): one `service` block, request/response message pairs, comments stating which fields come from trusted claims vs. request body. Copy for `proto/services/identity/v1/identity.proto`.

---

### `services/catalog/*` (extend existing service)

**Analog:** the file itself — every new entity (`Operator`, `Pier`, `Route`, `RoutePrice`) is a sibling of `Boat`, added by literally repeating the domain/app/http/migration quadruplet shown above, not a different pattern.

**Pier-scoping shared helper** (RESEARCH.md Pitfall 6 — write once, reuse across operator/pier/route/boat/price list+get functions):
```go
if claims.Role != "super_admin" {
	query = query.Where(pierIDIn(claims.PierIDs))
}
```
Put this in one `services/catalog/internal/app` helper function and call it from every new entity's list/get — do not repeat the branch five times.

**Presigned upload pattern** — no in-repo analog; RESEARCH.md's "Don't Hand-Roll" table names `minio-go`'s `PresignedPutObject` directly. Shape `PresignUpload` as a `pgx.Tx`-taking (or pool-only, if it doesn't need to persist anything itself) function in `services/catalog/internal/app/upload.go`, following `boat.go`'s function signature convention (`ctx, tx/pool, operatorID, ...`) even though there's no outbox event for this call.

**Cancellation-policy validation** — copy RESEARCH.md's Code Examples block verbatim (lines 429-454 of 02-RESEARCH.md, `ValidateCancellationPolicy`) into `services/catalog/internal/domain/route.go`'s `Validate()`.

**Effective-price query** — copy RESEARCH.md's SQL block verbatim (lines 415-426) into `services/catalog/internal/adapters/postgres/queries/route_prices.sql` as a `-- name: GetEffectivePrice :one` sqlc query.

---

### `pkg/auth/auth.go` + `pkg/httpx/claims.go` (extend in place)

**Analog:** the files themselves. RESEARCH.md Pattern 2 gives the exact diff shape:
```go
// pkg/auth/auth.go — extend, do not replace
type Claims struct {
	UserID     string
	OperatorID string
	Role       string
	PierIDs    []string // new (D-06) — empty for customer and super_admin
	Kind       Kind
}
```
Mirror the same addition in `pkg/httpx/claims.go`'s `Claims` struct (currently lines 22-26) and add `HeaderPierIDs = "X-Pier-Ids"` alongside the existing `HeaderUserID`/`HeaderOperatorID`/`HeaderRole`/`HeaderInternalToken` block (`pkg/httpx/claims.go` lines 14-19). `RequireInternal` (lines 46-65) gets one more `r.Header.Get(HeaderPierIDs)` read inside the existing `if uid := ...; uid != "" { ... }` block.

---

### `services/gateway/internal/adapters/http/bff.go` (extend in place)

**Analog:** the file itself — `ForwardClaims` (lines 159-169) is the exact function to extend:
```go
func ForwardClaims(h http.Header, c auth.Claims, internalToken string) {
	h.Del(httpx.HeaderUserID)
	h.Del(httpx.HeaderOperatorID)
	h.Del(httpx.HeaderRole)
	h.Del(httpx.HeaderInternalToken)
	// add: h.Del(httpx.HeaderPierIDs)

	h.Set(httpx.HeaderUserID, c.UserID)
	h.Set(httpx.HeaderOperatorID, c.OperatorID)
	h.Set(httpx.HeaderRole, c.Role)
	h.Set(httpx.HeaderInternalToken, internalToken)
	// add: h.Set(httpx.HeaderPierIDs, strings.Join(c.PierIDs, ","))
}
```
New public routes (`/api/v1/public/piers`, `/api/v1/public/routes`) copy `publicBoatsHandler` (lines 59-71) verbatim, swapping the connect-go call. New `/api/v1/auth/*` routes (OTP request/verify/refresh/logout) copy `whoamiHandler`'s cookie-read shape for `refresh`/`logout` and `upsertBoatHandler`'s body-decode-then-proxy shape (lines 78-120, including the `MaxBytesReader`/`protojson.Unmarshal`/`ForwardClaims` sequence) for the identity calls that need claims (none of OTP request/verify do — they mirror `publicBoatsHandler` instead, internal-token only).

---

### `apps/admin` (new app — component/provider/utility)

**Analog:** `apps/web`, copied minus `next-intl`.

**`apiFetch` pattern** — `apps/web/src/lib/api.ts` (full file, 22 lines) — copy verbatim into `apps/admin/src/lib/api.ts`, no changes needed.

**Query provider pattern** — `apps/web/src/app/[locale]/providers.tsx` (full file, 18 lines) — copy into `apps/admin`, drop the `[locale]` path segment per D-18 (Thai-only, no locale prefix).

**List page pattern** — `apps/web/src/components/boat-list.tsx` (full file, 53 lines): `useQuery` + `apiFetch` + loading/error/empty branches + `Card` rendering. Every admin list page (`operators`, `piers`, `routes`, `boats`, `staff`) follows this query/loading/error/empty shape, but renders through the new shared `data-table.tsx` (`@tanstack/react-table` v8, per RESEARCH.md Pitfall 2 — pin `8.21.3` exactly) instead of a `Card` list, and uses the shadcn `empty`/`skeleton` primitives named in 02-UI-SPEC.md rather than hand-rolled `<p>` states.

**Map picker / data table** — no in-repo analog exists (`maplibre-gl`, `@tanstack/react-table` are both new to the codebase); build per RESEARCH.md's Standard Stack + Recommended Project Structure (`apps/admin/src/components/{map-picker,data-table}.tsx`) and 02-UI-SPEC.md's component inventory (`sheet`, `dialog`, `alert-dialog`, `input-otp`, `table`, `badge`, `empty`, `skeleton`, `sonner`, `tooltip`, `spinner`, `separator`).

---

### `apps/web` (OTP login screens only, D-21 scope)

**Analog:** `apps/web/src/components/boat-list.tsx` for the query/loading/error/empty shape; `shadcn` `input-otp` component (per 02-UI-SPEC.md) for the 6-digit entry — do not hand-roll six `<input>`s.

## Shared Patterns

### Claims trust boundary (identity carries the code, gateway/httpx carry the plumbing)
**Source:** `pkg/httpx/claims.go` (`RequireInternal`, lines 46-65) + `pkg/auth/auth.go` (`Claims`, `Verify`)
**Apply to:** every new catalog RPC handler, every new identity RPC handler, gateway's new `/api/v1/auth/*` and `/api/v1/public/{piers,routes}` routes.
```go
claims, ok := httpx.FromContext(ctx)
if !ok {
	return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
}
```

### Outbox-per-write
**Source:** `services/catalog/internal/app/boat.go` lines 66-78 (`events.New` + `outbox.Insert` inside the same `pgx.Tx` as the state write)
**Apply to:** every new `UpsertX` in catalog, and identity's `UserCreated` publish on first OTP verify (D-04).

### Error-sentinel-to-Connect-code mapping
**Source:** `services/catalog/internal/adapters/http/routes.go` lines 88-97
**Apply to:** every new connect-go handler in both services — `domain.ErrInvalidArgument` → `CodeInvalidArgument`, `domain.ErrNotFound` → `CodeNotFound`, everything else → `CodeInternal`. Add `domain.ErrRateLimited`/`domain.ErrResendTooSoon` → `CodeResourceExhausted`/`CodeFailedPrecondition` (identity's new sentinels per RESEARCH.md Pattern 1) to the same switch shape — do not invent a parallel error-handling style.

### Pier/operator scoping branch (write once, call five times)
**Source:** RESEARCH.md Pitfall 6, no in-repo precedent yet (Phase 1 had no roles)
**Apply to:** every catalog list/get for Operator/Pier/Route/Boat/RoutePrice, and identity's user list — one shared helper function, `role == super_admin` bypasses the `pier_ids` filter, everything else still additionally filters by `operator_id` from claims (AUTH-05, unconditional).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `services/identity/internal/adapters/email/*.go` | utility/adapter | request-response | First `EmailSender`-shaped adapter in the codebase — no prior external-notification integration exists; follow RESEARCH.md Standard Stack (`net/smtp` stdlib for dev/Mailpit, `resend-go/v4` for prod) behind one small interface, not a Phase-1 pattern extension |
| `services/identity/internal/adapters/sms/*.go` | utility/adapter | request-response | Same — no prior SMS integration; v1 has only a dev/log stub (D-01) |
| `services/identity/internal/adapters/valkey/*.go` | utility/adapter | CRUD (counters) | First Valkey/Redis dependency in any service; RESEARCH.md Pattern 1's code block is the closest thing to an analog and should be copied directly rather than re-derived |
| `services/catalog/internal/app/upload.go` | service | file-I/O | First object-storage integration; `minio-go`'s `PresignedPutObject` call has no in-repo precedent — structure the function like `boat.go`'s use-cases (ctx/tx-or-pool/operatorID signature) for consistency even though the S3 call itself is new |
| `apps/admin/src/components/data-table.tsx` | component | CRUD | First use of `@tanstack/react-table` in the codebase — no wrapper exists yet; build one, reuse across all 5 admin list pages per RESEARCH.md's explicit "don't hand-roll it five times" guidance |
| `apps/admin/src/components/map-picker.tsx` | component | request-response | First use of `maplibre-gl`; no map component exists in `apps/web` yet either |

## Metadata

**Analog search scope:** `services/catalog/`, `services/_template/`, `services/gateway/`, `pkg/auth/`, `pkg/httpx/`, `proto/services/catalog/v1/`, `proto/events/catalog/v1/`, `apps/web/src/`, `deploy/kong/`
**Files scanned:** ~30 (all Go source under services/catalog, services/_template, services/gateway, pkg/auth, pkg/httpx; all TS/TSX under apps/web/src; catalog.proto + boat.proto)
**Pattern extraction date:** 2026-09-26
