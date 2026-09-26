---
phase: 01-platform-foundation
plan: 07
subsystem: events
tags: [kafka, franz-go, kotel, idempotency, dlq, otel, testcontainers]

requires:
  - phase: 01-04
    provides: "pkg/events (Envelope, TopicFor/DLQTopic), pkg/kafka.Producer, pkg/pgx.WithTx, pkg/testenv, services/_template/migrations/00001_platform.sql (processed_events table)"
provides:
  - "pkg/kafka: Consumer.Handle(eventType, HandlerFunc) — idempotent (processed_events insert-on-conflict in the handler tx), manual offset commit only after that tx commits (D-13)"
  - "pkg/kafka: bounded in-process retries (1s/5s/25s default) then DLQ with full failure metadata headers, never a silent drop (D-12)"
  - "pkg/kafka: trace continuity across the Kafka hop via kotel WithProcessSpan; span + log attrs limited to event_id/event_type/aggregate_id (D-45, D-52)"
  - "pkg/kafka: kafka.consumer.processed / kafka.consumer.lag / dlq OTel metrics (D-50)"
  - "pkg/kafka/producer.go: unexported produceRecord, reused by both Publish and the Consumer's DLQ path"
affects: [01-08, 01-09, 01-10, 01-11, 01-12, 01-13]

actuals:
  tokens: 8300
  tasks: 2
  commits: 1
  plan_head_before: 0c960c0fc6fe9db1bbc6fe884e797c5c5a9f07ad

tech-stack:
  added: []
  patterns:
    - "Consumer.processRecord always calls tracer.WithProcessSpan(rec) before dispatch, so a span+log line exists for every fetched record whether it's applied, a duplicate, skipped (no handler), or DLQ'd — the `result` attribute on kafka.consumer.processed distinguishes the four outcomes"
    - "kgo.BlockRebalanceOnPoll() requires AllowRebalance() to be called on EVERY PollFetches iteration, including the one that returns immediately with a ctx-cancellation fake fetch and no real records — kgo still adds a poller for that call. Skipping AllowRebalance on the ctx-done early-return path leaves the poller count permanently non-zero and deadlocks Client.Close()'s graceful group-leave forever (see Deviations)"
    - "Backoff retry sleeps select on the outer (cancellable) Run ctx, while the actual DB apply and DLQ produce use context.WithoutCancel of that ctx — so an in-flight apply/DLQ-write always finishes, but a consumer shutdown during backoff abandons the record uncommitted for clean redelivery (D-40)"
    - "DLQ records are produced verbatim (original Key/Value/Headers) plus 7 appended metadata headers, through Producer's new unexported produceRecord — Publish and the DLQ path share the same ProduceSync/error-wrap code, no duplication"

key-files:
  created:
    - pkg/kafka/consumer.go
    - pkg/kafka/consumer_integration_test.go
  modified:
    - pkg/kafka/producer.go
    - pkg/go.mod
    - pkg/go.sum

key-decisions:
  - "Wrote both tasks' full implementation (idempotent apply in Task 1, retries/DLQ/lag in Task 2) in a single pass before the first commit, since they share the same Run/processRecord control flow and splitting the diff would have meant writing then immediately rewriting the same functions. Both tasks' acceptance criteria and tests were still verified independently and in the order the plan specifies (Task 1's tracer verify re-run before starting Task 2's behavior); see Deviations for why there is one commit instead of two."
  - "Unmarshal failures (corrupt record bytes) are logged at ERROR and committed past rather than retried/DLQ'd — our own producer never emits malformed envelopes, so this path only exists for genuinely unparseable data that retrying cannot fix; not covered by a test since it's not part of this plan's must_haves."
  - "DLQ-produce-retry reuses the same Backoff slice (holding at the last configured delay once exhausted) rather than a second independent backoff schedule — the plan specifies 'retry it with the same backoff until ctx is done', and a second knob for a scenario with no test coverage would be unrequested complexity."

patterns-established:
  - "Every kafka.Consumer instance is constructed with NewConsumer(brokers, group, pool, dlqProducer, log) and registers handlers via Handle(eventType, fn) before calling Run(ctx) — services never touch kgo directly (PLAT-02), matching the existing pkg/kafka.Producer/pkg/outbox convention."

requirements-completed: [PLAT-05, PLAT-07, PLAT-06, PLAT-02]

coverage:
  - id: D1
    description: "Consumer.Handle(eventType, fn) applies fn exactly once per unique event_id — a duplicate delivery of the same event_id is a no-op — inside one Postgres tx with processed_events, and manual offset commit happens only after that tx commits"
    requirement: "PLAT-05"
    verification:
      - kind: integration
        ref: "pkg/kafka/consumer_integration_test.go#TestHandleAppliesOnce (go test -tags=integration ./pkg/kafka/...)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The consumer's process span continues the originating producer span's trace id across the Kafka hop (kotel WithProcessSpan, D-06)"
    requirement: "PLAT-06"
    verification:
      - kind: integration
        ref: "pkg/kafka/consumer_integration_test.go#TestHandleAppliesOnce (trace-id assertion against a SpanRecorder)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A handler that fails is retried in-process 3 times (1s/5s/25s default backoff, shortened in tests) then the original envelope is produced to <topic>.dlq with error/consumer_group/attempts/failed_at/source_topic/source_partition/source_offset headers, an ERROR log line, and a dlq counter increment; the source offset commits so the partition is not blocked"
    requirement: "PLAT-07"
    verification:
      - kind: integration
        ref: "pkg/kafka/consumer_integration_test.go#TestFailedHandlerLandsInDLQAfter3Retries"
        status: pass
    human_judgment: false
  - id: D4
    description: "If the consumer stops before committing an offset (mid-backoff or otherwise), the record is redelivered to the next group member and applied exactly once"
    requirement: "PLAT-07"
    verification:
      - kind: integration
        ref: "pkg/kafka/consumer_integration_test.go#TestUncommittedRecordIsRedelivered"
        status: pass
    human_judgment: false
  - id: D5
    description: "A consumer with no registered handlers starts no Kafka client and returns cleanly on shutdown; topics are derived from registered event types via events.TopicFor"
    verification:
      - kind: unit
        ref: "code inspection — Consumer.Run's len(topics)==0 branch; no dedicated test (not in must_haves' artifact/test list, trivial branch)"
        status: unknown
    human_judgment: true
    rationale: "Plan's must_haves list this as a truth but names only the three tests above as required artifacts — no test targets the no-handlers branch specifically. Low risk (single early-return guard, same shape as pkg/outbox's analogous checks) but not proven by an automated test in this plan."
  - id: D6
    description: "make test-integration and make lint stay green across the whole repo with the new consumer wired in"
    verification:
      - kind: other
        ref: "make test-integration && make lint && make test (all pass) plus standalone GOWORK=off builds for services/gateway and pkg"
        status: pass
    human_judgment: false

duration: ~24min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 7: Idempotent Kafka Consumer with Retries and DLQ Summary

**`pkg/kafka.Consumer.Handle` applies every event exactly once inside one Postgres tx with `processed_events`, commits offsets only after that tx, retries a failing handler three times (1s/5s/25s) then routes to `<topic>.dlq` with full failure metadata, and continues the producer's trace across the Kafka hop — proven by three integration tests against real Postgres + Redpanda.**

## Performance

- **Duration:** ~24 min
- **Completed:** 2026-09-26T04:55Z
- **Tasks:** 2
- **Files created:** 2, modified: 3

## Accomplishments

- `pkg/kafka.Consumer`: `HandlerFunc`, `NewConsumer`, `Handle`, `Topics`, `Run` — the consume half of the event pipeline, mirroring `pkg/outbox`'s producer-side conventions
- Idempotency by construction (D-13): every dispatch opens one tx, does `insert into processed_events (...) on conflict do nothing`, skips the handler on 0 rows affected (duplicate), and only commits the Kafka offset after that tx commits — services can no longer forget the check
- Bounded retries + DLQ (D-12, D-50): 4 total attempts (1 + 3 retries, default 1s/5s/25s backoff), then the original envelope bytes are produced verbatim to `<topic>.dlq` with `error`, `consumer_group`, `attempts`, `failed_at`, `source_topic`, `source_partition`, `source_offset` headers, an ERROR log line, and a `dlq` counter increment — the source offset still commits so the partition is never blocked by a poison message
- `kafka.consumer.processed` (attrs `event_type`, `result` ∈ applied/duplicate/skipped/dlq) and `kafka.consumer.lag` (attrs `topic`, `partition`) OTel metrics, both synchronous instruments (no observable-gauge callback cross-test contamination)
- Trace continuity (D-06, PLAT-06): `kotel.Tracer.WithProcessSpan` starts every record's processing span as a child of the fetch-extracted trace context, so the consumer's span carries the same trace id as the originating producer span
- `pkg/kafka/producer.go` gained unexported `produceRecord`, shared by `Publish` and the Consumer's DLQ path (no duplicated `ProduceSync`/error-wrap logic)

## Task Commits

Both tasks' full implementation (idempotent apply, retries, DLQ, lag, trace continuity) was written and verified together before the first commit — see Deviations for why.

1. **Task 1 + Task 2: Consumer.Handle with idempotency, retries, DLQ, metrics, trace continuity** - `0484cd1` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS update)

## Files Created/Modified

- `pkg/kafka/consumer.go` — `HandlerFunc`, `Consumer{Backoff}`, `NewConsumer`, `Handle`, `Topics`, `Run`, `processRecord`, `apply`, `sendToDLQ`, `dlqRetryDelay`, `observeLag`
- `pkg/kafka/consumer_integration_test.go` — `TestMain` (shared Postgres+Redpanda for this package), `TestHandleAppliesOnce`, `TestFailedHandlerLandsInDLQAfter3Retries`, `TestUncommittedRecordIsRedelivered`, plus `waitFor`/`consumeMatching`/`headerValue`/`counterValue` test helpers
- `pkg/kafka/producer.go` — extracted unexported `produceRecord`; `Publish` now delegates to it
- `pkg/go.mod`, `pkg/go.sum` — `go mod tidy` promoted `go.opentelemetry.io/otel/metric`, `otel/trace`, `otel/sdk`, `otel/sdk/metric` from unlisted-but-satisfied to direct requires (already transitively used by this plan's and the prior plan's test code; pre-existing go.mod under-recording, not new debt introduced by this plan)

## Decisions Made

- Wrote both tasks' implementation in one pass (see key-decisions in frontmatter for the full rationale) — verified independently per task before committing once.
- Unmarshal failures on corrupt record bytes: logged ERROR and committed past (not retried/DLQ'd) since our own producer never emits malformed envelopes and retrying can't fix corrupt bytes. Not covered by a dedicated test — out of this plan's must_haves.
- DLQ-produce-retry reuses the same `Backoff` slice, holding at the last configured delay once exhausted, rather than a second independent backoff knob.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `Run` deadlocked on shutdown: `Client.Close()` never returned after any consumer cancellation**
- **Found during:** Task 1 (first `TestHandleAppliesOnce` run) — the test hung until its own 10s/60s wait timed out; a `SIGQUIT`-style goroutine dump showed the `Close()` call's `LeaveGroupContext` blocked forever in `waitAndAddRebalanceMaybeSignal`, waiting for kgo's internal per-client "poller count" to reach zero.
- **Issue:** `kgo.BlockRebalanceOnPoll()` makes every `PollFetches` call add to that poller count — **including the call that returns immediately with a synthetic ctx-cancellation fetch and no real records** (confirmed by reading franz-go's `consumer.go` source: "We still need to add a poller to block rebalances if configured, since we are returning a fetch"). The original code's `if ctx.Err() != nil { return nil }` right after `PollFetches` skipped the loop's `cl.AllowRebalance()` call on that exact path, so the poller count added by the ctx-cancelled poll was never released. `Client.Close()`'s graceful group-leave needs that count at zero and blocked forever — meaning **every** `Run(ctx)` call would hang on shutdown once ctx was cancelled, not just in tests.
- **Fix:** Restructured `Run`'s loop to always call `cl.AllowRebalance()` exactly once per iteration regardless of outcome — a `ctxDone` flag now gates *which further work* happens (skip `EachRecord` dispatch, skip logging the synthetic fetch error) but never gates whether `AllowRebalance()` runs.
- **Files modified:** pkg/kafka/consumer.go
- **Verification:** `TestHandleAppliesOnce`, `TestFailedHandlerLandsInDLQAfter3Retries`, `TestUncommittedRecordIsRedelivered` all pass, each of which cancels a `Run(ctx)` call and waits for it to return within a bounded timeout.
- **Committed in:** `0484cd1` (the fix was made before this commit, so it's already included — not a separate commit)

**2. [Rule 1 - Bug] Synthetic ctx-cancellation fetch logged as a misleading ERROR on every clean shutdown**
- **Found during:** Task 1, same investigation as above — once the deadlock was fixed, every clean `Run` shutdown logged `level=ERROR msg="kafka: fetch error" topic="" partition=-1 error="context canceled"`, which is not a real fetch failure (D-52's "ERROR = DLQ/5xx" convention would make every routine shutdown look like an incident).
- **Fix:** Skip the fetch-errors logging loop entirely when the poll returned because ctx was already done — that's the only case producing this synthetic zero-topic/negative-partition fetch error.
- **Files modified:** pkg/kafka/consumer.go
- **Verification:** Re-ran all three tests; no ERROR lines appear on normal shutdown, only on the genuine DLQ path (`TestFailedHandlerLandsInDLQAfter3Retries`'s expected "event sent to dlq" ERROR line).
- **Committed in:** `0484cd1`

---

**Total deviations:** 2 auto-fixed (1 blocking-shutdown bug found before any commit, 1 misleading-log bug found in the same investigation).
**Impact on plan:** Both were necessary for correctness (a consumer that can never shut down is not shippable) and log hygiene (D-52). No scope creep — no new features or architecture beyond what the plan specified.

## Issues Encountered

- The kgo `BlockRebalanceOnPoll` + ctx-cancellation interaction above is undocumented in franz-go's public docs (only discoverable by reading `pkg/kgo/consumer.go`'s source and a goroutine dump); flagging here since every future service's consumer inherits this exact `Run` loop from `pkg/kafka`, so the fix only needed to happen once.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The full event pipeline (`pkg/events`, `pkg/outbox`, `pkg/kafka.Producer`, `pkg/kafka.Consumer`) is now complete and proven end-to-end against real Postgres 17 + Redpanda: publish (01-04) and idempotent consume with retries/DLQ (this plan) both green.
- Every future service's Kafka consumer builds directly on `kafka.NewConsumer`/`Handle` — idempotency, commit ordering, retries, and DLQ are enforced by construction, not by convention.
- No blockers for 01-08 onward.

## Self-Check: PASSED

- Both created files verified present on disk: `pkg/kafka/consumer.go`, `pkg/kafka/consumer_integration_test.go`.
- Commit hash `0484cd1` verified present in `git log --oneline`.
- Plan-level `<verification>` re-run clean at the end of this plan: all three consumer integration tests pass (`go test -tags=integration -run 'TestFailedHandlerLandsInDLQAfter3Retries|TestUncommittedRecordIsRedelivered|TestHandleAppliesOnce' ./pkg/kafka/...`), `make test-integration` (all packages `ok`, including the pre-existing `pkg/outbox` suite), `make test`, `make lint` (0 issues across both modules, buf lint, PII check), and standalone `GOWORK=off` builds for `services/gateway` and `pkg`.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*
