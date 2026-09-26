# Phase 2: Identity + Catalog - Research

**Researched:** 2026-09-26
**Domain:** Passwordless (OTP) auth with role/operator/pier scoping (Go + Kong + connect-go) and multi-entity admin catalog CRUD (Go + Next.js admin app)
**Confidence:** HIGH for reused Phase 1 patterns and version pins (verified against proxy.golang.org / npm / official docs this session); MEDIUM for OTP hashing/dev-SMS mechanics (Claude's Discretion in CONTEXT.md, no single canonical answer); **HIGH-severity gotcha found**: D-19's "MinIO in Compose for dev" targets a project whose community edition was archived in 2026 — see Pitfall 1.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**OTP login + delivery (identity)**
- **D-01:** OTP destinations: email OR phone accepted by the API. Email is delivered for real via Resend (prod). Phone goes through an `SMSSender` interface whose only v1 implementation is a dev/log sender; a real SMS provider is chosen near launch. (Two sender implementations for email — SMTP for dev, Resend for prod — is the one justified interface.)
- **D-02:** Dev/test delivery: **Mailpit** container in Compose; identity uses an SMTP sender in dev pointing at Mailpit. Playwright/integration tests read OTPs via the Mailpit HTTP API. No OTP ever written to logs (keeps D-45 of Phase 1 intact); no fixed/magic dev OTP.
- **D-03:** OTP rules: 6 digits, TTL 5 min, single-use, stored hashed; max 5 wrong attempts then the code is invalidated; resend cooldown 60 s; max 5 sends per destination per hour. Attempt/rate counters live in **Valkey** (identity gets a Valkey dependency; readyz includes it per Phase 1 D-39).
- **D-04:** First successful OTP verify for an unknown destination auto-creates `user(role=customer)` — no signup form. Every JWT therefore has a real `sub`. identity publishes `identity.UserCreated` (ids + role only, no email/phone — PII rule).

**Staff login, roles + scoping**
- **D-05:** staff / pier_admin / super_admin log in with the same email OTP flow — no passwords anywhere in v1. Role comes from the user record created by super_admin; an OTP login for an email that belongs to a staff user yields that user's role/claims.
- **D-06:** JWT claims gain **`pier_ids`** (list of uuids) alongside `operator_id` and `role`. `pkg/auth.Claims`, the BFF claim→header forwarding (new header e.g. `X-Pier-Ids`), and `pkg/httpx.Claims` are extended accordingly. customer and super_admin have empty `pier_ids`; super_admin bypasses operator/pier scope. — **Reversibility:** costly — token format, Kong/BFF header contract and every scoped query depend on it.
- **D-07:** Pier scoping covers everything pier-bound, **including boats**: `boats` gains `home_pier_id` (migration + additive field on `catalog.BoatUpserted` proto — new field number, never reuse). pier_admin may read/write a route only if its `pier_from` ∈ `pier_ids`; a boat only if `home_pier_id` ∈ `pier_ids`; a pier only if its id ∈ `pier_ids`. All queries are still additionally scoped by `operator_id` (AUTH-05). — **Reversibility:** costly — schema + published event field.
- **D-08:** Only **super_admin** creates operators and piers and assigns piers to pier_admin/staff users (AUTH-04, CAT-01). pier_admin edits/archives only piers in their `pier_ids`. ROADMAP success criterion 4 wording ("pier_admin creates … operators, piers") must be corrected to match — planner notes the doc update.
- **D-09:** super_admin bootstrap: identity idempotently ensures a super_admin user exists for env `SUPER_ADMIN_EMAIL` at startup (no seed script, no CLI).
- **D-10:** Revocation latency ≤ 15 min: the refresh endpoint re-reads role / operator_id / pier_ids / disabled flag from the identity DB on every refresh and rotates the refresh token (Phase 1 D-31 carried forward). Disabled user → refresh fails. No access-token denylist, no per-request session check.

**Catalog data model**
- **D-11:** Routes are **one-way**: A→B and B→A are two routes, each with its own prices, cancellation policy and (later) schedules. Admin UI offers "create return route" that pre-fills a copy with piers swapped.
- **D-12:** Every pier has an owning `operator_id`, but a route's `pier_to` may be any non-archived pier of **any** operator (shared island piers aren't duplicated). `pier_from` must belong to the route's operator (and be in the pier_admin's `pier_ids`).
- **D-13:** Cancellation policy per route = jsonb list `[{min_hours_before, refund_percent}]`, default `[{24,100},{2,50},{0,0}]`. Validation: strictly descending `min_hours_before`, must include a `0` tier, percent 0–100. Refund math later uses `pkg/money`'s rounding rule.
- **D-14:** Prices: `route_prices(route_id, ticket_type, amount_satang int64, effective_from date)`. Ticket types v1 = `adult`, `child`. Price in effect = row with the latest `effective_from` ≤ the **departure's local (Asia/Bangkok) date** — lets admins pre-set high-season prices. Booking (Phase 4) snapshots the price at booking time. `catalog.PriceChanged` published on insert/change. — **Reversibility:** costly — Phase 4 price lookup contract depends on it.
- **D-15:** Archive = soft delete (`archived_at`), never hard delete. Archiving a pier with non-archived routes (as `pier_from` or `pier_to`) is rejected with `FailedPrecondition` listing those routes — no cascade.
- **D-16:** Pier open hours = single `opens_at` / `closes_at` (local time, same every day), display-only; no effect on scheduling.
- **D-17:** Piers have required `name_th` + `name_en`; address is a single text field. Route display name is derived from its piers (no stored route name).

**Admin app**
- **D-18:** Admin UI is a **separate app `apps/admin`**: Next.js 16 App Router + TS + Tailwind + shadcn/ui + TanStack Query + an `apiFetch()` like `apps/web` (Phase 1 D-35 pattern: client-only fetching). **Thai only — no next-intl / no locale prefix.** Runs on its own port (e.g. `:3002`) through Kong; Kong CORS origins add the admin origin; cookies shared on localhost in dev. Role gating in the UI is cosmetic — backend enforces. — **Reversibility:** costly — separate app, CI job, Compose service, CORS/cookie config.
- **D-19:** Pier photo: browser uploads directly to object storage via a **presigned PUT URL** issued by catalog; catalog stores only the object key. jpeg/png/webp, ≤ 5 MB. **MinIO** in Compose for dev, S3-compatible in prod. Adds one container + an S3 client dependency to catalog.
- **D-20:** Maps: **MapLibre GL** with **OpenFreeMap** vector tiles (no API key); style URL configurable via env so the provider can be swapped. Admin map picker = click/drag marker to set lat/lng plus manual lat/lng inputs; no geocoding/address search in v1. (Supersedes the seed's "Leaflet/OSM" wording.)

**Public API**
- **D-21:** CAT-06 delivered as unauthenticated BFF routes under the existing Kong `/api/v1/public` route (no JWT plugin): list non-archived piers (id, names, lat/lng, photo URL) and routes (pier_from/pier_to ids, duration, current prices). No customer UI screen in this phase.

### Claude's Discretion
- Exact identity schema (users, operator membership, user_piers, otp tables), OTP hashing choice, Valkey key naming, refresh-token storage/rotation mechanics.
- connect-go service method shapes for identity and catalog, BFF REST route naming, pagination style.
- Admin UI layout: data tables vs drawer/dialog forms, navigation, empty states — can be refined by `/gsd-ui-phase 2`.
- Whether the admin app shares UI components with `apps/web` via copy or a small shared package (default: copy — no shared package until duplication hurts).
- Photo URL serving (public-read bucket vs presigned GET) as long as the public API returns a usable URL.

### Deferred Ideas (OUT OF SCOPE)
- Real SMS provider (ThaiBulkSMS/Twilio etc.) + SMS-pumping protection — before launch (Phase 7 / polish).
- Passkey/WebAuthn login for staff — Growth.
- Geocoding / address search in the map picker — later if admins ask.
- Per-weekday / holiday pier open hours — later.
- Immediate token revocation (denylist / session version) — only if 15 min proves too slow.
- Customer-facing map + search screens — Phase 4 (uses the D-21 public API).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AUTH-01 | Customer OTP login via email or phone, JWT httpOnly cookie, no password | Pattern 1 (OTP request/verify state machine), Standard Stack (Resend + net/smtp + Valkey), Pitfall 3 (OTP enumeration) |
| AUTH-02 | staff/pier_admin/super_admin log in, receive `role`+`operator_id` claims | D-05/D-06 already locked; Architecture Pattern 2 (Claims extension) reuses `pkg/auth` from Phase 1 |
| AUTH-03 | Kong verifies JWT; BFF forwards trusted `X-*` headers; services reject requests without them | Reuses Phase 1 Pattern 4 (Kong+BFF) and `pkg/httpx.RequireInternal` verbatim — only the header set grows (`X-Pier-Ids`) |
| AUTH-04 | super_admin creates pier_admin/staff users, assigns operator+piers | Architecture Pattern 3 (cross-service validation call: identity → catalog `GetPiers`) |
| AUTH-05 | Every admin query scoped by `operator_id` | Reuses Phase 1's `operator_id`-scoped-query convention (`services/catalog/internal/app/boat.go`), extended with `pier_ids` per D-07 |
| CAT-01 | super_admin creates/edits operators | Copy-the-boat-pattern (domain/app/postgres/http quadruplet) |
| CAT-02 | pier_admin CRUD piers (map picker, photo, open hours) | Standard Stack (MapLibre+OpenFreeMap, presigned upload), Pitfall 1 (MinIO archival) |
| CAT-03 | pier_admin CRUD routes (tiered cancellation policy) | D-13 jsonb validation rule — Code Examples section has a validator sketch |
| CAT-04 | pier_admin CRUD boats | Existing `Boat` aggregate + `home_pier_id` addition (D-07) |
| CAT-05 | pier_admin sets route_prices (adult/child, satang) | D-14 effective-dated pricing — Code Examples section has the "price in effect" query |
| CAT-06 | Public unauthenticated piers/routes listing | D-21 — reuses the existing `/api/v1/public/*` BFF pattern from `bff.go`'s `publicBoatsHandler` |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **ภาษา**: respond to the user in Thai; code/paths/commit messages/subagent prompts stay English (already followed in this document's prose; RESEARCH.md content itself is English per the task's `response_language` note).
- **ponytail full** on every code task: simplest working solution, YAGNI, stdlib/native before a new dependency, no unrequested abstractions. Applied throughout this research — e.g., recommending stdlib `net/smtp` over a third-party SMTP client, stdlib `crypto/hmac`/`crypto/sha256` over adding `golang.org/x/crypto/bcrypt` as a direct dependency, and vanilla `maplibre-gl` over the `react-map-gl` wrapper for a single map screen.
- Tech stack, architecture, security, observability, testing, and commit-scope constraints from the seed (Go 1.25+, chi, connect-go, sqlc+pgx, goose, franz-go, go-redis, slog, OTel; PostgreSQL 17 [pinned `postgres:17.9-alpine` in this repo]; Kong 3.9.1 pinned; database-per-service; outbox-everywhere; idempotent consumers; UTC-in-DB/Bangkok-display; money as integer satang; QR token hashed [Phase 5]; JWT httpOnly cookie via Kong; PDPA minimal PII; `.env` never committed; `trace_id` propagation; conventional commits scoped by service name) — all already enforced by the Phase 1 shared `pkg/*` this phase reuses. No new constraint conflicts found for Phase 2's scope.
- GSD workflow enforcement: file edits must flow through a GSD command (`/gsd-execute-phase` etc.) — not directly relevant to this research step but binding on the eventual planner/executor.

## Summary

Phase 2 has two vertical slices that share one trust boundary: **identity** (a brand-new service: OTP login, JWT issuance, staff/user management) and **catalog** (an existing service from Phase 1, extended from "boats only" to operators/piers/routes/boats/route_prices). Both slices are additive extensions of patterns Phase 1 already proved — no new architectural style is needed, only more instances of the same domain/app/postgres/http quadruplet `services/catalog/internal/{domain,app,adapters}` already demonstrates for `Boat`.

The highest-leverage reuse is `pkg/auth.Claims` and `pkg/httpx`'s claim-forwarding trust boundary (Pattern 4 from Phase 1 research, already implemented in `bff.go`/`claims.go`): this phase only *adds* a field (`PierIDs []string`) and a header (`X-Pier-Ids`), it does not redesign the mechanism. Similarly, `services/_template`'s one-binary-with-errgroup shape (`services/_template/cmd/main.go`) is copied verbatim by `make new-service identity`; identity needs no new runtime shape, only new env vars (`SUPER_ADMIN_EMAIL`, `RESEND_API_KEY`, `SMTP_HOST`, `VALKEY_ADDR`, `OTP_HASH_SECRET`) and one new external dependency (Valkey, via `go-redis/v9`) alongside the existing Postgres/Kafka wiring.

The one **genuine risk this research surfaced that CONTEXT.md's D-19 does not account for**: MinIO's community edition (the object-storage server D-19 names for the dev Compose stack) stopped publishing Docker images in October 2025 and had its GitHub repository formally archived on 2026-04-25 — it is dead upstream, not merely "old." The **client library** `minio-go` is unaffected (still actively released, v7.3.0 as of Aug 2026) and works unmodified against any S3-compatible endpoint, so nothing about the presigned-PUT *code* changes — only the *dev container* choice needs revisiting. See Pitfall 1 for the recommended swap (SeaweedFS's S3 gateway) and why it doesn't reopen D-19's actual decision (presigned PUT + S3-compatible storage), only its dev implementation detail.

**Primary recommendation:** Scaffold `identity` with `make new-service identity` and copy catalog's `Boat` quadruplet pattern for `Operator`/`Pier`/`Route`/`RoutePrice`; extend `pkg/auth.Claims`/`pkg/httpx.Claims` with `PierIDs` in place (do not create a v2 claims type); use `net/smtp` (stdlib) + Mailpit for the dev email sender and `resend-go/v4` for prod behind one `EmailSender` interface (per D-01); use `go-redis/v9` against the already-provisioned Valkey container for OTP counters; swap D-19's dev object-storage container from MinIO to SeaweedFS's S3 gateway (`minio-go` client unchanged) and flag this substitution to the user before locking it in, since it revises a locked decision's implementation detail.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| OTP request/verify, JWT issuance | API/Backend (identity) | Browser (OTP entry form in `apps/web`/`apps/admin`) | Passwordless auth logic, rate limiting, and token signing must live server-side; the browser only collects/submits the code |
| JWT verification at the edge | CDN/Gateway (Kong `jwt` plugin) | — | Kong OSS DB-less JWT plugin already proven in Phase 1 (D-28 spike passed) — unchanged in this phase |
| Claim → trusted header forwarding | Frontend Server/BFF (`services/gateway`) | — | `gateway` is the origin of the internal trust boundary (D-29/D-30) — every downstream service only ever reads headers, never JWTs, except identity/gateway themselves |
| Role/operator/pier scoping enforcement | API/Backend (every service's `app` layer) | — | Enforced in Go code against `httpx.FromContext(ctx)` claims, never trusted from the request body — same pattern as Phase 1's `UpsertBoat` |
| Operator/pier/route/boat/price CRUD | API/Backend (catalog) | Browser (`apps/admin` forms) | catalog owns all catalog state in its own Postgres DB; the admin app is a thin, client-only fetch layer with no business logic (D-18 pattern) |
| Pier photo upload | Browser (direct PUT to object storage) | API/Backend (catalog issues the presigned URL only) | The browser never proxies the file through catalog — catalog only mints a short-lived signed URL and later stores the resulting object key (D-19) |
| Map picker (lat/lng selection) | Browser (MapLibre GL client-side) | — | No geocoding/server round-trip in v1 (D-20) — purely a client-side widget |
| Public piers/routes listing | API/Backend (`gateway`'s `/api/v1/public/*`) | — | No customer UI in this phase (D-21) — the capability ends at the API contract |
| User/staff assignment (operator+piers) | API/Backend (identity, validated via a sync call to catalog) | — | identity does not have a local copy of catalog's operator/pier tables — it must validate referenced ids against catalog's connect-go API before persisting (see Architecture Pattern 3) |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | **v9.22.0** [VERIFIED: proxy.golang.org, published 2026-08-03] | OTP attempt/rate counters, resend cooldown (D-03) | The only maintained Go Redis client line; protocol-compatible with Valkey (already the project's chosen server) — no server-specific client needed |
| `github.com/resend/resend-go/v4` | **v4.7.0** [VERIFIED: proxy.golang.org, published 2026-09-25] | Prod transactional email sender (OTP codes) behind the project's `EmailSender` interface (D-01) | Official Resend Go SDK; `client.Emails.Send(params)` with `To`/`From`/`Subject`/`Html` fields is a two-line integration [CITED: resend.com/docs/send-with-go] |
| `net/smtp` (stdlib) | Go 1.25 stdlib | Dev email sender pointed at Mailpit (D-02) | Mailpit accepts unauthenticated SMTP on its dev listener; stdlib `net/smtp.SendMail` needs no third-party dependency for this one dev-only path — ladder rung 3 |
| `github.com/minio/minio-go/v7` | **v7.3.0** [VERIFIED: proxy.golang.org, published 2026-08-15] | Presigned PUT URL issuance for pier photos (D-19) | Despite the *server* project's troubles (Pitfall 1), the **client** SDK is a generic S3-API client, actively released, and works unmodified against any S3-compatible endpoint (MinIO, SeaweedFS, real AWS S3) |
| `maplibre-gl` (npm) | **6.11.2** [VERIFIED: npm registry] | Admin map picker + (later) customer route map (D-20) | Actively maintained OSS fork of the pre-license-change Mapbox GL JS; works directly against OpenFreeMap's vector tiles with no API key |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@tanstack/react-table` | **v8.21.3** — **pin the v8 line, not v9** [VERIFIED: npm registry] | Admin data tables (operators/piers/routes/boats/prices lists) | See Pitfall 2 — v9 (currently npm `latest`, v9.2.4) is a from-scratch rewrite (`useReactTable`→`useTable`, `flexRender`→a component) with no migration guide and no `next-intl`-style deprecation trail; every current shadcn/ui data-table tutorial, including the official one, is written against v8 |
| `testcontainers-go/modules/valkey` | **v0.44.0** [VERIFIED: proxy.golang.org — released in lockstep with the already-pinned `testcontainers-go` core v0.44.0] | Real Valkey in identity's integration tests | Matches the project's existing `modules/postgres`+`modules/redpanda` lockstep-versioning convention (`pkg/go.mod`) |
| `testcontainers-go/modules/mailpit` | **v0.44.0** [VERIFIED: proxy.golang.org — same lockstep line] | Real Mailpit in identity's integration tests, so `go test -tags=integration` can assert an OTP email was actually sent and read it back via Mailpit's HTTP API, independent of the dev Compose stack | Use for the OTP-email integration test; the dev Compose Mailpit container is for manual dev-loop testing, not CI |
| `testcontainers-go/modules/minio` (or a SeaweedFS generic container, see Pitfall 1) | **v0.44.0** [VERIFIED: proxy.golang.org — same lockstep line] | Real object storage in catalog's integration tests for the presigned-PUT round trip | Same lockstep-versioning rationale |
| Mailpit HTTP API (`GET /api/v1/messages`, `GET /view/latest.txt`) | Mailpit's own container (no Go/TS client library needed) | Reading the OTP out of the dev/test email without ever logging it | `fetch()`/`http.Get` directly against Mailpit's REST API is enough — no SDK exists or is needed [CITED: mailpit.axllent.org/docs/api-v1] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Vanilla `maplibre-gl` + a small React wrapper component | `react-map-gl` (v8.1.3) | `react-map-gl` adds a dependency and its own abstraction layer for exactly one map screen with click/drag-marker behavior — YAGNI per ponytail; add it only if Phase 4's customer map screen turns out to need many more map-driven features |
| `net/smtp` for the dev sender | A third-party SMTP client (e.g. `go-mail`) | Stdlib already does unauthenticated SMTP to a local Mailpit container — no TLS/auth complexity to justify a dependency |
| SeaweedFS S3 gateway (dev) | Pin a last-known MinIO image tag from before the Docker Hub delisting | Keeps D-19's literal wording but runs an archived, CVE-frozen image indefinitely for a container that will need to be replaced eventually anyway — only a stopgap, not a fix |
| `@tanstack/react-table` v8 | v9 (npm `latest`) | v9's rewrite is real but is the direction TanStack is going long-term; revisit the pin once official shadcn/ui docs and a stable migration guide exist for v9 |
| HMAC-SHA256 with a server-side pepper for OTP hashing | `golang.org/x/crypto/bcrypt` | bcrypt is the stronger textbook answer for password-shaped secrets, but OWASP's own OTP guidance (see Pitfall 4) notes hashing a 6-digit code offers weak offline-attack resistance either way given the ~1M-value keyspace — the pepper (a secret never stored in the DB) is what actually raises attacker cost for a DB-only compromise, and stdlib `crypto/hmac`+`crypto/sha256` gets there with zero new dependencies |

**Installation:**
```bash
# identity service (new module, add to services/identity/go.mod after make new-service)
go get github.com/redis/go-redis/v9@v9.22.0
go get github.com/resend/resend-go/v4@v4.7.0

# catalog service (existing module)
go get github.com/minio/minio-go/v7@v7.3.0

# pkg/ (shared testenv, dev-tool dependency only)
go get github.com/testcontainers/testcontainers-go/modules/valkey@v0.44.0
go get github.com/testcontainers/testcontainers-go/modules/mailpit@v0.44.0
go get github.com/testcontainers/testcontainers-go/modules/minio@v0.44.0   # or drop if SeaweedFS is chosen (see Pitfall 1) — testcontainers-go has no seaweedfs module; use a generic container.GenericContainer instead

# apps/admin (new app, scaffold like apps/web then add)
npm install maplibre-gl@6.11.2 @tanstack/react-table@8.21.3
```

**Version verification performed this session:**
- `go-redis/v9` — `proxy.golang.org/github.com/redis/go-redis/v9/@latest` → `v9.22.0`, 2026-08-03.
- `resend-go/v4` — `proxy.golang.org/github.com/resend/resend-go/v4/@latest` → `v4.7.0`, 2026-09-25.
- `minio-go/v7` — `proxy.golang.org/github.com/minio/minio-go/v7/@latest` → `v7.3.0`, 2026-08-15.
- `testcontainers-go/modules/{valkey,mailpit,minio}` — all report `v0.44.0`, matching the already-pinned core `testcontainers-go v0.44.0` in `pkg/go.mod` — confirms the lockstep-versioning convention holds for these three new modules too.
- `maplibre-gl` — `npm view maplibre-gl version` → `6.11.2`.
- `@tanstack/react-table` — `npm view @tanstack/react-table dist-tags` → `latest: 9.2.4`; `npm view @tanstack/react-table@8 version` → confirms `8.21.3` is the newest v8 release (still published, not deprecated).

## Package Legitimacy Audit

| Package | Registry | Age / Status | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `maplibre-gl` | npm | Established fork of Mapbox GL JS (pre-2020 lineage); `package-legitimacy check` flagged "too-new" only because of a same-day republish of the current version tag | 4.8M/week [VERIFIED: `gsd-tools query package-legitimacy check`] | `github.com/maplibre/maplibre-gl-js` | SUS (recency heuristic false-positive — see reasons) | Approved — treat as OK; the "too-new" signal is about the latest patch's publish timestamp, not the project's age or trust signals |
| `@tanstack/react-table` | npm | Long-established (TanStack/React Table lineage); same "too-new" republish signal | 19.6M/week [VERIFIED: `gsd-tools query package-legitimacy check`] | `github.com/TanStack/table` | SUS (recency heuristic false-positive) | Approved — pin the v8.21.3 line per Pitfall 2, not v9 |
| `github.com/redis/go-redis/v9` | Go proxy | Official `redis` GitHub org client, years of releases | N/A (Go modules have no npm-style download counts) | `github.com/redis/go-redis` | OK [ASSUMED: well-known official client, not run through the npm-only legitimacy checker — go/pypi/crates ecosystems aren't yet supported by `package-legitimacy check` in this environment] | Approved |
| `github.com/resend/resend-go/v4` | Go proxy | Official Resend SDK, active weekly-ish release cadence | N/A | `github.com/resend/resend-go` | OK [ASSUMED — same reasoning] | Approved |
| `github.com/minio/minio-go/v7` | Go proxy | Client SDK actively released independent of the server's troubles (Pitfall 1) | N/A | `github.com/minio/minio-go` | OK [ASSUMED — same reasoning] | Approved |

**Packages removed due to `[SLOP]` verdict:** none.
**Packages flagged as suspicious `[SUS]`:** `maplibre-gl`, `@tanstack/react-table` — both are recency-heuristic false positives (extremely high weekly download counts and long-established, well-known repos contradict a "new/hallucinated package" read). No `checkpoint:human-verify` is strictly required, but the planner may add one if the team wants a belt-and-suspenders confirmation before `npm install`.

*Go/PyPI packages could not be run through the automated npm-only `package-legitimacy check` tool in this environment; they were instead verified for existence/currency via `proxy.golang.org` (see Standard Stack) and are all official first-party SDKs for services already named in the locked decisions (Resend, Redis/Valkey, S3-compatible storage) — flagged `[ASSUMED]` per the provenance rule rather than `[VERIFIED]`, since registry existence alone doesn't confer verified status.*

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────┐
                         │   Browser (customer / staff / admin)    │
                         │  apps/web (TH/EN)   apps/admin (TH-only)│
                         └───────────────┬─────────────┬───────────┘
                                         │ OTP request/  │ presigned PUT
                                         │ verify, CRUD   │ (direct to storage)
                                         ▼                ▼
                         ┌──────────────────────┐   ┌─────────────────┐
                         │  Kong :8000           │   │ Object storage  │
                         │  /api  (JWT verify)   │   │ (S3-compatible, │
                         │  /api/v1/public (open)│   │  see Pitfall 1) │
                         └──────────┬────────────┘   └────────▲────────┘
                                    │ verified JWT cookie      │ (2) upload
                                    ▼                          │
                         ┌──────────────────────┐              │
                         │  gateway (BFF)        │              │
                         │  re-verify JWT →       │              │
                         │  X-User-Id/Operator-Id/│              │
                         │  Role/Pier-Ids headers │              │
                         └───┬───────────────┬────┘              │
             X-Internal-Token│               │X-Internal-Token   │
                    + claims │               │ + claims          │
                             ▼               ▼                    │
                  ┌───────────────┐   ┌───────────────────┐       │
                  │   identity     │   │     catalog        │      │
                  │ OTP/JWT/users  │◄──┤ operators/piers/   │      │
                  │ (own Postgres) │sync│ routes/boats/     │──────┘ (1) issue
                  │ Valkey (rate-  │val-│ route_prices      │  presigned URL
                  │  limit ctrs)   │id  │ (own Postgres)     │
                  └───────┬────────┘ate │        │           │
                          │ outbox   ids│        │ outbox     │
                          ▼             │        ▼            │
                  identity.events topic │  catalog.events topic
                  (UserCreated)         │  (OperatorUpserted, PierUpserted,
                                        │   RouteUpserted, BoatUpserted+home_pier_id,
                                        │   PriceChanged)
```

Trace the primary flows:
1. **Customer OTP login:** Browser → Kong `/api` (unauthenticated at this route since no cookie yet) → gateway → identity `RequestOTP`/`VerifyOTP` → identity mints JWT → cookie set on gateway's response → subsequent requests carry the cookie through Kong's `jwt` plugin.
2. **Staff/admin CRUD:** Browser (`apps/admin`) → Kong `/api` (JWT verified) → gateway (claims → headers) → catalog's connect-go handlers, each scoped by `operator_id`+`pier_ids` from `httpx.FromContext(ctx)`.
3. **Cross-service validation:** identity, when creating a staff user (AUTH-04), calls catalog's connect-go API directly (service-to-service, same internal-token trust boundary the gateway uses) to confirm the given `pier_ids` exist and belong to the given `operator_id` before writing the user row — see Pattern 3.
4. **Photo upload:** Browser asks catalog for a presigned URL (1), catalog mints it via `minio-go`/S3 client and returns it, browser PUTs the file directly to storage (2), never through catalog.
5. **Public listing:** Browser (later, Phase 4) or `curl` today → Kong `/api/v1/public` (no JWT) → gateway's public route handler → catalog `ListPiers`/`ListRoutes` with only the internal token, no claims — identical shape to Phase 1's `publicBoatsHandler`.

### Recommended Project Structure
```
services/
├── identity/                     # new — make new-service identity
│   ├── cmd/main.go                # copy of _template's run(); add Valkey client + email/SMS sender wiring
│   ├── internal/domain/
│   │   ├── user.go                # User{ID, Role, OperatorID, PierIDs, Email, Phone, DisabledAt}
│   │   └── otp.go                 # OTP{DestinationHash, CodeHash, ExpiresAt, Attempts}
│   ├── internal/app/
│   │   ├── otp.go                 # RequestOTP/VerifyOTP use-cases (Valkey rate limit + Postgres OTP row)
│   │   ├── user.go                # CreateStaffUser (calls catalog to validate pier_ids — Pattern 3)
│   │   └── refresh.go             # Refresh (re-reads role/operator_id/pier_ids/disabled, rotates token)
│   ├── internal/adapters/
│   │   ├── http/routes.go         # IdentityService connect-go handler
│   │   ├── postgres/               # sqlc-generated: users, otps, user_piers
│   │   ├── kafka/handlers.go       # Register (consumes nothing yet)
│   │   ├── email/                 # EmailSender interface + smtp.go (dev) + resend.go (prod)
│   │   ├── sms/                   # SMSSender interface + log.go (dev-only stub)
│   │   └── valkey/                # thin go-redis wrapper: rate-limit counters, resend cooldown
│   └── migrations/                 # 00001_platform.sql (copied) + 00002_users.sql + 00003_otps.sql
├── catalog/                        # existing — extend, don't re-scaffold
│   ├── internal/domain/
│   │   ├── boat.go                 # existing — add HomePierID field
│   │   ├── operator.go             # new
│   │   ├── pier.go                 # new — name_th/name_en, lat/lng, address, opens_at/closes_at, photo_key, archived_at
│   │   ├── route.go                 # new — pier_from/pier_to, duration, cancellation_policy jsonb
│   │   └── route_price.go           # new — route_id, ticket_type, amount_satang, effective_from
│   ├── internal/app/
│   │   ├── boat.go                  # existing — extend UpsertBoat for HomePierID + pier scoping
│   │   ├── operator.go, pier.go, route.go, route_price.go   # new — same UpsertX/ListX shape as boat.go
│   │   └── upload.go                 # new — PresignUpload (mints the S3 presigned PUT URL)
│   └── migrations/                   # 00003_operators.sql .. 00006_route_prices.sql, 00007_boats_home_pier.sql
└── gateway/
    └── internal/adapters/http/bff.go  # extend ForwardClaims with X-Pier-Ids; add /api/v1/public/piers, /api/v1/public/routes

apps/
├── web/                              # unchanged this phase (D-21: API only, no customer UI yet)
└── admin/                            # new — scaffold like apps/web minus next-intl
    └── src/
        ├── app/(routes per entity)/   # operators, piers, routes, boats, prices — list + form pages
        ├── components/
        │   ├── map-picker.tsx          # maplibre-gl wrapper, click/drag marker
        │   └── data-table.tsx           # @tanstack/react-table v8 wrapper, shared across entity list pages
        └── lib/api.ts                   # copy of apps/web's apiFetch()
```

### Pattern 1: OTP request/verify as a short-lived Postgres row + Valkey counters
**What:** `RequestOTP(destination)` generates a random 6-digit code, stores `sha256/hmac(code)` + `expires_at` in a Postgres `otps` row (not Redis — the code itself needs the durability/transactional guarantees of the same DB the user row lands in), while Valkey holds only the *counters* (attempts, resend cooldown, hourly send count) as short-TTL keys.
**When to use:** Any request/verify OTP flow where the code's write must be atomic with other Postgres state (e.g., auto-creating the user row on first successful verify, D-04).
**Example:**
```go
// Source: pattern derived from pkg/auth (Phase 1, RS256 Issuer/Verifier) +
// D-03's stated rules; OWASP guidance verified this session (see Pitfall 4).
func RequestOTP(ctx context.Context, tx pgx.Tx, rdb *redis.Client, destination string) error {
	destHash := hashDestination(destination) // sha256, not reversible — never store raw email/phone as a Valkey key in cleartext logs
	cooldownKey := "otp:cooldown:" + destHash
	hourlyKey := "otp:hourly:" + destHash

	if exists, _ := rdb.Exists(ctx, cooldownKey).Result(); exists == 1 {
		return domain.ErrResendTooSoon // 60s cooldown, D-03
	}
	count, err := rdb.Incr(ctx, hourlyKey).Result()
	if err != nil {
		return fmt.Errorf("app: incr hourly otp count: %w", err)
	}
	if count == 1 {
		rdb.Expire(ctx, hourlyKey, time.Hour)
	}
	if count > 5 {
		return domain.ErrRateLimited // 5/hour, D-03
	}

	code := generateCode() // crypto/rand, 6 digits — NOT math/rand
	codeHash := hmacOTP(code, otpPepper) // see "Alternatives Considered" for why HMAC over bcrypt

	if _, err := postgres.New(tx).InsertOTP(ctx, postgres.InsertOTPParams{
		DestinationHash: destHash,
		CodeHash:        codeHash,
		ExpiresAt:       pgtype.Timestamptz{Time: clock.Now().Add(5 * time.Minute), Valid: true},
	}); err != nil {
		return fmt.Errorf("app: insert otp: %w", err)
	}
	rdb.Set(ctx, cooldownKey, "1", 60*time.Second)

	return sender.Send(ctx, destination, code) // EmailSender or SMSSender, never logs `code`
}
```

### Pattern 2: Extending `pkg/auth.Claims` and `pkg/httpx.Claims` in place
**What:** D-06 requires `pier_ids` on every claim struct. Both existing structs (`pkg/auth/auth.go:34-39` `Claims{UserID, OperatorID, Role, Kind}`, `pkg/httpx/claims.go:20-24` `Claims{UserID, OperatorID, Role}`) get one new field (`PierIDs []string`), and the JWT wire format (`jwtClaims` at `pkg/auth/auth.go:41-46`) gets a `pier_ids` JSON field. The BFF's `ForwardClaims` (`services/gateway/internal/adapters/http/bff.go:159-169`) gets a fifth header set/deleted (`X-Pier-Ids`, comma-joined or JSON-array-encoded — planner's discretion) alongside the four it already handles.
**When to use:** This is the *only* correct way to add `pier_ids` — do not create a `ClaimsV2` type or a parallel struct; every existing caller of `auth.Claims`/`httpx.Claims` (both already covered by `auth_test.go`/`claims_test.go`) must keep compiling, which is the whole point of catching this early via the type system.
**Example (illustrative only — exact field/JSON names are Claude's Discretion per CONTEXT.md):**
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

### Pattern 3: Cross-service validation call (identity → catalog) for AUTH-04
**What:** identity's `CreateStaffUser(operatorID, pierIDs, ...)` cannot foreign-key-constrain against catalog's `operators`/`piers` tables (different Postgres database — database-per-service is absolute, per the seed's constraints). Before persisting the new user row, identity makes a synchronous connect-go call to catalog (e.g. a `ValidatePiers(operator_id, pier_ids) returns (ok bool)` RPC, or reuse a `ListPiers` call filtered by ids) using the same internal-token trust boundary that gateway uses when calling catalog — identity acts as its own "internal" caller here, not through gateway.
**When to use:** Any time one service's write needs to confirm a fact that only another service's database can answer. This is the same category of problem Phase 1's research already named (Pattern 4's "trusted internal caller"); the difference is direction — here it's service-to-service, not gateway-to-service, but the header contract (`X-Internal-Token` + no claims needed, since this is an internal system-to-system call) is identical to the pattern `publicBoatsHandler` already uses to call catalog with only the internal token.
**Example:**
```go
// Source: pattern derived from services/gateway/internal/adapters/http/bff.go's
// publicBoatsHandler (internal-token-only call, no user claims needed).
func (s *server) CreateStaffUser(ctx context.Context, req *connect.Request[identityv1.CreateStaffUserRequest]) (*connect.Response[identityv1.CreateStaffUserResponse], error) {
	validateReq := connect.NewRequest(&catalogv1.ValidatePiersRequest{
		OperatorId: req.Msg.OperatorId,
		PierIds:    req.Msg.PierIds,
	})
	validateReq.Header().Set(httpx.HeaderInternalToken, s.internalToken)
	resp, err := s.catalogClient.ValidatePiers(ctx, validateReq)
	if err != nil || !resp.Msg.Ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("one or more pier_ids do not belong to operator_id"))
	}
	// ... proceed to insert user + user_piers rows in one tx
}
```

### Anti-Patterns to Avoid
- **Storing the raw OTP code anywhere** (Postgres row, Valkey key, log line, span attribute): only the hash ever gets written down. This is an extension of Phase 1's existing `T-09-02`/PII-log threat mitigation, not a new concern.
- **Trusting `operator_id`/`pier_ids` from the request body on any write**: exactly Phase 1's Anti-Pattern 2, now with one more field. Every catalog write handler must read `operator_id`/`pier_ids` from `httpx.FromContext(ctx)`, never `req.Msg`.
- **Denormalizing catalog's operator/pier names into identity's JWT claims** "for convenience": claims carry ids only (`operator_id`, `pier_ids`) — names are looked up client-side via a sync call when needed for display, keeping the JWT small and avoiding a second source of truth for names that can change.
- **A parallel "admin" JWT verifier or a separate Kong route class for `apps/admin`**: `apps/admin` is just another cookie-carrying browser client behind the *same* Kong `/api` route and the *same* `pkg/auth.Verifier` — the only new Kong-level change is adding the admin origin to the existing CORS `origins` list (`deploy/kong/kong.yml`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Presigned upload URLs | A custom HMAC-signed-URL scheme for the object storage endpoint | `minio-go`'s `PresignedPutObject` (or the AWS SDK's `PutObject` presigner if the S3-compatible server prefers SigV4 via that path) | S3 presigned URLs already encode expiry, method, and a signature verified server-side by any S3-compatible implementation — reinventing this is a security-sensitive wheel with no upside |
| Map tile rendering / click-to-marker | A custom canvas/SVG map widget | `maplibre-gl`'s `Marker` class + `map.on('click', ...)` | MapLibre already handles pan/zoom/tile-loading/marker drag correctly across devices; a hand-rolled version would need to reimplement all of that for a strictly worse result |
| Admin table sorting/filtering/pagination | Custom `<table>` + manual `useState` sort/filter logic per entity page | `@tanstack/react-table` v8 (headless — pairs with shadcn's styled `<Table>` primitives) | Five near-identical admin list pages (operators/piers/routes/boats/prices) is exactly the "don't hand-roll it five times" case — one `data-table.tsx` wrapper reused across all five |
| OTP code generation | `math/rand` or string-concatenation of digits | `crypto/rand` via a small helper (`rand.Int(rand.Reader, big.NewInt(1_000_000))`, zero-padded to 6 digits) | `math/rand` is not cryptographically secure — an attacker who can predict the PRNG state predicts OTP codes; this is a real, well-known auth vulnerability class, not a style preference |
| Rate limiting / cooldown counters | A custom in-process map with mutexes (breaks across multiple replicas/restarts) | Valkey `INCR`+`EXPIRE` (already the D-03-mandated approach) | The project already provisions Valkey for this exact purpose; an in-process counter would silently reset on every deploy and not work at all once identity has more than one replica |
| JWT claim → header remapping | A custom Kong Lua plugin | The BFF's existing `ForwardClaims` (Phase 1 Pattern 4, already implemented) | Already decided and implemented in Phase 1 for exactly this reason — this phase only adds one more header, not a new mechanism |

**Key insight:** Every "don't hand-roll" item in this phase is either (a) a security-sensitive primitive (presigned URLs, crypto-random codes, rate limiting) where a custom version is a genuine vulnerability class, or (b) a UI-repetition problem (five CRUD tables, one map picker) where the existing ecosystem tool is strictly less code than N custom implementations. None of them are "add a framework you don't need" — consistent with ponytail's ladder.

## Common Pitfalls

### Pitfall 1: D-19 names MinIO for dev object storage, but MinIO's community edition is dead upstream
**What goes wrong:** A developer runs `docker pull minio/minio` (or `docker compose up` with a `minio/minio:latest` service) expecting the familiar community-edition dev container, and either gets a delisted-image pull failure or an image that will never receive another security patch.
**Why it happens:** MinIO stopped publishing Docker Hub images and pre-built binaries for the community edition in October 2025, then formally archived the `minio/minio` GitHub repository (read-only) on 2026-04-25 [CITED: itsfoss.com/news/minio-moves-away-from-open-source, stormdevelopments.ca/blog/minio-s-community-edition-is-archived-what-still-runs-in-2026 — two independently-authored sources corroborating the same timeline]. The project's own GitHub discussion threads (`minio/minio#21667`, `#21714`) confirm the maintainers' intent was to move commercial focus to "AIStor," leaving the OSS community edition frozen.
**How to avoid:** Swap the dev-container choice (only — not the `minio-go` client code, which is unaffected and still actively maintained) to an actively-maintained S3-compatible server. **Recommended: SeaweedFS's S3 gateway** — single binary, one Compose service, no license concerns: `docker run ... chrislusf/seaweedfs:<tag> server -s3 -dir=/data -ip.bind=0.0.0.0` [CITED: multiple 2026 "MinIO alternatives" comparison posts + SeaweedFS's own compose examples — cross-checked across independent sources]. `minio-go`'s `PresignedPutObject` works against SeaweedFS's S3 gateway with no code changes, since it's a generic S3-API client, not a MinIO-server-specific one. Because this revises the literal wording of a **locked** CONTEXT.md decision (D-19 names "MinIO" specifically), flag this substitution to the user for confirmation before the plan locks it in — see Assumptions Log A1.
**Warning signs:** `docker compose pull` failing on the object-storage service with an "image not found" or "manifest unknown" error; any CI step that tries to `docker pull minio/minio:latest` fresh (a cached local image would still work until someone does a clean pull).

### Pitfall 2: `@tanstack/react-table` v9 is npm's `latest` tag but is a breaking rewrite with no migration guide
**What goes wrong:** `npm install @tanstack/react-table` (no version pin) silently installs v9, and every tutorial/StackOverflow answer/the shadcn/ui docs themselves (`useReactTable`, `getCoreRowModel()`, `flexRender` as a function) fail to compile, because v9 renamed `useReactTable`→`useTable`, replaced the `get*RowModel` options with a declared `features` object, and turned `flexRender` into a component.
**Why it happens:** The v9 rewrite was never announced in TanStack's own changelog in a way most blog content picked up [CITED: WebSearch summary of the v9 API-shape change, cross-referenced against the still-v8-shaped official shadcn.io/ui.shadcn.com data-table examples found this session].
**How to avoid:** Pin `@tanstack/react-table` to `8.21.3` explicitly in `package.json` (`--save-exact`, matching this repo's existing convention for `apps/web`'s dependencies) rather than a `^8` or unpinned range.
**Warning signs:** TypeScript errors naming `useReactTable is not exported` or `flexRender is not a function` immediately after a fresh `npm install`.

### Pitfall 3: OTP request endpoint must not leak account existence
**What goes wrong:** If `RequestOTP(destination)` returns a different response (or timing) for "destination has an existing user" vs. "destination is unknown," an attacker enumerates which emails/phones are registered staff/customers.
**Why it happens:** D-04's auto-create-on-first-verify design makes it tempting to short-circuit `RequestOTP` for unknown destinations ("nothing to send OTP to yet") — but the OTP is sent to *any* destination equally; the account only gets created at *verify* time, not request time. As long as `RequestOTP` always sends a code and returns the same generic response regardless of whether a user record exists yet, there's no leak.
**How to avoid:** Implement `RequestOTP` to always insert an OTP row and always attempt delivery, keyed only by `destination`, with no early-return branch on "does a user exist for this destination." The user-existence check only happens inside `VerifyOTP`, and even there the response shape should not distinguish "new customer created" from "existing staff user matched" beyond what the JWT claims naturally carry.
**Warning signs:** Any `if user, ok := findUser(destination); !ok { return errNotFound }` branch inside the *request* (not verify) path.

### Pitfall 4: Hashing a 6-digit OTP code is weaker protection than it looks
**What goes wrong:** A developer assumes "we hash it, so it's secure at rest," but a 6-digit code has ~1,000,000 possible values — a database-only attacker with the hash can brute-force it in well under a second on ordinary hardware, hash or no hash, unless there's something else (a pepper, a secret the DB dump doesn't contain) raising the cost.
**Why it happens:** This is explicitly called out in OWASP's own guidance: "OTPs typically have a very small keyspace... which means a database attacker can brute-force any OTP hash quickly... hashing OTPs does not provide strong offline attack resistance in the way password hashing does" [CITED: OWASP ASVS/Cheat Sheet Series guidance, verified via WebSearch this session against `cheatsheetseries.owasp.org`/`OWASP/ASVS` GitHub sources].
**How to avoid:** Use HMAC-SHA256 with a server-side secret pepper (an env var, e.g. `OTP_HASH_SECRET`, never stored in the database) rather than a plain unsalted/unkeyed hash — this means a database-only compromise (SQL injection, backup leak) is *not* sufficient to brute-force codes; the attacker also needs the pepper, which lives only in the service's env/secret store. Combine with the already-locked D-03 controls (5-attempt lockout, 5-minute TTL, single-use) which bound the *online* brute-force window regardless of hashing strength — the pepper is specifically the *offline* (DB-dump) mitigation the online controls don't cover.
**Warning signs:** An `otps` table where `code_hash` is a plain `sha256(code)` with no secret input — recoverable by brute force from a DB dump alone in under a second.

### Pitfall 5: Adding an `address` field to a catalog *event* would trip the PII-shaped-field CI gate
**What goes wrong:** D-17 requires piers to have an `address` text field. If a future `PierUpserted` *event* (in `proto/events/catalog/v1/*.proto`) includes an `address` field, `make proto-check`'s `proto/pii-check.sh` will fail the build — its blacklist regex explicitly matches the whole-identifier `address` (`pkg/pii-check.sh:16`, pattern includes `|address|`).
**Why it happens:** The check is a blunt mechanical backstop (`proto/README.md`'s own words: "a backstop, not a substitute for reviewing new event fields"), and a business's mailing address is not personal data in the PDPA sense the guard is trying to catch, but the field name alone matches.
**How to avoid:** This is actually a non-issue *if* the planner follows D-21 literally — the public API (and by extension, any event a consumer would need) only requires `id, names, lat/lng, photo URL` for piers, not `address`. Keep `address` as a catalog-DB-only / sync-API-only field (`Pier` message in `proto/services/catalog/v1/catalog.proto`, not `proto/events/catalog/v1/*.proto`) and the CI gate never sees it. Flag this explicitly in the plan so a future contributor doesn't "helpfully" add `address` to an event payload.
**Warning signs:** `make proto-check` failing on a freshly-added `PierUpserted` event with `proto/pii-check.sh` output naming `address`.

### Pitfall 6: `pier_ids` scoping needs an explicit super_admin bypass, or every super_admin query breaks
**What goes wrong:** If every catalog/identity query naively filters `WHERE pier_id = ANY($pier_ids)`, a super_admin (whose claims carry an empty `pier_ids` per D-06) would see zero rows anywhere — the opposite of "super_admin bypasses operator/pier scope."
**Why it happens:** It's easy to write the scoping filter once and apply it uniformly without the `role == super_admin → skip the filter` branch CONTEXT.md's D-06 explicitly calls for.
**How to avoid:** Every scoped query function needs the same shape: `if claims.Role != "super_admin" { query = query.Where(pierIDIn(claims.PierIDs)) }`. Write one shared helper (e.g., in `catalog/internal/app`) that every entity's list/get function calls, rather than reimplementing the branch five times (operators/piers/routes/boats/prices) — this is also a "don't repeat a security-relevant branch five times" case, similar in spirit to the Don't-Hand-Roll table above.
**Warning signs:** An integration test logging in as `super_admin` and getting an empty list back from any catalog endpoint.

## Code Examples

### "Price in effect" query for a departure's local date (D-14)
```sql
-- Source: pattern derived from D-14's stated rule ("row with the latest
-- effective_from <= the departure's local (Asia/Bangkok) date").
-- name: GetEffectivePrice :one
select amount_satang
from route_prices
where route_id = $1
  and ticket_type = $2
  and effective_from <= $3  -- pass the departure's departure_date_local (date, not timestamptz)
order by effective_from desc
limit 1;
```

### Cancellation policy jsonb validation (D-13)
```go
// Source: pattern derived from D-13's stated validation rules; reuses
// pkg/money's existing rounding convention for refund math (not shown here,
// consumed later in Phase 4/5).
type CancellationTier struct {
	MinHoursBefore int32 `json:"min_hours_before"`
	RefundPercent  int32 `json:"refund_percent"`
}

func ValidateCancellationPolicy(tiers []CancellationTier) error {
	if len(tiers) == 0 {
		return fmt.Errorf("%w: cancellation policy must have at least one tier", domain.ErrInvalidArgument)
	}
	sawZero := false
	prev := int32(math.MaxInt32)
	for _, t := range tiers {
		if t.MinHoursBefore >= prev {
			return fmt.Errorf("%w: min_hours_before must strictly descend", domain.ErrInvalidArgument)
		}
		if t.RefundPercent < 0 || t.RefundPercent > 100 {
			return fmt.Errorf("%w: refund_percent must be 0-100", domain.ErrInvalidArgument)
		}
		if t.MinHoursBefore == 0 {
			sawZero = true
		}
		prev = t.MinHoursBefore
	}
	if !sawZero {
		return fmt.Errorf("%w: cancellation policy must include a 0-hour tier", domain.ErrInvalidArgument)
	}
	return nil
}
```

### Presigned PUT URL issuance (D-19)
```go
// Source: minio-go's PresignedPutObject signature, confirmed via
// pkg.go.dev/github.com/minio/minio-go/v7 this session (generic S3 client,
// works against any S3-compatible endpoint per Pitfall 1).
func PresignUpload(ctx context.Context, s3 *minio.Client, bucket, key string, contentType string) (string, error) {
	if !allowedContentTypes[contentType] { // jpeg/png/webp only, D-19
		return "", domain.ErrInvalidArgument
	}
	url, err := s3.PresignedPutObject(ctx, bucket, key, 10*time.Minute)
	if err != nil {
		return "", fmt.Errorf("app: presign upload: %w", err)
	}
	return url.String(), nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| MinIO as the default self-hosted S3-compatible dev/prod object store | SeaweedFS or Garage as the actively-maintained equivalent | Oct 2025 (Docker images stop) → Apr 2026 (repo archived) | Any project (including this one, via D-19) that assumed "MinIO in Compose" is still the default choice needs to re-verify that assumption in 2026 |
| `@tanstack/react-table` v8 (`useReactTable`) | v9 (`useTable`, features-object API) | v9 GA'd with docs rewritten Aug 2026 | Most existing tutorials/blog posts are now stale for anyone who installs unpinned; pin v8 explicitly until the ecosystem catches up |
| MinIO/AWS SDK presigned URLs as *the* upload pattern | Unchanged — still the standard pattern | N/A | Nothing to update here; this part of D-19 remains current best practice regardless of which server implements it |

**Deprecated/outdated:**
- MinIO community edition Docker images: no longer published; the GitHub source repo is archived/read-only. Treat any tutorial or blog post referencing `docker pull minio/minio` from before late 2025 as needing a substitution.
- The seed's "Leaflet/OSM" wording for maps: already superseded by CONTEXT.md's D-20 (MapLibre+OpenFreeMap) — this research confirms MapLibre is the current, actively-maintained choice, so no further action needed beyond what's already locked.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Swapping D-19's dev object-storage container from MinIO to SeaweedFS (or an equivalent actively-maintained S3-compatible server) is the right response to MinIO's archival, without reopening D-19's actual decision (presigned PUT + S3-compatible storage abstraction) | Pitfall 1, Alternatives Considered | If the user actually wants to keep pulling a frozen MinIO image (accepting the "no more security patches" tradeoff for a dev-only, non-internet-facing container), the planner should ask rather than silently swap — this is flagged precisely so that confirmation happens before the plan locks in a specific Compose service image |
| A2 | HMAC-SHA256 with an env-var pepper (`OTP_HASH_SECRET`) is an adequate OTP hashing scheme, deferring to D-03's already-locked online controls (5-attempt lockout, 5-min TTL, single-use) for the rest of the defense | Pattern 1, Pitfall 4, Alternatives Considered | If the team has a compliance requirement (e.g., a specific PDPA/financial-sector audit expectation) mandating a memory-hard KDF (bcrypt/argon2/scrypt) for *any* stored secret regardless of keyspace size, this recommendation under-delivers — flagged as `[ASSUMED]` for exactly this reason, per the provenance rule for compliance-adjacent claims |
| A3 | The dev-only `SMSSender` implementation has no test-visible way to retrieve the OTP code (unlike email, which Mailpit exposes via HTTP API) — CONTEXT.md's D-02 only describes an email test-harness, leaving the phone path effectively untested end-to-end in this milestone | Open Questions | If a phase requirement or UAT expects an automated integration test proving the *phone* OTP flow (not just email), the planner needs to design a test-only retrieval mechanism (e.g., a dev-only debug endpoint gated by `ENV=dev`) that CONTEXT.md doesn't currently specify |
| A4 | `X-Pier-Ids` as a comma-joined string (matching the existing single-value header convention of `X-User-Id`/`X-Operator-Id`/`X-Role`) is an acceptable wire format for the new claim header, rather than a JSON array or repeated headers | Pattern 2 | CONTEXT.md leaves the exact format to Claude's Discretion; if the planner picks a different encoding, every scoped query's header-parsing code must agree — flagged so the plan states the chosen format explicitly once, rather than each task guessing |

**If this table is empty:** not applicable — see entries above.

## Open Questions

1. **How does the dev/test suite verify the phone OTP path end-to-end, given D-02 only wires up Mailpit for email?**
   - What we know: D-01/D-02 describe an `SMSSender` interface with a "dev/log sender" as the only v1 implementation, and explicitly forbid ever logging the OTP code.
   - What's unclear: without a log line and without a Mailpit-equivalent capture point, there's no stated way for an integration test (or a human tester) to retrieve a phone-path OTP in dev.
   - Recommendation: the planner should either (a) scope automated tests to the email path only and treat phone as manually-verified/log-a-redacted-marker-not-the-code for this milestone, or (b) add a dev-only, `ENV=dev`-gated debug retrieval endpoint (not logging, an explicit query-by-destination endpoint) — flag this decision explicitly in the plan rather than leaving it implicit.

2. **Does `apps/admin`'s Kong route need its own JWT-plugin route, or does it share `/api` with `apps/web`?**
   - What we know: D-18 says the admin app runs on its own port through the *existing* Kong; role gating in the UI is "cosmetic — backend enforces."
   - What's unclear: whether `deploy/kong/kong.yml`'s existing single `/api` route (JWT plugin, `cookie_names: [access_token]`) is reused as-is (simplest — just add the admin origin to CORS) or whether a distinct route is added for clarity/observability (e.g., separate rate-limit tuning for admin traffic).
   - Recommendation: reuse the existing `/api` route (ladder rung 2: nothing new needed) — CORS origin list extension only. Revisit only if admin traffic ever needs different rate limits than customer traffic.

3. **Where does `ValidatePiers` (or equivalent) live in `catalog.proto`, and is it a new RPC or a filtered `ListPiers`?**
   - What we know: identity needs *some* sync call to confirm `pier_ids` belong to `operator_id` before creating a staff user (Pattern 3).
   - What's unclear: CONTEXT.md's "Claude's Discretion" list explicitly includes "connect-go service method shapes for identity and catalog" — this is squarely in that bucket, not something this research should lock down.
   - Recommendation: planner's call; either shape works, but whichever is chosen should return enough information (which of the requested ids don't exist / don't match) for `CreateStaffUser` to produce a specific `InvalidArgument` message, not just a boolean.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Valkey (already in `deploy/docker-compose.yml`) | identity's OTP rate-limit counters (D-03) | ✓ (provisioned in Phase 1, unused until now) | `valkey/valkey:9.0.6-alpine` | — |
| Mailpit | Dev/test email delivery + OTP readback (D-02) | ✗ — not yet in `deploy/docker-compose.yml` | — | Add as a new Compose service (`axllent/mailpit`, tag verified: recent `edge`/versioned tags actively published) — no fallback needed, this is a net-new addition this phase |
| MinIO or an S3-compatible replacement | Pier photo presigned upload (D-19) | ✗ — not yet in `deploy/docker-compose.yml`; **and the named tool (MinIO) is upstream-archived (Pitfall 1)** | — | SeaweedFS S3 gateway (`chrislusf/seaweedfs`) recommended; confirm with the user before locking the image tag |
| MapLibre GL / OpenFreeMap | Admin map picker (D-20) | ✓ — client-side npm package + a public, keyless tile CDN; no local infra dependency | `maplibre-gl@6.11.2` | — |
| Resend API (prod email) | Prod OTP email delivery (D-01) | External SaaS — not locally verifiable in this research session | — | Dev/test unaffected (uses Mailpit); prod requires a real `RESEND_API_KEY` at deploy time, same pattern as any other prod secret in `.env` |

**Missing dependencies with no fallback:** none — every gap above has a stated path forward.
**Missing dependencies with fallback:** Mailpit and the object-storage container both need to be *added* to `deploy/docker-compose.yml` and `deploy/services.txt`-equivalent wiring this phase (net-new infra, not a broken existing dependency).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib) for Go, `//go:build integration` tag convention for testcontainers tests — unchanged from Phase 1 (`01-RESEARCH.md`'s Validation Architecture section) |
| Config file | none — same as Phase 1 |
| Quick run command | `go test ./...` (matches `make test`) |
| Full suite command | `go test -tags=integration ./...` (matches `make test-integration`) — will now also spin up Valkey/Mailpit/object-storage testcontainers for identity's and catalog's new integration tests |

Frontend (`apps/admin`): same Phase-1-established gate — `tsc --noEmit`/`eslint`/`next build` — no test framework is specified by CONTEXT.md for the admin app beyond that. **Wave 0 gap:** if the plan wants an automated frontend check for the admin CRUD flows or the map picker beyond lint/build/typecheck, no Playwright/Vitest setup exists yet in this repo (same gap Phase 1's research already flagged and left open).

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-01 | Customer requests OTP via email, receives cookie session on verify | integration | `go test -tags=integration ./services/identity/... -run TestRequestAndVerifyOTP` (reads the code back via the Mailpit testcontainer's HTTP API) | ❌ Wave 0 — `services/identity` doesn't exist yet |
| AUTH-02 | staff/pier_admin/super_admin login yields correct `role`+`operator_id` claims | unit | `go test ./pkg/auth/...` (extend existing `auth_test.go`) | ✅ file exists, extend it |
| AUTH-03 | Services reject requests missing gateway headers | unit | `go test ./pkg/httpx/...` (extend existing `claims_test.go` for the new `X-Pier-Ids` header) | ✅ file exists, extend it |
| AUTH-04 | super_admin creates pier_admin/staff, assigned operator+piers | integration | `go test -tags=integration ./services/identity/... -run TestCreateStaffUserValidatesPiers` (needs a real catalog + real identity DB — cross-service test, same shape as Phase 1's `TestUpsertBoatPublishesBoatUpserted`-style cross-service assertions) | ❌ Wave 0 |
| AUTH-05 | Every admin query scoped by `operator_id` | integration | `go test -tags=integration ./services/catalog/... -run TestListPiersScopedByOperator` | ❌ Wave 0 — extends catalog's existing integration suite |
| CAT-01..CAT-05 | Operators/piers/routes/boats/prices CRUD, archive, scoping | integration | `go test -tags=integration ./services/catalog/...` (one test file per entity, following `cmd/main_integration_test.go`'s existing boat pattern) | ❌ Wave 0 — new entities, existing test file to pattern-match against |
| CAT-06 | Public unauthenticated piers/routes listing | integration | `go test -tags=integration ./services/gateway/...` (extend existing gateway integration test for the new `/api/v1/public/piers` and `/api/v1/public/routes` routes) | ❌ Wave 0 — gateway integration test file exists per Phase 1, extend it |

### Sampling Rate
- **Per task commit:** `go test ./...` (unit only, no Docker)
- **Per wave merge:** `go test -tags=integration ./...` (full suite, real Postgres+Redpanda+Valkey+Mailpit+object-storage testcontainers)
- **Phase gate:** Full suite green + `make lint` + `apps/admin`'s `tsc --noEmit`/`eslint`/`next build` + a manual OTP-login round trip (curl or Playwright, reading the code from the dev Mailpit UI) before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `services/identity/**` — the entire service; every identity integration test target depends on this existing first (scaffold via `make new-service identity`)
- [ ] `deploy/docker-compose.yml` needs a `mailpit` service and an object-storage service (name pending Pitfall 1's resolution) added, plus `deploy/services.txt`/Compose wiring for `identity`'s DB+topic
- [ ] `proto/services/identity/v1/identity.proto` + `proto/events/identity/v1/user.proto` — proto-gen has nothing to generate from yet for identity
- [ ] `services/catalog/internal/{domain,app,adapters}` — new files for `Operator`/`Pier`/`Route`/`RoutePrice`, following the existing `boat.go` pattern exactly (Wave 0 in the sense that no test can target these until the files exist, but the *pattern* to copy is fully proven by Phase 1)
- [ ] `apps/admin/**` — the entire app; scaffold before any admin-UI-facing check can run

## Security Domain

`security_enforcement` is enabled (ASVS level 1, block on `high`) per `.planning/config.json` — same as Phase 1.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Yes — this is the phase that implements it | 6-digit OTP via `crypto/rand`, HMAC-SHA256+pepper hash at rest, 5-min TTL, 5-attempt lockout, 5/hour send limit, 60s resend cooldown (all D-03, already locked) |
| V3 Session Management | Yes | Access JWT 15min/refresh 30d unchanged from Phase 1; refresh endpoint now re-reads `role`/`operator_id`/`pier_ids`/`disabled` from identity's DB every rotation (D-10) — this is the actual *session revocation* mechanism in the absence of a denylist |
| V4 Access Control | Yes — expanded scope | `operator_id`+`pier_ids` claim-based scoping in every catalog/identity query, enforced server-side only (never trust the request body) — direct extension of Phase 1's Anti-Pattern 2 mitigation |
| V5 Input Validation | Yes | Cancellation-policy jsonb shape validation (D-13, Code Examples), route_price satang validation (`pkg/money`), presigned-upload content-type/size allowlist (jpeg/png/webp, ≤5MB, D-19) |
| V6 Cryptography | Yes | RS256 JWT signing unchanged from Phase 1 (`pkg/auth`); new: OTP hash pepper must be a real secret (env var, never committed, never logged) — same `.env`-not-committed discipline Phase 1 already established for the RSA private key |
| V12 Files and Resources | Yes — new this phase | Presigned PUT URLs scoped to one object key, short expiry (recommend ≤10 min, well under S3's 7-day max), content-type allowlist enforced server-side before minting the URL (never trust the browser's declared `Content-Type` alone — S3-compatible servers can be configured to enforce the signed content-type, verify this at plan time) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| OTP brute force (online) | Tampering/EoP | 5-attempt lockout + 5-min TTL (D-03, already locked) |
| OTP brute force (offline, DB dump) | InfoDisc → EoP | HMAC-SHA256 + server-side pepper (Pitfall 4) — the pepper, not the hash algorithm alone, is what raises attacker cost |
| Account/destination enumeration via `RequestOTP` | InfoDisc | Uniform response regardless of destination's existing-user status (Pitfall 3) |
| Cross-tenant data access (IDOR) via missing `pier_ids`/`operator_id` filter | EoP | Shared scoping helper applied to every catalog/identity query (Pitfall 6) — same category as Phase 1's `T-10-01` |
| super_admin lockout via naive scoping filter | DoS (self-inflicted) | Explicit `role == super_admin` bypass branch in the shared scoping helper (Pitfall 6) |
| Presigned URL abuse (oversized/wrong-type upload, object-key path traversal) | Tampering/DoS | Content-type+size allowlist enforced server-side before minting the URL; object keys generated server-side (e.g. `uuid.NewV7()`-derived), never taken from client input verbatim |
| Stale/forged JWT `pier_ids` array after a staff reassignment | Tampering/EoP | Bounded by the existing 15-min access-token TTL + D-10's refresh-time re-read from the DB — same mitigation shape as Phase 1's revocation-latency design, just re-affirmed for the new claim |

## Sources

### Primary (HIGH confidence)
- `proxy.golang.org` — direct version/publish-date lookups for `redis/go-redis/v9`, `resend/resend-go/v4`, `minio/minio-go/v7`, `testcontainers-go/modules/{valkey,mailpit,minio}` (this session).
- `npm` registry (`npm view`) — direct version lookups for `maplibre-gl`, `@tanstack/react-table` (both `latest` and the pinned v8 line) (this session).
- Existing repo source read directly this session: `pkg/auth/auth.go`, `pkg/httpx/claims.go`, `pkg/httpx/health.go`, `services/gateway/internal/adapters/http/bff.go`, `services/catalog/internal/{domain,app,adapters}/*`, `deploy/kong/kong.yml`, `deploy/docker-compose.yml`, `proto/events/catalog/v1/boat.proto`, `proto/services/catalog/v1/catalog.proto`, `proto/README.md`, `proto/pii-check.sh`, `pkg/events/events.go`, `services/_template/cmd/main.go`, `services/gateway/cmd/main.go`, `apps/web/src/lib/api.ts`, `apps/web/package.json`.
- `gsd-tools query package-legitimacy check` — `maplibre-gl`, `@tanstack/react-table` (this session).

### Secondary (MEDIUM confidence)
- [Resend Go SDK docs](https://resend.com/docs/send-with-go) — Send API shape.
- [Mailpit API v1 docs](https://mailpit.axllent.org/docs/api-v1/) — message retrieval shape.
- [MapLibre GL JS docs](https://maplibre.org/maplibre-gl-js/docs/) + [OpenFreeMap quick start](https://openfreemap.org/quick_start/) — style URL + marker API.
- [it's FOSS — MinIO moves away from open source](https://itsfoss.com/news/minio-moves-away-from-open-source/) and [Storm Developments — MinIO's community edition is archived](https://stormdevelopments.ca/blog/minio-s-community-edition-is-archived-what-still-runs-in-2026/) — cross-checked independent sources for the MinIO archival timeline (Pitfall 1).
- [OWASP Multifactor Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html) + [OWASP/ASVS GitHub](https://github.com/OWASP/ASVS) — OTP hashing/rate-limiting guidance (Pitfall 4).
- SeaweedFS Compose/S3-gateway examples (multiple 2026 "MinIO alternatives" comparison posts, cross-referenced) — Pitfall 1 recommendation.

### Tertiary (LOW confidence)
- WebSearch summary characterizing `@tanstack/react-table` v9's API-shape change (no single canonical migration-guide source found, but corroborated by the still-v8-shaped official examples observed directly on `ui.shadcn.com`/`shadcn.io`) — Pitfall 2.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version number was checked against `proxy.golang.org`/`npm` this session, not recalled from training data.
- Architecture: HIGH — every pattern in this document is a direct, verified extension of code already read this session (`pkg/auth`, `pkg/httpx`, `bff.go`, `boat.go` quadruplet), not a novel design.
- Pitfalls: HIGH for Pitfall 1 (MinIO archival, two independent corroborating sources) and Pitfall 5 (verified by reading `proto/pii-check.sh` directly); MEDIUM for Pitfalls 2–4 and 6 (well-supported by search/OWASP but not independently reproduced in this environment, e.g. no actual `npm install @tanstack/react-table` was run to reproduce the v9 compile error).

**Research date:** 2026-09-26
**Valid until:** ~2026-10-26 (30 days) for version pins; the MinIO/Pitfall 1 finding should be re-confirmed at plan time if execution is delayed more than a few weeks, since it's a fast-moving/recent situation.
