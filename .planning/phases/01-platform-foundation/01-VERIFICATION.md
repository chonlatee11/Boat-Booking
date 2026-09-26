---
phase: 01-platform-foundation
verified: 2026-09-26T07:49:52Z
status: human_needed
score: 5/6 must-haves verified
covered_files:
  - ".planning/REQUIREMENTS.md"
  - ".planning/ROADMAP.md"
  - ".planning/phases/01-platform-foundation/01-01-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-01-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-02-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-02-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-03-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-03-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-04-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-04-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-05-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-05-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-06-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-06-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-07-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-07-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-08-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-08-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-09-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-09-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-10-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-10-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-11-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-11-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-12-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-12-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-13-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-13-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-REVIEW.md"
covered_digest: "v1:sha256:60c888207e9a50d13268d4b59fa303d7be4280b079ce64131828d70013af69a2"
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "Jenkins CI builds every service image and runs unit + integration tests on every push (PLAT-08)"
    test: "Push a commit under pkg/ (should image ALL services) and, separately, a commit only under services/schedule/ (should image ONLY schedule); watch the boat-booking multibranch job pick each up within its 2-minute scan"
    expected: "Both builds run Lint/Proto/Unit/Integration/Migrations/Template/Web stages; the pkg/ build's Images stage builds template+catalog+gateway+schedule; the schedule-only build's Images stage builds only schedule; neither pushes to Harbor unless the branch is main"
    why_human: "Requires a live Jenkins reacting to two real pushes over real scan cycles (per 01-13-PLAN.md's own deferred human-check); the verifier confirmed the selector logic (deploy/ci/changed-services_test.sh, 13/13 PASS) and that Jenkinsfile stages mirror `make ci`, but did not have Jenkins credentials to trigger/observe a live build in this session"
human_verification:
  - test: "Push a commit under pkg/ then a commit only under services/schedule/, and watch two Jenkins multibranch scan cycles"
    expected: "First build images all 4 services, second images only schedule; both run every non-image stage; neither pushes off main"
    why_human: "Live CI system behavior, no credentials available to the verifier in this session"
  - test: "With the stack up after `make proof`, open http://localhost:3000 (Grafana) -> Explore -> Tempo, search `{ span.aggregate_id = \"<boat id printed by make proof>\" }`, open the trace; then open dashboard \"platform\""
    expected: "One trace shows gateway HTTP span -> connect call to catalog -> Kafka publish span -> schedule consumer span, same trace id, no gap at the Kafka hop; \"Logs for this span\" jumps to the matching Loki lines; platform dashboard shows HTTP rate/consumer lag/processed-events/outbox-backlog panels with real data"
    why_human: "Trace continuity across the Kafka hop and dashboard usefulness are visual judgements; `make proof` and `make obs-check` already assert the underlying data exists programmatically (both PASS, verified independently in this session)"
  - test: "Open http://localhost:3001 in a 375px-wide viewport, then /en; press refresh; open devtools Network"
    expected: "/ redirects to /th; boat cards list the proof boat; Thai renders in IBM Plex Sans Thai (no system-font fallback); /en shows English strings; every XHR goes to localhost:8000 (Kong) not directly to gateway/catalog; refresh refetches"
    why_human: "Font rendering and mobile layout need a browser; the verifier confirmed by curl that /, /th, /en all return correct status codes, Thai copy is present, and the compiled CSS embeds \"IBM Plex Sans Thai\", but visual layout/no-fallback-font judgement needs a real viewport"
---

# Phase 1: Platform Foundation Verification Report

**Phase Goal:** A developer can scaffold a new service from a shared template, run the full local stack, and see one real event flow end-to-end with cross-service tracing
**Verified:** 2026-09-26T07:49:52Z
**Status:** human_needed
**Re-verification:** No — initial verification

**Note on ROADMAP `mode: mvp`:** ROADMAP.md tags Phase 1 `Mode: mvp`, but the phase goal is not in `As a ..., I want ..., so that ....` User Story form (`user-story.validate` returns `valid: false`), and this is a pure platform/infra-enablement phase, not a user-facing vertical slice. All 5 milestone phases carry the same `mode: mvp` tag, suggesting a blanket default rather than a deliberate SPIDR-split for Phase 1. Rather than refuse verification outright (which would leave the phase with no report, contradicting the explicit verification request and the well-specified numbered Success Criteria already in ROADMAP.md), this report proceeds with standard goal-backward verification against ROADMAP's Success Criteria + PLAN `must_haves`. Recommend clearing `mode: mvp` for Phase 1 in ROADMAP.md, or accepting this as an intentional non-MVP infra phase.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `make new-service <name>` scaffolds a working service (`cmd/`, `internal/{domain,app,adapters}`, `migrations/`, `CLAUDE.md`, `/healthz`+`/readyz`, root-`Dockerfile` image) wired to shared `pkg/*` (PLAT-01) | VERIFIED | Ran `make template-smoke` independently: scaffolded `tsmoke`, built the image from the root `Dockerfile`, container reported Docker health `healthy` via `-healthcheck`, then cleaned up (`PASS template-smoke`, git status clean afterward). `catalog`/`schedule` in the repo are real prior products of this same path. |
| 2 | `make up` starts the full stack (Redpanda, Postgres 17, Valkey, Kong 3.9.1 DB-less, Grafana Tempo/Loki/Prometheus); `make proto-gen` generates committed Go+TS clients (PLAT-02, PLAT-03, PLAT-04) | VERIFIED | `docker ps` shows all 14 `boatbooking-*` containers `Up`/`healthy` (postgres, redpanda, valkey, kong, grafana, loki, prometheus, tempo, otel-collector, catalog, schedule, gateway, web). `deploy/postgres/isolation-check.sh` run directly: `PASS own-db-access`/`PASS cross-db-denied` both directions — proves database-per-service by Postgres grant, not convention. `git ls-files gen/` shows `gen/go/**/*.pb.go`, `gen/go/**/*connect.go`, `gen/ts/**/*_pb.ts` all committed. Grepped all `services/*/go.mod`+source: no direct `pgxpool.New` outside `pkg/pgx`, confirming `pkg/*` is the sole access path (PLAT-02). |
| 3 | A state change via transactional outbox is applied exactly once in another service, visible as one trace in Tempo with structured slog JSON logs carrying `trace_id` (PLAT-05, PLAT-06) | VERIFIED | Ran `make proof` independently (not trusting the SUMMARY claim): `PASS applied`, `PASS exactly-once`, `PASS public-list`, `PASS single-trace` (one Tempo trace spans catalog+gateway+schedule by `aggregate_id`), `PASS logs-correlated` (Loki holds the matching `trace_id`). Also ran `make obs-check` independently: `PASS traces`, `PASS logs`, `PASS metrics`. This directly falsifies the orchestrator's concern that 01-07's early hand-marked PLAT-05/06/07 checkboxes were unproven — the re-run in this session reproduces the same result from scratch. |
| 4 | Failed event processing retries 3x with backoff then lands in `<topic>.dlq` with error metadata; consumer offsets commit only after successful apply (PLAT-07) | VERIFIED | Ran the single named test directly against real testcontainers Postgres+Redpanda (not the full suite): `go test -tags=integration -run TestFailedHandlerLandsInDLQAfter3Retries -v ./kafka/...` → attempts 1,2,3 logged, then `"kafka: event sent to dlq" attempts=4`, `--- PASS (3.57s)`. Code confirms `kgo.DisableAutoCommit()` and manual `CommitRecords` only after DLQ-or-success. |
| 5 | Jenkins CI builds every service image and runs unit + integration tests (testcontainers) on every push (PLAT-08) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Jenkins + agent containers are `Up`/`healthy`. Independently ran `bash deploy/ci/changed-services_test.sh` — all 13 table-driven cases PASS (adjacency, dedup, sort, pkg/proto/gen/_template/go.work/Makefile/Dockerfile→ALL, docs-only→empty, web-only→empty). 01-REVIEW.md confirms Jenkinsfile stages mirror `make ci` and Push is gated to `branch 'main'`. However, the *live* claim — that a real push actually triggers a scan and produces the claimed image-scoping in a real Jenkins run — needs Jenkins credentials this session doesn't have; 01-13-PLAN.md itself defers exactly this to end-of-phase human-check (two live push/scan cycles). Selector logic is proven; live end-to-end CI behavior is not independently observed in this session. |
| 6 | Next.js skeleton (TH/EN i18n, Thai-friendly font, mobile-first) calls the backend only through Kong with a verified JWT round-trip (PLAT-09, PLAT-10) | VERIFIED | `curl http://localhost:3001/` → `307 -> /th`; `/th` and `/en` both `200` and both render "Boat"/"เรือ" strings; compiled CSS chunk contains `"IBM Plex Sans Thai"` (3 occurrences) confirming `next/font` embedding, not a fallback. `apps/web/src/components/boat-list.tsx` fetches only via `useQuery`+`apiFetch()` (Client Component). Ran `make kong-roundtrip` independently: `PASS no-token-401`, `PASS foreign-key-401`, `PASS expired-401`, `PASS refresh-kind-401`, `PASS valid-token-200`, `PASS cors-preflight`, `PASS ratelimit-header`, `PASS public-boats-200`, `PASS upsert-boat-201`, `PASS rate-limit-429` — the full curl→Kong→BFF→catalog round trip plus JWT edge cases (PLAT-10). |

**Score:** 5/6 truths verified (1 present + wired, live behavior not independently exercised)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/_template/` | Scaffold source template | VERIFIED | Full `cmd/internal/{domain,app,adapters}/migrations` present; used successfully by `make template-smoke`, `make new-service catalog`, `make new-service schedule` |
| `pkg/{auth,httpx,kafka,outbox,pgx,events,clock,money}` | Shared libraries, sole infra access path | VERIFIED | All present, unit-tested (`ok` for auth/clock/events/httpx/money/testenv), no direct bypass found in service code |
| `Dockerfile` (root) | Single multi-stage build, `ARG SERVICE` | VERIFIED | Used by `make template-smoke`, produced a working distroless image |
| `deploy/docker-compose.yml` + `deploy/compose/service.yml.tmpl` | Full stack composition | VERIFIED | `make up` stack running and healthy |
| `deploy/kong/kong.yml.tmpl`, `deploy/kong/roundtrip.sh` | Kong DB-less JWT gateway + acceptance script | VERIFIED | `make kong-roundtrip` all-PASS |
| `deploy/proof.sh` | Walking-skeleton event-flow proof | VERIFIED | `make proof` all-PASS, re-run independently |
| `deploy/observability/*` | OTel Collector → Tempo/Loki/Prometheus → Grafana | VERIFIED | `make obs-check` all-PASS |
| `deploy/ci/{changed-services.sh,changed-services_test.sh,smoke.sh}`, `Jenkinsfile` | CI selector + pipeline | VERIFIED (static) / ⚠️ live-behavior unverified | Selector unit tests pass; Jenkinsfile reviewed; live push-triggered build not observed this session (no Jenkins credentials) |
| `apps/web/` | Next.js 16 TH/EN skeleton | VERIFIED | Serving on :3001, i18n + font + Kong-only fetch confirmed |
| `pkg/kafka/consumer.go` | Exactly-once + retry/DLQ consumer | VERIFIED | `TestFailedHandlerLandsInDLQAfter3Retries` re-run and PASS; `TestHandleAppliesOnce`/`TestUncommittedRecordIsRedelivered` present |
| `pkg/outbox/outbox.go` | Transactional outbox relay | VERIFIED (happy path) / see anti-patterns | Relay proven working end-to-end via `make proof`; a real latent bug exists on the unmarshal-failure path (see CR-01 below) — does not contradict any stated must-have truth for this phase (all of which describe the publish-failure-stops-batch and happy-path behavior, not unmarshal-failure handling) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `services/catalog` UpsertBoat | `pkg/outbox.Insert` | same pgx tx | VERIFIED | Confirmed by `make proof`'s `PASS applied` + `PASS exactly-once` |
| `pkg/outbox` relay | Redpanda `catalog.events` | `Producer.Publish` w/ `traceparent` header | VERIFIED | `make proof`'s `PASS single-trace` shows the Kafka hop preserves the trace id |
| `services/schedule` consumer | `pkg/kafka.Consumer.Handle` | `processed_events` tx | VERIFIED | `make proof`'s `PASS exactly-once`; `TestBoatUpsertedAppliedOnce` referenced in 01-11-SUMMARY, template pattern re-proven live by `TestFailedHandlerLandsInDLQAfter3Retries` re-run |
| `services/gateway` BFF | `services/catalog` (connect-go) | `ForwardClaims` + otelconnect | VERIFIED | `make kong-roundtrip`'s `PASS upsert-boat-201`/`PASS public-boats-200` |
| `apps/web/src/components/boat-list.tsx` | Kong `:8000` | `apiFetch()`, `credentials: 'include'` | VERIFIED | grep confirms `apiFetch(` call site is the only network call; curl confirms Kong-fronted responses |
| `Jenkinsfile` stages | `Makefile` `ci` targets | 1:1 stage-per-target mirror | VERIFIED (static only) | Confirmed by code review (01-REVIEW.md file list) and Jenkinsfile inspection; live trigger not exercised this session |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full walking-skeleton event flow (PLAT-05/06) | `make proof` | `PASS applied/exactly-once/public-list/single-trace/logs-correlated` | PASS |
| Observability pipeline (PLAT-06) | `make obs-check` | `PASS traces/logs/metrics` | PASS |
| Kong JWT round trip + edge cases (PLAT-10) | `make kong-roundtrip` | 10/10 `PASS` lines | PASS |
| Retry→DLQ with real Redpanda+Postgres (PLAT-07) | `go test -tags=integration -run TestFailedHandlerLandsInDLQAfter3Retries -v ./kafka/...` (single named test, not full suite) | `--- PASS (3.57s)` | PASS |
| Service scaffold → build → healthcheck (PLAT-01) | `make template-smoke` | `PASS template-smoke`, repo left clean | PASS |
| DB-per-service isolation (PLAT-03) | `bash deploy/postgres/isolation-check.sh` | `PASS own-db-access` / `PASS cross-db-denied` both directions | PASS |
| CI selector edge cases (PLAT-08) | `bash deploy/ci/changed-services_test.sh` | 13/13 `PASS` | PASS |
| Unit test suite | `make test` | all packages `ok` or `[no test files]`, zero failures | PASS |
| Web i18n/font/Kong-only routing (PLAT-09) | `curl` against `:3001` (`/`, `/th`, `/en`, CSS chunk) | 200s, correct redirects, Thai text present, `IBM Plex Sans Thai` in compiled CSS | PASS |
| Live Jenkins push-triggered build (PLAT-08) | n/a — no Jenkins credentials this session | n/a | ? SKIP → human verification |
| `make lint` (golangci-lint) | `make lint` | `golangci-lint: command not found` (not installed in this shell — it *is* installed in the CI/dev-tools image per `make dev-tools`) | ? SKIP — environment gap, not a code gap |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PLAT-01 | 01-09, 01-10 | Scaffold service via `make new-service` | SATISFIED | `make template-smoke` re-run, PASS |
| PLAT-02 | 01-01, 01-04, 01-07, 01-09, 01-10 | Shared `pkg/*` sole access path | SATISFIED | grep confirms no bypass |
| PLAT-03 | 01-01, 01-12 | `make up` full stack | SATISFIED | `docker ps` healthy, isolation-check PASS |
| PLAT-04 | 01-02 | `make proto-gen` committed | SATISFIED | `git ls-files gen/` shows generated files tracked |
| PLAT-05 | 01-04, 01-10, 01-11, 01-12 | Exactly-once cross-service event | SATISFIED | `make proof` re-run, PASS |
| PLAT-06 | 01-04, 01-08, 01-12 | Single trace + structured logs | SATISFIED | `make proof` + `make obs-check` re-run, PASS |
| PLAT-07 | 01-07 | Retry 3x → DLQ, commit-after-apply | SATISFIED | Named integration test re-run, PASS |
| PLAT-08 | 01-06, 01-13 | Jenkins CI builds+tests every push | PARTIALLY SATISFIED — selector logic and pipeline mirror proven; live push-cycle behavior deferred to human (see human_verification) |
| PLAT-09 | 01-05 | Next.js TH/EN, Thai font, mobile-first, Kong-only | SATISFIED | curl checks re-run, PASS |
| PLAT-10 | 01-01, 01-12 | Kong JWT round trip | SATISFIED | `make kong-roundtrip` re-run, PASS |

No orphaned requirements found — all 10 PLAT-IDs declared across plans and present in REQUIREMENTS.md map 1:1.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `pkg/outbox/outbox.go` | 236-253 | `events.Unmarshal` failure `return`s an error out of `WithTx`, rolling back `published_at` for already-Kafka-published earlier rows in the same batch, and permanently head-of-line-blocks every row after the poison one (no skip/DLQ path) | Warning (already documented as CR-01 in 01-REVIEW.md, critical-rated by code review but latent — "our own producer never emits malformed envelopes" per the symmetric consumer-side comment) | Does not contradict any must-have truth stated for this phase (all outbox must-haves describe the happy path and the publish-failure-stops-batch case, not unmarshal-failure handling); real production risk if it ever fires, should be tracked as a fix, not a phase-1 blocker |
| `services/gateway/.../bff.go:78-114`, `services/catalog/.../routes.go:48-56` | — | `Role` claim carried/forwarded end-to-end but never checked before a write (WR-01 in 01-REVIEW.md) | Warning | No PLAT requirement asks for role-based authorization (that's AUTH-0x, Phase 2); flagged for Phase 2 awareness only |
| `pkg/httpx/errors.go:18-50` | — | `WriteError` status mapping incomplete for ~11 of 17 connect codes (WR-02) | Warning | Latent — no current handler emits an unmapped code |
| `deploy/postgres/init.sh:42-50` | — | Unescaped password interpolation into single-quoted SQL (WR-03) | Info | Operator-controlled value, not attacker-controlled |
| `Makefile:310-311`, `pkg/httpx/otel.go:91-115` | — | `echo`-piped password visible via `ps` (IN-01); hand-maintained span-attribute allowlist (IN-02) | Info | Cosmetic / fails-safe already |
| `deploy/postgres/isolation-check.sh` | — | Script exists and works (re-run and PASS in this session) but has no `make` target wired to it | Info | Not exercised automatically by `make ci`/`make proof`; doesn't block the phase goal since it was runnable directly and passed |

No `TBD`/`FIXME`/`XXX` debt markers found anywhere under `pkg/`, `services/`, `apps/web/src`, or `deploy/`.

### Human Verification Required

### 1. Live Jenkins two-push-cycle CI scoping

**Test:** Push a commit under `pkg/`, wait for the `boat-booking` multibranch job's 2-minute scan; then push a commit only under `services/schedule/` and wait again.
**Expected:** First build's Images stage builds `template`, `catalog`, `gateway`, `schedule`; second build's Images stage builds only `schedule`; both builds run every non-image stage (Lint/Proto/Unit/Integration/Migrations/Template/Web); neither pushes to Harbor unless the branch is `main`.
**Why human:** Needs a live Jenkins reacting to real pushes over two real scan cycles — the verifier confirmed the selector's *logic* (`changed-services_test.sh`, 13/13 PASS) and that the Jenkinsfile's stages mirror `make ci`, but had no Jenkins credentials to trigger/observe an actual build this session. (Deferred from `checkpoint:human-verify` in 01-13-PLAN.md.)

### 2. Grafana trace continuity across the Kafka hop

**Test:** With the stack up after `make proof`, open Grafana (`:3000`) → Explore → Tempo, search `{ span.aggregate_id = "<boat id from make proof output>" }`, open the trace; then open the "platform" dashboard.
**Expected:** One trace shows gateway HTTP span → connect call to catalog → Kafka publish span → schedule consumer span, all one trace id, no gap at the Kafka hop; "Logs for this span" jumps to matching Loki lines; the platform dashboard's panels (HTTP rate, consumer lag, processed events, outbox backlog) show real data.
**Why human:** Trace continuity across the Kafka hop and dashboard usefulness are visual judgements. `make proof` (`PASS single-trace`) and `make obs-check` (`PASS traces/logs/metrics`) already assert the underlying data exists programmatically — this item is about the *visual* presentation, not the data's existence. (Deferred from 01-12-PLAN.md.)

### 3. Mobile viewport + font + network origin

**Test:** Open `http://localhost:3001` in a 375px-wide viewport, then `/en`; press refresh; open devtools Network.
**Expected:** `/` redirects to `/th`; boat cards list the proof boat; Thai renders in IBM Plex Sans Thai with no system-font fallback; `/en` shows English strings; every XHR goes to `localhost:8000` (Kong), never directly to gateway/catalog; refresh refetches.
**Why human:** Font rendering and mobile layout need a real browser. The verifier confirmed by `curl` that `/`, `/th`, `/en` all return correct status codes/redirects, Thai copy is present, and the compiled CSS embeds `"IBM Plex Sans Thai"` — but "no fallback" and mobile-layout correctness are visual judgements. (Deferred from 01-05-PLAN.md.)

### Gaps Summary

No gaps found. Every roadmap Success Criterion and every PLAT-01..10 requirement has direct, independently-reproduced evidence in this session (not just SUMMARY claims) — `make proof`, `make obs-check`, `make kong-roundtrip`, `make template-smoke`, `make test`, the isolation-check script, the changed-services selector tests, and one named integration test (`TestFailedHandlerLandsInDLQAfter3Retries`) were all re-run from scratch and passed. The orchestrator's specific concern — that 01-07 hand-marked PLAT-05/06/07 before their proof existed — is resolved: this verification independently re-derived the same PASS results 01-12's `make proof` claims, so the checkbox is now backed by evidence gathered in this session, not just trust in the SUMMARY.

The only thing keeping this out of a clean `passed` is that the live, multi-push Jenkins CI behavior (PLAT-08's "on every push" claim) and two purely-visual checks (Grafana trace continuity, mobile font/layout) require a human with Jenkins credentials and a browser — routed to `human_needed` per the standard decision tree, not because anything failed.

One genuine, already-documented code defect (CR-01, outbox relay unmarshal-failure rollback) remains unresolved; it does not block this phase's goal (the happy path is proven end-to-end) but should be fixed before the outbox carries production traffic with any possibility of malformed payloads.

---

_Verified: 2026-09-26T07:49:52Z_
_Verifier: Claude (gsd-verifier)_
