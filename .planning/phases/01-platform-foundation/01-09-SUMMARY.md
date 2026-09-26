---
phase: 01-platform-foundation
plan: 09
subsystem: infra
tags: [otel, slog, otelslog, otelhttp, errgroup, sqlc, pgx, kafka, outbox, chi, testcontainers]

requires:
  - phase: 01-platform-foundation (plans 01, 04, 07, 08)
    provides: "pkg/httpx.Claims/RequireInternal/RequireClaims (01-01), pkg/outbox.Insert/Relay + pkg/pgx.WithTx/NewPool + pkg/testenv (01-04), pkg/kafka.Consumer.Handle idempotent apply + DLQ (01-07), OTel Collector -> Tempo/Loki/Prometheus pipeline services now emit into (01-08)"
provides:
  - "pkg/httpx.NewLogger(service): stdout JSON/text fanned out to otelslog's OTel logs bridge, trace_id/span_id on the stdout side via a traceHandler wrapper"
  - "pkg/httpx.SetupOTel(ctx, service): always installs the TraceContext+Baggage propagator; wires OTLP trace/metric/log providers only when OTEL_EXPORTER_OTLP_ENDPOINT is set; allowlistExporter drops every span attribute not in the D-45 allowlist"
  - "pkg/httpx.Readiness/NewReadiness/Healthz: /readyz with a 1s result cache, 2s per-check timeout, and SetShuttingDown for D-40's ordered shutdown"
  - "pkg/httpx.Instrument(service, handler): otelhttp tracing (health paths filtered) + method/path/status/duration request log line"
  - "services/_template: the one-binary service runtime (errgroup of HTTP+relay+consumer, RELAY_ENABLED/CONSUMER_ENABLED env-gated, ordered SIGTERM shutdown) plus a real CI-tested POST /v1/pings slice exercising domain -> app(pgx.Tx) -> sqlc -> outbox -> the service's own Kafka consumer"
affects: [01-10-make-new-service, 01-11, 01-12-e2e-walking-skeleton, 01-13]

actuals:
  tokens: 23000
  tasks: 2
  commits: 2
  plan_head_before: a9963bbc185240c315cdb81a880895c2d9f06e34

tech-stack:
  added:
    - "go.opentelemetry.io/contrib/bridges/otelslog v0.20.1 — slog -> OTel logs bridge"
    - "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 — HTTP server tracing"
    - "go.opentelemetry.io/otel/exporters/otlp/{otlptrace/otlptracehttp,otlpmetric/otlpmetrichttp,otlplog/otlploghttp} v1.46.0/v0.22.0 — OTLP/HTTP exporters for traces/metrics/logs"
    - "go.opentelemetry.io/otel/sdk/log + otel/log v0.22.0 — OTel Logs SDK (paired with core v1.46.0)"
    - "golang.org/x/sync v0.22.0 (errgroup) — pinned below v0.23.0, which requires go1.26 and would break the repo's go1.25.x pin (same class of issue as franz-go/goose in 01-07)"
  patterns:
    - "SetupOTel always sets the global propagator so trace context keeps flowing across HTTP+Kafka even with no OTLP endpoint configured; the OTLP provider wiring itself is a single early-return away entirely, so 'telemetry off' and 'telemetry on' are the same code path minus three exporter constructions"
    - "allowlistExporter wraps any sdktrace.SpanExporter via tracetest.SpanStubFromReadOnlySpan/Snapshot to filter attributes before export — reusable for any future exporter without touching the TracerProvider wiring"
    - "Every real service's cmd/main.go is a copy of services/_template/cmd/main.go: env-gated relay/consumer as errgroup goroutines, a dedicated shutdown goroutine gating relay-flush on HTTP-drain completion, closing producer+pool only after g.Wait()"
    - "internal/adapters/postgres holds only sqlc-generated code (pgtype.UUID at the boundary); internal/app converts to/from google/uuid.UUID at the two call sites (toPgUUID/fromPgUUID) so domain/app types never carry pgx-specific types"

key-files:
  created:
    - pkg/httpx/logger.go
    - pkg/httpx/otel.go
    - pkg/httpx/otel_test.go
    - pkg/httpx/health.go
    - pkg/httpx/health_test.go
    - pkg/httpx/middleware.go
    - services/_template/go.mod
    - services/_template/go.sum
    - services/_template/cmd/main.go
    - services/_template/cmd/main_integration_test.go
    - services/_template/internal/domain/errors.go
    - services/_template/internal/domain/ping.go
    - services/_template/internal/app/ping.go
    - services/_template/internal/adapters/http/routes.go
    - services/_template/internal/adapters/kafka/handlers.go
    - services/_template/internal/adapters/postgres/sqlc.yaml
    - services/_template/internal/adapters/postgres/queries/pings.sql
    - services/_template/internal/adapters/postgres/db.go
    - services/_template/internal/adapters/postgres/models.go
    - services/_template/internal/adapters/postgres/pings.sql.go
    - services/_template/migrations/00002_pings.sql
  modified:
    - go.work
    - go.work.sum
    - pkg/go.mod
    - pkg/go.sum
    - services/gateway/go.mod
    - services/gateway/go.sum
    - Makefile

key-decisions:
  - "Instrument(service, handler) builds its own httpx.NewLogger(service) internally rather than taking a logger parameter, matching the plan's exact stated signature (Instrument(service string, h http.Handler) http.Handler) — the small cost is a second otelslog bridge handler instance per service (same underlying global LoggerProvider), not a correctness issue."
  - "golang.org/x/sync pinned to v0.22.0, not the plan's illustrative v0.23.0 — v0.23.0's go.mod requires go1.26, which would break the repo's go1.25.x toolchain pin (same pattern as 01-07's franz-go/goose pin, verified against proxy.golang.org before choosing)."
  - "sqlc's pgx/v5 codegen maps postgres uuid columns to pgtype.UUID, not google/uuid.UUID — internal/app/ping.go owns two small unexported converters (toPgUUID/fromPgUUID) at the two call sites into internal/adapters/postgres, keeping pgx-specific types out of internal/domain and internal/app's public signatures."

requirements-completed: []

coverage:
  - id: D1
    description: "services/_template boots HTTP+relay+consumer as errgroup goroutines, reports /healthz 200 and /readyz 200 {db,kafka} once dependencies answer, and shuts down cleanly within 15s of SIGTERM/ctx-cancel in the D-40 order (readyz 503 -> stop HTTP -> flush relay after HTTP drains -> close Kafka+DB -> OTel shutdown)"
    requirement: PLAT-01
    verification:
      - kind: integration
        ref: "services/_template/cmd/main_integration_test.go#TestTemplateReadyAndGracefulShutdown (go test -tags=integration -run TestTemplateReadyAndGracefulShutdown github.com/chonlatee11/boat-booking/services/__NAME__/...)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every non-health route sits behind httpx.RequireInternal; POST /v1/pings without X-Internal-Token, or with the token but no claim headers, returns 401 — operator id is taken from claims, never the request body"
    requirement: PLAT-01
    verification:
      - kind: integration
        ref: "services/_template/cmd/main_integration_test.go#TestPingRoundTrip (two 401 assertions)"
        status: pass
    human_judgment: false
  - id: D3
    description: "POST /v1/pings writes a pings row and a __NAME__.PingRecorded outbox row in one tx (empty payload, ids only — D-45); the service's own consumer acks it exactly once via processed_events, and replaying the same envelope leaves processed_events at exactly one row for that event_id"
    requirement: PLAT-02
    verification:
      - kind: integration
        ref: "services/_template/cmd/main_integration_test.go#TestPingRoundTrip (create -> ack-within-15s -> republish -> processed_events count == 1)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Note validation: empty note and a note over 280 characters both return 400 invalid_argument (domain.ValidateNote, also enforced by the pings table's check constraint)"
    requirement: PLAT-02
    verification:
      - kind: integration
        ref: "services/_template/cmd/main_integration_test.go#TestPingRoundTrip (empty-note and 281-char-note assertions)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Exported spans carry only allowlisted attributes (D-45) — client.address, user_agent.original, url.query are dropped before export, http.route survives"
    requirement: PLAT-06
    verification:
      - kind: unit
        ref: "pkg/httpx/otel_test.go#TestAllowlistExporterDropsNonAllowlistedAttributes"
        status: pass
    human_judgment: false
  - id: D6
    description: "Logs are slog JSON on stdout with service/env (+trace_id/span_id when a span is active), fanned out through the otelslog bridge; LOG_FORMAT=text switches stdout to text; /readyz caches results for 1s and times out each check at 2s; SetShuttingDown forces 503"
    verification:
      - kind: unit
        ref: "pkg/httpx/health_test.go (5 tests: 200, 503-failing-check, 1s-cache-then-refresh, shutting-down-503, healthz-always-200)"
        status: pass
    human_judgment: false
  - id: D7
    description: "With OTEL_EXPORTER_OTLP_ENDPOINT unset the service still runs (propagator set, no exporters constructed)"
    verification:
      - kind: integration
        ref: "Both TestTemplateReadyAndGracefulShutdown and TestPingRoundTrip run with OTEL_EXPORTER_OTLP_ENDPOINT unset in the test environment (verified absent) and pass"
        status: pass
    human_judgment: false
  - id: D8
    description: "make sqlc-gen (pinned sqlc/sqlc:1.31.1) produces committed code with no diff on regeneration; make test, make test-integration, and make lint stay green repo-wide with the template wired into go.work"
    verification:
      - kind: other
        ref: "make sqlc-gen && git diff --exit-code -- services/_template/internal/adapters/postgres/ (clean); make test, make test-integration (all ok, including pre-existing pkg/kafka and pkg/outbox suites), make lint (0 issues across pkg/gateway/_template modules, buf lint, PII check) — all re-run clean at the end of this plan"
        status: pass
    human_judgment: false

duration: ~45min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 9: Service Template — Boot, Health/Readiness, Ordered Shutdown, and a Real Outbox->Kafka->Consumer Slice Summary

**`services/_template` is the one-binary runtime every future service copies — chi HTTP + outbox relay + Kafka consumer as errgroup goroutines, env-gated and gracefully shut down in the D-40 order — proven by a real `POST /v1/pings` slice that writes an outbox row in the same tx as its Postgres insert and gets acked back by the service's own idempotent consumer, plus new `pkg/httpx` OTel/logging/readiness helpers every service (including the gateway) now shares.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-09-26T05:47:53Z
- **Tasks:** 2 completed
- **Files:** 21 created, 7 modified

## Accomplishments

- `pkg/httpx.NewLogger`/`SetupOTel`/`Instrument`/`Readiness`: the shared observability + trust-boundary + health surface every service (including the already-shipped gateway) builds on — stdout JSON/text logs fanned out to the OTel logs bridge, OTLP trace/metric/log providers wired only when configured, span attribute allowlisting (D-45), and a cached/timeout-bounded `/readyz`
- `services/_template/cmd/main.go`: `run(ctx)` boots pool/producer/relay/consumer as `errgroup` goroutines, `RELAY_ENABLED`/`CONSUMER_ENABLED` default from `DATABASE_URL` presence, and shuts down in the exact D-40 order — readyz 503, stop HTTP, flush the relay only after HTTP has drained, close Kafka+DB, then OTel shutdown — all within a 15s budget
- A real `POST /v1/pings` slice (`internal/domain`, `internal/app`, `internal/adapters/{postgres,http,kafka}`) exercises every platform layer end-to-end: HTTP trust boundary (401s) -> validation (400s) -> one Postgres tx with an outbox row -> the service's own Kafka consumer acking it -> idempotent replay leaving `processed_events` at exactly one row
- `make sqlc-gen` (pinned `sqlc/sqlc:1.31.1`) generates and commits `internal/adapters/postgres/{db,models,pings.sql}.go` for every service — the template is now a real, CI-tested member of the workspace (`go.work`), not a template-only stub

## Task Commits

1. **Task 1: Template boots end-to-end** - `91ed847` (feat)
2. **Task 2: Template sample slice POST /v1/pings** - `df932a2` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP update)

## Files Created/Modified

- `pkg/httpx/logger.go` — `NewLogger`, `traceHandler` (adds trace_id/span_id from ctx to stdout only), `fanoutHandler` (hand-rolled 2-sink slog fanout)
- `pkg/httpx/otel.go` — `SetupOTel`, `allowlistExporter` (span attribute allowlist, D-45)
- `pkg/httpx/otel_test.go` — allowlist filtering unit test
- `pkg/httpx/health.go` — `Readiness`, `NewReadiness`, `Handler`, `SetShuttingDown`, `Healthz`
- `pkg/httpx/health_test.go` — 200/503/1s-cache/shutting-down/healthz tests
- `pkg/httpx/middleware.go` — `Instrument`, `requestLog`, `statusWriter`
- `services/_template/go.mod`, `go.sum` — new module in `go.work`
- `services/_template/cmd/main.go` — `run(ctx)`, `-healthcheck` subcommand, `parseBoolEnv`
- `services/_template/cmd/main_integration_test.go` — `TestTemplateReadyAndGracefulShutdown`, `TestPingRoundTrip`
- `services/_template/internal/domain/{errors,ping}.go` — `Ping`, `ValidateNote`, sentinel errors
- `services/_template/internal/app/ping.go` — `RecordPing`, `AckPing`, `PingRecorded`, uuid<->pgtype converters
- `services/_template/internal/adapters/http/routes.go` — `POST /v1/pings`
- `services/_template/internal/adapters/kafka/handlers.go` — `Register`
- `services/_template/internal/adapters/postgres/{sqlc.yaml,queries/pings.sql,db.go,models.go,pings.sql.go}` — sqlc-generated Postgres layer
- `services/_template/migrations/00002_pings.sql` — `pings` table
- `go.work` — added `./services/_template`
- `pkg/go.mod`, `pkg/go.sum` — new OTel exporter/log/otelslog/otelhttp dependencies
- `services/gateway/go.mod`, `go.sum` — transitive dependency propagation (see Deviations)
- `Makefile` — `sqlc-gen` target

## Decisions Made

See `key-decisions` in frontmatter: `Instrument`'s own internal logger construction (matches the plan's literal signature), the `golang.org/x/sync` v0.22.0 pin (v0.23.0 requires go1.26), and the `toPgUUID`/`fromPgUUID` converters at the app/postgres boundary.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `golang.org/x/sync` v0.23.0 requires go1.26, breaking the repo's go1.25.x pin**
- **Found during:** Task 1 (`go build` in `services/_template` after adding `errgroup`)
- **Issue:** The plan's context named `golang.org/x/sync v0.23.0`, but that version's own `go.mod` declares `go 1.26.0` — incompatible with this repo's pinned `go 1.25.7` toolchain (same class of issue 01-07 hit with franz-go/goose).
- **Fix:** Pinned `golang.org/x/sync v0.22.0` instead (verified its `go.mod` declares `go 1.25.0`, compatible).
- **Files modified:** `services/_template/go.mod`, `go.sum`
- **Verification:** `go build ./...` and the full integration suite pass.
- **Committed in:** `91ed847` (Task 1 commit)

**2. [Rule 3 - Blocking] `services/gateway/go.mod`/`go.sum` needed new transitive dependency checksums**
- **Found during:** Task 1, standalone (`GOWORK=off`) build verification of `services/gateway`
- **Issue:** `pkg`'s new OTel exporter/log/otelslog/otelhttp imports expanded its dependency graph; `services/gateway` depends on `pkg` via a local `replace`, so its own `go.sum` needed the new transitive checksums for a standalone (non-workspace) build to succeed, even though gateway's own code never imports those packages directly.
- **Fix:** Ran `GOWORK=off go mod tidy` inside `services/gateway`.
- **Files modified:** `services/gateway/go.mod`, `services/gateway/go.sum`
- **Verification:** `GOWORK=off go build ./...` succeeds from `services/gateway`; workspace-mode `make lint`/`make test` unaffected (still green).
- **Committed in:** `91ed847` (Task 1 commit)

**3. [Rule 1 - Bug] `t.Cleanup` LIFO ordering bug left `run(ctx)` never cancelled before the test's shutdown-wait cleanup ran**
- **Found during:** Task 2, first `TestPingRoundTrip` run — the test failed with "run did not return within 15s of test cleanup cancellation" despite the round trip itself succeeding.
- **Issue:** The test registered two separate `t.Cleanup` calls — one calling `cancel()`, a second waiting on `errCh` with a 15s timeout. `t.Cleanup` runs LIFO, so the second-registered (wait) cleanup ran *before* the first-registered (`cancel()`) cleanup, meaning the wait started against a `run(ctx)` that had not yet been told to shut down.
- **Fix:** Merged both into a single `t.Cleanup` that calls `cancel()` then waits, making the ordering explicit in one place instead of relying on registration order.
- **Files modified:** `services/_template/cmd/main_integration_test.go`
- **Verification:** `TestPingRoundTrip` passes; `run` returns `nil` well within the 15s budget.
- **Committed in:** `df932a2` (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (2 blocking dependency-pin issues, 1 test-only ordering bug).
**Impact on plan:** All three were necessary for correctness/buildability; none touched production behavior beyond the two `go.sum`/`go.mod` dependency-graph fixes, which are transitive-checksum bookkeeping, not new functionality. No scope creep.

## Issues Encountered

None beyond the deviations above.

## User Setup Required

None — no external service configuration required. `OTEL_EXPORTER_OTLP_ENDPOINT` remains unset for local/CI runs by design (the no-op path is what this plan's tests exercise); wiring it into `deploy/docker-compose.yml` for real deployed services is out of this plan's scope (files_modified didn't include it) and is expected to land with `make new-service` (plan 10) / the walking-skeleton plan (01-12).

## Next Phase Readiness

- `services/_template` is a real, `go.work`-registered, CI-tested service — `make test`, `make test-integration`, and `make lint` all pass repo-wide with it wired in, and `make sqlc-gen` is idempotent (no diff on regeneration).
- Plan 10 (`make new-service`) can now copy this template verbatim and rename `__NAME__` — every platform guarantee (health/readiness, ordered shutdown, logging/tracing, the outbox->Kafka->consumer loop, the HTTP trust boundary) travels with the copy for free.
- No blockers for 01-10 onward.

## Self-Check: PASSED

- All 21 created files confirmed present on disk (see Files Created/Modified above); `git log --oneline` confirms both commits (`91ed847`, `df932a2`).
- Plan-level `<verification>` re-run clean at the end of this plan: `TestTemplateReadyAndGracefulShutdown` and `TestPingRoundTrip` both pass; `make test`, `make test-integration` (all packages `ok`, including the pre-existing `pkg/kafka` and `pkg/outbox` suites), and `make lint` (0 issues across `pkg`/`services/gateway`/`services/_template`, `buf lint`, PII check) all green; `make sqlc-gen` produces no diff against the committed generated files.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
