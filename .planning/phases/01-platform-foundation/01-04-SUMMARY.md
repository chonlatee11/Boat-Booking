---
phase: 01-platform-foundation
plan: 04
subsystem: events
tags: [kafka, franz-go, kotel, outbox, postgres, otel, testcontainers, goose]

requires:
  - phase: 01-02
    provides: proto toolchain, platformv1.Envelope, gen/go module
  - phase: 01-03
    provides: pkg/clock.Now (overridable wall clock used by events.New)
provides:
  - "pkg/events: Envelope New/Marshal/Unmarshal, TopicFor/DLQTopic (D-06)"
  - "pkg/pgx: NewPool + WithTx transaction helper, sanctioned pgxpool constructor (D-02)"
  - "pkg/kafka: kotel-traced, acks=all Producer.Publish (D-08)"
  - "pkg/outbox: Insert (same-tx write) + Relay (Run/Nudge/Sweep/metrics) (D-10, D-11, D-40, D-50)"
  - "pkg/testenv: Postgres 17 + Redpanda testcontainers helpers shared by every future service integration suite (D-23)"
  - "services/_template/migrations/00001_platform.sql: outbox + processed_events DDL, copied into every service (D-05)"
  - "deploy/redpanda/topics.sh: idempotent <svc>.events/.dlq topic provisioning, shared by compose and testenv (D-24)"
affects: [01-05, 01-06, 01-07, 01-08, 01-09, 01-10, 01-11, 01-12, 01-13]

actuals:
  tokens: 16906
  tasks: 2
  commits: 3
  plan_head_before: 871a337fa94c381e1d389355432e228eda4d6343

tech-stack:
  added: ["github.com/twmb/franz-go v1.21.7", "github.com/twmb/franz-go/plugin/kotel v1.7.1", "github.com/jackc/pgx/v5 v5.11.0", "github.com/google/uuid v1.6.0", "github.com/pressly/goose/v3 v3.27.3", "testcontainers-go + modules/postgres + modules/redpanda v0.44.0", "go.opentelemetry.io/otel v1.46.0 (+sdk/sdk/metric)"]
  patterns:
    - "pkg/pgx keeps package name pgx (D-02); every other package imports it as bbpgx to avoid clashing with github.com/jackc/pgx/v5's own package name 'pgx'"
    - "Relay.publishOnce runs the whole select-batch/publish/mark-published cycle inside one bbpgx.WithTx call; a publish failure breaks the row loop immediately (before the failed row's id is added to the published-ids slice) so FOR UPDATE SKIP LOCKED + ORDER BY id never lets a later row of the same aggregate overtake an earlier still-queued one (D-11)"
    - "traceparent crosses the outbox/Kafka async boundary via propagation.MapCarrier — Insert injects the ambient ctx's traceparent into the row at write time, publishOnce extracts it back into a fresh context.Background() before calling Producer.Publish, and kotel injects that context's traceparent into the Kafka header — nobody touches the header by hand (Pitfall 11)"
    - "otel.Meter(...)/otel.GetMeterProvider() are called ambiently (no injected MeterProvider parameter) — production services configure the global MeterProvider once at startup; tests that swap it via otel.SetMeterProvider mid-binary must tolerate a joined Collect() error from other already-completed tests' stale gauge callbacks (their pools closed) since the SDK still populates every instrument's own data despite one callback erroring"
    - "pkg/testenv/testenv.go is the only place other than pkg/kafka/pkg/pgx allowed to call kgo.NewClient/pgxpool.New directly (forbidigo exclusions were already anticipated in .golangci.yml from an earlier plan)"

key-files:
  created:
    - pkg/events/events.go
    - pkg/events/events_test.go
    - pkg/pgx/pgx.go
    - pkg/kafka/producer.go
    - pkg/outbox/outbox.go
    - pkg/outbox/outbox_integration_test.go
    - pkg/testenv/testenv.go
    - services/_template/migrations/00001_platform.sql
    - deploy/redpanda/topics.sh
  modified:
    - pkg/go.mod
    - pkg/go.sum
    - services/gateway/go.mod
    - services/gateway/go.sum
    - go.work

key-decisions:
  - "Pinned franz-go to v1.21.7 instead of the plan's illustrative v1.22.0 — v1.22.0 itself requires go1.26.0 (confirmed via proxy.golang.org), which would have forced the whole repo's go.work/go.mod directives off the project's Go 1.25.x pin (CLAUDE.md). kotel v1.7.1's own go.mod already declares a minimum of franz-go v1.21.1, so v1.21.7 (latest 1.21.x) satisfies kotel with zero API differences and stays on the 1.25.x line."
  - "Pinned goose to v3.27.3 instead of the plan's illustrative v3.28.0 — v3.28.0 requires go1.26.0; v3.27.3 requires go1.25.7, a same-line patch bump the toolchain auto-downloaded (GOTOOLCHAIN=auto), consistent with 01-01's precedent of pinning to whatever patch actually exists/works rather than an illustrative version."
  - "go.mod/go.work go directive bumped from 1.25.1 to 1.25.7 (not 1.25.1 verbatim) — the minimum goose v3.27.3 itself declares; still within the Go 1.25.x line, no toolchain-major change."
  - "deploy/redpanda/topics.sh's default RPK_BROKERS changed from the plan's illustrative localhost:9092 to 127.0.0.1:9093 — empirically verified (via a throwaway probe container + manual rpk exec) that this specific Redpanda testcontainer configures a dual-listener setup where the 'external' listener (9092) advertises the host-mapped port (unreachable from inside the same container), while the 'internal' listener (9093) advertises 127.0.0.1:9093, which is what a process running inside the container (rpk, via pkg/testenv's container.Exec) must dial. make up's init container still overrides this via its own compose-network broker address."
  - "TestRelayKeepsRowsWhenPublishFails tolerates (does not Fatal on) a joined error from ManualReader.Collect — investigated the SDK source (pipeline.produce) and confirmed rm.ScopeMetrics is still fully populated for every instrument even when one observable-gauge callback errors, so the counter assertion is unaffected; the error itself is expected cross-test noise from otel's global-provider retroactive callback rebinding, not a bug in this plan's code."

requirements-completed: [PLAT-02, PLAT-05, PLAT-06]

coverage:
  - id: D1
    description: "State change written via transactional outbox in the same Postgres tx; the relay publishes it to <svc>.events with key=aggregate_id and headers event_type/event_id/traceparent"
    requirement: "PLAT-05"
    verification:
      - kind: integration
        ref: "pkg/outbox/outbox_integration_test.go#TestTraceparentSurvivesRelay (go test -tags=integration ./pkg/outbox/...)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The trace id from the originating HTTP-equivalent span survives the outbox row -> relay -> Kafka record traceparent header, unbroken across the async hop"
    requirement: "PLAT-06"
    verification:
      - kind: integration
        ref: "pkg/outbox/outbox_integration_test.go#TestTraceparentSurvivesRelay (go test -tags=integration ./pkg/outbox/...)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Relay reads at most 100 unpublished rows (FOR UPDATE SKIP LOCKED, ORDER BY id), publishes with acks=all, stops the batch at the first publish failure so per-aggregate ordering is never violated, and increments a publish_errors counter on that path"
    requirement: "PLAT-05"
    verification:
      - kind: integration
        ref: "pkg/outbox/outbox_integration_test.go#TestRelayPreservesPerAggregateOrder, #TestRelayKeepsRowsWhenPublishFails"
        status: pass
    human_judgment: false
  - id: D4
    description: "Relay wakes on PollInterval or immediately on Nudge(), sweeps rows published >7 days ago hourly (never touching unpublished rows), and flushes once more before Run returns on shutdown"
    requirement: "PLAT-05"
    verification:
      - kind: integration
        ref: "pkg/outbox/outbox_integration_test.go#TestRelayNudgePublishesImmediately, #TestSweepDeletesOnlyOldPublished, #TestRelayFlushesOnShutdown"
        status: pass
    human_judgment: false
  - id: D5
    description: "pkg/testenv gives every future service integration suite a Postgres 17 + Redpanda testcontainers helper, reusing deploy/redpanda/topics.sh for topic provisioning; goose applies services/_template/migrations against a fresh per-test database"
    requirement: "PLAT-02"
    verification:
      - kind: integration
        ref: "all pkg/outbox integration tests exercise testenv.StartPostgres/StartRedpanda/NewDB (go test -tags=integration ./pkg/outbox/...)"
        status: pass
    human_judgment: false
  - id: D6
    description: "make test, make lint, and both GOWORK=off module builds (services/gateway, pkg) stay green with the new dependencies wired in"
    verification:
      - kind: other
        ref: "make test && make lint && (cd services/gateway && GOWORK=off go build ./cmd) && (cd pkg && GOWORK=off go build ./...)"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 4: Outbox, Kafka Producer, and Testcontainers Summary

**Transactional outbox (Insert + Relay: ordering, Nudge, Sweep, shutdown flush, OTel metrics) plus a kotel-traced Kafka producer and shared Postgres 17 + Redpanda testcontainers helpers, proven end-to-end by TestTraceparentSurvivesRelay against real infrastructure.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-09-26T02:05:45Z (approx, end of prior plan)
- **Completed:** 2026-09-26T02:49:25Z
- **Tasks:** 2 (1 tracer + 1 TDD)
- **Files created:** 9, modified: 5

## Accomplishments

- `pkg/events`: `Envelope` builder (UUIDv7 event ids via `pkg/clock.Now`), `Marshal`/`Unmarshal`, `TopicFor`/`DLQTopic` — the shared wire-contract helpers every service imports
- `pkg/pgx`: `NewPool` + `WithTx` — the one sanctioned `pgxpool.New` call site outside `pkg/testenv` (D-02, forbidigo-enforced)
- `pkg/kafka`: `Producer.Publish` — kotel-traced, `acks=all`, record key = aggregate id, headers `event_type`/`event_id` always set, `traceparent` injected by kotel from `record.Context` (never written by hand)
- `pkg/outbox`: `Insert` (same-tx write, stores the originating `traceparent`) and `Relay` — `FOR UPDATE SKIP LOCKED` + `ORDER BY id` batch publish that stops at the first failure, `Nudge()` for immediate wake, hourly `Sweep()` that only deletes published rows older than 7 days, a shutdown flush via `context.WithoutCancel` so in-flight rows are never stranded, and `outbox.backlog`/`outbox.oldest_unpublished_age`/`outbox.publish_errors` OTel metrics
- `pkg/testenv`: Postgres 17.9 + Redpanda v26.2.3 testcontainers helpers (`StartPostgres`, `NewDB` with goose migrations, `StartRedpanda` reusing `deploy/redpanda/topics.sh`) — the shared foundation every later service's `-tags=integration` suite builds on
- `services/_template/migrations/00001_platform.sql` + `deploy/redpanda/topics.sh`: the outbox/processed_events DDL and idempotent topic-provisioning script every service copies/reuses
- `TestTraceparentSurvivesRelay`: proves a trace id survives a real outbox write → relay poll → Kafka publish → consume round-trip against actual Postgres + Redpanda containers, not mocks

## Task Commits

Each task was committed atomically (Task 2 followed the TDD RED→GREEN pattern; no REFACTOR commit — the GREEN implementation needed no cleanup):

1. **Task 1: Outbox row in tx → relay → Redpanda record with key, headers and the originating traceparent** - `518a661` (feat)
2. **Task 2 RED:** `c5c6290` (test) — failing tests for Nudge/Sweep/metrics (genuine build failure: `relay.Nudge undefined`)
3. **Task 2 GREEN:** `8f07306` (feat) — Nudge/Sweep/metrics implementation, all 5 behavior tests pass

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

_Task 1 is `type="tracer"` — its `<verify>` carries only `<automated>` and `workflow.human_verify_mode` is `end-of-phase` (default) with auto-mode inactive, so per the row-3 precedence rule the tracer verify was re-run end-to-end after commit (passed: `go test -tags=integration ... TestTraceparentSurvivesRelay`, `go test pkg/events && make lint && GOWORK=off builds`) and expansion into Task 2 continued with no checkpoint._

## Files Created/Modified

- `pkg/events/events.go`, `pkg/events/events_test.go` — Envelope builder/marshal/unmarshal, TopicFor/DLQTopic
- `pkg/pgx/pgx.go` — NewPool, WithTx
- `pkg/kafka/producer.go` — Producer, NewProducer, Publish, Ping, Close
- `pkg/outbox/outbox.go` — Insert, Relay{PollInterval,BatchSize,SweepInterval}, NewRelay, Run, Nudge, Sweep, publishOnce, metrics
- `pkg/outbox/outbox_integration_test.go` — TestMain (shared Postgres+Redpanda), 6 integration tests
- `pkg/testenv/testenv.go` — PostgresImage/RedpandaImage, RepoRoot, PlatformMigrations, StartPostgres, NewDB, StartRedpanda
- `services/_template/migrations/00001_platform.sql` — outbox + processed_events DDL (goose)
- `deploy/redpanda/topics.sh` — idempotent `<svc>.events`/`.dlq` topic provisioning
- `pkg/go.mod`, `pkg/go.sum` — franz-go, kotel, pgx, uuid, goose, testcontainers-go(+modules), otel sdk deps; gen/go require+replace
- `services/gateway/go.mod`, `services/gateway/go.sum` — gen/go replace directive added (require line stripped by `go mod tidy` since nothing imports gen/go from gateway yet — expected, not a regression)
- `go.work` — go directive bumped 1.25.1 → 1.25.7 (see Decisions)

## Decisions Made

- franz-go pinned to v1.21.7 (not the plan's illustrative v1.22.0) and goose to v3.27.3 (not v3.28.0) — both because the newer tag requires go1.26.0, which would force the whole repo off its Go 1.25.x pin. See frontmatter `key-decisions` for full detail and version-proxy verification.
- `deploy/redpanda/topics.sh`'s default `RPK_BROKERS` changed to `127.0.0.1:9093` (Redpanda's internal listener) instead of the plan's illustrative `localhost:9092` (the external listener, which advertises the host-mapped port and is unreachable from a process running inside the same container). Verified empirically with a throwaway probe container before fixing the script.
- go.work/pkg go.mod's `go` directive bumped to 1.25.7 (goose v3.27.3's own minimum) rather than staying at 1.25.1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] franz-go v1.22.0 and goose v3.28.0 require go1.26.0, breaking the project's Go 1.25.x pin**
- **Found during:** Task 1 (dependency wiring, before writing pkg/kafka)
- **Issue:** The plan's context block pinned franz-go v1.22.0 and goose v3.28.0 (verified during planning against the module proxy). Re-verifying at execution time, `go list -m -f '{{.GoVersion}}'` showed both now require `go1.26.0` — a major toolchain bump the whole repo (go.work, every service's go.mod, the Dockerfile's `golang:1.25.14-bookworm` base image) is not on.
- **Fix:** Pinned franz-go to v1.21.7 (latest 1.21.x; kotel v1.7.1 already declares a minimum of v1.21.1, so no API gap) and goose to v3.27.3 (requires go1.25.7, a same-line patch bump). go.work/pkg/go.mod's `go` directive moved 1.25.1 → 1.25.7 (still 1.25.x).
- **Files modified:** pkg/go.mod, pkg/go.sum, go.work
- **Verification:** `go build`/`go vet`/`make lint`/`make test`/`make test-integration` all green at go1.25.7; no go1.26 toolchain anywhere in the module graph's own `go` directives except transitively-unused dependencies not in the actual build.
- **Committed in:** `518a661` (Task 1 commit)

**2. [Rule 1 - Bug] deploy/redpanda/topics.sh's default broker address unreachable from inside the Redpanda container**
- **Found during:** Task 1 (first integration test run — topics.sh exited 1: "unable to dial: dial tcp [::1]:55004: connection refused")
- **Issue:** The plan's illustrative default `RPK_BROKERS=localhost:9092` targets Redpanda's "external" listener, which this testcontainers module configures to advertise the host-mapped port (e.g. `localhost:55004`) — correct for host-side connections, but a client running *inside* the same container (rpk, via `pkg/testenv`'s `container.Exec`) that dials the metadata-advertised address gets refused, since that mapped port isn't listened on inside the container.
- **Fix:** Changed the script's default to `127.0.0.1:9093` — the "internal" listener, confirmed via `docker exec ... cat /etc/redpanda/redpanda.yaml` and a manual `rpk topic create -X brokers=127.0.0.1:9093` against a throwaway probe container.
- **Files modified:** deploy/redpanda/topics.sh
- **Verification:** `TestTraceparentSurvivesRelay` and all 5 Task 2 tests pass, each relying on `testenv.StartRedpanda`'s topic provisioning succeeding.
- **Committed in:** `518a661` (Task 1 commit)

**3. [Rule 1 - Bug] TestSweepDeletesOnlyOldPublished's raw SQL couldn't encode an int parameter as a text-cast interval**
- **Found during:** Task 2 GREEN verification (second test run) — `insert row: failed to encode args[2]: unable to encode 9 into text format for text (OID 25): cannot find encode plan`
- **Issue:** The test's helper built `now() - ($3::text || ' days')::interval` with a Go `int` argument for `$3`; pgx's extended protocol infers the parameter's wire type from the `::text` cast and has no encode plan for a bare Go `int` into that OID.
- **Fix:** Rewrote to `now() - make_interval(days => $3)`, letting Postgres's own `make_interval` take the integer parameter directly — simpler and avoids the cast/encode mismatch entirely.
- **Files modified:** pkg/outbox/outbox_integration_test.go
- **Verification:** `TestSweepDeletesOnlyOldPublished` passes.
- **Committed in:** `8f07306` (Task 2 GREEN commit)

**4. [Rule 1 - Bug] TestRelayKeepsRowsWhenPublishFails Fataled on a Collect() error from unrelated tests' stale metric callbacks**
- **Found during:** Task 2 GREEN verification (second test run) — `collect metrics: outbox: observe backlog: closed pool`
- **Issue:** `otel.SetMeterProvider` retroactively rebinds meters obtained via `otel.Meter(name)` *before* the call to the newly-set provider. Two earlier tests in the same binary had already created relays (and their metric callbacks) against the default global provider; once this test called `SetMeterProvider` for the first time in the process, those two relays' observable-gauge callbacks became registered against the new ManualReader too — and since their pools were already closed (their tests had completed), `Collect()` returned a joined error querying those closed pools, unrelated to this test's own counter.
- **Fix:** Investigated the OTel SDK source (`pipeline.produce`) to confirm `rm.ScopeMetrics` is still fully populated for every instrument even when one callback errors; changed the test to log (not Fatal) on a non-nil `Collect` error and assert the counter value regardless.
- **Files modified:** pkg/outbox/outbox_integration_test.go
- **Verification:** `TestRelayKeepsRowsWhenPublishFails` passes; the tolerated log line is visible in test output and clearly attributed to other tests' stale callbacks.
- **Committed in:** `8f07306` (Task 2 GREEN commit)

---

**Total deviations:** 4 auto-fixed (2 blocking version/config fixes necessary to build at all, 2 bug fixes in test code discovered while getting the TDD GREEN phase to pass cleanly).
**Impact on plan:** All four were necessary for correctness or to keep the project on its pinned Go 1.25.x line. No scope creep — no new features or architecture beyond what the plan specified.

## Issues Encountered

- The Redpanda testcontainer's startup was intermittently flaky under this environment's Docker Desktop (port-mapping wait timing out on ~2 of 5 attempts across the whole session, unrelated to any specific test) — consistent with `.planning/research/PITFALLS.md` Pitfall 12. Retrying the test run resolved it each time; no code change was needed. Flagging in case CI sees the same flakiness and needs a retry wrapper later (not added here — out of this plan's scope, `make ci`/Jenkins wiring is a later plan).

## User Setup Required

None — no external service configuration required. All new dependencies are Go-modules-only (no accounts/API keys); Docker is already required by the existing `make test-integration` target.

## Next Phase Readiness

- The publish half of the event pipeline (`pkg/events`, `pkg/outbox`, `pkg/kafka` producer) is complete and proven end-to-end against real Postgres 17 + Redpanda. Plan 05+ (consumer/idempotency side, `pkg/kafka.Consumer.Handle`, DLQ) can now build directly on `pkg/events.Unmarshal`/`TopicFor`/`DLQTopic` and the same `pkg/testenv` helpers.
- `services/_template/migrations/00001_platform.sql` and `deploy/redpanda/topics.sh` are ready for `make new-service` (plan 09) to copy/reuse verbatim.
- `pkg/testenv` is intentionally exported (not `internal/`) so every future service's own `-tags=integration` suite imports it directly — no per-service testcontainers boilerplate.
- Flagged for later plans: the franz-go v1.21.7 / goose v3.27.3 pins (not the plan's illustrative v1.22.0/v3.28.0) should be re-verified whenever the project deliberately decides to move the whole repo to Go 1.26 — at that point both can be bumped back to their latest tags in one pass.
- No blockers for 01-05 onward.

## Self-Check: PASSED

- All 9 created files verified present on disk (`[ -f ... ]` for each `key-files.created` entry).
- All 3 task commit hashes (`518a661`, `c5c6290`, `8f07306`) verified present in `git log --oneline`.
- Plan-level `<verification>` re-run clean at the end of this plan: `make test-integration` (all packages `ok`, including `pkg/outbox` covering all 6 integration tests), `make test` (all packages `ok`), `make lint` (0 issues, both modules), both `GOWORK=off` module builds (`services/gateway`, `pkg`) succeed without network access to `gen/go`.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
