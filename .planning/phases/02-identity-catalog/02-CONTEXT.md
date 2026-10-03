# Phase 2: Identity + Catalog - Context

**Gathered:** 2026-09-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Customers and staff authenticate with passwordless email OTP and receive RS256 JWT sessions (httpOnly cookies) carrying `role`, `operator_id` and `pier_ids`; Kong verifies, the BFF forwards claims as trusted headers, services enforce operator + pier scoping. super_admin manages operators, piers and staff users; pier_admin manages piers/routes/boats/prices within their assigned piers through a separate admin app (`apps/admin`). A public, unauthenticated API lists piers and routes with coordinates for the map. Requirements: AUTH-01..AUTH-05, CAT-01..CAT-06.

Not in this phase: schedule templates/departures (Phase 3), booking/availability (Phase 4), customer-facing search/map UI screens (Phase 4 — this phase delivers the public API only, CAT-06), real SMS provider, staff check-in scanner (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### OTP login + delivery (identity)
- **D-01:** OTP destinations: email OR phone accepted by the API. Email is delivered for real via Resend (prod). Phone goes through an `SMSSender` interface whose only v1 implementation is a dev/log sender; a real SMS provider is chosen near launch. (Two sender implementations for email — SMTP for dev, Resend for prod — is the one justified interface.)
- **D-02:** Dev/test delivery: **Mailpit** container in Compose; identity uses an SMTP sender in dev pointing at Mailpit. Playwright/integration tests read OTPs via the Mailpit HTTP API. No OTP ever written to logs (keeps D-45 of Phase 1 intact); no fixed/magic dev OTP.
- **D-03:** OTP rules: 6 digits, TTL 5 min, single-use, stored hashed; max 5 wrong attempts then the code is invalidated; resend cooldown 60 s; max 5 sends per destination per hour. Attempt/rate counters live in **Valkey** (identity gets a Valkey dependency; readyz includes it per Phase 1 D-39).
- **D-04:** First successful OTP verify for an unknown destination auto-creates `user(role=customer)` — no signup form. Every JWT therefore has a real `sub`. identity publishes `identity.UserCreated` (ids + role only, no email/phone — PII rule).

### Staff login, roles + scoping
- **D-05:** staff / pier_admin / super_admin log in with the same email OTP flow — no passwords anywhere in v1. Role comes from the user record created by super_admin; an OTP login for an email that belongs to a staff user yields that user's role/claims.
- **D-06:** JWT claims gain **`pier_ids`** (list of uuids) alongside `operator_id` and `role`. `pkg/auth.Claims`, the BFF claim→header forwarding (new header e.g. `X-Pier-Ids`), and `pkg/httpx.Claims` are extended accordingly. customer and super_admin have empty `pier_ids`; super_admin bypasses operator/pier scope. — **Reversibility:** costly — token format, Kong/BFF header contract and every scoped query depend on it.
- **D-07:** Pier scoping covers everything pier-bound, **including boats**: `boats` gains `home_pier_id` (migration + additive field on `catalog.BoatUpserted` proto — new field number, never reuse). pier_admin may read/write a route only if its `pier_from` ∈ `pier_ids`; a boat only if `home_pier_id` ∈ `pier_ids`; a pier only if its id ∈ `pier_ids`. All queries are still additionally scoped by `operator_id` (AUTH-05). — **Reversibility:** costly — schema + published event field.
- **D-08:** Only **super_admin** creates operators and piers and assigns piers to pier_admin/staff users (AUTH-04, CAT-01). pier_admin edits/archives only piers in their `pier_ids`. ROADMAP success criterion 4 wording ("pier_admin creates … operators, piers") must be corrected to match — planner notes the doc update.
- **D-09:** super_admin bootstrap: identity idempotently ensures a super_admin user exists for env `SUPER_ADMIN_EMAIL` at startup (no seed script, no CLI).
- **D-10:** Revocation latency ≤ 15 min: the refresh endpoint re-reads role / operator_id / pier_ids / disabled flag from the identity DB on every refresh and rotates the refresh token (Phase 1 D-31 carried forward). Disabled user → refresh fails. No access-token denylist, no per-request session check.

### Catalog data model
- **D-11:** Routes are **one-way**: A→B and B→A are two routes, each with its own prices, cancellation policy and (later) schedules. Admin UI offers "create return route" that pre-fills a copy with piers swapped.
- **D-12:** Every pier has an owning `operator_id`, but a route's `pier_to` may be any non-archived pier of **any** operator (shared island piers aren't duplicated). `pier_from` must belong to the route's operator (and be in the pier_admin's `pier_ids`).
- **D-13:** Cancellation policy per route = jsonb list `[{min_hours_before, refund_percent}]`, default `[{24,100},{2,50},{0,0}]`. Validation: strictly descending `min_hours_before`, must include a `0` tier, percent 0–100. Refund math later uses `pkg/money`'s rounding rule.
- **D-14:** Prices: `route_prices(route_id, ticket_type, amount_satang int64, effective_from date)`. Ticket types v1 = `adult`, `child`. Price in effect = row with the latest `effective_from` ≤ the **departure's local (Asia/Bangkok) date** — lets admins pre-set high-season prices. Booking (Phase 4) snapshots the price at booking time. `catalog.PriceChanged` published on insert/change. — **Reversibility:** costly — Phase 4 price lookup contract depends on it.
- **D-15:** Archive = soft delete (`archived_at`), never hard delete. Archiving a pier with non-archived routes (as `pier_from` or `pier_to`) is rejected with `FailedPrecondition` listing those routes — no cascade.
- **D-16:** Pier open hours = single `opens_at` / `closes_at` (local time, same every day), display-only; no effect on scheduling.
- **D-17:** Piers have required `name_th` + `name_en`; address is a single text field. Route display name is derived from its piers (no stored route name).

### Admin app
- **D-18:** Admin UI is a **separate app `apps/admin`**: Next.js 16 App Router + TS + Tailwind + shadcn/ui + TanStack Query + an `apiFetch()` like `apps/web` (Phase 1 D-35 pattern: client-only fetching). **Thai only — no next-intl / no locale prefix.** Runs on its own port (e.g. `:3002`) through Kong; Kong CORS origins add the admin origin; cookies shared on localhost in dev. Role gating in the UI is cosmetic — backend enforces. — **Reversibility:** costly — separate app, CI job, Compose service, CORS/cookie config.
- **D-19:** Pier photo: browser uploads directly to object storage via a **presigned PUT URL** issued by catalog; catalog stores only the object key. jpeg/png/webp, ≤ 5 MB. **MinIO** in Compose for dev, S3-compatible in prod. Adds one container + an S3 client dependency to catalog.
- **D-20:** Maps: **MapLibre GL** with **OpenFreeMap** vector tiles (no API key); style URL configurable via env so the provider can be swapped. Admin map picker = click/drag marker to set lat/lng plus manual lat/lng inputs; no geocoding/address search in v1. (Supersedes the seed's "Leaflet/OSM" wording.)

### Public API
- **D-21:** CAT-06 delivered as unauthenticated BFF routes under the existing Kong `/api/v1/public` route (no JWT plugin): list non-archived piers (id, names, lat/lng, photo URL) and routes (pier_from/pier_to ids, duration, current prices). No customer UI screen in this phase.

### Claude's Discretion
- Exact identity schema (users, operator membership, user_piers, otp tables), OTP hashing choice, Valkey key naming, refresh-token storage/rotation mechanics.
- connect-go service method shapes for identity and catalog, BFF REST route naming, pagination style.
- Admin UI layout: data tables vs drawer/dialog forms, navigation, empty states — can be refined by `/gsd-ui-phase 2`.
- Whether the admin app shares UI components with `apps/web` via copy or a small shared package (default: copy — no shared package until duplication hurts).
- Photo URL serving (public-read bucket vs presigned GET) as long as the public API returns a usable URL.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Seed + project definition
- `PROJECT.md` §2 (Users & Roles), §3 (Core Domain Model), §5.1 (identity/catalog ownership + events), §6.3 (Admin screens), §7 (UI/UX direction), §9 (Technical Guardrails) — the seed.
- `.planning/PROJECT.md` — core value, constraints (PDPA minimal PII, operator scoping, satang money), key decisions (tiered cancellation default).
- `.planning/REQUIREMENTS.md` — AUTH-01..AUTH-05, CAT-01..CAT-06.
- `.planning/ROADMAP.md` — Phase 2 goal + success criteria (criterion 4 wording to be corrected per D-08).

### Prior phase decisions
- `.planning/phases/01-platform-foundation/01-CONTEXT.md` — D-06/D-07 (envelope + proto layout), D-13 (idempotent consumer), D-14/D-15 (internal layout + sqlc), D-27..D-31 (RS256, Kong spike result, BFF claim forwarding, internal token, access 15 min + refresh 30 d), D-35 (client-only fetching), D-39 (readyz incl. Valkey), D-42..D-45 (clock, money, errors, PII guard).
- `.planning/phases/01-platform-foundation/01-SECURITY.md` — trust-boundary threats already mitigated for gateway/catalog.

### Research
- `.planning/research/STACK.md` — Resend Go SDK, testcontainers, Kong 3.9.1 pin.
- `.planning/research/ARCHITECTURE.md` — Pattern 4 (Kong + BFF claim→header), Anti-Pattern 2 (header trust boundary).
- `.planning/research/PITFALLS.md` — auth/scoping pitfalls.
- `.planning/research/FEATURES.md` — admin/catalog feature expectations.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/auth` (`auth.go`): RS256 `Issuer`/`Verifier`, `Claims{UserID, OperatorID, Role, Kind}`, access/refresh kinds + cookie names — extend `Claims` with `PierIDs`; identity uses `Issuer`.
- `pkg/auth/cmd/devtoken`: dev token minting — keep working with new claims.
- `services/_template` + `make new-service identity`: scaffolds identity (outbox, consumer, migrations, readyz).
- `services/catalog`: boats aggregate end-to-end (domain/app/sqlc/http/outbox) — the pattern to copy for piers, routes, prices, operators.
- `services/gateway/internal/adapters/http/bff.go`: `tokenFromRequest`, `ForwardClaims`, `/api/v1/public/boats` public-route pattern, body limits.
- `apps/web`: `apiFetch`, TanStack providers, shadcn `button`/`card` — template for `apps/admin`.
- `deploy/kong/kong.yml`: `/api/v1/public` (no JWT) vs `/api` (JWT via `access_token` cookie); CORS origin list to extend.

### Established Patterns
- Thin layers `domain` / `app` (functions taking `pgx.Tx`) / `adapters`; no single-impl interfaces; integration tests against real Postgres.
- Outbox for every publish; event payloads carry ids only.
- Connect error codes canonical, `httpx.WriteError` maps to HTTP.
- Env-only config via `MustEnv`/`EnvOr`; shared services list drives DB + topic provisioning.

### Integration Points
- New service `identity` → services list (DB + `identity.events` topic), Compose, Kong routes for `/api/v1/auth/*` (OTP request/verify, refresh, logout must be reachable without an access token).
- `boats` migration adds `home_pier_id`; `catalog.BoatUpserted` gains a field; schedule's consumer (Phase 1) must tolerate it.
- Compose adds Mailpit, MinIO, `apps/admin`; Jenkins changed-path logic must include `apps/admin`.

</code_context>

<specifics>
## Specific Ideas

- Passwordless everywhere — the user explicitly prefers one OTP flow for customers and staff.
- Real pier-level isolation (not just operator-level) was chosen deliberately even though it costs more plumbing.
- Price schedule by departure date so operators can pre-load high-season prices.
- Admin is Thai-only; customer web stays TH/EN.

</specifics>

<deferred>
## Deferred Ideas

- Real SMS provider (ThaiBulkSMS/Twilio etc.) + SMS-pumping protection — before launch (Phase 7 / polish).
- Passkey/WebAuthn login for staff — Growth.
- Geocoding / address search in the map picker — later if admins ask.
- Per-weekday / holiday pier open hours — later.
- Immediate token revocation (denylist / session version) — only if 15 min proves too slow.
- Customer-facing map + search screens — Phase 4 (uses the D-21 public API).

</deferred>

---

*Phase: 02-identity-catalog*
*Context gathered: 2026-09-26*
