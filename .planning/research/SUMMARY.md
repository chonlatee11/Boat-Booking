# Project Research Summary

**Project:** Boat-Booking (ferry/boat departure booking platform, Thailand island crossings)
**Domain:** Event-driven Go microservices + Kafka(Redpanda), database-per-service Postgres, Next.js PWA, multi-pier/multi-operator marketplace
**Researched:** 2026-09-25
**Confidence:** MEDIUM-HIGH

## Executive Summary

This is a multi-operator ferry/speedboat booking marketplace for Thai island crossings: search, hold seats, pay (PromptPay/card), digital QR ticket, built as 7 Go microservices plus a thin BFF communicating over Kafka (Redpanda) with database-per-service Postgres, deployed on a single EC2 via Docker Compose. Research confirms the seed architecture is sound and matches how experts build this class of system: transactional outbox, idempotent consumers, choreographed sagas, `SELECT...FOR UPDATE` inventory locking, no redesign needed. The main risk is process, not technology: this is a lot of distributed-systems machinery for a solo developer, so Phase 0's shared `pkg/*` template (outbox, idempotency, tracing) is the highest-leverage and highest-risk work in the project.

The stack holds up almost entirely as specified, with three concrete actions before/during Phase 0: pin Kong to `kong:3.9.1` explicitly (OSS is frozen there from 3.10 onward), bump Go to 1.25 (connect-go v1.20 requires it), and avoid `html5-qrcode` (unmaintained) in favor of `qr-scanner`/native `BarcodeDetector` for the Phase 5 check-in scanner. Feature scope for Milestone 1 (Phase 0-4) is well-aligned with competitor products (12Go, Ferryhopper, Direct Ferries): search, guest checkout, PromptPay+card, QR ticket, admin catalog/schedule CRUD, no-overbooking are all table stakes; seat maps, native apps, LINE notifications are correctly deferred.

The dominant risk cluster is correctness under concurrency and async delivery, not missing features: overbooking via schedule/booking inventory drift, the seat-hold-expiry-vs-late-payment-webhook race, non-idempotent Kafka consumers/webhooks, and Asia/Bangkok local-date-vs-UTC bugs in schedule generation. All are addressable with patterns already implied by the seed (outbox, idempotent-consumer table, DB CHECK constraints, explicit `departure_date_local` column); the work is disciplined implementation and testing, not architectural change. Payment provider choice (Opn/Omise vs 2C2P) remains correctly deferred to Phase 4, with Opn's maintained Go SDK and webhook helper as the current lean.

## Key Findings

### Recommended Stack

The prescribed stack (Go/chi/connect-go/sqlc+pgx/goose/franz-go/go-redis/slog/OTel, Postgres, Redis/Valkey, Redpanda, Kong+BFF, buf, Next.js, Leaflet/OSM, Resend, Opn/2C2P) is current and correct for Sept 2026 with minor version bumps. No stack change is warranted; only pin-and-monitor adjustments.

**Core technologies:**
- Go 1.25.x — required floor for connect-go v1.20+; bump from the seed's "1.23+"
- chi v5.3.2 + connect-go v1.20.0 — thin BFF routing + curl-debuggable sync RPC with generated Go/TS clients
- sqlc v1.31 + pgx/v5 + goose v3 — typed SQL, per-service migrations, stable pairing
- franz-go v1.22.x — fastest/most maintained pure-Go Kafka client, used for outbox-relay producer and manual-commit consumer (never `segmentio/kafka-go`)
- Postgres 17.x (upgrade from seed's 16) — no reason to start a major behind on self-managed EC2
- Redpanda (pinned tag) — single-binary Kafka-protocol broker, no ZooKeeper
- **Kong OSS pinned to `kong:3.9.1`** — critical: OSS is frozen there from 3.10 onward, `kong:latest` will silently break
- Next.js 16.x (seed says 15) — current stable; 15 still fine if there's a reason to lag
- Payment: **Opn (Omise) preferred for v1** — maintained Go SDK with webhook handler helper, reduces the highest-risk part of Phase 4; confirm with a real sandbox spike at Phase 4

### Expected Features

Feature research (12Go, Ferryhopper, Direct Ferries, Bookaway, Thai operator sites) confirms the seed's M1 scope is right-sized — nothing essential missing, nothing premature included.

**Must have (table stakes, all in M1):**
- Search by pier/date/pax, results with time/price/seats-left badge
- Adult/child passenger count + 10-min seat hold with no overbooking (stated core value)
- Guest checkout (email/phone OTP), no forced signup
- PromptPay QR (non-negotiable for Thai market) + card (for foreign tourists)
- Digital QR ticket + email delivery/resend, My Bookings lookup without login
- Cancellation policy displayed pre-purchase (refund execution deferred)
- Admin CRUD for piers/routes/boats/schedule; Thai/English bilingual UI

**Should have (differentiators, in M1):**
- Split list+map layout (genuine differentiator vs form-and-list competitors)
- Multi-pier/multi-operator platform with per-operator admin scoping (the actual competitive wedge vs single-operator Thai sites)
- Native-feeling Thai micro-copy

**Defer (v2+/Growth, correctly deferred):**
- Staff QR check-in scanner (Phase 5), refund/departure-cancel saga execution (Phase 6), SSE real-time availability (Phase 7)
- LINE notifications, foreigner pricing tier, pickup/transfer bundling, promo codes, multi-operator billing (Growth)
- Anti-features: seat maps, native apps, full accounting/tax invoices, live GPS tracking, unlimited free date-changes

### Architecture Approach

Choreographed sagas (no orchestrator) across 7 Go microservices + BFF, connected by transactional-outbox-published Kafka events with idempotent-consumer tables, and one synchronous connect-go hop maximum beyond the BFF. Booking-service is the single owner of real inventory (`SELECT...FOR UPDATE` on `departure_inventory`); schedule-service owns admin-set capacity; kept in sync via async events plus one sanctioned sync call (`CheckCapacityReducible`).

**Major components:**
1. **Kong + BFF** — Kong verifies JWT/TLS/routing (pinned OSS 3.9.1); BFF re-parses JWT and sets trusted `X-*` headers for downstream services (avoids needing a custom Kong plugin)
2. **booking-service** — sole writer of `departure_inventory.booked/held`; the load-bearing service for the no-overbook guarantee
3. **pkg/outbox + pkg/kafka** — shared transactional outbox relay and franz-go wrapper (manual commit, OTel trace-header propagation, DLQ-on-exhaustion) reused verbatim by all 7 services
4. **Hold-expiry mechanism** — Redis TTL as a low-latency *trigger* only; Postgres cron sweep is the authoritative backstop; both must share one idempotent "expire if still pending" code path

### Critical Pitfalls

1. **Microservices+Kafka overhead for a solo dev** — treat Phase 0 as make-or-break; the shared `pkg/*` template must be genuinely copy-paste-reusable across all 7 services or the coordination tax sinks the timeline.
2. **Overbooking via schedule/booking inventory drift** — enforce all capacity mutations through one locked code path, add a DB CHECK constraint (`booked+held<=capacity`), test both the basic capacity race and a capacity-reduction-mid-booking race in Phase 3 CI.
3. **Seat-hold-expiry racing a late payment webhook** — `PaymentSucceeded` for an already-`expired` booking must attempt re-hold-or-refund (never silently drop or blindly re-confirm); build and test this explicitly in Phase 4 alongside the happy path.
4. **Non-idempotent consumers/webhooks treated as "checked the box"** — idempotency check must be first, in the same DB transaction, before any non-transactional side effect; test redelivery and out-of-order delivery explicitly.
5. **Asia/Bangkok local-date vs UTC storage bugs** — store an explicit `departure_date_local` column alongside the UTC instant; never `DATE()`/`date_trunc` a raw UTC timestamp for calendar-day logic.

## Implications for Roadmap

Research confirms the seed's own Phase 0-4 ordering for Milestone 1 is architecturally forced by data/correctness dependencies. Suggested phase structure (matches PROJECT.md, validated by research):

### Phase 1: Foundation
**Rationale:** Every later service copies this template; a bug in the outbox relay, missing trace-header carrier, or non-enforced partition-key-by-aggregate-id discovered later means retouching every already-built service. Also the phase most likely to blow a solo dev's timeline if scope isn't held tight.
**Delivers:** Go workspace + `make new-service` scaffolding, shared `pkg/{events,kafka,outbox,httpx,auth,pgx}` proven with one real throwaway event, Docker Compose dev stack (Redpanda pinned, Postgres, Redis, Kong pinned 3.9.1, Grafana LGTM), Jenkins to ECR to EC2 pipeline, Next.js skeleton with i18n routing decided.
**Addresses:** Platform foundation requirements
**Avoids:** Pitfall 1 (solo-dev infra overhead), Pitfall 3 (idempotency-as-checkbox), Pitfall 6 (timezone convention), Pitfall 9 (PII discipline), Pitfall 11 (trace propagation across Kafka)

### Phase 2: Identity + Catalog
**Rationale:** Forced dependency — schedule needs `catalog.BoatUpserted` for default capacity; nothing downstream can be tested without operators/piers/routes/boats existing. First phase with real PII-bearing entities.
**Delivers:** identity-service (OTP login, JWT httpOnly cookie, roles, operator scoping), catalog-service (CRUD operators/piers/routes/boats/prices), admin UI + map picker, Kong+BFF trusted-header wiring.
**Uses:** chi, connect-go, sqlc+pgx, Leaflet/react-leaflet+OSM
**Implements:** Kong JWT verify to BFF claim-to-header remapping (architecture Pattern 4)

### Phase 3: Schedule
**Rationale:** Booking cannot be meaningfully tested (including the mandatory capacity race test) without departures existing, and departures cannot exist without routes/boats from Phase 2.
**Delivers:** schedule-service (templates to generate departures N days ahead, edit/close/cancel/add-special), admin calendar/list view, `schedule.*` event publishing.
**Uses:** goose migrations, franz-go outbox publishing pattern
**Implements:** first real event producer — validates the outbox+idempotency+tracing trio from Phase 1

### Phase 4: Booking Core
**Rationale:** Load-bearing phase for the "no overbook" core value. Correctly lands before payment so the capacity race and hold-expiry sweep are proven under concurrency before the harder payment-after-expiry race is layered on top.
**Delivers:** booking-service (`departure_inventory` projection, `CreateBooking` with 10-min hold, `GetAvailability`, `CheckCapacityReducible`), public Search to Results to Detail to Checkout(mock) flow, capacity race integration test in CI.
**Avoids:** Pitfall 2 (overbooking/drift) — DB CHECK constraint + two race tests required here, not deferred

### Phase 5: Payment + Ticket + Notification
**Rationale:** Correctly one phase, not three — the end-to-end saga (BookingCreated to PaymentSucceeded to BookingConfirmed to TicketIssued to email) has no useful partial-completion state.
**Delivers:** payment-service (provider interface + mock, then real Opn/2C2P integration, idempotent webhook), ticket-service (QR issuance, atomic `ValidateTicket`), notification-service (email, idempotent via `notification_log`), full booking saga + hold-expired saga, Ticket + My Bookings pages.
**Addresses:** Remaining M1 table-stakes features (QR ticket, email confirmation, PromptPay+card)
**Avoids:** Pitfall 4 (hold-expiry/late-payment race), Pitfall 5 (Thai payment webhook idempotency), Pitfall 7 (money-as-float discipline), Pitfall 8 (QR replay — atomic `ValidateTicket`)

### Phase Ordering Rationale

- **Data dependency chain is strict, not stylistic:** Catalog to Schedule to Booking to Payment/Ticket is forced, matching the vertical-slice discipline the seed already mandates.
- **Correctness-risk ordering:** the simpler race (capacity contention, Phase 4) must be proven before the harder, compounding race (payment-after-expiry, Phase 5).
- **Cross-cutting concerns front-loaded to Phase 1/2:** idempotency, timezone convention, PII discipline, trace propagation are far more expensive to retrofit once multiple services depend on the wrong shape.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 5 (Payment):** Provider choice (Opn vs 2C2P) is explicitly still open; re-verify current provider docs at planning time, design idempotency/replay simulation against sandbox limitations.
- **Phase 1 (Kong config):** A spike to confirm Kong OSS 3.9.1's DB-less JWT plugin covers needed claim-forwarding (or falls back to Traefik+BFF) should happen during Phase 1 planning.

Phases with standard patterns (skip research-phase):
- **Phase 2 (Identity + Catalog):** Standard CRUD + JWT auth patterns, well-documented.
- **Phase 3 (Schedule):** Standard outbox-publishing service pattern, template established in Phase 1/2.
- **Phase 4 (Booking Core):** Patterns (locking, race testing) fully specified in ARCHITECTURE.md/PITFALLS.md already.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified against Sept 2026 official docs/release notes; payment-provider recommendation is MEDIUM (no hands-on sandbox test) |
| Features | MEDIUM | Cross-verified across multiple competitor sources and Thai-market norms; no primary operator interviews |
| Architecture | MEDIUM | Industry-standard, cross-corroborated patterns validating the seed; exact library API names are directional |
| Pitfalls | MEDIUM-HIGH | General distributed-systems/booking-domain patterns HIGH; Thai payment-gateway specifics and Redpanda-in-Docker guidance MEDIUM |

**Overall confidence:** MEDIUM-HIGH

### Gaps to Address

- **Payment provider (Opn vs 2C2P):** Spike both against real sandbox credentials at the start of the payment phase before finalizing.
- **Kong OSS 3.9.1 sufficiency:** Unverified until a real spike; Traefik+BFF is the documented fallback.
- **Postgres 17 vs 18:** A legitimate risk-tolerance call, not a research gap — either is safe.
- **Real-time-availability threshold:** No hard data on when live SSE availability becomes necessary vs on-load fetch being sufficient — deferred by design.

## Sources

### Primary (HIGH confidence)
- Go, connect-go, sqlc, pgx, goose, franz-go, go-redis, OpenTelemetry Go, PostgreSQL, Redpanda, Kong, Next.js, buf official docs/release notes/GitHub (full list in STACK.md Sources)
- Redis AGPLv3 announcement, Kong Gateway version support policy + GitHub discussions (OSS freeze at 3.9.1)
- Omise/Opn official docs (PromptPay, SDK), 2C2P official developer docs
- Redpanda official Docker Compose / High Availability docs

### Secondary (MEDIUM confidence)
- 12Go, Ferryhopper, Direct Ferries, Bookaway, Boonsiri Ferry, Thai Ferry Tickets competitor product pages and FAQs
- PromptPay/LINE ecosystem and dual-pricing market analyses (Tazapay, Xendit, SCMP, Bangkok Post)
- Third-party outbox pattern / OTel-Kafka / testcontainers-flakiness writeups (dev.to, Medium, SigNoz, oneuptime)

### Tertiary (LOW confidence)
- None flagged as standalone LOW — all findings cross-checked against at least one official source or multiple independent secondary sources

---
*Research completed: 2026-09-25*
*Ready for roadmap: yes*
