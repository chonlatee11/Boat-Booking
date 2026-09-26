# Phase 1: Platform Foundation - Research

**Researched:** 2026-09-26
**Domain:** Go microservices scaffolding, event-driven plumbing (outbox + Kafka/Redpanda), cross-service OTel tracing, Kong DB-less JWT gateway, Next.js 16 skeleton
**Confidence:** HIGH

## Summary

Phase 1 has almost no open design questions left — `01-CONTEXT.md` already locks 53 decisions (D-01..D-53) covering module layout, event envelope, outbox/DLQ shape, sqlc/config conventions, dev stack, Kong/JWT, Next.js skeleton, Docker, health/shutdown, shared `pkg/*` helpers, and lint gates. `.claude/CLAUDE.md` already pins every core dependency version. This research therefore does **not** re-litigate stack choices — it verifies the *supporting* packages CLAUDE.md doesn't cover, and fills in **implementation patterns and gotchas** for the pieces D-01..D-53 specify the *shape* of but not the *exact code*: OTel-over-Kafka wiring, the dual stdout+OTel slog handler, Kong's RS256 declarative-config quirk, golangci-lint v2's config schema, and the IBM Plex Sans Thai subset trap.

The single most valuable finding: **`github.com/twmb/franz-go/plugin/kotel` already exists and is version-locked with franz-go** (confirmed v1.7.1 pairs with franz-go v1.22.0 on the module proxy). It supplies a `kotel.Tracer` that implements `kgo.Hook` and auto-injects/extracts W3C trace context via `kotel.NewRecordCarrier(record)` — this **replaces** the hand-rolled carrier-loop code sketched in `.planning/research/ARCHITECTURE.md` Pattern 8 ("franz-go has no first-party OTel plugin as of this research" — that statement is now superseded; see State of the Art). Use the plugin, don't hand-roll header injection.

**Primary recommendation:** Build `pkg/kafka` and `pkg/outbox` around `kotel.Hooks()` for span creation and `kotel.NewRecordCarrier` for storing/restoring the `traceparent` on the outbox row (Pitfall 11's exact fix), use `connectrpc.com/otelconnect` for the HTTP/connect-go side, and `go.opentelemetry.io/contrib/bridges/otelslog` for the slog→OTel-Logs bridge — no hand-rolled OTel plumbing anywhere in `pkg/*`.

## User Constraints (from CONTEXT.md)

### Locked Decisions

D-01..D-53 are locked in `01-CONTEXT.md` and are the authoritative source for Phase 1 scope, module layout, envelope shape, outbox/DLQ behavior, config conventions, dev stack versions (Postgres 17, Valkey, Harbor over ECR), Kong RS256 JWT design, Next.js skeleton decisions, Docker/health/shutdown behavior, shared `pkg/*` helper contracts, and lint gates. Full text is in `01-CONTEXT.md` <decisions> — copied by reference, not restated here verbatim (53 items), to avoid drift between the two files. **The planner MUST read `01-CONTEXT.md` directly**, not just this research file, before writing tasks. Key ones this research directly builds on:

- D-06/D-08: envelope shape, `traceparent` travels only in Kafka headers, `Publish(ctx, topic, aggregateID, envelope)` contract.
- D-12/D-13: DLQ headers, idempotent-consumer wrapper `Consumer.Handle(eventType, func(ctx, tx, env) error)`, manual commit only.
- D-27/D-28/D-29/D-30: RS256 JWT, Kong spike time-boxed 1 day with Traefik fallback pre-approved, BFF claim→header remapping, internal-network + shared-secret trust boundary.
- D-46/D-47/D-48: golangci-lint + forbidigo banning `time.Now`/`fmt.Print*`, ESLint+Prettier+tsc strict, lefthook pre-commit.
- D-50/D-52/D-53: OTel→Collector→Prometheus metrics, slog JSON mandatory fields, OTel logs SDK bridge from slog.

### Claude's Discretion

Per D-decisions section "Claude's Discretion": exact chi middleware order, connect interceptor wiring, otel SDK bootstrap code, Makefile target internals, Compose service naming, Grafana panel layout, Redpanda single-node flags, Kong `kong.yml` structure beyond spike criteria, `_template`'s sample HTTP endpoint used to trigger the proof event. **This research's Code Examples section below is written to fill exactly this discretion space** — the otel bootstrap, interceptor wiring, and kotel usage shown are recommendations for the planner to adopt, not additional locked decisions.

### Deferred Ideas (OUT OF SCOPE)

- Server-rendered/SEO data for pier/route pages → Phase 7.
- Refresh-token endpoint + rotation → Phase 2 (identity); `pkg/auth` only models both token kinds in Phase 1.
- DLQ viewer + replay UI → Phase 6; Phase 1 only needs `make dlq-list`.
- Outbox relay via `LISTEN/NOTIFY` or multi-instance relay → only if latency/scale demands it later.
- Kubernetes → out of scope entirely (Harbor chosen partly to ease a later move, but k8s itself stays out).
- Production Redpanda durability flags, retention sizing, backup story → Phase 5.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLAT-01 | `make new-service <name>` scaffolds cmd/, internal/{domain,app,adapters}, migrations/, Dockerfile, CLAUDE.md, /healthz+/readyz wired to pkg/* | D-03/D-04/D-05/D-37/D-39 (locked); Code Examples: root Dockerfile + `-healthcheck` subcommand pattern, distroless gotcha |
| PLAT-02 | Shared `pkg/{events,kafka,outbox,httpx,auth,pgx}` are the only way services touch Kafka/outbox/auth/Postgres | D-02/D-08/D-13/D-30/D-42..D-45 (locked); Code Examples: kotel-based `pkg/kafka`, idempotent-consumer wrapper (verified franz-go API) |
| PLAT-03 | `make up` starts Redpanda+Postgres 17+Valkey+Kong 3.9.1 DB-less+Grafana Tempo/Loki/Prometheus+all services | D-18/D-19/D-24/D-25/D-26/D-41 (locked); Environment Availability below; Pitfall: Redpanda dev-vs-prod flags (existing PITFALLS.md #10) |
| PLAT-04 | `make proto-gen` (buf) generates committed Go+TS from proto/events + proto/services | D-07/D-09 (locked); CLAUDE.md pins buf v1.73.x, connect-es v1.7.x — verified current on npm this session |
| PLAT-05 | State change via outbox in one service applied exactly once in another via `processed_events`, proven end-to-end (`catalog.BoatUpserted`) | D-01/D-10/D-11/D-13 (locked); Architecture Patterns 1–2 below (existing ARCHITECTURE.md, still valid) |
| PLAT-06 | `trace_id` propagates HTTP→outbox row→Kafka headers→consumer as one Tempo trace; slog JSON logs carry `trace_id` | D-06/D-52/D-53 (locked); **kotel finding above** supersedes the manual-carrier approach in ARCHITECTURE.md Pattern 8; Code Examples: kotel wiring, otelslog dual-handler |
| PLAT-07 | Failed processing retries 3× with backoff → `<topic>.dlq` with error metadata; offsets commit only after success | D-12/D-13 (locked); verified franz-go `DisableAutoCommit`/`CommitRecords` API this session |
| PLAT-08 | Jenkins CI builds every service image, runs unit+integration (testcontainers) on every push | D-21/D-22/D-23 (locked); verified testcontainers-go v0.44.0 lockstep (core+postgres+redpanda modules) this session |
| PLAT-09 | Next.js skeleton (i18n TH/EN, mobile-first, Thai font) calling backend only through Kong | D-32..D-36 (locked); Code Examples: next-intl App Router shape, IBM Plex Sans Thai subset gotcha |
| PLAT-10 | Kong 3.9.1 DB-less+JWT round-trip verified; Traefik fallback decided if it fails | D-27/D-28 (locked); **Kong RS256 declarative-config gotcha** below (`secret` field still required even for RS256) |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Service scaffolding (`make new-service`) | Build tooling (Makefile/shell, repo root) | — | Not a runtime tier; a copy+sed script per D-03, deliberately not a Go generator |
| HTTP + sync RPC (chi + connect-go) | API / Backend (per service `cmd/main.go`) | — | One binary per service serves both plain HTTP and connect-go handlers on the same `http.Handler` |
| JWT verification | API Gateway (Kong) | Frontend Server (BFF) | Kong verifies signature/expiry and rejects bad tokens at the edge; BFF re-parses the same cookie JWT to do claim→header remapping (Pattern 4) — Kong's stock plugin doesn't remap custom claims |
| Claim→header trust boundary | API / Backend (`pkg/httpx` middleware, every service) | — | Enforced twice: network isolation (no public ports on services) + shared-secret header check (D-30) — this is the real security boundary, not the header format |
| Event publish (outbox write) | API / Backend (same tx as state change) | — | Never a direct Kafka call from a handler (Anti-Pattern 1, existing research) |
| Event relay (outbox → Kafka) | API / Backend (`pkg/outbox` background goroutine, same binary) | — | Not a separate service/tier — `errgroup` goroutine inside each service's one binary per D-04 |
| Event consume + idempotent apply | API / Backend (`pkg/kafka` consumer, same binary) | Database / Storage (`processed_events` table) | Idempotency check + state write + offset commit ordering all live in one Postgres tx per D-13 |
| DLQ | Database / Storage (Kafka topic `<svc>.events.dlq`) | API / Backend (producer side of the retry-then-DLQ handler) | Not a service — a topic + `make dlq-list` CLI wrapper (D-12) |
| Distributed tracing | Cross-cutting (`pkg/httpx`, `pkg/kafka`, OTel Collector) | Database / Storage (Tempo) | Instrumented once in shared packages so no service opts out; Tempo is the storage tier for traces |
| Structured logging | Cross-cutting (`pkg/httpx.NewLogger`) | Database / Storage (Loki via OTel Collector) | slog JSON to stdout always; OTel logs bridge ships the same records to Loki — two sinks, one log call |
| PWA skeleton / i18n | Browser / Client (Next.js Client Components) | Frontend Server (SSR shell only) | D-35: all data fetching is client-only in Phase 1; Server Components render layout/shell only — deliberately thin SSR tier for now |
| Static assets / fonts | CDN / Static (`next/font/google` self-hosted at build) | — | `next/font` downloads and self-hosts the Google Font at build time — no runtime CDN dependency on fonts.google.com |

## Standard Stack

Do not re-verify core versions already pinned in `.claude/CLAUDE.md` (Go 1.25, chi v5.3.2, connect-go v1.20.0, sqlc v1.31.x, pgx v5.x, goose v3.x, franz-go v1.22.x, go-redis v9.19.x, OTel Go v1.47.0, PostgreSQL 17, Redpanda v25.x/26.x, Kong 3.9.1, Next.js 16.x, buf v1.73.x) — this section covers only the **supporting** packages Phase 1 needs that CLAUDE.md doesn't already pin.

### Core (Phase-1-specific additions to CLAUDE.md's list)

| Library | Version (verified this session) | Purpose | Why Standard |
|---------|------|---------|--------------|
| `github.com/twmb/franz-go/plugin/kotel` | v1.7.1 | OTel trace/metric hooks for franz-go — the Kafka side of PLAT-06 | Same-org plugin, version-locked with franz-go (both resolve together via `go get`); implements `kgo.Hook` for produce/fetch spans and exposes `kotel.NewRecordCarrier` for header inject/extract. `[VERIFIED: proxy.golang.org module list + read github.com/twmb/franz-go/plugin/kotel@v1.7.1/{kotel,tracer,carrier}.go this session]` |
| `connectrpc.com/otelconnect` | v0.10.0 | OTel interceptor for connect-go (HTTP/sync side of PLAT-06) | Official connectrpc.com-namespaced package (same publisher as connect-go itself); `NewInterceptor(...)` returns a `connect.Interceptor` usable via `connect.WithInterceptors(...)` on both client and server. `[VERIFIED: proxy.golang.org module list + read connectrpc.com/otelconnect@v0.10.0/interceptor.go this session — confirms NewInterceptor/WrapUnary/WrapStreamingClient/WrapStreamingHandler exist]` |
| `go.opentelemetry.io/contrib/bridges/otelslog` | v0.20.1 | Bridges Go `slog` records into the OTel Logs SDK → Collector → Loki (D-53) | Official `open-telemetry/opentelemetry-go-contrib` package; `NewHandler(name, options...)` returns an `slog.Handler`. Logs SDK went stable in OTel Go v1.47.0 per CLAUDE.md's own research — this bridge is the intended consumer of that stability. `[VERIFIED: proxy.golang.org module list + read .../otelslog@v0.20.1/handler.go this session — confirms NewLogger/NewHandler exist]` |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.71.0 | HTTP server/client span instrumentation for chi's `net/http` handlers | Official contrib package; wraps any `http.Handler`, so it composes with chi's router directly (`otelhttp.NewHandler(chiRouter, "service-name")`). `[VERIFIED: proxy.golang.org module list this session]` |
| `golang.org/x/sync/errgroup` | v0.23.0 (x/sync) | Runs HTTP server + outbox relay + Kafka consumer as goroutines under one supervised group with graceful shutdown (D-04) | Official Go org module (`golang.org/x/...`), stdlib-adjacent — exactly the "run N goroutines, propagate first error, cancel the rest" primitive D-04 needs; no reason to hand-roll or add a third-party alternative. `[VERIFIED: proxy.golang.org module list this session]` |
| `github.com/google/uuid` | v1.6.0 | `event_id`/entity IDs as UUID v7 (D-44) | `NewV7()` and `NewV7FromReader()` exist in this version. `[VERIFIED: github.com/google/uuid@v1.6.0 version7.go — read this session, confirms func NewV7() (UUID, error) exists]` |

### Supporting (dev tooling / frontend)

| Library | Version (verified this session) | Purpose | When to Use |
|---------|------|---------|-------------|
| `next-intl` | v4.14.7 (npm) | i18n routing with always-present locale prefix (D-33) | `[ASSUMED — see Package Legitimacy Audit]` package identity; version number `[VERIFIED: npm registry `npm view next-intl version` this session]` |
| `@tanstack/react-query` | v5.103.2 (npm) | Client-side data fetching/caching wrapped around `apiFetch()` (D-35) | `[ASSUMED — see Package Legitimacy Audit]` package identity; version `[VERIFIED: npm registry this session]` |
| `lefthook` (npm wrapper around the Go binary) | v2.1.14 (npm) | Pre-commit hook runner (D-48) | `[ASSUMED — see Package Legitimacy Audit]` package identity; version `[VERIFIED: npm registry this session]` |
| `tailwindcss` | v4.3.3 (npm) | Utility CSS (D-32) | v4 is current major and Next.js 16's default scaffold target — confirms CLAUDE.md's Next.js 16 pairing needs no separate Tailwind pin decision. `[VERIFIED: npm registry this session]` |
| `shadcn` (CLI, package renamed from `shadcn-ui`) | v4.21.0 (npm) | Component scaffolding (D-32) | Use `shadcn` not the old `shadcn-ui` package name — the CLI copies component source into the repo rather than installing a runtime dependency, so this is a devDependency/one-shot tool, not a shipped package. `[VERIFIED: npm registry this session]` |
| `golangci-lint` (v2 line) | v2.14.0 | Go lint gate (D-46) | **v2 is current — its config schema is a breaking change from v1** (see State of the Art below); CLAUDE.md doesn't pin this so treat v2 as the target. `[VERIFIED: proxy.golang.org module list this session for github.com/golangci/golangci-lint/v2]` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| kotel hooks for franz-go tracing | Hand-rolled `propagation.MapCarrier` loop (as sketched in `ARCHITECTURE.md` Pattern 8, written before this session verified kotel exists) | Manual carrier code is more lines, no span attributes (topic/partition/offset semconv), and is exactly the kind of "don't hand-roll" case — use kotel |
| otelconnect for connect-go tracing | Hand-rolled connect.Interceptor that manually starts spans | otelconnect already handles unary + streaming, client + server, and sets messaging/RPC semantic conventions correctly — no reason to reinvent |
| otelslog bridge for logs→Loki | Ship logs to Loki via a separate promtail/Docker-logging-driver sidecar, decoupled from OTel | D-53 already chose the OTel bridge path (simpler Compose topology, one exporter pipeline for traces+metrics+logs) — this research confirms the bridge exists and is stable, no alternative needed |

**Installation (Go, appended to service/pkg go.mod files as needed):**
```bash
go get github.com/twmb/franz-go/plugin/kotel@v1.7.1
go get connectrpc.com/otelconnect@v0.10.0
go get go.opentelemetry.io/contrib/bridges/otelslog@v0.20.1
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.71.0
go get golang.org/x/sync@v0.23.0
go get github.com/google/uuid@v1.6.0
```

**Installation (Next.js, `apps/web`):**
```bash
npm install next-intl@4.14.7 @tanstack/react-query@5.103.2
npm install -D lefthook@2.1.14
npx shadcn@4.21.0 init
```

**Version verification performed this session:** every Go module above was resolved against the live module proxy via `go get`/`go list -m -versions` in a scratch module (not training-data recall); every npm package above was resolved via `npm view <pkg> version` against the live npm registry. `google/uuid`'s `NewV7()` and `franz-go/plugin/kotel`'s hook/carrier API were confirmed by reading the actual downloaded source files, not just the module listing.

## Package Legitimacy Audit

Ran `gsd-tools package-legitimacy check --ecosystem npm` against every new npm package this phase introduces beyond CLAUDE.md's already-audited list.

| Package | Registry | Age signal | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `next-intl` | npm | flagged "too-new" (latest version published 2026-09-24 — this is a *release-cadence* signal, not package age) | 4,162,034/wk | github.com/amannn/next-intl | SUS | Flagged — planner adds `checkpoint:human-verify` before install, per protocol. Context for the human check: 4.1M weekly downloads and an active, non-deprecated repo strongly indicate a legitimate, actively-maintained package — the SUS verdict here is a false-positive class (frequent releases trigger "too-new", not actual newness). |
| `@tanstack/react-query` | npm | flagged "too-new" (latest published 2026-09-21) | 49,316,307/wk | github.com/TanStack/query | SUS | Flagged — same false-positive class; 49M/wk is one of the most-downloaded npm packages in the ecosystem. Planner still adds the checkpoint per protocol. |
| `lefthook` | npm | flagged "too-new" (latest published 2026-09-14) | 3,255,283/wk | github.com/evilmartians/lefthook | SUS | Flagged — has a `postinstall: node postinstall.js` script (downloads the Go binary for the host platform, standard behavior for CLI-wrapper npm packages like esbuild/lefthook). Planner adds the checkpoint; verify the postinstall script's behavior (binary download, no other network calls) before first `npm install` in CI. |
| `tailwindcss`, `shadcn`, `next` | npm | not re-checked (already covered by CLAUDE.md's own research pass) | — | — | — | No new audit needed — CLAUDE.md's Technology Stack section already treats these as researched/pinned |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** `next-intl`, `@tanstack/react-query`, `lefthook` — all three are SUS purely on the tool's "too-new" heuristic (recent latest-version publish date), not on downloads/repo/deprecation signals, which are all strongly positive. The planner must still add a `checkpoint:human-verify` task before each `npm install` per the Package Legitimacy Gate protocol — this note exists so the human check is a 10-second confirmation, not a blind trust call.

*Go packages are not covered by `gsd-tools package-legitimacy check` (npm/pypi/crates only). Every Go package recommended above was instead verified this session by resolving it against the live Go module proxy (`go list -m -versions`, `go get` in a scratch module) — see the Standard Stack table's `[VERIFIED: proxy.golang.org module list]` tags. This satisfies the ecosystem-appropriate registry-verification step for Go even though the automated SLOP/SUS/OK classifier doesn't run for this ecosystem.*

## Architecture Patterns

`.planning/research/ARCHITECTURE.md` already documents Patterns 1–9 for this project (outbox relay SQL, idempotent-consumer table, choreography-not-orchestration for sagas, Kong+BFF claim remapping, Redis-trigger+cron-sweep hold expiry, connect-go 2-hop rule, OTel-over-Kafka, go.work+buf layout) and remains valid — **do not re-derive these**, the planner should read that file directly for Patterns 1, 2, 4, 5, 7, 9. This section only adds what's **new or corrected** by this session's verification pass.

### System Architecture Diagram (Phase 1 slice only)

```
┌─────────────────────────────────────────────────────────────────┐
│ Next.js 16 (apps/web) — Client Components fetch via TanStack     │
│ Query → apiFetch() → Kong (never direct to gateway/services)     │
└──────────────────────────────┬────────────────────────────────────┘
                                │ HTTPS, JWT httpOnly cookie
┌───────────────────────────────▼─────────────────────────────────┐
│ Kong 3.9.1 DB-less — TLS/routing, jwt plugin verifies RS256      │
│ signature+expiry only (no claim remapping — see gotcha below)   │
└───────────────────────────────┬─────────────────────────────────┘
                                │ internal Compose network only
┌───────────────────────────────▼─────────────────────────────────┐
│ services/gateway (BFF) — parses JWT from cookie, sets            │
│ X-User-Id/X-Operator-Id/X-Role/X-Internal-Token, calls catalog   │
│ via connect-go (1 hop)                                            │
└───────────┬───────────────────────────────────┬───────────────────┘
            │ connect-go (otelconnect)          │ same pattern
┌───────────▼─────────┐                ┌────────▼─────────┐
│ services/catalog     │──HTTP POST────▶│ services/schedule │
│ writes outbox row     │  (proof event  │ consumes via      │
│ in same tx as         │   is Kafka,    │ pkg/kafka +        │
│ BoatUpserted write     │   not HTTP —   │ processed_events   │
│                        │   arrow shows  │ (idempotent apply)  │
└───────────┬────────────┘  causality)    └────────▲────────────┘
            │ pkg/outbox relay (poll+publish, kotel span)         │
            ▼                                                     │
┌───────────────────────────────────────────────────────────────┐│
│ Redpanda — topic catalog.events (key=aggregate_id)             │┘
│ kotel injects traceparent header on produce, extracts on fetch │
└───────────────────────────────┬─────────────────────────────────┘
                                │ on 3 failed retries
                                ▼
                    catalog.events.dlq (error metadata headers)

Every span above (HTTP + outbox write + Kafka publish + Kafka
consume + processed_events apply) reports to one OTel Collector →
Tempo trace; slog JSON (stdout + otelslog bridge) → Loki, correlated
by trace_id.
```

### Pattern 8-revised: OTel-over-Kafka using `kotel`, not a hand-rolled carrier loop

**What:** `.planning/research/ARCHITECTURE.md` Pattern 8 (written before this session) sketches a manual `propagation.MapCarrier` loop for injecting/extracting `traceparent` into `kgo.Record.Headers`, stating "franz-go has no first-party OTel plugin as of this research." **That statement is superseded** — `github.com/twmb/franz-go/plugin/kotel` is a same-org plugin, version-locked with franz-go on the module proxy (v1.7.1 pairs with franz-go v1.22.0), and does exactly this job plus adds span attributes.

**When to use:** Everywhere `pkg/kafka` and `pkg/outbox` touch a `kgo.Record` — this is the shared-package code every service inherits, so get it right once here.

**Verified shape** (all APIs below confirmed by reading the downloaded source this session):

```go
// pkg/kafka — client construction, once per service
tracer := kotel.NewTracer(kotel.TracerProvider(otel.GetTracerProvider()))
kt := kotel.NewKotel(kotel.WithTracer(tracer))
cl, err := kgo.NewClient(
    kgo.SeedBrokers(brokers...),
    kgo.ConsumerGroup(serviceName),
    kgo.ConsumeTopics(topics...),
    kgo.DisableAutoCommit(),                 // verified: kgo.DisableAutoCommit() exists — config.go:2342
    kgo.WithHooks(kt.Hooks()...),            // registers produce+fetch span hooks
)

// pkg/outbox relay — producer side, per outbox row
// The outbox row must store the *originating* traceparent (Pitfall 11's
// fix) because the relay runs in a separate goroutine/poll cycle from the
// HTTP request that wrote the row — trace context does not survive a
// process boundary via ctx alone.
carrier := propagation.MapCarrier{"traceparent": row.TraceParent}
ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
record := &kgo.Record{Topic: row.Topic, Key: []byte(row.AggregateID), Value: row.Payload}
record.Context = ctx   // kotel's OnProduceRecordBuffered hook starts the
                        // "publish" span from record.Context and injects
                        // traceparent into record.Headers automatically —
                        // do NOT also manually set the header, kotel owns it
cl.Produce(ctx, record, func(r *kgo.Record, err error) { /* mark published_at on success */ })

// pkg/kafka consumer — dispatch loop, per fetched record
// kotel's OnFetchRecordBuffered hook already extracted traceparent into
// record.Context by the time your poll loop sees the record.
ctx, span := tracer.WithProcessSpan(record)  // verified: tracer.WithProcessSpan(r *kgo.Record) (context.Context, trace.Span)
defer span.End()
err := handle(ctx, tx, envelope)             // idempotent-consumer wrapper, Pattern 2 (existing research, unchanged)
```

- `record.Context` is the field kotel reads/writes on both produce and fetch — set it before `Produce`, read the resulting `Context` field after `Fetch` (via `tracer.WithProcessSpan`, which handles the "if nil, use Background" case for you).
- This satisfies Pitfall 11's exact prescription (store `traceparent` on the outbox row, re-inject at publish time) without hand-rolling the header-writing loop.

### Pattern: connect-go tracing via `otelconnect` (HTTP/sync side of PLAT-06)

```go
// once per service, wherever connect handlers/clients are constructed
interceptor, err := otelconnect.NewInterceptor() // verified: NewInterceptor(options ...Option) (*Interceptor, error)
// server side
path, handler := catalogv1connect.NewCatalogServiceHandler(svc, connect.WithInterceptors(interceptor))
mux.Mount(path, handler)
// client side (e.g. gateway → catalog)
client := catalogv1connect.NewCatalogServiceClient(httpClient, baseURL, connect.WithInterceptors(interceptor))
```

Combine with `otelhttp.NewHandler(chiRouter, serviceName)` at the top of the HTTP server so plain `/healthz`/`/readyz` requests and connect-go RPCs share one trace-propagating entry point.

### Pattern: dual-sink slog (stdout JSON + OTel Logs bridge) without a new dependency

D-52/D-53 need every log line to (a) print as JSON to stdout for `docker logs`/`go run` and (b) ship to the OTel Collector → Loki. `otelslog.NewHandler(name)` gives you an `slog.Handler` for (b); stdlib's `slog.NewJSONHandler(os.Stdout, opts)` gives you (a). There is no stdlib multi-handler — the correct-size solution is a ~15-line custom `slog.Handler` that fans out to both (Don't Hand-Roll table below explains why this one *should* be hand-rolled, not imported):

```go
// pkg/httpx — the entire "multi-handler" needed, no new dependency
type fanoutHandler struct{ handlers []slog.Handler }

func (f *fanoutHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
    for _, h := range f.handlers { if h.Enabled(ctx, lvl) { return true } }
    return false
}
func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
    for _, h := range f.handlers {
        if h.Enabled(ctx, r.Level) {
            if err := h.Handle(ctx, r.Clone()); err != nil { return err }
        }
    }
    return nil
}
func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    next := make([]slog.Handler, len(f.handlers))
    for i, h := range f.handlers { next[i] = h.WithAttrs(attrs) }
    return &fanoutHandler{handlers: next}
}
func (f *fanoutHandler) WithGroup(name string) slog.Handler {
    next := make([]slog.Handler, len(f.handlers))
    for i, h := range f.handlers { next[i] = h.WithGroup(name) }
    return &fanoutHandler{handlers: next}
}

logger := slog.New(&fanoutHandler{handlers: []slog.Handler{
    slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
    otelslog.NewHandler(serviceName).Handler(), // wrap if API differs; see otelslog docs — confirm at implementation time
}})
```

### Gotcha: Kong DB-less `jwt` plugin still requires a `secret` field even for RS256

**What goes wrong:** D-27 says "Kong and BFF hold only the public key." Naively, a DB-less `jwt_secrets` credential entry for an RS256 consumer might omit the `secret` field entirely (it's meaningless for RS256 — the actual verification uses `rsa_public_key`). **Kong's declarative-config validation rejects this** — per Kong's own JWT plugin docs, "the declarative configuration used in decK and the Kong Ingress Controller imposes additional validation requirements... all fields other than `rsa_public_key` fields are required," meaning `key`, `algorithm`, and a (dummy) `secret` value must all be present in `kong.yml` even though `secret` is never actually used to verify an RS256 token. `[CITED: developer.konghq.com/plugins/jwt/]`

**How to avoid:** When writing the Kong spike's `kong.yml` (D-28), the dev consumer's `jwt_secrets` entry needs all four fields:
```yaml
consumers:
  - username: dev-consumer
    jwt_secrets:
      - key: dev-issuer          # must match the JWT's `iss` claim
        algorithm: RS256
        secret: unused-for-rs256  # required by DB-less validation, never used to verify
        rsa_public_key: |
          -----BEGIN PUBLIC KEY-----
          ...
          -----END PUBLIC KEY-----
```
Missing `secret` in DB-less mode is a plausible reason the Kong spike (time-boxed 1 day per D-28) could fail for a reason unrelated to Kong's actual capability — know this before spending spike time debugging a validation error that looks like "Kong can't do RS256," when it actually can.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Kafka trace-context inject/extract + span attributes | A `propagation.MapCarrier` loop over `kgo.Record.Headers` | `github.com/twmb/franz-go/plugin/kotel` (`kotel.NewTracer`, `kotel.Hooks()`, `kotel.NewRecordCarrier`) | Same-org, version-locked plugin already does this correctly with proper messaging semconv attributes — verified to exist and match franz-go's version this session |
| connect-go request tracing | A custom `connect.Interceptor` that starts/ends spans manually | `connectrpc.com/otelconnect.NewInterceptor()` | Handles unary + streaming, client + server, sets RPC semantic conventions — no reason to reinvent |
| Outbox relay loop, DLQ retry/backoff, idempotency-check-then-apply | Reimplementing any of these per-service | `pkg/outbox` (relay), `pkg/kafka` (DLQ + `Consumer.Handle` wrapper) — already the locked D-10..D-13 design | This IS the "don't hand-roll 7 times" lesson from `.planning/research/PITFALLS.md` Pitfall 1 — the whole point of Phase 1 is writing this once |
| Golang UUID v7 generation | A manual timestamp+random UUID builder | `google/uuid.NewV7()` | Verified to exist in the pinned version; RFC 9562 compliance is exactly the kind of "looks simple, has edge cases" problem a maintained library already solved |
| Multi-target slog output (stdout JSON + OTel bridge) | Importing a third-party slog-multi/fanout library | A ~15-line custom `slog.Handler` implementing the 4-method interface (shown above) | This is the flip case of "don't hand-roll" — the problem is small enough (4 trivial methods, no edge cases) that adding a dependency for it would violate ponytail's YAGNI/stdlib-first rule; only 2 sinks are needed, not N |
| Postgres connection pooling, health check, tx helper | A custom pool wrapper | `pkg/pgx` around `pgxpool` (already the locked design, D-02) | pgx/v5's pool already handles this; `pkg/pgx` is a thin `WithTx` helper, not a reimplementation |

**Key insight:** Every "don't hand-roll" item above except the slog fanout has a maintained library that's *already version-compatible with the rest of the pinned stack* (confirmed via the module proxy this session) — the temptation to hand-roll usually comes from not knowing the library exists (kotel, otelconnect) rather than the library being genuinely inadequate. The slog fanout is the one deliberate exception: 2 known sinks, ~15 lines, zero real edge cases — exactly ponytail's "can it be one line (or close to it)?" rung, not a "don't hand-roll" case.

## Common Pitfalls

`.planning/research/PITFALLS.md` already covers the pitfalls most load-bearing for Phase 1 in depth (Pitfall 1: solo-dev microservices overhead; Pitfall 3: idempotency "checked the box" vs. actually tested; Pitfall 9: PII leaking into events/logs/traces; Pitfall 10: Redpanda single-EC2 fragility; Pitfall 11: OTel-over-Kafka silently breaking; Pitfall 12: testcontainers CI flakiness; Pitfall 13: Next.js App Router+i18n+PWA gotchas) — **read that file**, do not re-derive. This section adds only what's new from this session's verification.

### Pitfall: golangci-lint v2's config schema is a breaking change from v1

**What goes wrong:** D-46 specifies a root `.golangci.yml` with `forbidigo` banning `time.Now`/`fmt.Print*`. Copying a v1-style config (top-level `linters-settings:`, `forbid: [{p: '...'}]`, `exclude-generated: lax`) into a `golangci-lint` v2 install produces either silent misconfiguration or an outright parse error.

**Why it happens:** golangci-lint's current major (v2.14.0, verified this session — not pinned in CLAUDE.md, which predates or doesn't cover this tool) restructured the config format: settings moved under `linters.settings.*`, the `forbidigo` `forbid` entries now require an explicit `pattern:` key (the old shorthand `p:` field or bare string no longer works), and there's a mandatory top-level `version: "2"` field.

**How to avoid:** Write `.golangci.yml` in v2 shape from the start:
```yaml
version: "2"
linters:
  enable: [errcheck, govet, staticcheck, gosec, sqlclosecheck, forbidigo]
  settings:
    forbidigo:
      forbid:
        - pattern: '^time\.Now$'
          msg: "use pkg/clock.Now() instead — see D-42"
        - pattern: '^fmt\.Print.*$'
          msg: "use pkg/httpx logger instead"
  exclusions:
    paths:
      - gen/
```
`[CITED: golangci-lint.run/docs/product/migration-guide/]`

**Warning signs:** `golangci-lint run` exits with a config-parse error, or (worse) silently ignores the `forbidigo` rules because the v1-shaped `forbid: [{p: ...}]` entries don't match v2's expected `pattern:` key and are dropped rather than erroring.

**Phase to address:** Phase 1, when `.golangci.yml` is first written (D-46) — verify with a deliberate failing test (`time.Now()` call in a throwaway file) that the lint actually fires before relying on it as a merge gate.

### Pitfall: IBM Plex Sans Thai needs an explicit `thai` subset or Thai text silently falls back to a system font

**What goes wrong:** D-34 specifies IBM Plex Sans Thai via `next/font/google`. `next/font/google`'s generated font-loader functions require an explicit `subsets` array; if a copy-pasted example from a Latin-only font tutorial is used (`subsets: ['latin']`), the downloaded/self-hosted font file **contains no Thai glyphs**, and the browser silently substitutes a system font for all Thai text — with no error, no warning, and no visible difference in the English UI (making it easy to miss in casual QA if the reviewer is testing in English).

**How to avoid:** Explicitly pass `subsets: ['thai', 'latin']` (Thai first, since Thai-language readability is the product's stated priority):
```ts
import { IBM_Plex_Sans_Thai } from 'next/font/google';
const plexSansThai = IBM_Plex_Sans_Thai({ subsets: ['thai', 'latin'], weight: ['400', '500', '700'] });
```
`[CITED: fonts.google.com/specimen/IBM+Plex+Sans+Thai — confirms a `thai` subset exists for this family]`

**Warning signs:** Thai text on the page renders in a visibly different typeface than English text on the same page (most obvious at large sizes, e.g. departure times/prices).

**Phase to address:** Phase 1 (Next.js skeleton, D-34) — cheap to catch now with one visual check of Thai copy on the skeleton page, expensive to notice later if every page is built assuming the font "just works."

### Pitfall: kotel's `record.Context` must be set before `Produce`, not derived from the relay's poll-loop context

**What goes wrong:** If `pkg/outbox`'s relay calls `cl.Produce(ctx, record, cb)` where `record.Context` is left `nil` or set to the relay's own poll-loop background context (rather than the context reconstructed from the outbox row's stored `traceparent`), kotel's `OnProduceRecordBuffered` hook starts the "publish" span from the *wrong* parent — either a fresh root span (broken trace continuity) or, if `record.Context` is `nil`, kotel handles it gracefully (`context.Background()`) but the resulting span has no parent, silently defeating PLAT-06's "one trace spanning HTTP + Kafka" requirement without throwing any error.

**How to avoid:** The relay's per-row publish code must explicitly do the `Extract`-then-set-`record.Context` step shown in the Pattern above, for every row, sourced from that row's stored `traceparent` column — not the relay goroutine's own ambient context.

**Warning signs:** Tempo shows two disconnected traces (one ending at the outbox-write span, one starting fresh at the Kafka-publish span) instead of one continuous trace — this is exactly the failure mode Pitfall 11 (existing research) warns about, and this is the specific line of code where it would actually happen with kotel in the mix.

**Phase to address:** Phase 1 — this is the literal mechanism behind PLAT-06's acceptance check; verify by visually inspecting one real trace in Tempo's UI, not just by confirming the code compiles.

## Code Examples

### go.work (repo root, dev convenience only — not a build input)

```
go 1.25

use (
    ./pkg
    ./gen/go
    ./services/gateway
    ./services/catalog
    ./services/schedule
    ./services/_template
)
```
Per D-02, `./pkg` is ONE module (not one per subpackage) and `./gen/go` is a separate module — matches the locked decision, differs slightly from `ARCHITECTURE.md`'s earlier sketch of one `go.work use` per `pkg/*` subpackage (that sketch predates D-02 and should not be followed).

### Root Dockerfile with `-healthcheck` subcommand (D-37, distroless has no shell)

```dockerfile
# syntax=docker/dockerfile:1
ARG SERVICE
FROM golang:1.25 AS build
ARG SERVICE
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg ./pkg
COPY gen/go ./gen/go
COPY services/${SERVICE} ./services/${SERVICE}
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o /out/service ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
HEALTHCHECK CMD ["/service", "-healthcheck"]
```
```go
// cmd/main.go — first thing, before any other startup work
if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
    resp, err := http.Get("http://localhost:" + port + "/readyz")
    if err != nil || resp.StatusCode != 200 { os.Exit(1) }
    os.Exit(0)
}
```
This is the whole mechanism — no library needed; distroless has no `curl`/`wget`, so the binary checks itself.

### Kong DB-less `kong.yml` — JWT RS256 spike shape (D-27/D-28)

```yaml
_format_version: "3.0"
consumers:
  - username: dev-consumer
    jwt_secrets:
      - key: dev-issuer
        algorithm: RS256
        secret: unused-for-rs256   # required by DB-less validation even though unused — see gotcha above
        rsa_public_key: |
          -----BEGIN PUBLIC KEY-----
          <dev keypair public key from .env>
          -----END PUBLIC KEY-----
services:
  - name: gateway
    url: http://gateway:8080
    routes:
      - name: gateway-route
        paths: ["/api"]
    plugins:
      - name: jwt
        config:
          claims_to_verify: [exp]
      - name: cors
      - name: rate-limiting
        config: { minute: 60, policy: local }
```

### testcontainers-go — verified lockstep versions (PLAT-08)

```go
// go.mod requires, all v0.44.0 — verified via `go list -m -versions` this session
// github.com/testcontainers/testcontainers-go v0.44.0
// github.com/testcontainers/testcontainers-go/modules/postgres v0.44.0
// github.com/testcontainers/testcontainers-go/modules/redpanda v0.44.0
```
Pin all three to the same `v0.44.0` — CLAUDE.md's "keep them on the same version" guidance is confirmed correct: as of this session the three modules are in fact all at the same latest tag on the proxy.

## State of the Art

| Old Approach / Old Research Claim | Current Finding | When Changed | Impact |
|--------------------------|------------------|---------------|--------|
| `.planning/research/ARCHITECTURE.md` Pattern 8: "franz-go has no first-party OTel plugin as of this research — wire it manually" | `github.com/twmb/franz-go/plugin/kotel` exists, is maintained in the same repo/org as franz-go, and is version-locked with it | Confirmed this session (2026-09-26) by resolving both modules against the live proxy and reading kotel's source | `pkg/kafka`/`pkg/outbox` should use `kotel.Hooks()` + `kotel.NewRecordCarrier` instead of the hand-rolled carrier loop — less code, gets span attributes for free |
| golangci-lint v1 config shape (`linters-settings:`, `forbid: [{p: ...}]`) | v2 is current (v2.14.0), with a breaking config schema (`version: "2"`, `linters.settings.*`, `pattern:` required key) | golangci-lint v2 GA, ongoing 2026 release cadence | D-46's `.golangci.yml` must be written in v2 shape from day one — CLAUDE.md doesn't pin this tool, so there's no existing pinned decision to update, just a gotcha to avoid on first write |
| Tailwind v3-era class-based config assumptions | Tailwind v4.3.3 is current (CSS-first config, no `tailwind.config.js` required by default) | Tailwind v4 GA (2025), still current major in 2026 | Matches Next.js 16's default scaffold — no separate version decision needed, but planner/executor should expect CSS-based `@theme` config, not a JS config file, if following current `create-next-app` defaults |

**Deprecated/outdated:** `shadcn-ui` npm package name — renamed to `shadcn`; use `npx shadcn@4.21.0 init`, not `npx shadcn-ui@latest init`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `next-intl`, `@tanstack/react-query`, `lefthook` are the correct, non-hallucinated package names for their stated purposes (not just registry-existent) | Standard Stack, Package Legitimacy Audit | Low — all three are extremely well-known, high-download packages the researcher recognized independently of the registry lookup; still formally `[ASSUMED]` under the package-name-provenance rule since identity was recalled from training knowledge before registry confirmation, not sourced from Context7/official docs |
| A2 | `otelslog.NewHandler(name).Handler()` / `.Handler()` accessor shape is exactly right — the source was read for `NewHandler`/`NewLogger` signatures but the fanout code example's exact call chain wasn't compiled/run | Code Examples (dual-sink slog) | Low-medium — if the exact accessor differs, the executor will hit a compile error immediately (not a silent bug) and can adjust from the package's own doc comments, which were confirmed to exist |
| A3 | next-intl's exact `defineRouting`/`middleware.ts` API shape for "locale always prefixed, default `th` on detection failure" (D-33) matches the general pattern described, but the full middleware code wasn't fetched/verified this session — only the layout/request-config shape was confirmed via WebFetch | Architecture Patterns (implied), Standard Stack | Medium — next-intl's API has changed across major versions before; if v4's `defineRouting` signature differs from what the executor assumes, this is a Phase 1 implementation-time doc-check, not a design risk (the locale-prefix requirement itself is simple and library-agnostic) |

**If this table is empty:** N/A — see above; all three items are low/medium risk and self-correcting at implementation time (compile errors or a five-minute doc check), not silent correctness risks.

## Open Questions

1. **Exact `otelslog` handler composition API**
   - What we know: `otelslog.NewHandler(name, ...Option) *Handler` and `otelslog.NewLogger(name, ...Option) *slog.Logger` both exist (confirmed via source read).
   - What's unclear: whether `*Handler` needs an explicit method call to satisfy `slog.Handler` for use inside the custom fanout handler, or whether `*Handler` already *is* an `slog.Handler` (likely, given the package's stated purpose) — the source's `handler.go` type embedding wasn't fully traced.
   - Recommendation: at implementation time, `go doc go.opentelemetry.io/contrib/bridges/otelslog.Handler` for five seconds before wiring the fanout — this is a two-line fix either way, not worth blocking planning on.

2. **Whether the Kong spike (D-28) needs the dummy `secret` field to also pass `buf`/decK linting if decK is used for kong.yml validation**
   - What we know: Kong's own DB-less validation requires the field per official docs.
   - What's unclear: whether the team plans to use `decK` for any local validation step (not mentioned in CONTEXT.md) — if so, the same requirement applies there too.
   - Recommendation: not a blocker — the `kong.yml` example above already includes the field; flag only if a future decK-based CI step is added.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | `make up`, testcontainers, root Dockerfile builds | ✓ | 28.1.1 | — |
| Docker Compose (v2 plugin) | `make up`, `deploy/docker-compose.yml` | ✓ | v2.39.2 | — |
| Go | all services, `pkg/*` | ✓ | 1.25.1 | — |
| Node.js | `apps/web`, npm tooling | ✓ | v24.10.0 | — |
| npm | frontend package install | ✓ | 11.6.0 | — |
| buf CLI | `make proto-gen` | ✗ | — | Install via `go install github.com/bufbuild/buf/cmd/buf@v1.73.x` or Homebrew as part of Phase 1 setup — not a blocker, just an install step the plan must include |
| goose CLI | `make migrate-<svc>` | ✗ | — | `go install github.com/pressly/goose/v3/cmd/goose@v3.28.0` |
| sqlc CLI | `make sqlc-gen` | ✗ | — | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.x` (or use the sqlc Docker image if a local Go-based install has issues) |
| golangci-lint CLI | `make lint`, lefthook pre-commit | ✗ | — | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0` — note v2 binary path differs from the old v1 module path |
| lefthook binary | pre-commit hooks (D-48) | ✗ | — | `npm install -D lefthook` (installs the platform binary via postinstall) or `go install github.com/evilmartians/lefthook@latest` |
| rpk (Redpanda CLI) | `make dlq-list` (D-12) | ✗ | — | Ships inside the Redpanda container image — `docker compose exec redpanda rpk topic consume ...` works without a host install; host `rpk` is a convenience, not a requirement |
| psql | manual DB inspection | ✗ | — | Use `docker compose exec postgres psql` instead, or install via package manager — not required for any Makefile target |

**Missing dependencies with no fallback:** none — every missing CLI tool above either has a documented install command (Go tools via `go install`, already using the same pinned versions as CLAUDE.md/this research) or a container-exec fallback that requires no host install.
**Missing dependencies with fallback:** buf, goose, sqlc, golangci-lint, lefthook (all installable via `go install`/`npm install` — the plan should include a one-time `make dev-tools` or equivalent setup target), rpk and psql (both available via `docker compose exec` without a host install).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib) for Go, no framework needed; `//go:build integration` tag convention for testcontainers tests (D-23) |
| Config file | none — `go test` needs no config; `testcontainers-go` config is in-code (`postgres.Run(...)`, `redpanda.Run(...)`) |
| Quick run command | `go test ./...` (no Docker, no `integration` tag — matches `make test` per D-23) |
| Full suite command | `go test -tags=integration ./...` (matches `make test-integration` per D-23) |

Frontend: no test framework is specified in D-32..D-36 for Phase 1 — the skeleton has no business logic to unit-test yet beyond `tsc --noEmit`/`eslint`/`next build` (D-47), which already function as the frontend's Phase 1 quality gate. **Wave 0 gap:** if the plan wants even one smoke test for the Next.js skeleton (e.g., Kong round-trip page renders), a minimal Playwright or Vitest setup is not yet present in the repo — flag as a gap only if the plan intends automated frontend verification beyond lint/build/typecheck.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-01 | `make new-service` produces a compiling, template-shaped service | unit/build | `go build ./services/_template/...` (template itself is CI-built per D-03) | ❌ Wave 0 — `services/_template` doesn't exist yet, this phase creates it |
| PLAT-02 | `pkg/kafka`/`pkg/outbox`/`pkg/httpx`/`pkg/auth`/`pkg/pgx` are the sole integration points | unit | `go test ./pkg/...` | ❌ Wave 0 — packages don't exist yet |
| PLAT-03 | `make up` brings up the full stack healthy | manual (+ CI smoke) | `docker compose up -d && docker compose ps` (all healthy); CI equivalent: a Jenkins stage that runs `make up` and polls `/readyz` on gateway | ❌ Wave 0 — compose file doesn't exist yet |
| PLAT-04 | `make proto-gen` produces committed, up-to-date generated code | integration (CI gate) | `buf generate && git diff --exit-code gen/` (matches D-09's "gen/ diff check") | ❌ Wave 0 — `proto/`, `buf.gen.yaml` don't exist yet |
| PLAT-05 | `BoatUpserted` outbox→Kafka→consumer→`processed_events` exactly-once | integration (testcontainers: real Postgres + Redpanda) | `go test -tags=integration ./services/schedule/... -run TestBoatUpsertedAppliedOnce` | ❌ Wave 0 — the proof-event integration test doesn't exist yet; this is the single highest-value test in this phase |
| PLAT-06 | One Tempo trace spans HTTP + outbox + Kafka + consumer; slog carries `trace_id` | manual (Tempo UI inspection) + integration (assert `traceparent` on outbox row and on the produced Kafka record) | `go test -tags=integration ./pkg/outbox/... -run TestTraceparentSurvivesRelay` (assert the header exists on the produced record, not full Tempo assertion — Tempo inspection stays manual) | ❌ Wave 0 |
| PLAT-07 | Retry 3× → DLQ with metadata; offset commits only after success | integration (testcontainers) | `go test -tags=integration ./pkg/kafka/... -run TestFailedHandlerLandsInDLQAfter3Retries` | ❌ Wave 0 |
| PLAT-08 | Jenkins builds every image + runs unit+integration on every push | manual (+ CI self-test) | Jenkinsfile changed-paths logic (D-22) verified by pushing a change under `pkg/` and confirming ALL services rebuild, then a change under one `services/<name>/` and confirming only that service rebuilds | ❌ Wave 0 — Jenkinsfile doesn't exist yet |
| PLAT-09 | Next.js skeleton: i18n TH/EN, mobile-first, Thai font, Kong-only calls | manual (visual) + build gate | `npm run lint && npm run typecheck && npm run build` (D-47); manual: switch `/th`↔`/en`, inspect Thai glyph rendering, confirm no direct fetch bypassing Kong via browser devtools network tab | ❌ Wave 0 — `apps/web` doesn't exist yet |
| PLAT-10 | Kong DB-less JWT round-trip (curl → Kong → BFF → stub service) | manual (spike) + integration (repeatable curl script) | A `make dev-token && curl -H "Cookie: ..." http://localhost:8000/api/v1/boats` script, checked into `deploy/kong/` or `Makefile`, so the round-trip is re-runnable, not just a one-time spike | ❌ Wave 0 — this IS the spike itself (D-28), time-boxed 1 day |

### Sampling Rate

- **Per task commit:** `go test ./...` (fast, no Docker) — matches D-23's `make test`.
- **Per wave merge:** `go test -tags=integration ./...` (testcontainers, matches D-23's `make test-integration`) + `npm run lint && npm run typecheck && npm run build` for the web skeleton.
- **Phase gate:** Full suite green (`make test-integration`, `make lint`, `next build`) + the Kong round-trip curl script + a manual Tempo screenshot showing one trace spanning the Kafka hop, before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `services/_template/**` — the template service itself; every later integration test target depends on this existing first
- [ ] `pkg/kafka`, `pkg/outbox`, `pkg/httpx`, `pkg/auth`, `pkg/pgx` — shared packages; no tests can run against them until they exist
- [ ] `proto/events/catalog/v1/boat.proto`, `proto/services/catalog/v1/*.proto`, `buf.gen.yaml` — proto-gen has nothing to generate from yet
- [ ] `deploy/docker-compose.yml`, `deploy/redpanda/topics.sh`, `deploy/postgres/init.sh` — `make up` has no compose file yet
- [ ] `deploy/ci/docker-compose.yml` (Jenkins+Harbor), `Jenkinsfile` — CI doesn't exist yet (D-21)
- [ ] `apps/web` (Next.js skeleton) — no frontend code exists yet
- [ ] `deploy/kong/kong.yml` + dev-token Makefile target — the Kong spike's artifacts
- [ ] Integration test for `TestBoatUpsertedAppliedOnce` (PLAT-05) and `TestFailedHandlerLandsInDLQAfter3Retries` (PLAT-07) — these are the two highest-value tests in the phase and should be written test-first per the project's general TDD leaning where practical, even though this is greenfield infra work

*(This is a greenfield phase — every listed gap is expected; none of it is a surprise finding, just the literal "nothing exists yet" starting state restated as a test-readiness checklist for the planner.)*

## Security Domain

`security_enforcement` is enabled (ASVS level 1, block on `high`) per `.planning/config.json`.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Partial (JWT verification exists in Phase 1; issuance/OTP is Phase 2) | RS256 JWT verified by Kong at the edge (D-27); `pkg/auth` verifier reused later by identity |
| V3 Session Management | Partial | Access JWT 15min + refresh cookie 30 days, httpOnly/Secure/SameSite=Lax (D-31) — token *shape* is Phase 1, refresh *endpoint* is Phase 2 |
| V4 Access Control | Yes | Claim→header trust boundary enforced by network isolation + shared-secret header check in `pkg/httpx` (D-30) — this is the actual Phase 1 access-control surface, since no business-level roles exist yet |
| V5 Input Validation | Yes | proto schema defines wire-level types (int64 money, no floats per D-43); connect-go's generated code rejects malformed wire payloads before handler code runs |
| V6 Cryptography | Yes | RS256 asymmetric signing (private key never leaves identity's `.env` in dev, D-27); never hand-roll JWT verification — `pkg/auth` wraps a maintained JWT library (not specified by decision number, but implied by "pkg/auth contains the real issuer... and verifier" — planner should pick a maintained Go JWT library, e.g. one supporting RS256 verification with `jwk`/PEM public keys, rather than hand-rolling signature verification) |
| V9 Communications | Partial | TLS termination at Kong (D-27 implies HTTPS at the edge); internal Compose network traffic is unencrypted service-to-service in dev (acceptable for a single-EC2 Compose deployment per the project's own scoped tradeoffs, not a gap to fix in Phase 1) |
| V7 Error Handling / Logging | Yes | `pkg/httpx.WriteError` maps connect error codes → HTTP status + `{code, message}` JSON (D-44); PII-safe logging fields only (D-45, D-52) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Header spoofing (`X-Operator-Id`/`X-Role` set by a caller that isn't the BFF) | Spoofing / Elevation of Privilege | Network isolation (services expose no host ports, only reachable via Compose internal network) + `X-Internal-Token` shared-secret check in `pkg/httpx` middleware (D-30) — both must hold, network isolation alone is not sufficient defense-in-depth |
| Forged/expired JWT reaching a service | Spoofing | Kong's `jwt` plugin verifies signature+expiry before any request reaches the BFF/services (D-27); services never re-trust a JWT directly, only the already-verified header claims |
| Direct-to-Kafka publish bypassing the outbox (a future contributor "just calls `producer.Produce()`") | Tampering (breaks the audit trail / atomicity guarantee) | Structural prevention: `pkg/kafka`'s only exported publish path is the one `pkg/outbox` relay uses; no service code should import a raw `kgo.Client` producer directly — enforce via code review / package API surface, not a runtime check |
| PII denormalized into an event "for convenience" | Information Disclosure | Proto review checklist (existing PITFALLS.md Pitfall 9) — no PII-shaped fields in any `.proto` event message; enforced by human review in Phase 1 since no automated proto-PII-linter is specified |
| DLQ payloads retaining sensitive data indefinitely with no visibility | Information Disclosure / Repudiation | DLQ entries carry the *original envelope* (D-12) — since envelopes are already PII-free by the above rule, this is a non-issue by construction, not a separate control to add |

## Sources

### Primary (HIGH confidence)
- `go list -m -versions` / `go get` against the live Go module proxy (proxy.golang.org) this session — verified: `golang.org/x/sync`, `github.com/google/uuid`, `connectrpc.com/otelconnect`, `go.opentelemetry.io/contrib/bridges/otelslog`, `github.com/twmb/franz-go/plugin/kotel`, `github.com/twmb/franz-go`, `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`, `github.com/testcontainers/testcontainers-go` (+ `/modules/postgres`, `/modules/redpanda`), `github.com/golangci/golangci-lint/v2`
- Direct source read this session: `github.com/google/uuid@v1.6.0/version7.go`, `github.com/twmb/franz-go/plugin/kotel@v1.7.1/{kotel,tracer,carrier}.go`, `connectrpc.com/otelconnect@v0.10.0/interceptor.go`, `go.opentelemetry.io/contrib/bridges/otelslog@v0.20.1/handler.go`, `github.com/twmb/franz-go@v1.22.0/pkg/kgo/{consumer_group.go,config.go}`
- `npm view <pkg> version` against the live npm registry this session — `next-intl`, `@tanstack/react-query`, `lefthook`, `next`, `tailwindcss`, `shadcn`
- `gsd-tools package-legitimacy check --ecosystem npm` this session — `next-intl`, `@tanstack/react-query`, `lefthook`

### Secondary (MEDIUM confidence)
- [Kong JWT plugin docs](https://developer.konghq.com/plugins/jwt/) via WebFetch this session — RS256 DB-less `secret`+`rsa_public_key`+`algorithm` field requirements
- [golangci-lint v1→v2 migration guide](https://golangci-lint.run/docs/product/migration-guide/) via WebFetch this session — config schema breaking changes
- [next-intl App Router getting-started](https://next-intl.dev/docs/getting-started/app-router) via WebFetch this session — `getRequestConfig`, `NextIntlClientProvider`, plugin registration shape
- [IBM Plex Sans Thai — Google Fonts](https://fonts.google.com/specimen/IBM+Plex+Sans+Thai) via WebSearch this session — confirms `thai` subset exists for this family

### Tertiary (existing project research, carried forward, LOW re-verification this session but originally MEDIUM-HIGH)
- `.planning/research/ARCHITECTURE.md` — Patterns 1, 2, 3, 4, 5, 6, 7, 9 (unchanged); Pattern 8 corrected by this session's kotel finding
- `.planning/research/PITFALLS.md` — Pitfalls 1, 3, 9, 10, 11, 12, 13 (unchanged, still authoritative for Phase 1)
- `.planning/research/STACK.md`, `.planning/research/SUMMARY.md` — background only, superseded by `.claude/CLAUDE.md`'s more current version pins per this phase's brief

## Metadata

**Confidence breakdown:**
- Standard stack (new Phase-1-specific packages): HIGH — every version/API claim in this file was verified against a live registry or by reading source this session, not recalled from training data alone
- Architecture (kotel/otelconnect/otelslog wiring): HIGH for API existence/shape; MEDIUM for the exact `fanoutHandler`/`otelslog.Handler` composition detail (Open Question 1) — a five-minute `go doc` check at implementation time closes this gap
- Pitfalls (golangci-lint v2, Kong RS256, IBM Plex Sans Thai subset): MEDIUM — each is CITED to one authoritative source via WebFetch/WebSearch this session, not independently cross-checked against a second source
- Package legitimacy (next-intl, @tanstack/react-query, lefthook): MEDIUM — SUS verdicts are a known false-positive class (recent-release-date heuristic) explained with download/repo evidence, but formal disposition per protocol still routes to a human checkpoint

**Research date:** 2026-09-26
**Valid until:** ~30 days for the pinned versions (fast-moving npm/Go ecosystem, matches CLAUDE.md's own research cadence); the architectural findings (kotel exists, Kong RS256 field requirement, golangci-lint v2 schema) are structural facts unlikely to change before Phase 1 executes

## RESEARCH COMPLETE
