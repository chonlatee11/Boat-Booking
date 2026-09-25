# Pitfalls Research

**Domain:** Event-driven ferry/boat departure booking platform — Go microservices + Kafka(Redpanda), database-per-service Postgres, Redis seat holds, Thai payment (PromptPay/card), QR tickets, solo developer, Docker Compose on single EC2
**Researched:** 2026-09-25
**Confidence:** MEDIUM-HIGH (general distributed-systems/booking-domain patterns are HIGH confidence, well-established industry knowledge; Thai payment-gateway specifics and Redpanda-in-Docker production guidance are MEDIUM — cross-checked against official docs and community sources but gateway behavior can change without notice, verify against current Opn/2C2P docs at Phase 4 planning time)

## Critical Pitfalls

### Pitfall 1: Microservices + Kafka is the wrong default cost for a solo developer

**What goes wrong:**
Eight services (identity, catalog, schedule, booking, payment, ticket, notification, gateway) each with their own DB, outbox relay, Kafka consumer group, health checks, Dockerfile, and CI job multiply the amount of "glue" a single person must write and keep alive. A solo dev spends more time on service scaffolding, event contracts, and local orchestration than on the actual booking logic. This is the single biggest risk to the project shipping at all — not overbooking, not payment bugs.

**Why it happens:**
The architecture was chosen up front (seed doc) for "correctness at scale" reasons (database-per-service, event sourcing hygiene) before there was a working product. Distributed-systems patterns that make sense for a multi-team org get applied by one person, who now pays the coordination tax without having a team to divide it across.

**How to avoid:**
- Treat Phase 0 as the highest-leverage phase: `make new-service` scaffolding, shared `pkg/*` (kafka, outbox, httpx, auth, pgx), and CLAUDE.md per service must be genuinely copy-paste-and-go. Every minute saved here is repeated 7+ times.
- Keep the "7 services" boundary honest but resist adding more services in v1 (no separate search-projection service, no separate reporting service — both explicitly deferred, correctly).
- Default to the smallest viable version of each cross-cutting concern (one outbox table shape, one DLQ shape, one consumer-idempotency table shape) reused verbatim across services — don't let each service invent its own variant.
- Time-box Phase 0: if the compose stack + one working ping-pong event (schedule → booking) doesn't work within the plan's timeline, cut scope (e.g., drop Kong for a bare Go BFF, drop full Tempo/Loki/Prometheus stack for just structured logs + traces in v1) rather than let infra bleed into Phase 1+.
- Plan phases as vertical slices (already captured in PROJECT.md "working notes") — a phase that touches only one layer across all services produces nothing usable and doubles context-switching cost for one person.

**Warning signs:**
- Phase 0 taking noticeably longer than planned, or spilling into "just one more piece of infra" scope creep.
- Each new service's boilerplate (outbox relay, otel setup, health checks) taking materially different amounts of code — signals the template isn't actually reusable.
- Spending more session time on `docker compose` debugging than on domain logic by Phase 2.

**Phase to address:** Phase 0 (foundation) is the make-or-break point. Re-verify at the start of Phase 1 that adding a service (identity or catalog) took roughly the time budgeted for "business logic," not "infra."

---

### Pitfall 2: Overbooking via schedule/booking inventory drift

**What goes wrong:**
`schedule-service` owns the capacity value an admin sets; `booking-service` owns the real-time `booked`/`held` projection built from schedule events. If the projection isn't kept perfectly in sync — a `DepartureCreated` event lost, applied twice, or applied out of order relative to a later `DepartureCapacityChanged` — the booking service either rejects valid bookings (lost revenue) or allows bookings past physical capacity (overbooking, the one thing PROJECT.md says must never happen). A second, subtler variant: an admin reduces capacity concurrently with customers holding seats, and the reducibility check (`CheckCapacityReducible`) races with `CreateBooking` transactions on the same row.

**Why it happens:**
Two services own two halves of one invariant (capacity vs. usage) connected only by asynchronous events. Any gap between "event published" and "event applied" is a window where the booking service's view of the world is stale. Developers often test the happy path (create → book) but not: event replay after consumer restart, duplicate delivery, or capacity change arriving mid-hold.

**How to avoid:**
- All capacity mutations against `departure_inventory` go through one code path that does `SELECT ... FOR UPDATE` inside a single Postgres transaction — no exceptions, including for the projection update triggered by `DepartureCapacityChanged`.
- `CheckCapacityReducible` must be answered from the same row under the same lock discipline used by `CreateBooking` — treat it as "would this booking transaction still be legal," not a separate read.
- Make `departure_inventory` creation itself idempotent (`ON CONFLICT DO NOTHING`/upsert) keyed by `departure_id`, since `DepartureCreated` can be redelivered.
- Ship the capacity race integration test (50 concurrent `CreateBooking` against 10 seats → exactly 10 succeed) in Phase 3 and keep it running in CI on every commit afterward, not just once.
- Add a second, cheaper test: concurrent `CreateBooking` + `CheckCapacityReducible`/`DepartureCapacityChanged` applied mid-stream — this is the scenario the primary race test doesn't cover and is more likely in production (admin changes capacity while customers are actively booking).
- Add a periodic reconciliation job (even a simple nightly one) that recomputes `booked` from actual `paid` bookings and alerts on drift — cheap insurance against silent projection bugs.

**Warning signs:**
- `departure_inventory.booked + held > capacity` ever observed (add a DB CHECK constraint so this is impossible, not just monitored).
- Booking counts in `booking` DB disagreeing with departure capacity shown in `catalog`/`schedule` admin UI.
- Consumer lag spikes on `schedule.events` correlating with support reports of "seat was available but booking failed" or vice versa.

**Phase to address:** Phase 3 (Booking core) is where this must be airtight before Phase 4 adds real payment. Add the DB CHECK constraint and the second race test in Phase 3; add reconciliation job as a stretch item, or explicitly defer with a tracked follow-up.

---

### Pitfall 3: Outbox/consumer idempotency implemented as "checked the box" rather than actually tested

**What goes wrong:**
Four distinct failure modes hide behind "we have an outbox and idempotency table":
1. **Duplicate publish** — outbox relay publishes an event, crashes before marking it sent, restarts, and republishes it (this is expected/fine *if* consumers are truly idempotent — but often they aren't for side effects like sending an email).
2. **Out-of-order per aggregate** — if the relay doesn't preserve insertion order per `aggregate_id`, or Kafka partitioning isn't strictly by `aggregate_id`, a consumer can see `BookingConfirmed` before `BookingCreated`.
3. **Poison messages** — a malformed or unexpected-shape event (e.g., old proto version, unexpected NULL) crashes the consumer every retry, blocking the whole partition until it's DLQ'd — or worse, if retries aren't capped, blocks forever.
4. **DLQ without replay** — events land in `*.dlq` and are never looked at again; the underlying bug (e.g., a schema mismatch) silently drops bookings/refunds until someone happens to check.

**Why it happens:**
`processed_events(event_id PK)` prevents *state* being applied twice, but non-idempotent side effects (send email, call payment provider) inside the same handler as the state update are easy to leave unguarded — the id check happens, but the side effect fires before the check short-circuits, or the check is placed after the side effect by mistake. Ordering bugs happen because it's easy to partition by the wrong key (e.g., partition by `event_type` instead of `aggregate_id`, or forget partitioning entirely on a new topic). Poison messages are rare in dev (clean data) and common in prod (real-world edge cases, partial rollouts of a new proto field).

**How to avoid:**
- Idempotency check must be the *first* thing the handler does, in the same DB transaction as the state write, before any external call (email, payment) — external calls that can't be transactional (SMTP, HTTP) should themselves be deduped via a separate log table (`notification_log` already planned — verify it's actually checked before send, not just recorded after).
- Enforce partition key = `aggregate_id` at the `pkg/kafka` wrapper level, not per-service discretion — make it structurally hard to get wrong (e.g., producer helper takes `aggregate_id` as a required argument, not a header you might forget).
- Cap retries (3 in-process, per PROJECT.md) then route to DLQ automatically — never let a bad message spin a consumer forever or, worse, `panic`-loop the container (which would also break `docker compose` health/restart semantics).
- DLQ needs to be visible from day one even without the full "DLQ viewer + replay" admin UI (deferred to Phase 6) — at minimum, log DLQ writes at ERROR level with full context so they show up in the existing observability stack, and write a `make dlq-list` / SQL query as a stopgap.
- Write one integration test per service that: publishes the same event twice and asserts the side effect (row written, email "sent") happens once; publishes events for one aggregate out of order and asserts the consumer either handles it correctly or defers/rejects it explicitly (don't let "probably fine" ship untested).

**Warning signs:**
- `processed_events` table check exists but is a separate line from the state-changing query rather than same transaction.
- No test exercises redelivery or out-of-order delivery — only single-happy-path tests exist.
- DLQ topics have zero tooling to inspect, meaning the team wouldn't notice growth in the DLQ for weeks.

**Phase to address:** Phase 0 — bake enforced partition-key-by-aggregate and idempotency-first pattern into `pkg/kafka`/`pkg/outbox` so every later service inherits it correctly. Verify with tests in Phase 2 (schedule, first real event producer/consumer pair) and Phase 3–4 (booking/payment sagas, where side effects are money and email). Minimal DLQ visibility is a Phase 0/1 concern even though the full viewer is Phase 6.

---

### Pitfall 4: Seat hold expiry racing a late payment webhook

**What goes wrong:**
Customer holds a seat for 10 minutes, initiates PromptPay payment near the deadline, and the provider's webhook (`PaymentSucceeded`) arrives *after* the hold has already expired and been swept (Redis TTL fired, cron sweep released `held`, booking marked `expired`). Now the system has a "successful payment" for a booking that no longer holds a seat. The naive failure modes are: (a) silently drop the late payment → customer paid, got nothing, demands refund and is furious at the dock; (b) blindly re-confirm the booking → seat may have been re-sold to someone else in the meantime → actual overbooking.

**Why it happens:**
Two independent timers (Redis TTL for the hold, and the payment provider's own processing/webhook latency, which is not bounded by your 10-minute UI countdown) are not coordinated. PromptPay QR payment confirmation can take anywhere from seconds to a couple of minutes depending on the customer's bank; webhook delivery itself can also be delayed or retried by the provider. This is explicitly called out in PROJECT.md's "Hold expired saga" but is easy to under-test because it only manifests near the boundary.

**How to avoid:**
- Implement the hold-expired saga exactly as scoped: on late `PaymentSucceeded` for an expired booking, attempt re-hold (check if the seat is still available) — if yes, re-confirm the booking; if no, auto-refund and notify the customer with a clear "seat became unavailable, refunded" message rather than a generic error.
- Make the re-hold attempt itself go through the same `SELECT ... FOR UPDATE` capacity path as `CreateBooking` — it's not a special case, it's just another attempt to acquire inventory.
- Widen the effective grace period on the *consumer* side, not just the UI countdown: don't hard-delete/reuse the booking row the instant Redis TTL fires — mark it `expired` but keep it queryable for a short grace window (e.g., a few minutes) so the late-payment saga has something concrete to reconcile against, and log/alert if a late payment arrives outside that window (should be rare, needs a human).
- Prefer polling payment status from your own backend (booking service asks payment service "did this intent succeed?") over trusting client-side redirect/polling alone for confirming success — the webhook is the source of truth, UI polling is just UX.
- Test explicitly: create booking, let hold expire (or force-expire in test), then deliver `PaymentSucceeded` late — assert one of {re-confirmed, refunded}, never "booking confirmed with no seat" or "payment lost."

**Warning signs:**
- Support tickets of the form "I paid but my booking shows cancelled/expired."
- `payment.PaymentSucceeded` events with no matching non-expired `booking` row and no compensating refund event within N minutes.
- Hold TTL and payment provider's typical webhook latency were never compared/measured against each other.

**Phase to address:** Phase 4 (Payment + Ticket) — this is explicitly one of the sagas that must exist before payment goes live, not an edge case to patch later. Build and test it alongside the happy-path booking saga, not after.

---

### Pitfall 5: Thai payment gateway webhook reliability and idempotency assumptions

**What goes wrong:**
Both Opn (Omise) and 2C2P deliver payment confirmation via webhook/callback. Common integration mistakes with these providers specifically:
- Trusting the webhook payload at face value instead of re-querying the charge/transaction status from the provider's API before confirming a booking (webhook spoofing / replay risk).
- Not deduplicating on the provider's transaction reference — a webhook can be delivered more than once (network retry on the provider side), and if the handler isn't keyed on `provider_ref` with a unique constraint, the same payment can double-confirm a booking or double-trigger a refund.
- 2C2P specifically sends some backend responses as JWT-encoded payloads that must be decoded/verified, not read as plain JSON — missing this produces silent failures that look like "webhook never arrived."
- Refund flows are rail-dependent (PromptPay vs. card refunds behave differently, may have different settlement timing) — assuming a single uniform "refund succeeded" state machine across payment methods breaks when a customer paid one way and expects a symmetric refund.
- Sandbox environments for both providers don't perfectly replicate production timing/failure behavior (duplicate webhooks, out-of-order callbacks, timeouts) — teams that only test the happy path in sandbox ship idempotency bugs that only appear under real bank-network conditions.

**Why it happens:**
Webhook-driven payment confirmation is inherently "trust but verify" — it's tempting to treat the webhook body as ground truth because it's convenient, and Thai gateway docs are not always as thorough on idempotency guidance as Stripe's. Provider interface abstraction (already planned) helps but only if idempotency logic lives in shared/generic code (`payment-service`), not duplicated per-provider.

**How to avoid:**
- Build the provider interface with idempotency as a first-class contract: every webhook handler must (1) verify signature/authenticity per provider's method, (2) look up `provider_ref` in a unique-constrained table before doing anything, (3) re-fetch charge status from the provider API rather than trusting the webhook payload alone for anything that changes booking state.
- Decide Opn vs. 2C2P at Phase 4 planning time as scoped, but build against the mock provider first and design the interface so idempotency/replay behavior can be tested without hitting the real sandbox (simulate duplicate/out-of-order/delayed webhooks in integration tests — a self-hosted sandbox emulator or a hand-rolled fake webhook sender both work).
- Log every raw webhook payload (with signature verification result) to `webhook_log` before processing, independent of whether processing succeeds — this is your only forensic trail when a provider's dashboard and your DB disagree.
- Treat refund as its own explicit state machine per payment method rather than a single boolean — at minimum distinguish "refund requested," "refund processing," "refund settled," and know which of those PromptPay vs. card can skip or take longer on.
- Re-verify current provider docs at Phase 4 planning time — this area moves and today's guidance may be superseded by the time you integrate.

**Warning signs:**
- Webhook handler updates booking/payment state directly from payload fields without a corresponding "fetch and verify from provider API" step.
- No unique constraint on `provider_ref` in the payments table.
- No test ever sends the same webhook payload twice.
- Refund UI/logic assumes a single `refunded: true/false` flag.

**Phase to address:** Phase 4 (Payment + Ticket). Decide provider, but design the provider interface and idempotency contract before wiring either Opn or 2C2P specifically, so switching providers later doesn't require re-doing the safety logic.

---

### Pitfall 6: Timezone bugs — Asia/Bangkok "local departure date" vs UTC storage

**What goes wrong:**
DB stores UTC (correct default), but a ferry departure is fundamentally tied to a *local calendar date* in Thailand (Asia/Bangkok, UTC+7, no DST). Common bugs: a departure at 23:30 Bangkok time on day N gets stored as 16:30 UTC on day N, which is fine — but a departure at 00:30 Bangkok on day N (just after midnight) stores as 17:30 UTC on day N-1, and any code that derives "which day is this departure" by taking `date(occurred_at_utc)` instead of converting to Bangkok first will show it under the wrong day in the admin calendar, in search-by-date, and in schedule-template generation ("generate departures for the next N days" — off-by-one at month/day boundaries if template generation logic works in UTC dates). A second common bug: schedule templates defined by "day of week" (e.g., "every Saturday, 06:00 departure") generating the wrong local day if the generator computes day-of-week from a UTC timestamp instead of the Bangkok calendar date.

**Why it happens:**
"Store UTC, display local" is the right rule for timestamps, but a ferry departure's *identity* (which search bucket it belongs to, which day's capacity report it counts toward) is a local-date concept, not a instant-in-time concept. Mixing the two — using UTC date math where Bangkok date math is needed — is the single most common timezone bug class in travel/booking systems, and Thailand's lack of DST makes it *look* safe in casual testing (a fixed +7 offset always works... until someone does `date_trunc('day', ts)` in UTC instead of converting first).

**How to avoid:**
- Establish one rule immediately in Phase 0/1 shared code: any time a "which calendar day" question is asked (search by date, schedule template generation, daily reports, "close bookings for today"), convert to `Asia/Bangkok` first, then take the date — never `date_trunc`/`DATE()` on a raw UTC timestamp.
- Store departure "local date" as an explicit column (not derived ad hoc each time) alongside the UTC instant — e.g., `departure_date_local DATE` + `departure_at_utc TIMESTAMPTZ`. This removes the entire class of bug because nothing has to remember to convert.
- Write unit tests specifically for departures between 00:00–01:00 and 23:00–00:00 Bangkok time, and for schedule-template generation across a UTC month/day boundary — these are the cases that pass with "midday" test fixtures and fail in production.
- Frontend: Next.js i18n + date display must always format using `Asia/Bangkok`, not the browser's local timezone, for departure times shown to Thai and foreign customers alike (a departure is fixed to the pier's timezone regardless of where the customer is browsing from) — this is a common Next.js/date-fns/Intl gotcha (`toLocaleString` defaults to system/browser timezone, not a target timezone, unless explicitly passed).

**Warning signs:**
- Any code that calls a date-only conversion on a `TIMESTAMPTZ` column without an explicit `AT TIME ZONE 'Asia/Bangkok'`.
- Frontend date formatting without an explicit timezone argument.
- Admin reports/search showing a late-night or early-morning departure "on the wrong day" during manual testing.

**Phase to address:** Phase 0 (establish the convention + shared helper) and Phase 2 (schedule-service, where template generation and departure creation first materialize dates) — this is foundational and expensive to retrofit once bookings/reports depend on the wrong date column.

---

### Pitfall 7: Money as float or missing precision discipline

**What goes wrong:**
PROJECT.md correctly mandates integer satang, but the pitfall is inconsistent enforcement: proto/JSON payloads using a float/double for price fields somewhere in the pipeline (e.g., a quick admin form binds to a JS number and sends `129.5` baht instead of `12950` satang), or a downstream service doing floating-point arithmetic on an integer-satang value it received (e.g., computing a percentage refund with `float64` and rounding inconsistently between payment-service and booking-service, producing off-by-one-satang mismatches that fail reconciliation).

**Why it happens:**
Frontend forms naturally think in baht with decimals; it's easy for the boundary conversion (baht ↔ satang) to happen in the wrong place or inconsistently between the customer-facing checkout, the admin pricing UI, and internal service-to-service calls. Percentage-based cancellation refunds (the tiered policy: 100%/50%/0%) are a specific danger zone — rounding a 50% refund of an odd satang amount needs one canonical rounding rule (e.g., round half down, always in favor of a specific party) applied everywhere, or two services will compute different amounts for "the same" refund.

**How to avoid:**
- Proto schema for any money field is `int64` (satang) from day one — never `double`/`float` — enforce this in code review / proto lint, not just convention.
- Baht↔satang conversion happens in exactly one place per direction: one shared frontend utility for "display baht → send satang," one shared Go helper for "format satang → display baht." No service does ad hoc `* 100` / `/ 100` inline.
- Define the rounding rule for percentage-based refunds once (e.g., in `pkg/` or a documented decision) and unit test it against odd amounts (e.g., 199 satang × 50% must always resolve the same way everywhere it's computed).
- Add a lightweight invariant test: total booking amount = sum of per-passenger-type line items exactly (integer arithmetic, no drift) — catches conversion bugs immediately rather than in a reconciliation report weeks later.

**Warning signs:**
- Any `float`/`double`/JS `Number` used for a price field beyond the display layer.
- Refund amounts that don't sum back to the original booking total across partial-refund scenarios.
- Manual QA finds a checkout total that's off by a few satang from admin pricing.

**Phase to address:** Phase 1 (catalog — where ticket prices are first defined) and Phase 4 (payment/refund — where arithmetic on money actually happens). Establish the convention in `pkg/` and proto definitions from Phase 0.

---

### Pitfall 8: QR ticket security — replay, screenshot sharing, offline validation gaps

**What goes wrong:**
A QR ticket that's just "a token that means valid" is vulnerable to: (a) **replay** — same QR shown to two different staff scanners (or the same one twice) if `ValidateTicket` doesn't atomically mark the ticket as used on first successful scan; (b) **screenshot/forward sharing** — a customer forwards their ticket QR to a friend who boards on the same booking; without a "used" state transition tied to the *first* successful scan, both people can board; (c) **offline validation ambiguity** — the plan explicitly defers "offline mode" to a later phase, but even in v1, if a staff scanner has any local caching or if `ValidateTicket` isn't a hard synchronous call, there's a window where two scans of the same QR both see "valid" before either write lands.

**Why it happens:**
QR-based ticketing is deceptively simple to demo (generate QR, scan QR, done) but the state transition on scan is the actual security boundary, and it's easy to build the "generate + display" half well while treating "scan + validate + mark used" as an afterthought, especially since check-in (Phase 5) is deferred — meaning the ticket-service's `ValidateTicket` contract gets designed in Phase 4 but not exercised end-to-end with real scanning UX until Phase 5. If the Phase 4 design doesn't already make first-scan-wins atomic, retrofitting it later is riskier because the ticket-service and its callers are more entrenched.

**How to avoid:**
- Design `ValidateTicket` as an atomic "check-and-set": a single transaction that reads ticket status, and if `valid`, sets it to `used` (or a scan-specific status) and returns success only if that row-level update actually changed a row — use `UPDATE ... WHERE status = 'valid' RETURNING *` (or equivalent), not read-then-write.
- QR payload is a random 32-byte token (already scoped) with only a hash stored server-side (already scoped) — keep it that way; never encode booking/passenger info in the QR itself (no PII, no guessable IDs), since a leaked/shared image should reveal nothing useful beyond "this specific ticket."
- Decide and document the policy for "wrong departure" vs "already used" vs "invalid" — staff need three distinct, unambiguous signals (not just green/red) so a legitimately re-scanned ticket (staff mis-scan, retry) isn't confused with actual fraud.
- Even though full offline mode is deferred, make sure the v1 online-only scanner fails *closed* (no connectivity = "cannot validate, do not board" rather than silently allowing entry) — this is a one-line UX decision now that's much cheaper than an actual offline-conflict-resolution design later.

**Warning signs:**
- `ValidateTicket` implemented as separate `SELECT` then `UPDATE` calls (race window).
- Any place QR content includes more than an opaque token (e.g., a booking ID or JWT with claims) — instantly increases blast radius of a screenshot leak.
- No distinct staff-facing state for "already used" vs "wrong departure" vs "unknown token."

**Phase to address:** Phase 4 (ticket-service, `ValidateTicket` contract) — design the atomic validate-and-mark-used semantics even though real scanning UX lands in Phase 5, so the contract doesn't need to change later.

---

### Pitfall 9: PII leaking into Kafka events, logs, or traces

**What goes wrong:**
PROJECT.md's rule ("events carry only IDs, no PII") is easy to violate in three sneaky ways: (1) a service publishes an event with a denormalized field "for convenience" (e.g., `BookingCreated` includes `customer_email` so notification-service doesn't need a sync call) — this immediately puts PII on Kafka, replicated to every consumer, retained per topic retention policy, and visible in Redpanda console/logs to anyone with cluster access; (2) structured logs (`slog` JSON) accidentally include full request/event payloads at DEBUG or even INFO level, including names/emails/phone numbers, and those logs flow into Loki with a long retention; (3) OpenTelemetry trace attributes/span tags include PII (e.g., tagging a span with `customer.email` for debuggability) — traces in Tempo are also a long-lived store outside the primary PDPA-scoped databases (identity/booking).

**Why it happens:**
PII-free events require an extra sync call (e.g., notification-service asking booking/identity for the email at send time) which feels like unnecessary latency/complexity compared to "just put it in the event." Logging PII is almost always accidental — a `%+v` struct dump, or a middleware that logs the full request body for debugging and never gets scoped down before merging.

**How to avoid:**
- Enforce "no PII in event payloads" structurally: review every `.proto` event definition for PII-shaped fields (name, email, phone, address) as a checklist item before merging any new event type — this is cheap to catch in proto review and expensive to catch after events are already flowing/retained.
- notification-service fetches recipient contact info via sync call (already the documented design) — verify this is actually implemented that way, not "for now" denormalized as a shortcut under deadline pressure.
- Add a log-scrubbing convention in `pkg/httpx`/`slog` setup: never log full request/event bodies at INFO; if DEBUG-level payload logging exists for local dev, make sure it's compiled/configured out (or at minimum documented as never-enable-in-prod) before Phase 0 ships the shared logging package.
- Same discipline for OTel span attributes — attribute allowlist (ids, statuses, durations) rather than "attach the whole object," enforced by convention in `pkg/kafka`'s otel instrumentation helper so individual services don't each decide.
- Keep PII physically confined to identity/booking DBs as scoped — no other service's schema should ever have a `name`/`email`/`phone` column; a quick grep across `services/*/migrations/` for those column names is a cheap periodic check.

**Warning signs:**
- Any `.proto` event message with a `string email`/`string phone`/`string name` field outside identity/booking's own internal (non-event) data.
- `grep -r "email\|phone" services/*/migrations/` finding hits outside identity/booking.
- Log lines in Loki containing recognizable customer names/emails when searched.

**Phase to address:** Phase 0 (establish proto review checklist + logging/otel conventions in shared `pkg/`) and Phase 1 (identity/catalog — first real PII-bearing entities) since retrofitting "don't leak PII" after events/logs have already been shipping it means historical data in Kafka/Loki/Tempo retention windows is already exposed.

---

## Moderate Pitfalls

### Pitfall 10: Redpanda/Kafka on a single EC2 — disk, retention, and restart-order fragility

**What goes wrong:**
A single-broker Redpanda instance under Docker Compose has no replication — disk corruption or an EBS volume issue loses the log. Retention misconfiguration (default time/size-based retention) can either grow unbounded and fill the disk (taking down the broker, and with it every producer/consumer across all 7 services) or expire events sooner than an outbox-relay/consumer recovery scenario needs them. Docker Compose restart order matters: if Postgres/Redis/Redpanda aren't healthy before dependent services start, services crash-loop on startup, and Compose's default restart policy can mask a real config problem behind "it eventually came up." Running Redpanda with dev-oriented flags (e.g., a `dev-container` style mode that disables fsync for speed) in what is nominally "prod" (the single EC2) risks silent data loss on an instance stop/restart — convenient in local dev, dangerous if the same compose file or flags leak into `docker-compose.prod.yml`.

**Why it happens:**
Redpanda's Docker quickstart guidance is explicitly oriented at dev/test; using the same container image and a similar compose shape for the "real" single-EC2 deployment is natural (it is the whole point of the project's chosen tradeoff — Kubernetes is explicitly out of scope) but the flags and settings appropriate for a laptop dev loop are not always the same ones appropriate for a persistent prod instance, even a single non-HA one.

**How to avoid:**
- Maintain genuinely separate `docker-compose.yml` (dev) and `docker-compose.prod.yml` (prod) configs for Redpanda specifically — dev can use fast/ephemeral flags, prod must use durable settings (fsync on, explicit retention policy sized to disk, a named/persistent volume, not an anonymous one).
- Set explicit topic retention (time and/or size) for every topic rather than relying on broker defaults — size retention conservatively against the actual EBS volume size, with monitoring/alerting on disk usage before it's an outage.
- Use `depends_on` with `condition: service_healthy` (health checks, not just "container started") for Postgres/Redpanda/Redis in Compose, so dependent Go services don't start against a not-yet-ready broker/DB.
- Accept the single-EC2/single-broker tradeoff as scoped (it's explicitly the v1 decision), but write down the actual disaster-recovery story now, even if minimal: what happens if the EC2 instance is replaced — is the Redpanda data volume backed up/snapshotted? A single unreplicated broker with no backup is a full data-loss risk, not just a downtime risk.
- Watch consumer lag per service from day one (already planned via Prometheus/Grafana) — on a single broker with no auto-scaling, a slow consumer (e.g., notification-service blocking on a slow SMTP call) can silently back up and eventually hit retention limits, silently dropping unconsumed events.

**Warning signs:**
- No explicit topic retention configuration checked into `deploy/` — relying on cluster defaults.
- `docker-compose.prod.yml` reusing dev-oriented Redpanda flags/image tags without review.
- No EBS snapshot/backup story for the Redpanda data volume, only for Postgres.
- Consumer lag dashboards not wired up until "later" — by which point a real incident has already happened undetected.

**Phase to address:** Phase 0 (compose stack) for health-check ordering and dev/prod config separation; revisit explicitly before any real customer traffic (end of Phase 4 / pre-launch) for retention sizing and backup story, since that's when actual payment-bearing events start accumulating.

---

### Pitfall 11: OpenTelemetry trace propagation silently breaking across Kafka

**What goes wrong:**
HTTP-to-HTTP trace propagation (via Kong/BFF → services) works "for free" with standard middleware, giving false confidence that tracing is solved. Kafka is a different propagation boundary: the trace context (`traceparent`) has to be manually injected into message headers on publish and extracted on consume — if the outbox relay (which is a separate process/step from the original request handler) doesn't carry the trace context through the outbox row into the published message headers, the trace chain breaks exactly at the async boundary, which is the most valuable place to have it (debugging a saga that spans 3+ services and an async hop is precisely when you need the end-to-end trace).

**Why it happens:**
The outbox pattern intentionally decouples "handle request, write outbox row" from "relay publishes to Kafka" — this decoupling is good for reliability but means the trace context has to be explicitly persisted (e.g., a `trace_id`/`traceparent` column on the outbox row, per PROJECT.md's event envelope which already includes `trace_id`) and re-attached by the relay at publish time, not assumed to flow automatically the way in-process context propagation does.

**How to avoid:**
- Store the full W3C `traceparent` (not just a bare `trace_id`) on the outbox row at write time, inside the same transaction as the state change — the envelope's `trace_id` field should be sourced from this, and the relay must inject it into Kafka message headers on publish, not generate a fresh one.
- Build this into `pkg/kafka`'s producer/consumer wrapper once (inject on publish, extract-and-continue-span on consume) so every service gets it automatically — this is exactly the kind of cross-cutting concern that's worth getting right in the shared package rather than per-service.
- Verify end-to-end, not just unit-test the wrapper: Phase 0's "trace across service (HTTP + Kafka) visible in Tempo" success criterion should specifically include one async hop (publish → outbox relay → consume), not just a synchronous HTTP chain.
- Watch for span-per-poll noise from the outbox relay itself (a naive relay creates a trace/span every poll cycle even when there's nothing to publish) — this pollutes Tempo with low-value traces; only create/continue spans when there's an actual event to publish.

**Warning signs:**
- Traces in Tempo that visibly "restart" (new trace ID) after crossing a Kafka publish/consume boundary instead of continuing the same trace.
- Outbox table has no trace-context column, only `trace_id` derived ad hoc at relay time.
- Debugging a real saga (e.g., booking → payment → ticket) requires manually correlating logs by `booking_id` because traces don't connect the services.

**Phase to address:** Phase 0 — this is explicitly called out as a Phase 0 success criterion ("trace ข้าม service เห็นใน Tempo") and must include the Kafka hop, not just HTTP, or the gap won't be discovered until a real cross-service bug in Phase 3/4 needs it.

---

### Pitfall 12: testcontainers-based integration tests slow/flaky in Jenkins CI

**What goes wrong:**
Per-service integration tests spinning up real Postgres + Redpanda via testcontainers are valuable (they catch what mocks can't — actual transaction/locking behavior, actual Kafka ordering/partitioning) but come with known failure classes: fixed/conflicting port allocation across parallel test runs, slow container startup dominating CI time as more services are added, and Jenkins agents needing Docker-in-Docker or a mounted Docker socket (a CI infra setup step that's easy to get wrong once, then forget). With 7+ services each wanting their own testcontainers-backed suite, CI time can balloon, tempting shortcuts (skipping integration tests, running them less often) that undermine exactly the tests this project most needs (the capacity race test, saga tests).

**Why it happens:**
Testcontainers is designed to "just work" locally on a dev machine with plenty of resources and no port contention; Jenkins CI agents are shared, resource-constrained, and may run multiple jobs concurrently, surfacing port and resource contention that never appears in local dev. As more services are added (Phase 0 → Phase 4), the naive approach of "every service's test suite spins up its own fresh Redpanda + Postgres containers" multiplies both startup time and flakiness surface linearly.

**How to avoid:**
- Let testcontainers use dynamic/random port allocation (its default behavior) rather than fixed ports — if any config pins ports for "convenience," remove it; fixed ports are a documented, common cause of CI flakiness with Kafka+ZK-style setups.
- Confirm the Jenkins agent has Docker available (socket-mounted or DinD) as a Phase 0 CI setup task, verified with a trivial testcontainers smoke test before any real service test depends on it.
- Reuse a single Redpanda/Postgres container across a test suite/package (testcontainers supports container reuse) rather than one fresh container per test, to cut startup overhead — only isolate per-test what actually needs isolation (e.g., a fresh DB per test via schema/transaction rollback, not a fresh container).
- Keep the fast/cheap test layer wide (unit tests, mocked Kafka producer for business-logic-only tests) and the testcontainers layer narrow and targeted (the capacity race test, saga happy-path, idempotency/redelivery tests) — don't testcontainers-ify every trivial case; that's what pushes CI time from minutes to tens of minutes.
- Track CI wall-clock time per phase as a soft budget — if `make test-integration` creeps past a few minutes, it's a signal to prune scope (fewer container spins, more shared fixtures) before it becomes an excuse to skip running it.

**Warning signs:**
- CI runs intermittently failing with "port already allocated" or container-startup-timeout errors that pass on retry.
- `make test-integration` wall-clock time steadily increasing each phase without a corresponding increase in what it verifies.
- Developers (i.e., future-you) starting to run `go test ./... -short` habitually to skip the integration suite because it's slow, meaning the capacity-race/saga tests stop running before every commit as intended.

**Phase to address:** Phase 0 (Jenkins Docker access + testcontainers smoke test) and Phase 3 (first real load-bearing integration test — the capacity race). Revisit the "one container per service suite vs. shared" decision once 3–4 services have integration suites (around Phase 2–3), before it becomes expensive to change.

---

### Pitfall 13: Next.js App Router + i18n + PWA combined gotchas

**What goes wrong:**
Three areas that are each individually well-documented but interact badly when combined in App Router: (1) i18n routing (`/th/...` vs `/en/...` or locale detection) needs to cooperate with static/dynamic rendering choices — over-relying on `generateStaticParams` for locale-prefixed routes can silently produce stale content for frequently-changing data (availability, prices) if pages aren't correctly marked dynamic where they need to be; (2) Server Components fetching data directly means locale-aware formatting (dates in Asia/Bangkok, currency in Thai baht formatting, Thai number/comma conventions) must be threaded through server-rendered content correctly, not just left to client-side `Intl` calls that might run with the wrong assumed locale/timezone; (3) PWA (service worker, manifest, installability) caching strategy can easily cache availability/pricing data that must always be fresh — a naive "cache everything" service worker breaks the "no overbooking, always show current availability" requirement by serving a stale search-results page from cache.

**Why it happens:**
App Router, i18n, and PWA are each optional additions to a base Next.js app and each has its own tutorial-level guidance; the interactions between them (which pages must never be statically cached, how locale threads through Server Components, what the service worker must explicitly exclude from caching) are the kind of cross-cutting detail that's easy to miss when following each guide independently.

**How to avoid:**
- Explicitly mark availability/search/checkout routes as dynamic (not statically generated/cached) — these must always hit the BFF live; only marketing-style pages (home, static route info) are safe to statically optimize.
- Service worker/PWA caching strategy should use a network-first (or network-only) strategy for any API call touching availability, pricing, or booking state, and cache-first only for static assets (images, fonts, shell) — decide and document this split before wiring up the service worker, not as an afterthought.
- Centralize timezone/locale formatting in one shared utility (both server and client use the same helper, always explicit about `Asia/Bangkok` for departure times regardless of viewer's browser locale/timezone — ties back to Pitfall 6) rather than scattering `Intl.DateTimeFormat`/`toLocaleString` calls with implicit defaults across components.
- Test the "install as PWA, go briefly offline, come back online" flow manually at least once before considering PWA "done" — the most common PWA bug is a stale cached shell showing wrong/old data with no visible indication it's stale.

**Warning signs:**
- Search results or availability badges appearing to not update after a booking is made elsewhere, especially right after a page reload (classic stale-cache symptom).
- Departure times displaying differently depending on the browsing device's system timezone/locale.
- Lighthouse/PWA audit passing (installable, has manifest) while functional testing shows stale data being served.

**Phase to address:** Phase 0 (Next.js skeleton — decide the i18n routing approach and static/dynamic boundaries early) and Phase 7 (Polish & launch, where PWA/service worker is explicitly scoped) — but the "never cache availability" rule should be a documented constraint from Phase 0 even if the service worker itself isn't built until Phase 7, so no earlier page gets built assuming static caching is fine.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| Denormalize PII (email/name) into an event payload "to save a sync call" | Faster to build notification flow | Violates PDPA guardrail; PII now retained in Kafka/DLQ/logs indefinitely | Never |
| Skip the second capacity race test (concurrent booking + admin capacity change) | Saves Phase 3 time | Silent overbooking path ships untested | Only if explicitly tracked as a Phase 3 follow-up, not silently dropped |
| Use `docker-compose.yml` dev Redpanda flags for the prod EC2 compose file too | One less config to maintain | Silent data loss risk on instance restart | Never for the prod compose file |
| Read-then-write `ValidateTicket` instead of atomic check-and-set | Simpler code initially | Replay/double-boarding window | Never — this is a one-line difference in query shape, no real cost to doing it right from the start |
| Float/JS Number for money anywhere outside the display boundary | Marginally simpler frontend form code | Reconciliation drift, refund mismatches | Never |
| Skip explicit topic retention config, rely on Redpanda defaults | One less thing to configure in Phase 0 | Disk fill outage or premature event expiry later | Only briefly in local dev compose; must be explicit before any real customer data |
| DLQ with logging only, no query/replay tooling, through Phase 4 | Saves building admin UI before it's needed (Phase 6 scope) | Silent data loss if a bug routes real bookings/refunds to DLQ unnoticed | Acceptable if DLQ writes are at minimum logged at ERROR with full context and periodically eyeballed — not acceptable as fully silent |
| Skip testcontainers reuse/pooling optimizations, one container per test | Simpler test code | CI time balloons as services are added | Acceptable through Phase 1–2; revisit once suites multiply |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| Opn (Omise) PromptPay | Trusting webhook payload directly to confirm payment | Verify signature, then re-fetch charge status from Omise API before confirming booking |
| 2C2P | Reading backend response payload as plain JSON | Decode/verify the JWT-encoded payload per 2C2P's backend-response method |
| Either Thai payment provider | Assuming sandbox behavior (timing, duplicate/ordering) matches production | Explicitly test duplicate and out-of-order webhook delivery in integration tests, not just sandbox happy-path manual testing |
| Redis (seat hold TTL) | Relying solely on keyspace-notification expiry events to release holds | Keep the cron sweep as the source of truth backstop — keyspace notifications can be missed (not delivered reliably, e.g., if Redis restarts or the subscriber briefly disconnects) |
| Redpanda in Docker Compose | Using the same image/flags for local dev convenience and the "prod" single-EC2 deployment | Separate dev vs prod compose configs; prod durable settings (fsync, persistent volume, explicit retention) |
| Kong (or chosen gateway) JWT forwarding | Trusting `X-User-*` style headers from any caller, not just the gateway | Services must reject/ignore those headers unless the request demonstrably came through the gateway (network policy/mTLS/shared secret) — a header-based trust boundary is only as good as what enforces "only the gateway can set it" |
| OpenTelemetry + Kafka | Assuming trace context "just propagates" across a publish/consume boundary like it does over HTTP | Manually inject/extract `traceparent` via outbox row column → message headers, built once into `pkg/kafka` |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Search page querying `catalog` + batch `booking.GetAvailability` live on every request (no projection) | Slow search page as departures/routes grow | Explicitly planned as v1 tradeoff; add caching (short TTL) on availability batch reads before building a full projection service | Likely fine through v1 launch scale; revisit if search p95 latency becomes user-visible or route/departure count grows an order of magnitude |
| Cron sweep for expired holds scanning all bookings instead of an indexed/targeted query | Sweep job slows down as booking volume grows | Index/query specifically on `hold_expires_at` with a status filter, not a full table scan | Noticeable once bookings table reaches tens of thousands of rows without an index |
| Outbox relay polling interval too slow or too fast | Too slow = event latency hurts UX (e.g., slow ticket issuance); too fast = wasted DB polling load | Pick a sane default (e.g., 200ms–1s) and make it configurable per service; consider Postgres `LISTEN/NOTIFY` to wake the relay instead of pure polling once latency matters | Becomes noticeable once booking saga latency (create → confirm → ticket → email) is visibly slow to users |
| Single Postgres instance, multiple databases (all 7 services) sharing one instance's resources | One service's slow/locking query (e.g., a bad admin report query) can starve unrelated services' connections/CPU | Enforce connection pool limits per service, watch slow query logs, keep the "database-per-service" logical separation real even on shared hardware | Breaks first under a bad ad hoc admin report query, or once total connection count approaches Postgres's `max_connections` |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| QR ticket includes booking ID, JWT claims, or other structured/guessable data instead of an opaque random token | Screenshot/leak reveals more than "this ticket exists"; guessable tokens enable enumeration | 32-byte random token, hash stored server-side, nothing else encoded (already scoped correctly — verify it's implemented that way) |
| `ValidateTicket` implemented as separate read + write | Replay/double-boarding via race | Atomic conditional update (`UPDATE ... WHERE status='valid'`) |
| Trusting gateway-forwarded auth headers without verifying the request actually came through the gateway | Header spoofing lets a caller impersonate any role/operator | Network-level enforcement that only the gateway can reach services directly (internal network, no public exposure of service ports) |
| Payment webhook endpoint accepting unsigned/unverified payloads | Forged "payment succeeded" webhook confirms an unpaid booking | Verify provider signature on every webhook before processing; log-and-reject on failure |
| Query without `operator_id` scoping on any admin/staff endpoint | Cross-tenant data leak (pier_admin sees another operator's bookings/reports) | Enforce `operator_id` scoping at a shared middleware/query-builder layer, not per-handler discretion |
| OTP flow without rate limiting | Brute-force/abuse of guest login | Rate limit OTP requests and verification attempts per phone/email/IP from Phase 1 |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Generic "booking failed" error when a hold expires mid-checkout or a capacity race loses | Confused/frustrated customer, may retry blindly or abandon | Specific message ("this departure filled up while you were checking out — see other times") with a clear next action |
| Countdown timer with no clear recovery path when it hits zero | Customer stuck on a dead checkout page | On expiry, redirect to departure detail with a clear "your hold expired, seats may still be available" message and one-tap re-hold |
| Payment status shown only via a spinner with no timeout/fallback messaging | Customer unsure if payment "worked" during real-world PromptPay confirmation delay (can take longer than expected) | Explicit "waiting for your bank to confirm, this can take a minute" messaging, with a fallback path (check My Bookings, resend confirmation) if polling takes unusually long |
| Departure times shown without explicit timezone context to foreign tourists | Confusion about whether time is local or home-country time | Always show times unambiguously as pier-local (Asia/Bangkok), consider a small "(local time)" label for foreign-locale users |
| QR ticket hard to scan/read in bright outdoor pier sunlight (small QR, low contrast) | Boarding delays, staff friction | Large high-contrast QR (already scoped as a UI direction — verify actual implementation, not just intent) |

## "Looks Done But Isn't" Checklist

- [ ] **Capacity race test:** Passing the basic 50-concurrent-vs-10-seats test isn't enough — verify it also covers a capacity reduction happening mid-test, and that it runs in CI on every commit, not just once locally.
- [ ] **Idempotent consumers:** `processed_events` table existing isn't enough — verify the idempotency check happens *before* any non-transactional side effect (email send, payment API call), and that a redelivery test actually exists per service.
- [ ] **Hold-expired saga:** "Hold expires, seat releases" happy path isn't enough — verify the late-payment-after-expiry path (re-hold or auto-refund) is implemented and tested, not just designed on paper.
- [ ] **PII-free events:** "We don't put emails in events" as a stated rule isn't enough — grep every `.proto` event definition and every non-identity/booking migration for PII-shaped fields.
- [ ] **Timezone handling:** "We use `Asia/Bangkok` for display" isn't enough — verify schedule-template generation and any date-bucketing logic explicitly converts before taking a date, with tests at midnight boundaries.
- [ ] **Payment webhook idempotency:** "We have a webhook handler" isn't enough — verify a unique constraint on provider transaction reference and a test that sends the same webhook twice.
- [ ] **QR validation:** "Scanning shows valid/invalid" isn't enough — verify the state transition on scan is atomic (can't double-validate the same ticket under concurrent scans).
- [ ] **Trace propagation:** "We see traces in Tempo" isn't enough — verify a trace that crosses an actual Kafka publish/consume hop (not just HTTP) stays as one trace, not two.
- [ ] **DLQ:** "Failed events go to a DLQ topic" isn't enough — verify there's at least a way to see what's in it (even a raw `rpk topic consume` note in a runbook) so it isn't a black hole through Phase 5.
- [ ] **Redpanda prod config:** "It runs in Docker Compose" isn't enough — verify the prod compose file uses durable settings (not dev-mode flags) and has an explicit retention policy, separate from the dev compose file.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|------------------|
| Overbooking already occurred | HIGH | Manually contact affected customers, offer next available departure or refund; add the missing DB CHECK constraint and race test retroactively; run reconciliation query across all active departures to find any other drifted rows |
| PII found leaked into Kafka topic retention / logs | MEDIUM–HIGH | Purge/rotate the affected topic retention early if possible, redact logs, fix the producing code, document the incident (PDPA obligations may require notification depending on scope/severity — treat seriously) |
| Discovered non-idempotent webhook double-charged or double-refunded a customer | MEDIUM | Manually reconcile the specific transaction with the payment provider's dashboard, refund/adjust as needed, add the missing unique constraint + redelivery test before it can recur |
| Redpanda single-broker data loss (disk/instance issue, no backup) | HIGH | If no backup existed, lost events since last consumer-committed offset are unrecoverable from Kafka itself — reconstruct from service databases where possible (Postgres is still source of truth for state; events are largely a delivery mechanism, so recovery hinges on whether all consumers had already applied what they needed before the loss) |
| Timezone bug caused departures to show/report under the wrong local date | LOW–MEDIUM | Backfill/correct the explicit `departure_date_local` column from `departure_at_utc` with correct conversion logic; audit any reports already generated off the wrong column |
| testcontainers CI flakiness causing skipped/ignored integration tests | LOW | Fix port allocation / container reuse config; re-enable and re-run the full suite; treat any period where it was silently skipped as needing a manual re-verification of capacity-race and saga behavior |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| Microservices/Kafka overhead sinking solo-dev timeline | Phase 0 | Time-box Phase 0; measure how long adding the first real service (Phase 1 identity) takes relative to plan |
| Overbooking via schedule/booking inventory drift | Phase 3 | Capacity race test (50 concurrent → 10 seats) + capacity-reduction-mid-booking test, both in CI; DB CHECK constraint on `booked+held<=capacity` |
| Outbox/consumer idempotency mistakes (dup, out-of-order, poison, DLQ black hole) | Phase 0 (shared pkg), verified Phase 2–4 | Redelivery test and out-of-order test per consuming service; DLQ writes visible in logs/dashboard |
| Seat hold expiry vs late payment webhook race | Phase 4 | Explicit test: force-expire a hold, then deliver `PaymentSucceeded` late; assert re-hold-or-refund, never a silent drop |
| Thai payment webhook reliability/idempotency | Phase 4 | Unique constraint on provider ref; duplicate-webhook test; signature verification test |
| Timezone bugs (Bangkok local date vs UTC) | Phase 0 (convention) / Phase 2 (schedule generation) | Unit tests at midnight boundaries; explicit `departure_date_local` column, no ad hoc `DATE()` on UTC timestamps |
| Money as float / rounding drift | Phase 0 (proto convention) / Phase 1 (catalog pricing) / Phase 4 (refunds) | Proto lint for `int64` money fields; refund rounding unit test against odd amounts |
| QR ticket security (replay, screenshot, offline gaps) | Phase 4 | Atomic `ValidateTicket` test (concurrent scans of same token → exactly one success) |
| PII leaking into events/logs/traces | Phase 0 (conventions) / Phase 1 (first PII entities) | Proto review checklist; grep for PII-shaped columns outside identity/booking; log sampling review |
| Redpanda/Kafka single-EC2 operational fragility | Phase 0 (compose health-check ordering, dev/prod split) | Explicit topic retention config checked in; prod compose reviewed separately from dev compose before go-live |
| OpenTelemetry trace propagation across Kafka | Phase 0 | Phase 0 success criterion explicitly includes one Kafka publish/consume hop staying in the same trace in Tempo |
| testcontainers CI flakiness/slowness | Phase 0 (Jenkins Docker access) / Phase 3 (first real integration suite) | CI wall-clock time tracked per phase; no fixed ports in testcontainers config |
| Next.js App Router + i18n + PWA interaction gotchas | Phase 0 (routing/dynamic boundaries) / Phase 7 (PWA/service worker) | Manual "install PWA, go offline, come back" test before considering PWA done; availability routes confirmed dynamic, not statically cached |

## Sources

- [Omise: PromptPay integration docs](https://docs.omise.co/promptpay) — HIGH (official provider documentation)
- [Omise: Integrations overview](https://docs.omise.co/integrations) — HIGH (official provider documentation)
- [2C2P Developer Docs: How to integrate](https://developer.2c2p.com/docs/redirect-api-integrate-with-payment) — HIGH (official provider documentation)
- [2C2P Developer Docs: Payment Response (Backend)](https://developer.2c2p.com/docs/api-payment-response-backend) — HIGH (official provider documentation, JWT-encoded backend response format)
- [Redpanda Docs: Start a Single Broker with Docker](https://docs.redpanda.com/labs/docker-compose/single-broker/) — HIGH (official docs, dev/test-only guidance for Docker deployment)
- [Redpanda Docs: High Availability](https://docs.redpanda.com/current/deploy/redpanda/manual/high-availability/) — HIGH (official docs on replication/durability tradeoffs of single-broker setups)
- [testcontainers-go: Flaky test "port is already allocated" discussion](https://github.com/testcontainers/testcontainers-go/discussions/1917) — MEDIUM (community-reported, cross-checked pattern, common root cause)
- [testcontainers-java: Flaky kafka-cluster example issue](https://github.com/testcontainers/testcontainers-java/issues/4479) — MEDIUM (community-reported, same underlying pattern applies to Go client)
- [Conduktor: Testing Kafka Applications — Testcontainers, Embedded Kafka, and Mocks](https://www.conduktor.io/blog/testing-kafka-testcontainers-embedded-mocks) — MEDIUM (vendor blog, general industry-consistent guidance on layering test strategies)
- Transactional outbox pattern, saga pattern (choreography), consumer idempotency, database-per-service tradeoffs, timezone-as-local-date modeling, QR/ticketing replay protection, PDPA data-minimization principles — HIGH confidence, well-established distributed-systems and booking-domain patterns from general software engineering practice, cross-referenced against the project's own PROJECT.md sagas/guardrails (§5.3–5.5, §9)

---
*Pitfalls research for: Event-driven ferry/boat booking platform (Go microservices + Kafka, solo developer)*
*Researched: 2026-09-25*
