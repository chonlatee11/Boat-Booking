# Phase 1: Platform Foundation - Pattern Map

**Mapped:** 2026-09-26
**Files analyzed:** ~40 (scaffolding + shared pkg + first two services + infra + web skeleton)
**Analogs found:** 0 / ~40 (greenfield repo)

## Greenfield Notice

`git ls-files` confirms the tracked tree contains only `.claude/CLAUDE.md`, `.planning/**`, and `PROJECT.md`. There is **no existing service, package, config, or component code** anywhere in this repo. Phase 1 is the first code phase — it creates `services/_template`, `pkg/*`, `proto/`, `deploy/`, and `apps/web`, all from scratch.

**Consequence for the planner:** there are no in-repo analogs to copy from for any file below. Every "Pattern Source" in this document points to `01-RESEARCH.md` (Code Examples / Architecture Patterns / Don't Hand-Roll sections) or to `01-CONTEXT.md` decisions (D-01..D-53), not to existing repo files. Once `services/_template` and `pkg/*` exist (end of Phase 1), they become the canonical analogs for Phase 2+ — this document should be regenerated/superseded then.

## File Classification

| New File (representative) | Role | Data Flow | In-repo Analog | Match Quality |
|---|---|---|---|---|
| `services/_template/cmd/main.go` | controller (bootstrap) | request-response + event-driven (HTTP+consumer+relay in one binary) | none | no-analog |
| `services/_template/internal/domain/*.go` | model | CRUD | none | no-analog |
| `services/_template/internal/app/*.go` | service (use-case) | CRUD / transform | none | no-analog |
| `services/_template/internal/adapters/postgres/*.go` (sqlc-generated + queries) | model/service | CRUD | none | no-analog |
| `services/_template/internal/adapters/http/*.go` | controller | request-response | none | no-analog |
| `services/_template/internal/adapters/kafka/*.go` | event-driven consumer/producer wiring | event-driven | none | no-analog |
| `services/_template/migrations/00001_platform.sql` | migration | batch (DDL) | none | no-analog |
| `pkg/kafka/*.go` (producer, consumer, kotel wiring) | utility/service | event-driven, pub-sub | none | no-analog |
| `pkg/outbox/*.go` (relay) | service | batch + event-driven | none | no-analog |
| `pkg/httpx/*.go` (middleware, logger, error writer, health) | middleware/utility | request-response | none | no-analog |
| `pkg/auth/*.go` (RS256 issuer/verifier) | utility | request-response | none | no-analog |
| `pkg/pgx/*.go` (pool + WithTx) | utility | CRUD | none | no-analog |
| `pkg/clock/*.go` | utility | transform | none | no-analog |
| `pkg/money/*.go` | utility | transform | none | no-analog |
| `pkg/events/*.go` (envelope types) | model | event-driven | none | no-analog |
| `proto/events/catalog/v1/boat.proto`, `proto/services/*/v1/*.proto` | config (schema) | event-driven / request-response | none | no-analog |
| `services/catalog/*` (real skeleton, writes outbox) | controller+service | CRUD + event-driven | none (built on `services/_template`) | no-analog |
| `services/schedule/*` (real skeleton, consumes) | controller+service | event-driven | none (built on `services/_template`) | no-analog |
| `services/gateway/*` (BFF) | controller/middleware | request-response | none | no-analog |
| `deploy/docker-compose.yml`, `deploy/ci/docker-compose.yml` | config | batch/infra | none | no-analog |
| `deploy/redpanda/topics.sh`, `deploy/postgres/init.sh` | utility (provisioning script) | batch | none | no-analog |
| `deploy/observability/**` (OTel Collector, Grafana provisioning) | config | streaming (telemetry) | none | no-analog |
| `kong/kong.yml` | config | request-response | none | no-analog |
| `Dockerfile` (root, `ARG SERVICE`) | config | file-I/O (build) | none | no-analog |
| `Makefile` | config | batch | none | no-analog |
| `.golangci.yml`, `.eslintrc`, `lefthook.yml` | config | — | none | no-analog |
| `apps/web/app/[locale]/layout.tsx` | component/provider | request-response (SSR shell) | none | no-analog |
| `apps/web/lib/apiFetch.ts` | utility/hook | request-response | none | no-analog |
| `apps/web/app/[locale]/**/page.tsx` (client components) | component | request-response | none | no-analog |

## Pattern Assignments

Since there are no in-repo analogs, each cluster below cites the RESEARCH.md section and CONTEXT.md decision IDs to build from instead of a codebase file.

### `services/_template/cmd/main.go` (bootstrap, all roles)
**Pattern source:** `01-CONTEXT.md` D-04 (one binary, errgroup for HTTP+relay+consumer, graceful shutdown), D-39/D-40 (`/healthz`,`/readyz`, SIGTERM budget); `01-RESEARCH.md` Code Examples → "Root Dockerfile with `-healthcheck` subcommand" (lines ~390-419) for the `-healthcheck` CLI branch pattern.
```go
// cmd/main.go — first thing, before any other startup work
if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
    resp, err := http.Get("http://localhost:" + port + "/readyz")
    if err != nil || resp.StatusCode != 200 { os.Exit(1) }
    os.Exit(0)
}
```
Then wire `errgroup.Group` running: HTTP server (chi + otelhttp + connect handlers), outbox relay goroutine (disabled by env, D-04), Kafka consumer goroutine (disabled by env) — no existing repo file to copy this shape from; this is the first one.

### `pkg/kafka/*.go` (producer/consumer, event-driven)
**Pattern source:** `01-RESEARCH.md` "Pattern 8-revised: OTel-over-Kafka using `kotel`" (lines ~187-231) — verified `kotel.NewTracer`, `kgo.DisableAutoCommit()`, `tracer.WithProcessSpan(record)` API shapes. `01-CONTEXT.md` D-08 (`Publish(ctx, topic, aggregateID, envelope)` contract), D-12/D-13 (DLQ + idempotent `Consumer.Handle` wrapper).
```go
cl, err := kgo.NewClient(
    kgo.SeedBrokers(brokers...),
    kgo.ConsumerGroup(serviceName),
    kgo.ConsumeTopics(topics...),
    kgo.DisableAutoCommit(),
    kgo.WithHooks(kt.Hooks()...),
)
```
No repo analog — this is new shared infrastructure.

### `pkg/outbox/*.go` (relay)
**Pattern source:** `01-CONTEXT.md` D-10/D-11 (poll 500ms + nudge channel, `FOR UPDATE SKIP LOCKED` batch of 100, `RequiredAcks(AllISR)`); `01-RESEARCH.md` same Pattern 8-revised section for the `record.Context` = extracted `traceparent` step (Pitfall: "kotel's `record.Context` must be set before `Produce`", lines ~362-370) — this is the exact mechanism, not optional.
No repo analog.

### `pkg/httpx/*.go` (middleware, logger, error writer)
**Pattern source:** `01-RESEARCH.md` "dual-sink slog (stdout JSON + OTel Logs bridge)" fanoutHandler code (lines ~246-281); `01-CONTEXT.md` D-30 (claim-header trust boundary: internal-network + `X-Internal-Token` check → 401 on missing/mismatch), D-44 (`WriteError` maps connect error code → HTTP status + JSON), D-52 (mandatory log fields).
No repo analog — `Claims`/`FromContext(ctx)` API is new.

### `services/catalog` → `services/schedule` (proof event flow)
**Pattern source:** `01-RESEARCH.md` System Architecture Diagram (lines ~145-185) — catalog writes outbox row in same tx as `BoatUpserted`, relay publishes to `catalog.events`, schedule consumes via `pkg/kafka` + `processed_events`. `01-CONTEXT.md` D-01. Both services are literal copies of `services/_template` (D-03) with business logic added — so the template IS their analog once it exists, but doesn't yet.

### `services/gateway` (BFF)
**Pattern source:** `01-CONTEXT.md` D-29/D-30 (parses JWT cookie, sets `X-User-Id`/`X-Operator-Id`/`X-Role`/`X-Internal-Token`, exposes `GET /api/v1/boats`); `01-RESEARCH.md` "Pattern: connect-go tracing via otelconnect" (lines ~232-244) for client-side interceptor wiring to catalog.

### `kong/kong.yml`
**Pattern source:** `01-RESEARCH.md` "Kong DB-less `kong.yml` — JWT RS256 spike shape" (lines ~421-448), and the gotcha immediately above it re: the required dummy `secret` field for RS256 in DB-less validation (lines ~283-300). Copy this YAML shape verbatim as the starting point.

### `apps/web/**` (Next.js skeleton)
**Pattern source:** `01-CONTEXT.md` D-32..D-36; `01-RESEARCH.md` pitfall "IBM Plex Sans Thai needs explicit `thai` subset" (lines ~347-360) — copy the exact `subsets: ['thai', 'latin']` snippet, not a Latin-only tutorial example. `next-intl` App Router shape per D-33 (locale always prefixed, default `th`).

### `deploy/ci/docker-compose.yml`, testcontainers usage
**Pattern source:** `01-RESEARCH.md` "testcontainers-go — verified lockstep versions" (lines ~450-458), all three modules pinned to v0.44.0.

### `.golangci.yml`
**Pattern source:** `01-RESEARCH.md` pitfall "golangci-lint v2's config schema is a breaking change from v1" (lines ~319-345) — copy the v2-shaped YAML given there (`version: "2"`, `linters.settings.forbidigo.forbid[].pattern`), not a v1-style config from memory/tutorials.

## Shared Patterns

### OTel trace propagation (HTTP → outbox → Kafka → consumer)
**Source:** `01-RESEARCH.md` lines 187-244 (kotel + otelconnect patterns)
**Apply to:** `pkg/kafka`, `pkg/outbox`, `pkg/httpx`, every service's `cmd/main.go` bootstrap, `services/gateway`'s connect client construction.

### Idempotent consumer + DLQ
**Source:** `01-CONTEXT.md` D-12/D-13; `01-RESEARCH.md` "Don't Hand-Roll" table (lines ~302-313)
**Apply to:** `pkg/kafka.Consumer.Handle`, used identically by `services/schedule` and every future consuming service.

### Claim-header trust boundary
**Source:** `01-CONTEXT.md` D-30
**Apply to:** `pkg/httpx` middleware, applied by every service except `services/gateway` (which produces the headers) and `services/_template` (inherits it for free).

### Config via env-only helpers
**Source:** `01-CONTEXT.md` D-16
**Apply to:** every service's `cmd/main.go` — `MustEnv`/`EnvOr` from `pkg/httpx` or a small `pkg/config`-less helper set; no library.

## No Analog Found

Every file in this phase has no in-repo analog (see Greenfield Notice). Full list is the "File Classification" table above. Planner must build directly from `01-RESEARCH.md` Code Examples / Architecture Patterns and `01-CONTEXT.md` D-01..D-53, not from codebase search.

## Metadata

**Analog search scope:** entire tracked tree (`git ls-files`) — confirmed no `services/`, `pkg/`, `proto/`, `deploy/`, or `apps/` directories exist yet.
**Files scanned:** 27 tracked files total in repo (all `.planning/*`, `.claude/CLAUDE.md`, `PROJECT.md` — no application code).
**Pattern extraction date:** 2026-09-26
**Regeneration note:** once `services/_template` and `pkg/*` are built in this phase, they become the canonical analog source for all Phase 2+ pattern mapping — this file should not be reused as-is beyond Phase 1.
