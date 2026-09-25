# Architecture Research

**Domain:** Event-driven booking/inventory system (ferry departures), Go microservices + Kafka
**Researched:** 2026-09-25
**Confidence:** MEDIUM (patterns are industry-standard and cross-corroborated across multiple independent sources + framework docs; no library version was pinned against a live registry, so treat exact API names as directional, not copy-paste truth)

This file **validates and deepens** the seed's architecture (`PROJECT.md` §5, §12) — it does not redesign service boundaries, event topics, envelope, outbox, idempotent consumers, capacity flow, or the three sagas, all of which the seed already specifies correctly. Where the seed left a mechanism under-specified (outbox relay internals, hold-expiry reliability, webhook-after-expiry race, JWT→header trust, trace propagation), this file fills in how such systems are built in practice.

## Standard Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│  Next.js 15 PWA (apps/web)                                           │
└───────────────────────────────┬──────────────────────────────────────┘
                                 │ HTTPS (JWT httpOnly cookie)
┌────────────────────────────────▼──────────────────────────────────────┐
│  Kong Gateway  — TLS, routing, rate limit, JWT verify plugin          │
│  → injects X-User-Id / X-Operator-Id / X-Role as upstream headers     │
└────────────────────────────────┬──────────────────────────────────────┘
                                 │ trusted headers only from Kong's network
┌────────────────────────────────▼──────────────────────────────────────┐
│  gateway (BFF) — chi HTTP, aggregate search, SSE availability fan-out │
└───┬─────────┬─────────┬─────────┬─────────┬─────────┬─────────┬──────┘
    │ connect-go (sync, ≤2 hops)                                        │
┌───▼───┐ ┌───▼────┐ ┌──▼──────┐ ┌──▼──────┐ ┌──▼──────┐ ┌──▼───┐ ┌──▼──────────┐
│identity│ │catalog │ │schedule │ │booking  │ │payment  │ │ticket│ │notification │
└───┬───┘ └───┬────┘ └──┬──────┘ └──┬──────┘ └──┬──────┘ └──┬───┘ └──┬──────────┘
    │         │         │            │            │           │        │
    └─────────┴─────────┴────────────┴────────────┴───────────┴────────┘
                       Kafka/Redpanda (async, event-per-aggregate topics)
                       + transactional outbox per service (poll+publish)
┌──────────────────────────────────────────────────────────────────────┐
│  PostgreSQL 16 — one logical DB per service (own instance in v1)      │
│  Redis — booking hold TTL + identity OTP/rate-limit                   │
└──────────────────────────────────────────────────────────────────────┘
```

All 7 services + gateway + Redpanda + Postgres + Redis + Kong + Grafana(Tempo/Loki/Prometheus) run as `docker compose` on one EC2 host in v1 — see Integration Points for the compose topology.

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|-------------------------|
| Kong | TLS termination, routing, JWT verification (rejects invalid/expired tokens), rate limiting | Managed plugin (`jwt`) does verify + reject; claim→header remapping needs a small custom/community plugin or is pushed down to the BFF (see Pattern 6) |
| gateway (BFF) | Aggregate reads for search (catalog + booking availability batch), SSE fan-out of `AvailabilityChanged`, sets/reads the JWT cookie | Thin chi service; talks to other services only via connect-go, never touches their DBs |
| identity | Auth source of truth: users, OTP, JWT issuance, operator membership/roles | Issues JWT; every other service treats JWT claims as read-only once past Kong/BFF |
| catalog | Static-ish reference data: operators, piers, routes, boats, prices | CRUD + `GetRoute`/`ListPiers` sync; publishes upsert events for other services to cache |
| schedule | Owns *admin-set capacity* and departure lifecycle (create/edit/cancel) | Publishes `schedule.*` events; calls booking sync (`CheckCapacityReducible`) before committing a capacity cut |
| booking | Owns *real inventory* (booked/held) and the booking state machine; **single source of truth for overbooking prevention** | `SELECT ... FOR UPDATE` on `departure_inventory` row inside one Postgres tx per hold/confirm/expire/cancel |
| payment | Payment intents, provider webhooks, refunds | Provider-agnostic interface + mock; webhook handler idempotent on provider ref |
| ticket | QR issuance/validation, check-in log | Token = random 32 bytes, DB stores hash only |
| notification | Outbound email/SMS with delivery idempotency | `notification_log` keyed by `(event_id, channel)` |
| Redpanda | Kafka-compatible broker, topic-per-aggregate + DLQ | Single binary in dev/v1-prod; swap to MSK later without app changes |
| Redis | Hold TTL signal (not source of truth) + OTP/rate-limit | `booking` and `identity` only — no cross-service Redis sharing |

## Recommended Project Structure

The seed's repo layout (`PROJECT.md` §12) is correct; the detail worth adding is what goes inside `internal/` per service and how `pkg/` stays generic:

```
Boat-Booking/
├── go.work                          # local dev convenience only — NOT what CI/Docker builds use
├── proto/
│   ├── events/*.proto                # past-tense facts, one file per aggregate
│   ├── services/*.proto              # connect-go service defs (commands/queries)
│   └── buf.gen.yaml                  # single template, multiple plugin blocks (see Pattern 8)
├── gen/{go,ts}/                      # generated, committed (simpler than gen-in-CI for a solo dev)
├── pkg/
│   ├── events/                       # envelope struct + proto-generated types, no business logic
│   ├── kafka/                        # franz-go client wrapper: producer, consumer group, otel hooks, DLQ
│   ├── outbox/                       # outbox table schema (goose migration snippet) + relay worker
│   ├── httpx/                        # chi middleware: request-id, trusted-header parsing, error envelope
│   ├── auth/                         # JWT verify (identity/BFF only) + claims-from-header (everyone else)
│   └── pgx/                          # pool setup, tx helper (`WithTx`), health check
├── services/<name>/
│   ├── cmd/main.go                   # wiring only: config → adapters → app → HTTP/Kafka servers
│   ├── internal/
│   │   ├── domain/                   # entities, value objects, state machine — no framework imports
│   │   ├── app/                      # use cases (CreateBooking, ConfirmBooking...) orchestrate domain + ports
│   │   └── adapters/
│   │       ├── postgres/             # sqlc-generated queries + repository impl
│   │       ├── kafka/                # outbox writer (app→domain events), consumer handlers
│   │       └── connect/              # inbound connect-go handlers + outbound clients to other services
│   ├── migrations/                   # goose, includes the outbox + processed_events tables
│   ├── CLAUDE.md                     # owns / publishes / consumes / DB-isolation reminder
│   └── Dockerfile
├── apps/web/                         # Next.js — talks to gateway only, never to services directly
└── deploy/docker-compose.yml         # dev topology: infra + all 8 Go services + web
```

### Structure Rationale

- **`internal/domain` has zero imports from `pkg/kafka` or `pkg/pgx`.** This is what makes the capacity race testable with `go test` against an in-memory/sqlite fake before paying for testcontainers — keep the `SELECT ... FOR UPDATE` logic in `adapters/postgres`, called from `app`, with `domain` only deciding "is this transition legal."
- **`adapters/kafka` in each service contains both the outbox *writer* (adds a row in the same tx as the state change) and the *consumer* handlers** — but the outbox *relay* (the poll-and-publish loop) lives in `pkg/outbox` as a reusable worker, parameterized per service by table name and topic. Don't reimplement the relay loop 7 times.
- **`pkg/` never imports a `services/*` package.** If a helper only one service needs, it stays in that service's `internal/`, not `pkg/`. This is the guardrail that keeps `pkg/` genuinely shared instead of becoming a dumping ground copied blindly by `make new-service`.

## Architectural Patterns

### Pattern 1: Transactional Outbox Relay (Go, polling)

**What:** Business tx writes state + an `outbox` row in the same Postgres transaction. A separate relay goroutine polls the table, publishes to Kafka, marks rows delivered.

**When to use:** Every write that must result in a published event (i.e., always, per the seed's guardrail — no direct `producer.Produce()` calls from handlers).

**Trade-offs:** Simpler than Debezium/CDC (seed's explicit choice — correct for v1: one binary, no extra infra, full control over batching); costs a poll-interval of latency (1–5s is the usual range) and requires the relay itself to be horizontally-safe (see below).

**Concrete shape for this project:**

```sql
create table outbox (
  id            bigserial primary key,   -- monotonic, use for ordering — NOT created_at
  aggregate_id  uuid not null,           -- departure_id / booking_id — becomes Kafka key
  event_type    text not null,
  payload       bytea not null,          -- proto-marshaled envelope
  created_at    timestamptz not null default now(),
  published_at  timestamptz              -- null = not yet relayed
);
create index on outbox (published_at) where published_at is null;
```

- **Polling:** `SELECT ... WHERE published_at IS NULL ORDER BY id LIMIT N FOR UPDATE SKIP LOCKED`, publish batch, then `UPDATE ... SET published_at = now() WHERE id = ANY(...)`. `FOR UPDATE SKIP LOCKED` is what lets you later run the relay as more than one replica without double-publishing the same row concurrently (not needed at v1 scale — one relay goroutine per service is enough on a single EC2 box — but costs nothing to include now).
- **Ordering per aggregate:** Kafka gives ordering *within a partition*; use `aggregate_id` as the message key (matches the seed's partition-key convention exactly) so all events for one `departure_id`/`booking_id` land in the same partition and are read in the order the relay published them. Order the relay's own publish loop by the outbox `id` (bigserial), never by `created_at` — wall-clock timestamps can tie or skew even on one machine under load.
- **At-least-once, never exactly-once:** a crash between "broker ack received" and "UPDATE published_at" republishes the same row on the next poll. This is why every consumer must dedupe (Pattern 2) — the outbox pattern's correctness depends on that pairing, not on the relay being clever.
- **Failure mode to design for:** if Redpanda is down, the relay should back off and retry, not drop rows — the table itself is the durable queue, so a stalled relay is safe (events pile up, nothing is lost) as long as nothing purges unpublished rows.

### Pattern 2: Idempotent Consumer Table

**What:** Every consumer checks a `processed_events(event_id uuid primary key, processed_at timestamptz)` table (or a column on the entity itself) before applying an event, and inserts into it in the *same transaction* as the side effect.

**When to use:** Every Kafka consumer and every payment webhook handler (the seed already mandates this — the detail worth adding is *how* to make the check-and-apply atomic).

**Trade-offs:** One extra row + one extra `INSERT ... ON CONFLICT DO NOTHING` per event; the alternative (checking then applying non-atomically) reintroduces the race the pattern exists to close.

**Shape:**

```go
func (r *bookingRepo) ApplyPaymentSucceeded(ctx context.Context, evt PaymentSucceeded) error {
    return r.pgx.WithTx(ctx, func(tx pgx.Tx) error {
        tag, err := tx.Exec(ctx,
            `insert into processed_events (event_id, event_type) values ($1,$2) on conflict do nothing`,
            evt.EventID, evt.Type)
        if err != nil { return err }
        if tag.RowsAffected() == 0 {
            return nil // already applied — commit is a no-op, consumer offset still commits
        }
        // ... held -= n; booked += n; booking status = paid ...
        return nil
    })
}
```

Kafka commit happens **after** the transaction commits (manual offset commit, no auto-commit — matches the seed's convention), so a crash between apply and commit just reprocesses the same event on redelivery, which the `on conflict do nothing` makes a safe no-op.

### Pattern 3: Choreography for the 3 sagas — do not add an orchestrator in v1

**What:** All three sagas in the seed (booking happy path, departure cancelled, hold expired) are chains where each service reacts to the previous service's published event and emits its own. This is choreography.

**When to use:** Choreography fits when the workflow is a straight-ish chain with few branches and the team is small (solo dev here) — it avoids standing up and operating a stateful orchestrator (e.g., Temporal) for three flows that are each 3–5 steps.

**Trade-offs:** Choreography gets hard to reason about as branch count grows and makes end-to-end testing require the whole chain running. At the scale of this project (7 services, 3 sagas, single dev) that cost is acceptable; an orchestrator would be premature infrastructure for v1.

**Recommendation:** Keep choreography, but treat each saga's "what happens on step N failing" as an explicit second event chain, not an afterthought:
- **Booking happy path:** `BookingCreated → payment creates intent → webhook → PaymentSucceeded → booking confirms → BookingConfirmed → ticket issues → TicketIssued → notification sends`. Failure at any step should emit a *fact* event (`PaymentFailed`), not just log-and-stop — booking needs to hear `PaymentFailed` to release the hold if the customer's countdown hasn't already expired it.
- **Departure cancelled:** fan-out from one event (`DepartureCancelled`) to N bookings — implement this as booking querying its own DB for all bookings on that departure and emitting one `BookingCancelled` per booking (not N separate Kafka-triggered handlers), so the "cancel all bookings for a departure" step is one local transaction loop, not a distributed fan-out that can partially fail.
- **Hold expired:** see Pattern 5 below — this is the saga most likely to have a real production bug (the payment-after-expiry race), so give it the most test coverage.
- **Revisit in a later milestone** only if a 4th or 5th saga needs cross-cutting timeout/retry logic that choreography can't express cleanly (e.g., multi-step refund with manual ops approval) — that is a legitimate trigger for introducing an orchestrator, not something to build speculatively now.

### Pattern 4: Kong + BFF — JWT → trusted claim headers

**What:** Kong terminates TLS and verifies the JWT signature/expiry at the edge (reject bad tokens before they reach any Go service). Claims must then reach services as headers those services can trust *without re-verifying the JWT themselves*.

**Gap in the seed to close:** Kong's stock `jwt` plugin authenticates and forwards a fixed small set of consumer-identity headers; it does **not** natively remap arbitrary custom claims (e.g. `operator_id`, `role`) into arbitrary `X-*` headers. There are two ways to close this, both fine for v1:

1. **Custom/community Kong plugin** (Lua, e.g. patterns like `kong-jwt2header`) that copies named claims into headers at the Kong layer.
2. **Push the remapping into the BFF instead of Kong** — Kong verifies signature/expiry only; the BFF (which already sits behind Kong on every request) re-parses the same JWT from the cookie and sets `X-User-Id` / `X-Operator-Id` / `X-Role` before calling downstream services via connect-go.

**Recommendation for this project: option 2.** It keeps Kong config declarative (routes/plugins only, no custom Lua to maintain solo) and keeps the claim→header logic in Go where it's testable with `go test` and shares code with `pkg/auth`. The hard rule downstream: every service's `pkg/httpx` middleware must **reject any request whose trusted headers didn't come from the internal network** (Kong/BFF-only network segment in docker-compose / security group in EC2) — an external caller must never be able to set `X-Operator-Id` directly. This is the actual security boundary, not the header format.

### Pattern 5: Hold expiry — Redis is a trigger, Postgres cron sweep is the truth

**What:** The seed already specifies both Redis keyspace-notification TTL *and* a DB-side cron sweep for hold expiry. Research confirms this dual mechanism is the right call, not redundant belt-and-suspenders:

- Redis keyspace `expired` notifications are **at-most-once, fire-and-forget** — a subscriber restart (deploy, crash), a Redis restart (Pub/Sub state isn't persisted), or a slow consumer's buffer overflow silently drops the event. Nothing about Redis pub/sub guarantees the "release this hold" side effect ever fires.
- The `expired` event also fires only when Redis *actually deletes* the key (lazy or active-expiry cycle), which can lag the nominal TTL — so even a healthy subscriber sees "expired" a little after the true deadline.

**Recommendation:** Treat Redis TTL purely as a **low-latency trigger** for the happy path (subscriber releases the hold within ~seconds of expiry, so the UI updates fast) — never as the sole mechanism. The **DB-side cron sweep is the correctness backstop**: a periodic job (every 30–60s is plenty for a 10-minute hold window) does `UPDATE bookings SET status='expired' ... WHERE status='pending_payment' AND hold_expires_at < now()` inside the same `SELECT ... FOR UPDATE` transaction shape as the rest of booking's inventory writes, then writes the `BookingExpired` outbox row. This means: even if Redis pub/sub loses every notification for an hour, no hold outlives its `hold_expires_at` by more than one sweep interval, and overbooking guarantees hold regardless of Redis's reliability.

### Pattern 6: Payment webhook arriving after hold expiry (the real race)

**What:** Customer pays right as (or just after) the 10-minute hold expires; the provider webhook lands in payment-service *after* booking-service has already swept the hold to `expired` and released `held -= n`.

**This is exactly the seed's "Hold expired saga" §5.4 race clause** — worth making explicit as a concrete sequence so it's testable:

```
t=10:00  booking created, hold_expires_at = t+10m
t=10:09  customer completes payment on provider's page
t=10:10  cron sweep runs first: booking → expired, held -= n, outbox BookingExpired
t=10:10  provider webhook arrives: payment succeeded, payment-service publishes PaymentSucceeded
t=10:11  booking-service consumes PaymentSucceeded for a booking that is already `expired`
```

**Resolution (matches seed intent, spelled out as an implementable rule):** booking's `PaymentSucceeded` consumer must branch on current booking status inside the same locked transaction:
- If status is `pending_payment` → normal path: `held -= n; booked += n`, status → `paid`, emit `BookingConfirmed`.
- If status is `expired` → **attempt re-hold**: re-run the same `SELECT ... FOR UPDATE` capacity check as a fresh `CreateBooking` would (available seats for that departure ≥ n). If capacity still allows it, flip status back to `paid` directly (skip `pending_payment`), `booked += n`, emit `BookingConfirmed`. If capacity is now full (someone else took the seats), do **not** confirm — emit a `PaymentSucceeded`-triggered `RefundRequested`/auto-refund path so payment-service issues a refund and notification tells the customer their seats were lost, money is coming back. This branch is why payment-service must never assume a booking event implies the booking still exists in that state — it only reacts to its own idempotency, booking owns the truth.
- Either branch must stay idempotent against the outer consumer-table pattern (Pattern 2) — a redelivered `PaymentSucceeded` for an already-`paid` booking is a no-op, not a double-credit.

This is the single highest-value integration test to write in Phase 4 beyond the capacity race test already mandated in Phase 3.

### Pattern 7: connect-go for sync calls — respect the 2-hop limit

**What:** connect-go services expose Connect/gRPC/gRPC-Web on one `http.Handler`; curl-debuggable, gen's a TS client too (useful if the BFF or even the frontend ever calls a service type-safely).

**The seed's rule — "no sync chain longer than 2 hops" — is the architecturally important constraint to enforce in practice**, not just state. Concretely:

- **Hop 1:** BFF → service (e.g., BFF → booking.`GetAvailability`). Fine.
- **Hop 2 (the limit):** service → service (e.g., schedule → booking.`CheckCapacityReducible`). Also fine — this is the one sync fan-out the seed's capacity flow requires.
- **What to never build:** BFF → schedule → booking → payment as one blocking chain. If a use case seems to need 3 hops of synchronous calls, that's the signal the third step should be event-driven (publish + a saga step) instead of another connect-go call. The 2-hop rule is really "one caller, one callee, no callee that itself calls a third service synchronously on the request path" — enforce it at code review time on every new connect-go client added to a service's `adapters/connect`.
- **Timeouts matter more than usual here:** because a hop can block a caller's own request, every connect-go client needs an explicit context deadline shorter than the caller's own SLA (e.g., `CheckCapacityReducible` called with a 2s deadline) so a slow booking-service doesn't cascade into schedule-service request pileup.

### Pattern 8: OpenTelemetry trace propagation across Kafka with franz-go

**What:** W3C Trace Context (`traceparent`, plus optionally `tracestate`) is injected as Kafka record headers on publish and extracted on consume, giving one continuous trace across HTTP → outbox → Kafka → consumer, matching the seed's "trace_id ต่อกันข้าม Kafka" requirement from Phase 0.

**Implementation shape (franz-go has no first-party OTel plugin as of this research — wire it manually in `pkg/kafka`):**

```go
// producer side, inside pkg/outbox's relay (or wherever a Record is built)
carrier := propagation.MapCarrier{}
otel.GetTextMapPropagator().Inject(ctx, carrier)
headers := make([]kgo.RecordHeader, 0, len(carrier))
for k, v := range carrier {
    headers = append(headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
}
record := &kgo.Record{Topic: topic, Key: []byte(aggregateID), Value: payload, Headers: headers}

// consumer side, inside pkg/kafka's consume loop, before dispatching to the handler
carrier := propagation.MapCarrier{}
for _, h := range record.Headers {
    carrier[h.Key] = string(h.Value)
}
ctx := otel.GetTextMapPropagator().Extract(ctx, carrier)
ctx, span := tracer.Start(ctx, "consume "+record.Topic, trace.WithLinks(...))
```

- Put this once in `pkg/kafka`'s producer wrapper and consumer wrapper — every service gets it for free via the shared package, no per-service wiring.
- The same `trace_id` should also be carried in the envelope's own `trace_id` field (the seed already puts it there) so it's visible in structured logs even without a trace backend attached — belt-and-suspenders that costs nothing.
- Verify this concretely in Phase 0's acceptance check ("trace ข้าม service เห็นใน Tempo") by triggering one HTTP request that produces a Kafka event and confirming Tempo shows one trace spanning the HTTP span and the Kafka consume span, not two disconnected traces — this is a real gap to catch early since it's easy to wire the HTTP side (broadly documented, standard chi/otelhttp middleware) and forget the Kafka side (needs the manual carrier code above).

### Pattern 9: Go monorepo with go.work + proto/buf → Go and TS

**What:** `go.work` at the repo root lists every service module + `pkg/*` for local cross-module editing without `replace` directives; each service still builds from its own `go.mod` for its Docker image (the workspace file is a dev convenience, not a build input).

```
go work init ./pkg/events ./pkg/kafka ./pkg/outbox ./pkg/httpx ./pkg/auth ./pkg/pgx \
             ./services/identity ./services/catalog ./services/schedule \
             ./services/booking ./services/payment ./services/ticket \
             ./services/notification ./services/gateway
```

For proto: one `proto/` tree, one `buf.gen.yaml` with two plugin blocks (Go structs + connect-go stubs → `gen/go/...`, and a TS/connect-es plugin → `gen/ts/...`); `buf generate` run from repo root, output **committed** rather than generated-in-CI (simpler for a solo dev — no risk of drifting generated code between a laptop and CI, and `apps/web` can import `gen/ts` directly without a build step). Managed mode in `buf.gen.yaml` keeps `go_package` out of the `.proto` files themselves, which matters here because `proto/events/*.proto` is shared by 7 Go services and the frontend — none of them should need proto-file edits just to fix an import path.

## Data Flow

### Booking happy-path request flow (illustrates hop counts and event direction)

```
Customer → PWA → Kong (JWT verify) → BFF (claims → headers)
  → connect-go call: booking.CreateBooking (hop 1)
      booking: SELECT...FOR UPDATE → held+=n → insert booking(pending) → outbox row (same tx)
      → 202 to customer with hold_expires_at            [Redis TTL key set, non-authoritative]
  ↓ (async, outbox relay polls + publishes)
  Kafka: booking.events → BookingCreated (key=booking_id)
  ↓
  payment consumes BookingCreated → creates provider intent → returns QR to customer via BFF poll
  ↓ (customer pays; provider calls payment's public webhook)
  payment: webhook idempotent-apply → outbox → Kafka: payment.events → PaymentSucceeded
  ↓
  booking consumes PaymentSucceeded → branch on booking.status (Pattern 6) → held-=n,booked+=n → outbox → BookingConfirmed
  ↓
  ticket consumes BookingConfirmed → issue QR (hash stored) → outbox → TicketIssued
  ↓
  notification consumes TicketIssued → send email → notification_log (idempotent)
```

Every arrow after "outbox relay polls" is Kafka (async, one-directional, at-least-once); every arrow before it inside one service is a single Postgres transaction. No service ever calls another service synchronously more than the one documented BFF→booking hop in this flow — everything past that is choreography, which is what keeps this chain from violating the 2-hop rule even though it crosses 5 services.

### Capacity flow (ownership split, restated as data direction)

```
schedule (capacity, admin-set)  --sync CheckCapacityReducible-->  booking (booked+held, real)
schedule --DepartureCreated/CapacityChanged (async)--> booking applies to departure_inventory
booking is the ONLY writer of departure_inventory.booked / .held — schedule never writes it directly
```

## Scaling Considerations

Not a near-term concern for a single-EC2, solo-dev v1 — noted briefly since the seed explicitly defers Kubernetes/MSK:

| Scale | Architecture Adjustments |
|-------|--------------------------|
| v1 (one EC2, docker-compose) | Current design already correct: outbox relay is single-instance per service, Postgres is one instance/many logical DBs, Redpanda single node |
| If booking-service inventory contention becomes the bottleneck | The `SELECT ... FOR UPDATE` row lock is per-`departure_id`, so contention is per-departure, not global — a popular single departure selling out fast is the only realistic hot spot; mitigate with shorter transactions, not architecture change |
| If read load on search grows | The seed's Key Decision #6 (query-live via BFF, no projection service) is the first thing to revisit — add a `search` projection service consuming all events into a denormalized read table, exactly as the seed's Phase 8 already anticipates. Don't build it speculatively now |

## Anti-Patterns

### Anti-Pattern 1: Publishing to Kafka directly from a request handler

**What people do:** Call `producer.Produce()` inside the same handler that mutates Postgres, outside the outbox table.
**Why it's wrong:** Breaks atomicity — a crash or Postgres commit failure after a successful publish (or vice versa) creates an event with no matching state change, or state with no event, silently violating the "no overbook" guarantee's audit trail.
**Do this instead:** Every state-changing write goes through the outbox table in the same transaction (Pattern 1) — no exceptions, including for events that feel "obviously safe" like a cache-invalidation-style upsert.

### Anti-Pattern 2: Trusting `X-Operator-Id`/`X-Role` headers from any request that isn't provably internal

**What people do:** Have every service parse `X-Operator-Id` off the incoming request without checking where the request came from, because "Kong always sets it."
**Why it's wrong:** Any service reachable on a network path that isn't locked down to Kong/BFF-only becomes a privilege-escalation vector — a caller sets `X-Operator-Id` to someone else's operator and gets full data-scope override.
**Do this instead:** Enforce the trust boundary at the network layer (compose internal network / security group, no public port on any service but Kong) **and** in `pkg/httpx` middleware reject requests whose headers weren't set by a documented internal caller — treat this as a Phase 0/1 guardrail, not a later hardening pass.

### Anti-Pattern 3: Letting the hold-expiry cron sweep and the Redis TTL both try to be authoritative

**What people do:** Have the Redis-notification handler *and* the cron sweep both attempt the same `held -= n` mutation without one being clearly primary, risking a double-decrement race between the two paths.
**Why it's wrong:** Two independent paths writing the same counter invites exactly the kind of race the whole system exists to prevent.
**Do this instead:** Both paths call the *same* idempotent "expire this booking if still pending and past due" transaction (`UPDATE ... WHERE status='pending_payment' AND hold_expires_at < now()`, guarded by the same idempotent-consumer/no-op-if-already-expired shape as Pattern 2) — Redis is just what triggers it early, cron is what guarantees it eventually; they must share one code path, not two.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Payment provider (Opn/2C2P — decide at Phase 4) | Inbound webhook to payment-service, provider-agnostic interface + mock implementation for Phases 0–3 | Webhook must be idempotent on provider transaction ref (unique constraint), not just event_id, since providers themselves sometimes retry webhook delivery |
| Resend/SES | Outbound API call from notification-service only | Never call from any other service — keeps PII (email address) resolution scoped to notification + identity/booking |
| Grafana Tempo/Loki/Prometheus | OTel exporter from every Go service via `pkg/httpx`+`pkg/kafka` instrumentation | Stand this up in Phase 0 — retrofitting tracing after 7 services exist is much more expensive than baking it into the service template |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|----------------|-------|
| PWA ↔ gateway (BFF) | HTTPS/JSON via Kong, SSE for availability | PWA never talks to any service directly, even connect-go's TS-friendliness doesn't change this — Kong is the only public edge |
| BFF ↔ services | connect-go (sync, 1 hop from BFF) | BFF is a caller only, never a callee of another service |
| schedule ↔ booking | connect-go `CheckCapacityReducible` (sync, the one sanctioned 2nd hop) + async `Departure*` events both directions of ownership | Capacity value vs. inventory value are two different fields on two different services by design — do not merge them |
| booking ↔ payment/ticket/notification | Async only (Kafka), no sync calls in either direction | Matches "no sync chain past 2 hops" — everything past booking is event-driven |
| Any service ↔ any other service's Postgres | **Forbidden** | Database-per-service is absolute; cross-service data need = consume an event into a local projection, or a sync connect-go call, never a second DSN |

## Build Order Implications (vertical slices)

The seed's Phase 0–4 ordering is architecturally sound; here's the dependency reasoning that confirms it, useful for phase-planning:

1. **Phase 0 (foundation) must produce a working outbox relay + idempotent-consumer skeleton + OTel-through-Kafka wiring in `pkg/*`, proven with one throwaway event**, not just scaffolding. Every later phase's service copies this template — a bug in the relay's `FOR UPDATE SKIP LOCKED` batching or a missing trace-header carrier discovered in Phase 3 means re-touching every service built before it. This is why the seed's own working notes flag Phase 0 as needing the heaviest review, and research confirms that instinct: the outbox+idempotency+tracing trio is exactly the part with the most subtle, hard-to-retrofit correctness properties (Patterns 1, 2, 8).
2. **Identity + Catalog (Phase 1) before Schedule (Phase 2) before Booking (Phase 3)** is forced by data dependency, not just feature grouping: schedule needs `catalog.BoatUpserted` for default capacity; booking needs `schedule.DepartureCreated` to create its inventory projection row. Booking cannot be meaningfully tested (including the mandatory capacity race test) without departures existing, and departures cannot exist without routes/boats existing.
3. **Booking core (Phase 3) is the load-bearing phase for the "no overbook" core value** — it's correct to land it before Payment/Ticket/Notification (Phase 4) specifically so the capacity race test and the hold-expiry sweep (Pattern 5) are proven under concurrency *before* the payment-after-expiry race (Pattern 6) is layered on top. Building payment first would mean testing the hardest race (Pattern 6) without the simpler race (capacity contention) already validated underneath it.
4. **Payment + Ticket + Notification (Phase 4) is correctly one phase, not three**, because they only make sense together: the saga that proves the system end-to-end (BookingCreated → PaymentSucceeded → BookingConfirmed → TicketIssued → email) has no meaningful partial-completion state — shipping payment without ticket+notification leaves a paid booking with no deliverable, which isn't a usable vertical slice.
5. **Kong + BFF trusted-header wiring (Pattern 4) belongs in Phase 1**, not Phase 0, because it needs identity's JWT issuance to exist to test against — but the *middleware shape* in `pkg/httpx` that every later service reuses should be written once in Phase 0/1 and never touched again, same reasoning as point 1.

## Sources

- [The Transactional Outbox in Go: Reliable Events Without a Message Broker](https://dev.to/gabrielanhaia/the-transactional-outbox-in-go-reliable-events-without-a-message-broker-2hfh)
- [Outbox Pattern Internals: Ordering Guarantees, Relay Mechanics, and the Failure Modes Nobody Documents](https://dev.to/neeraj_singhi_golang/outbox-pattern-internals-ordering-guarantees-relay-mechanics-and-the-failure-modes-nobody-50be)
- [Transactional Outbox Pattern in Go with PostgreSQL — Rost Glukhov](https://www.glukhov.org/app-architecture/integration-patterns/transactional-outbox-pattern-go/)
- [How to Implement the Outbox Pattern in Go and PostgreSQL — freeCodeCamp](https://www.freecodecamp.org/news/how-to-implement-the-outbox-pattern-in-go-and-postgresql/)
- [Complete Guide to tracing Kafka clients with OpenTelemetry in Go — SigNoz](https://signoz.io/blog/opentelemetry-kafka/)
- [Distributed Tracing in Kafka Messages with OpenTelemetry in Golang — Medium](https://ceylanomer.medium.com/distributed-tracing-in-kafka-messages-with-opentelemetry-3cb944219fe1)
- [How to Propagate Trace Context Across Kafka Producers and Consumers](https://oneuptime.com/blog/post/2026-02-06-propagate-trace-context-kafka-producers-consumers/view)
- [Connect RPC — Getting started (Go)](https://connectrpc.com/docs/go/getting-started/)
- [Connect RPC — gRPC compatibility](https://connectrpc.com/docs/go/grpc-compatibility/)
- [GitHub — connectrpc/connect-go](https://github.com/connectrpc/connect-go)
- [Kong JWT plugin docs](https://developer.konghq.com/plugins/jwt/)
- [kong-jwt2header — GitHub](https://github.com/yesinteractive/kong-jwt2header)
- [kong-plugin-jwt-claims-advanced — GitHub](https://github.com/tucows/kong-plugin-jwt-claims-advanced)
- [Redis keyspace notifications — official docs](https://redis.io/docs/latest/develop/pubsub/keyspace-notifications/)
- [Redis keyspace notifications and expired events — where they stop being reliable — Stack Harbor](https://stackharbor.com/en/knowledge-base/redis-keyspace-notifications-expired-events/)
- [How to Handle Missed Keyspace Notifications in Redis — oneuptime](https://oneuptime.com/blog/post/2026-03-31-redis-handle-missed-keyspace-notifications/view)
- [Saga Orchestration vs Choreography — Temporal blog](https://temporal.io/blog/to-choreograph-or-orchestrate-your-saga-that-is-the-question)
- [Saga Pattern Demystified: Orchestration vs Choreography — ByteByteGo](https://blog.bytebytego.com/p/saga-pattern-demystified-orchestration)
- [How to Use Go Workspaces for Monorepos](https://oneuptime.com/blog/post/2026-02-01-go-workspaces-monorepos/view)
- [Building a Monorepo in Golang — Earthly Blog](https://earthly.dev/blog/golang-monorepo/)
- [Buf Docs — Generating code](https://buf.build/docs/generate/)
- [Hands-on Buf Monorepo for Go gRPC: A Multi-Module Protobuf Architecture — Medium](https://medium.com/@cassius.paim/hands-on-buf-monorepo-for-go-grpc-a-multi-module-protobuf-architecture-2fd47d16b6a2)
- [Go: Structuring repositories with protocol buffers — David Bond](https://blog.dsb.dev/posts/structuring-repositories-with-protocol-buffers/)
- Project seed: `/home/chonlatee/Desktop/Lab/Boat-Booking/PROJECT.md` §3, §5, §12 (service boundaries, sagas, Kafka conventions, repo structure — validated, not redesigned)

---
*Architecture research for: event-driven ferry booking/inventory platform (Go microservices + Kafka)*
*Researched: 2026-09-25*
