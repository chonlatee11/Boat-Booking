# Roadmap: Boat-Booking

## Overview

Five phases deliver Milestone 1 end-to-end: จอง → จ่าย → ตั๋ว. The chain is dependency-forced, not stylistic — Phase 1 builds the shared service template and event plumbing every later service copies; Phase 2 stands up identity and the catalog data (operators/piers/routes/boats) everything downstream references; Phase 3 turns catalog data into real departures; Phase 4 proves the no-overbook core value under concurrency against those departures; Phase 5 closes the loop with real payment, ticket issuance, notification, and production deploy. Each phase is a vertical slice — something a human can actually use or verify — never a horizontal layer.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

- [ ] **Phase 1: Platform Foundation** - Service template, shared pkg/*, dev stack, and one proven event flow with cross-service tracing
- [ ] **Phase 2: Identity + Catalog** - OTP login with roles/scoping, and admin-managed piers/routes/boats/prices customers can browse
- [ ] **Phase 3: Schedule** - Schedule templates generate real departures that admins manage and downstream services consume as events
- [ ] **Phase 4: Booking Core** - Search → hold seats → mock checkout, with no-overbook guaranteed under concurrency (CI-proven)
- [ ] **Phase 5: Payment + Ticket + Notification** - Real payment, QR ticket issuance, email notification, full booking saga, and production deploy

## Phase Details

### Phase 1: Platform Foundation

**Goal**: A developer can scaffold a new service from a shared template, run the full local stack, and see one real event flow end-to-end with cross-service tracing
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: PLAT-01, PLAT-02, PLAT-03, PLAT-04, PLAT-05, PLAT-06, PLAT-07, PLAT-08, PLAT-09, PLAT-10
**Success Criteria** (what must be TRUE):

  1. Developer runs `make new-service <name>` and gets a working service (cmd/, internal/{domain,app,adapters}, migrations/, CLAUDE.md, /healthz + /readyz; image from the root Dockerfile) wired to shared `pkg/*`
  2. Developer runs `make up` to start the full stack (Redpanda, Postgres 17, Valkey, Kong 3.9.1 DB-less, Grafana Tempo/Loki/Prometheus); `make proto-gen` generates committed Go + TS clients from buf schemas
  3. A state change written via transactional outbox in one service is applied exactly once in another service (`processed_events`), visible as a single trace spanning HTTP + Kafka in Tempo with structured slog JSON logs carrying `trace_id`
  4. Failed event processing retries 3x with backoff then lands in `<topic>.dlq` with error metadata; consumer offsets commit only after successful apply
  5. Jenkins CI builds every service image and runs unit + integration tests (testcontainers) on every push; Next.js skeleton (i18n TH/EN, mobile-first, Thai-friendly font) calls the backend only through Kong with a verified JWT round-trip (Traefik fallback decided if Kong spike fails)

**Plans**: 14/14 plans executed
**UI hint**: yes

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Kong edge spike (RS256 JWT round-trip, Traefik fallback) + workspace spine + lint/pre-commit baseline (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — Proto toolchain: Envelope, BoatUpserted, CatalogService, `make proto-gen` / `proto-check` (wave 2)
- [x] 01-03-PLAN.md — `pkg/clock` (Bangkok local date) + `pkg/money` (satang, rounding rule) (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-04-PLAN.md — Outbox → Redpanda publish slice with traceparent, relay behaviour, testcontainers env (wave 3)
- [x] 01-05-PLAN.md — Next.js 16 web skeleton (TH/EN, Thai font, TanStack Query → Kong) — package legitimacy checkpoint (wave 3)
- [x] 01-06-PLAN.md — Jenkins controller + SSH agent + Harbor CI stack, Jenkinsfile (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-07-PLAN.md — Consumer exactly-once (processed_events), retries 1s/5s/25s → DLQ (wave 4)
- [x] 01-08-PLAN.md — Observability stack: Collector → Tempo/Loki/Prometheus → Grafana + platform dashboard (wave 4)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01-09-PLAN.md — Service template runtime (health, readiness, ordered shutdown, OTel/slog) + sample slice (wave 5)

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 01-10-PLAN.md — `make new-service` + catalog write side (UpsertBoat → BoatUpserted), template smoke image (wave 6)

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 01-11-PLAN.md — schedule consumes BoatUpserted exactly once + gateway BFF re-scaffolded from template (wave 7)

**Wave 8** *(blocked on Wave 7 completion)*

- [x] 01-12-PLAN.md — Full stack `make up` + `make proof` (single trace, exactly-once) + developer inner loop (wave 8)

**Wave 9** *(blocked on Wave 8 completion)*

- [x] 01-13-PLAN.md — `make ci` = Jenkins pipeline, changed-services scoping, push to Harbor only on main (wave 9)

**Gap closure**

- [x] 01-14-PLAN.md — G-01-7: Tempo → Loki "Logs for this span" ±1m window (spanStartTimeShift/spanEndTimeShift)

### Phase 2: Identity + Catalog

**Goal**: Customers and staff can authenticate with correct roles/scoping, and pier_admin can manage the catalog data (piers/routes/boats/prices) that customers browse publicly
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, CAT-01, CAT-02, CAT-03, CAT-04, CAT-05, CAT-06
**Success Criteria** (what must be TRUE):

  1. Customer requests an OTP via email or phone and receives a session (JWT in httpOnly cookie) without creating a password account
  2. staff / pier_admin / super_admin log in and receive `role` + `operator_id` claims; Kong verifies the JWT and the BFF forwards claims as trusted headers; requests missing those headers are rejected by services
  3. super_admin creates pier_admin / staff users assigned to an operator and pier; every admin query is scoped by `operator_id` so a pier_admin never sees another operator's data
  4. super_admin creates/edits operators and creates piers; pier_admin edits/archives the piers assigned to them (with map picker) and creates/edits/archives routes (with tiered cancellation policy), boats, and per-route ticket prices (adult/child, integer satang) via the admin UI (wording corrected per 02-CONTEXT D-08)
  5. Public search lists piers and routes with coordinates for the map, without authentication

**Plans**: 14/17 plans executed (4 gap-closure plans pending)
**UI hint**: yes

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — Storage + package decisions (checkpoints) and pier_ids claim end-to-end through Kong/BFF/RequireInternal (D-06)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — identity service: email/phone OTP (Valkey, Mailpit/Resend), auto-created customers, UserCreated, JWT + refresh token
- [x] 02-03-PLAN.md — catalog operators + piers with the shared Scope rule (super_admin / pier_admin / staff) and public pier list
- [x] 02-04-PLAN.md — gateway generic admin RPC proxy + claim-less public proxy (replaces per-endpoint BFF handlers)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-05-PLAN.md — edge sessions: /api/v1/auth/* cookie routes, Kong api-auth route, refresh rotation + reuse detection, super_admin bootstrap
- [x] 02-06-PLAN.md — catalog routes, tiered cancellation policy, effective-dated prices, public routes with current prices, pier/route archive (D-15)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 02-07-PLAN.md — identity UserService: super_admin manages staff/pier_admin users with catalog-validated piers, disable/re-enable
- [x] 02-08-PLAN.md — catalog boats under home-pier scope + archive, pier photo presign (D-19), proof/roundtrip updated
- [x] 02-09-PLAN.md — apps/admin scaffold (Thai, :3002): OTP login, role-aware shell, shared DataTable, operators CRUD, CI/Compose wiring
- [x] 02-10-PLAN.md — apps/web customer OTP login (TH/EN), signed-in header, silent refresh

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 02-11-PLAN.md — admin Piers page: MapLibre picker, direct-to-storage photo upload (storage container), archive with D-15 blocked state
- [x] 02-12-PLAN.md — admin Routes (policy editor, return route, prices) and Boats pages
- [x] 02-13-PLAN.md — admin Staff page: create/edit/disable staff users

**Gap closure** *(UAT 02-UAT.md, all wave 1, parallel)*

- [x] 02-14-PLAN.md — G-02-3: OTP SMTP message UTF-8 MIME headers + RFC 2047 Subject (identity notify)
- [ ] 02-15-PLAN.md — G-02-8 (backend), G-02-18: archived-operator pier-create message + concurrent UpsertBoat integration test (catalog)
- [ ] 02-16-PLAN.md — G-02-7, G-02-8 (UI): map-picker validated lat/lng + tile-failure reset; pier Sheet non-archived operators, 24h HH:MM hours
- [ ] 02-17-PLAN.md — G-02-12, G-02-9: policy editor 0-default + last-tier guard; same-named pier label disambiguation; proof.sh unique pier name

### Phase 3: Schedule

**Goal**: pier_admin can define recurring schedules and turn them into real departures that admins manage and downstream services consume as events
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: SCHED-01, SCHED-02, SCHED-03, SCHED-04, SCHED-05, SCHED-06, SCHED-07
**Success Criteria** (what must be TRUE):

  1. pier_admin creates schedule templates (days of week, departure time, boat, capacity, effective date range)
  2. pier_admin generates departures from templates N days ahead; re-running the generator does not duplicate departures
  3. pier_admin views departures in calendar and list views, filterable by pier/route/date
  4. pier_admin edits a departure's time/boat/capacity — a capacity reduction below `booked + held` is rejected synchronously (`CheckCapacityReducible`) — and can close a departure for booking, cancel it, or add an ad-hoc special departure
  5. Departures store `departure_date_local` (Asia/Bangkok) alongside the UTC timestamp; schedule publishes `DepartureCreated/Updated/CapacityChanged/Cancelled` keyed by `departure_id`

**Plans**: TBD
**UI hint**: yes

Plans:

- [ ] 03-01: TBD

### Phase 4: Booking Core

**Goal**: A customer can search, hold seats, and reach a mock checkout, with no-overbook guaranteed under real concurrency
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: BOOK-01, BOOK-02, BOOK-03, BOOK-04, BOOK-05, BOOK-06, BOOK-07, BOOK-08, BOOK-09, BOOK-10
**Success Criteria** (what must be TRUE):

  1. booking-service maintains `departure_inventory (capacity, booked, held)` projected from schedule events, enforced by a DB `CHECK (booked + held <= capacity)` constraint
  2. Customer searches by origin pier, destination pier, date, and passenger count and sees departures (time, boat, price, seats-left badge green/orange/red) in a card list + map split on desktop and a list/map toggle on mobile
  3. Customer views departure detail (adult/child selector, total price, cancellation policy) and holds N seats for 10 minutes via `CreateBooking`; the hold is rejected when `available < N`
  4. Expired holds release seats automatically via Redis TTL trigger plus a DB cron sweep, both calling the same idempotent "expire if still pending and past due" SQL; checkout shows a 10-minute countdown, contact form, and price summary with mock payment confirm
  5. 50 concurrent `CreateBooking` calls on a 10-seat departure yield exactly 10 bookings, and concurrent capacity reduction during active bookings never yields `booked + held > capacity` — both integration tests run in CI; booking publishes `BookingCreated/Confirmed/Expired/Cancelled/AvailabilityChanged` with no PII in payloads

**Plans**: TBD
**UI hint**: yes

Plans:

- [ ] 04-01: TBD

### Phase 5: Payment + Ticket + Notification

**Goal**: A customer completes a real payment, receives a QR ticket by email, and can look it up again — the full booking saga works end-to-end including the late-payment-after-expiry race — and the stack runs in production
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: PAY-01, PAY-02, PAY-03, PAY-04, PAY-05, PAY-06, TKT-01, TKT-02, TKT-03, TKT-04, TKT-05, NOTF-01, NOTF-02, DEP-01, DEP-02
**Success Criteria** (what must be TRUE):

  1. Customer pays via PromptPay QR (with polling status update) or credit/debit card through a real provider (Opn or 2C2P, chosen via sandbox spike at plan time), with a mock provider available for dev/testing
  2. Provider webhooks are signature-verified, deduplicated by a unique provider-reference constraint, and status is re-fetched from the provider before publishing `PaymentSucceeded`; `BookingExpired` cancels the unpaid payment intent; a late `PaymentSucceeded` arriving after hold expiry re-holds seats when available or auto-refunds otherwise, in one tx — covered by an integration test for this race
  3. On `BookingConfirmed`, ticket-service issues one ticket with a random 32-byte token (hash stored, no PII) and QR image; customer views the ticket page with a large sun-readable QR and save-as-image; `ValidateTicket` is atomic (valid → used); pier_admin views a read-only passenger list per departure
  4. Customer looks up My Bookings via login or booking reference + email; on `TicketIssued`, notification-service emails the ticket (recipient fetched via sync call, never from the event) idempotently via `notification_log`, and the customer can request a resend from the ticket page
  5. `docker-compose.prod.yml` runs the full stack on one EC2 with production Redpanda settings and a documented Postgres backup step; Jenkins pushes images to Harbor and deploys via `docker compose pull && up -d`

**Plans**: TBD
**UI hint**: yes

Plans:

- [ ] 05-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Platform Foundation | 14/14 | In Progress|  |
| 2. Identity + Catalog | 14/17 | In Progress|  |
| 3. Schedule | 0/? | Not started | - |
| 4. Booking Core | 0/? | Not started | - |
| 5. Payment + Ticket + Notification | 0/? | Not started | - |
