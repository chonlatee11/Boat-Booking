# Requirements: Boat-Booking

**Defined:** 2026-09-25
**Core Value:** ลูกค้าจองจากมือถือ → จ่าย → โชว์ QR ที่ท่าได้ โดยระบบไม่ overbook เด็ดขาด

## v1 Requirements

Milestone 1 = seed Phase 0–4: platform foundation → identity + catalog → schedule → booking core → payment + ticket + notification. Each maps to roadmap phases.

### Platform Foundation

- [x] **PLAT-01**: Developer can scaffold a service with `make new-service <name>` producing `cmd/`, `internal/{domain,app,adapters}`, `migrations/`, `CLAUDE.md`, `/healthz` + `/readyz`, wired to shared `pkg/*`; images build from the single root `Dockerfile` (`ARG SERVICE`, per D-37)
- [x] **PLAT-02**: Shared `pkg/{events,kafka,outbox,httpx,auth,pgx}` exist and are the only way services touch Kafka, outbox, auth, and Postgres
- [x] **PLAT-03**: Developer can run `make up` to start Redpanda + Postgres 17 (one instance, DB per service via DSN; D-18) + Valkey + Kong 3.9.1 (DB-less) + Grafana Tempo/Loki/Prometheus + all services via Docker Compose
- [x] **PLAT-04**: Developer can run `make proto-gen` (buf) to generate Go + TS from `proto/events/` and `proto/services/` (connect-go); generated code is committed
- [x] **PLAT-05**: A state change written in one service reaches another service via transactional outbox relay (ordered per `aggregate_id`, at-least-once) and is applied exactly once via `processed_events` in the same tx — proven with one real end-to-end event in Phase 0
- [x] **PLAT-06**: A `trace_id` propagates HTTP → outbox row → Kafka headers → consumer and shows as a single trace in Tempo; logs are structured slog JSON with `trace_id`
- [x] **PLAT-07**: Failed event processing retries 3× with backoff then lands in `<topic>.dlq` with error metadata; consumer offsets commit only after successful apply
- [x] **PLAT-08**: Jenkins CI builds every service image and runs unit + integration tests (testcontainers Postgres + Redpanda) on every push
- [x] **PLAT-09**: Next.js (App Router, TS, Tailwind, shadcn/ui) skeleton with Thai/English i18n routing, Thai-friendly font, mobile-first layout, calling backend only through Kong
- [x] **PLAT-10**: Kong 3.9.1 DB-less + JWT plugin round-trip (curl → Kong → BFF → stub service) verified in Phase 0; Traefik fallback decided if it fails

### Identity & Access

- [x] **AUTH-01**: Customer can request an OTP via email or phone and receive a session (JWT in httpOnly cookie) without creating a password account
- [x] **AUTH-02**: staff / pier_admin / super_admin can log in and receive `role` + `operator_id` claims
- [ ] **AUTH-03**: Kong verifies the JWT; BFF forwards claims as trusted `X-*` headers; services reject requests lacking gateway headers (network-isolated from public)
- [ ] **AUTH-04**: super_admin can create pier_admin / staff users and assign them to an operator and pier
- [ ] **AUTH-05**: Every admin query is scoped by `operator_id` — a pier_admin never sees another operator's data

### Catalog

- [ ] **CAT-01**: super_admin can create and edit operators
- [ ] **CAT-02**: pier_admin can create/edit/archive piers (name, coordinates via map picker, address, photo, open hours)
- [ ] **CAT-03**: pier_admin can create/edit/archive routes (pier_from → pier_to, travel duration, tiered cancellation policy defaulting to >24h 100% / 2–24h 50% / <2h 0%)
- [ ] **CAT-04**: pier_admin can create/edit boats (name, default capacity, status active/maintenance)
- [ ] **CAT-05**: pier_admin can set ticket prices per route per ticket type (adult, child) stored as integer satang
- [ ] **CAT-06**: Public search can list piers and routes with coordinates for the map without authentication

### Schedule

- [ ] **SCHED-01**: pier_admin can create schedule templates (days of week, departure time, boat, capacity, effective date range)
- [ ] **SCHED-02**: pier_admin can generate departures from templates N days ahead; re-running does not duplicate departures
- [ ] **SCHED-03**: pier_admin can view departures in calendar and list views filtered by pier/route/date
- [ ] **SCHED-04**: pier_admin can edit a departure's time, boat, and capacity; a capacity reduction below `booked + held` is rejected synchronously (`CheckCapacityReducible`)
- [ ] **SCHED-05**: pier_admin can close a departure for booking, cancel it, or add an ad-hoc special departure
- [ ] **SCHED-06**: Departures store `departure_date_local` (Asia/Bangkok) alongside the UTC timestamp so date bucketing never uses raw UTC
- [ ] **SCHED-07**: schedule publishes `DepartureCreated/Updated/CapacityChanged/Cancelled` keyed by `departure_id`

### Booking & Search

- [ ] **BOOK-01**: booking-service maintains `departure_inventory (capacity, booked, held)` from schedule events with a DB `CHECK (booked + held <= capacity)` constraint
- [ ] **BOOK-02**: Customer can search by origin pier, destination pier, date, and passenger count and see departures with time, boat, price, and a seats-left badge (green / orange / red)
- [ ] **BOOK-03**: Search results show card list + map split on desktop and a list/map toggle on mobile
- [ ] **BOOK-04**: Customer can view departure detail with adult/child selector, total price before checkout, and the route's cancellation policy
- [ ] **BOOK-05**: Customer can hold N seats for 10 minutes (`CreateBooking`); the hold is rejected when `available < N`
- [ ] **BOOK-06**: Expired holds release seats automatically via Redis TTL trigger plus DB cron sweep, both calling the same idempotent "expire if still pending and past due" SQL
- [ ] **BOOK-07**: Customer sees a checkout page with 10-minute countdown, name + email/phone form, and price summary (mock payment confirm until Phase 4)
- [ ] **BOOK-08**: 50 concurrent `CreateBooking` calls on a 10-seat departure yield exactly 10 bookings — integration test runs in CI
- [ ] **BOOK-09**: Concurrent capacity reduction during active bookings never yields `booked + held > capacity` — integration test runs in CI
- [ ] **BOOK-10**: booking publishes `BookingCreated/Confirmed/Expired/Cancelled/AvailabilityChanged` keyed by `departure_id` with no PII in payloads

### Payment

- [ ] **PAY-01**: payment-service exposes a provider interface with a mock provider; the real provider (Opn or 2C2P) is chosen via sandbox spike at Phase 4 planning
- [ ] **PAY-02**: Customer can pay via PromptPay QR and sees the checkout update when payment is confirmed (polling)
- [ ] **PAY-03**: Customer can pay via credit/debit card
- [ ] **PAY-04**: Provider webhooks are signature-verified, deduplicated by a unique provider-reference constraint, and status is re-fetched from the provider API before publishing `PaymentSucceeded`
- [ ] **PAY-05**: `PaymentSucceeded` confirms the booking (`held → booked`) in one tx; if the hold already expired, booking re-holds when seats remain or auto-refunds otherwise — integration test covers the late-webhook race
- [ ] **PAY-06**: `BookingExpired` cancels the unpaid payment intent

### Ticket

- [ ] **TKT-01**: On `BookingConfirmed`, ticket-service issues one ticket with a random 32-byte token (hash stored, no PII) and QR image
- [ ] **TKT-02**: Customer can view the ticket page with a large sun-readable QR, departure info, and save-as-image
- [ ] **TKT-03**: ticket-service exposes `ValidateTicket` (atomic valid → used) and `GetTicket` via connect-go (UI consumer arrives in the check-in milestone)
- [ ] **TKT-04**: pier_admin can view a read-only passenger list per departure (name, pax count, booking status)
- [ ] **TKT-05**: Customer can view My Bookings via login or booking reference + email

### Notification

- [ ] **NOTF-01**: On `TicketIssued`, notification-service emails the ticket, fetching the recipient email via sync call (never from the event), idempotent via `notification_log`
- [ ] **NOTF-02**: Customer can request a resend of the ticket email from the ticket page

### Deployment

- [ ] **DEP-01**: `docker-compose.prod.yml` runs the full stack on one EC2 with production Redpanda settings (no dev fsync flags), retention configured, and a documented Postgres backup step
- [ ] **DEP-02**: Jenkins pushes images to Harbor (replaces ECR per Phase 1 D-21) and deploys to EC2 via `docker compose pull && up -d` at the end of Milestone 1

## v2 Requirements

Deferred to future milestones (seed Phase 5–8). Tracked but not in current roadmap.

### Check-in

- **CHK-01**: Staff can scan a QR ticket with the camera (qr-scanner / BarcodeDetector — not html5-qrcode) and see valid / used / wrong-departure instantly
- **CHK-02**: Staff can view the passenger list per departure and check in manually
- **CHK-03**: Admin sees boarded count per departure in real time
- **CHK-04**: Staff app caches today's passenger lists for partial offline use

### Refund & Departure Changes

- **REF-01**: Cancelling a departure cancels all its bookings, refunds paid ones, voids tickets, and notifies customers (saga)
- **REF-02**: Customer can cancel a booking and receive the tiered refund per the route policy
- **REF-03**: Admin can move a booking to another departure
- **REF-04**: Admin can view the DLQ and replay events

### Polish & Launch

- **POL-01**: Seats-left badges update live via SSE from `AvailabilityChanged`
- **POL-02**: PWA install + add-to-homescreen
- **POL-03**: SEO landing pages per pier/route
- **POL-04**: Admin dashboard (today's bookings, revenue, near-full departures) and reports with CSV export
- **POL-05**: k6 load test of book → pay flow; deploy runbook

### Growth

- **GRW-01**: LINE login and LINE ticket/reminder notifications
- **GRW-02**: Foreigner ticket type and pricing
- **GRW-03**: Promo codes (single flat code)
- **GRW-04**: Search projection service
- **GRW-05**: Multi-operator billing/payout
- **GRW-06**: Migrate to ECS/k8s and MSK

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Native iOS/Android apps | PWA covers it; app-store cycles slow a solo dev |
| Seat map / assigned seating | Boats swap and capacity shifts; headcount-only booking is the explicit domain decision |
| Full accounting / tax invoice (ใบกำกับภาษี) | Deep compliance surface orthogonal to booking; simple receipt in v1 |
| Kubernetes | Docker Compose on one EC2 for v1; services already separated for later migration |
| Debezium / CDC outbox | Go poll relay is enough at v1 scale |
| Open-date tickets | Conflicts with no-overbook guarantee; needs a holdback-inventory design first |
| Round-trip combined checkout | Needs an order-level wrapper the 1-booking-per-departure model lacks; explicit decision later |
| Live boat GPS tracking | No data source; not core to pre-departure booking |
| Promo rule engine (stacking, tiers) | No traffic to justify; single flat code in Growth |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PLAT-01 | Phase 1 | Complete |
| PLAT-02 | Phase 1 | Complete |
| PLAT-03 | Phase 1 | Complete |
| PLAT-04 | Phase 1 | Complete |
| PLAT-05 | Phase 1 | Complete |
| PLAT-06 | Phase 1 | Complete |
| PLAT-07 | Phase 1 | Complete |
| PLAT-08 | Phase 1 | Complete |
| PLAT-09 | Phase 1 | Complete |
| PLAT-10 | Phase 1 | Complete |
| AUTH-01 | Phase 2 | Complete |
| AUTH-02 | Phase 2 | Complete |
| AUTH-03 | Phase 2 | Gaps Found |
| AUTH-04 | Phase 2 | Gaps Found |
| AUTH-05 | Phase 2 | Gaps Found |
| CAT-01 | Phase 2 | Gaps Found |
| CAT-02 | Phase 2 | Gaps Found |
| CAT-03 | Phase 2 | Gaps Found |
| CAT-04 | Phase 2 | Gaps Found |
| CAT-05 | Phase 2 | Gaps Found |
| CAT-06 | Phase 2 | Gaps Found |
| SCHED-01 | Phase 3 | Pending |
| SCHED-02 | Phase 3 | Pending |
| SCHED-03 | Phase 3 | Pending |
| SCHED-04 | Phase 3 | Pending |
| SCHED-05 | Phase 3 | Pending |
| SCHED-06 | Phase 3 | Pending |
| SCHED-07 | Phase 3 | Pending |
| BOOK-01 | Phase 4 | Pending |
| BOOK-02 | Phase 4 | Pending |
| BOOK-03 | Phase 4 | Pending |
| BOOK-04 | Phase 4 | Pending |
| BOOK-05 | Phase 4 | Pending |
| BOOK-06 | Phase 4 | Pending |
| BOOK-07 | Phase 4 | Pending |
| BOOK-08 | Phase 4 | Pending |
| BOOK-09 | Phase 4 | Pending |
| BOOK-10 | Phase 4 | Pending |
| PAY-01 | Phase 5 | Pending |
| PAY-02 | Phase 5 | Pending |
| PAY-03 | Phase 5 | Pending |
| PAY-04 | Phase 5 | Pending |
| PAY-05 | Phase 5 | Pending |
| PAY-06 | Phase 5 | Pending |
| TKT-01 | Phase 5 | Pending |
| TKT-02 | Phase 5 | Pending |
| TKT-03 | Phase 5 | Pending |
| TKT-04 | Phase 5 | Pending |
| TKT-05 | Phase 5 | Pending |
| NOTF-01 | Phase 5 | Pending |
| NOTF-02 | Phase 5 | Pending |
| DEP-01 | Phase 5 | Pending |
| DEP-02 | Phase 5 | Pending |

**Coverage:**

- v1 requirements: 53 total
- Mapped to phases: 53
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-25*
*Last updated: 2026-09-25 after roadmap creation*
