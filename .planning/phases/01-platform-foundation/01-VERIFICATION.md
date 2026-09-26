---
phase: 01-platform-foundation
verified: 2026-09-26T19:15:00Z
status: passed
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
  - ".planning/phases/01-platform-foundation/01-14-PLAN.md"
  - ".planning/phases/01-platform-foundation/01-14-SUMMARY.md"
  - ".planning/phases/01-platform-foundation/01-REVIEW-FIX.md"
  - ".planning/phases/01-platform-foundation/01-REVIEW.md"
  - ".planning/phases/01-platform-foundation/01-UAT.md"
  - "deploy/observability/grafana/provisioning/datasources/datasources.yaml"
  - "pkg/outbox/outbox.go"

covered_digest: "v1:sha256:4584de7d9a6ca0105213c09388e1ac296c490772da12fefc7c163b7f2a1016be"
behavior_unverified: 1
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 5/6
  gaps_closed:
    - "G-01-7 (UAT): Grafana 'Logs for this span' returned empty for inner spans (e.g. catalog UpsertBoat) because Tempo's tracesToLogsV2 had no time shift, so the Loki query window equalled the exact span duration, which ends before the outer request log line is written. Fixed by 01-14 (spanStartTimeShift: '-1m', spanEndTimeShift: '1m', filterByTraceID unchanged)."
  gaps_remaining: []
  regressions: []
behavior_unverified_items:

  - truth: "Jenkins CI builds every service image and runs unit + integration tests on every push, with per-commit changed-service image scoping (PLAT-08)"
    test: "After phase 1 merges to main, push a commit under pkg/ (should image ALL services) and, separately, a commit only under services/schedule/ (should image ONLY schedule); watch the boat-booking multibranch job pick each up"
    expected: "Both builds run every non-image stage; the pkg/ build's Images stage builds template+catalog+gateway+schedule; the schedule-only build's Images stage builds only schedule; neither pushes to Harbor unless the branch is main"
    why_human: "01-UAT.md test 8 ran this live on a throwaway non-main branch: the build went green, every non-image stage ran, and the Images stage built all 4 services (select-all), Push correctly skipped. But non-main builds diff against origin/main, which is docs-only until this phase merges, so every non-main branch build selects all services — the per-commit scoped case (schedule-only change -> only schedule imaged) is structurally impossible to exercise before the merge. UAT itself concluded 'Re-check on main after merge (base = GIT_PREVIOUS_SUCCESSFUL_COMMIT)', which needs a human with Jenkins access post-merge."
human_verification:

  - test: "Push a commit under pkg/ then, after merging this phase to main, a commit only under services/schedule/, and watch two Jenkins multibranch scan cycles"
    expected: "First build images all 4 services (already observed live in UAT test 8, off a throwaway branch); second build (on/after main, diffing GIT_PREVIOUS_SUCCESSFUL_COMMIT) images only schedule; neither pushes off main"
    why_human: "Per-commit scoping cannot be exercised pre-merge (see behavior_unverified_items above) — this is the one remaining live-CI check, deferred structurally rather than skipped"
---

# Phase 1: Platform Foundation Verification Report

**Phase Goal:** A developer can scaffold a new service from a shared template, run the full local stack, and see one real event flow end-to-end with cross-service tracing
**Verified:** 2026-09-26T19:15:00Z
**Status:** passed (UAT test 8 confirmed live by user 2026-09-26)
**Re-verification:** Yes — after gap closure (UAT gap G-01-7, closed by plan 01-14)

## Re-Verification Context

This is a second verification pass. The first verification (2026-09-26T07:49:52Z) scored 5/6 truths VERIFIED with 1 ⚠️ PRESENT_BEHAVIOR_UNVERIFIED (live Jenkins CI behavior) and routed to `human_needed` with three human-verification items. A subsequent UAT session (`01-UAT.md`, 73 pass / 1 issue / 1 blocked out of 75) exercised those human items live and found one genuine defect:

- **G-01-7** (UAT test 7, `severity: minor`): Grafana's "Logs for this span" returned nothing when clicked on an inner span (e.g. catalog `UpsertBoat`), even though the correct log line existed in Loki with the matching `trace_id`. Root cause: Tempo's `tracesToLogsV2` had no `spanStartTimeShift`/`spanEndTimeShift`, so the Loki query window was exactly `[span.start, span.end]` — and the request log is written at the end of the *outer* otelhttp span, after inner child spans (like the connect-go RPC handler span) have already ended.

Plan `01-14` (gap-closure, single task) fixed this by adding `spanStartTimeShift: '-1m'` / `spanEndTimeShift: '1m'` to the Tempo datasource's `tracesToLogsV2` in Grafana's provisioning file, commit `e54e24b`. This report independently re-verifies that fix (not trusting the SUMMARY or the orchestrator's own Grafana-API check) and re-checks the rest of the phase for regressions.

The other two human-verification items from the first pass (mobile viewport/font/Kong-only routing; the non-log parts of Grafana trace continuity and the platform dashboard) were exercised live in the same UAT session (tests 3, 4, 7) and passed — they are closed out below, not carried forward.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `make new-service <name>` scaffolds a working service wired to shared `pkg/*` (PLAT-01) | ✓ VERIFIED | No code under `services/_template/` or `pkg/*` changed since the first verification. `git log` shows only observability config (`01-14`) and three code-review fixes (outbox, otel, Makefile) touched since then — none affect the template. Re-confirmed via UAT test 52-54 (D1-D3, all pass, `source: automated`). |
| 2 | `make up` starts the full stack; `make proto-gen` generates committed Go+TS clients; database-per-service enforced (PLAT-02, PLAT-03, PLAT-04) | ✓ VERIFIED | `docker ps` shows all 21 `boatbooking-*` + Harbor containers `Up`/`healthy` (postgres, redpanda, valkey, kong, grafana, loki, prometheus, tempo, otel-collector, catalog, schedule, gateway, web, ci-jenkins, ci-jenkins-agent). Independently re-ran `make proof` from scratch in this session (see #3) — it depends on the full stack being correctly composed. |
| 3 | A state change via transactional outbox is applied exactly once in another service, visible as one trace in Tempo with structured slog JSON logs carrying `trace_id` (PLAT-05, PLAT-06) | ✓ VERIFIED | Ran `make proof` independently in this session (not trusting any prior claim): `PASS applied`, `PASS exactly-once`, `PASS public-list`, `PASS single-trace` (trace `be114e8cddf1594db64b82eb99bfb4e4` spans gateway+catalog+schedule), `PASS logs-correlated`. Queried the Tempo API directly (via Grafana's datasource proxy) and confirmed the same 3-service span tree (gateway POST → catalog UpsertBoat/publish → schedule receive/process). |
| 4 | Grafana's "Logs for this span" resolves for **inner** spans, not just the outer request span, closing UAT gap G-01-7 (PLAT-06) | ✓ VERIFIED (behaviorally, not just presence) | Independently reproduced both the old bug and the new fix using the real trace/log data from the `make proof` run above, replicating exactly what Grafana's `tracesToLogsV2` constructs: queried Loki (`{service_name="catalog"} \| trace_id=\`be114e8c...\``) over the catalog `UpsertBoat` span's **exact** `[start,end]` window (the pre-fix behavior) → **empty result**, reproducing G-01-7. Queried the same filter over the **±1m-widened** window (the post-fix, currently-provisioned behavior) → returned the catalog `"http request"` log line at a timestamp between the inner span's end and the outer span's end, with the correct `trace_id`. Also confirmed live via the Grafana API that the running datasource's `tracesToLogsV2` is exactly `{datasourceUid: loki, filterByTraceID: true, spanStartTimeShift: "-1m", spanEndTimeShift: "1m"}` — matching the committed `datasources.yaml` (`git diff` for `e54e24b` is exactly 2 added lines). Because the query used an explicit `trace_id` filter (mirroring `filterByTraceID: true`), the widened window did not pull in any other trace's logs — the third must-have ("Loki result is still scoped to the clicked trace") is directly confirmed, not inferred. |
| 5 | Failed event processing retries 3x with backoff then lands in `<topic>.dlq`; consumer offsets commit only after successful apply; outbox poison rows no longer wedge the batch (PLAT-07) | ✓ VERIFIED | `pkg/kafka` and `pkg/outbox` unchanged in substance since the first verification except a real bug fix: commit `6707df5` fixed the `CR-01` defect flagged in the prior VERIFICATION.md (an outbox unmarshal failure used to roll back the whole publish batch and permanently wedge on the poison row) — it now logs, marks the row published-and-skipped, and continues, with a new regression test `TestRelaySkipsPoisonRowWithoutLosingEarlierProgress`. `make test` re-run in this session: all packages `ok` or `[no test files]`, zero failures. |
| 6 | Jenkins CI builds every service image and runs unit + integration tests, with per-commit changed-service image scoping, on every push (PLAT-08) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | UAT test 8 ran this live (not skipped) on a throwaway branch: build went `SUCCESS`, every non-image stage ran, Images stage built all 4 services (select-all is correct off a non-main branch), Push correctly skipped. But per-commit scoping (a `services/schedule/`-only change building *only* schedule) cannot be exercised before this phase merges to main, because non-main Jenkins builds diff against `origin/main`, which is docs-only until then — so every non-main branch always selects all services regardless of what changed. The selector's *unit-level* logic is proven (`deploy/ci/changed-services_test.sh`, 13/13 PASS, re-confirmed in the first verification pass). |

**Score:** 5/6 truths verified (1 present + wired, live per-commit-scoping behavior structurally deferrable only to post-merge)

### Deferred Items

None additional — item #6 above is not deferred to a later milestone phase (no later ROADMAP phase covers CI scoping); it is deferred to *after this same phase's merge to main*, which is a structural precondition of the test itself, not a scope choice. It stays a human-verification item, not a `deferred` (Step 9b) entry.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `deploy/observability/grafana/provisioning/datasources/datasources.yaml` | Tempo → Loki trace-to-logs link with ±1m window, filtered by trace id | ✓ VERIFIED | File contains `spanStartTimeShift: '-1m'` / `spanEndTimeShift: '1m'` under `tracesToLogsV2`; `filterByTraceID: true` and `datasourceUid: loki` unchanged. `git show e54e24b --stat` confirms exactly 2 lines added, nothing else touched. Live Grafana API (`/api/datasources/uid/tempo`) reflects the identical values after container restart. |
| `pkg/outbox/outbox.go` | Transactional outbox relay, no poison-row wedge | ✓ VERIFIED | Unmarshal-failure branch now logs, marks published, continues (commit `6707df5`); `publishErrors` metric now also increments on this path (commit `425f926`). `go vet`/`go build`/`go test` clean per `01-REVIEW-FIX.md`; re-ran `make test` in this session, all green. |
| `services/_template/`, `pkg/{auth,httpx,kafka,outbox,pgx,events,clock,money}`, `Dockerfile`, `deploy/docker-compose.yml`, `deploy/kong/*`, `deploy/proof.sh`, `deploy/observability/*`, `deploy/ci/*`, `Jenkinsfile`, `apps/web/` | All previously-verified phase-1 artifacts | ✓ VERIFIED (regression) | No changes since the first verification except the observability config and the three code-review fixes already covered above. `make proof`, `make test`, `docker ps` health, and `git log` for these paths confirm no drift. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `services/catalog` UpsertBoat | `pkg/outbox.Insert` | same pgx tx | ✓ VERIFIED | `make proof`'s `PASS applied` + `PASS exactly-once`, re-run this session |
| `pkg/outbox` relay | Redpanda `catalog.events` | `Producer.Publish` w/ `traceparent` header | ✓ VERIFIED | `make proof`'s `PASS single-trace`; Tempo API directly queried confirms `catalog.events publish` span in the same trace |
| `services/schedule` consumer | `pkg/kafka.Consumer.Handle` | `processed_events` tx | ✓ VERIFIED | `make proof`'s `PASS exactly-once`; Tempo API shows `catalog.events receive`/`process` spans in the same trace |
| Grafana Tempo datasource `tracesToLogsV2` | Loki datasource (`uid: loki`) | `spanStartTimeShift`/`spanEndTimeShift`-widened, trace-id-filtered LogQL | ✓ VERIFIED | Directly reproduced the exact LogQL Grafana constructs against the live Loki API for both the old (unshifted) and new (±1m) windows — old returns `[]`, new returns the matching log line, both scoped to one `trace_id` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|---------------------|--------|
| Tempo `tracesToLogsV2` link | Loki query `start`/`end` range | `span.startTimeUnixNano ∓ 60s` (computed client-side by Grafana) | Yes — verified by replaying the same computation against the real Tempo trace and real Loki data | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full walking-skeleton event flow (PLAT-05/06) | `make proof` | `PASS applied/exactly-once/public-list/single-trace/logs-correlated`, boat `01a0dd7e-...`, trace `be114e8c...` | PASS |
| Grafana API reflects the G-01-7 fix | `curl .../api/datasources/uid/tempo` (via `docker exec` for the admin password, matching the plan's own no-`.env`-read pattern) | `{"datasourceUid":"loki","filterByTraceID":true,"spanEndTimeShift":"1m","spanStartTimeShift":"-1m"}` | PASS |
| G-01-7 regression check: old (unshifted) window on inner span | Loki `query_range` with `start=span.start`, `end=span.end`, filtered on `trace_id` | `[]` (empty) — reproduces the pre-fix bug exactly | PASS (confirms the bug existed and is window-caused) |
| G-01-7 fix check: new (±1m) window on inner span | Loki `query_range` with `start=span.start-60s`, `end=span.end+60s`, filtered on `trace_id` | Returns the catalog `"http request"` log line, correct `trace_id` | PASS |
| Unit test suite | `make test` | All packages `ok` or `[no test files]`, zero failures | PASS |
| Outbox poison-row fix (CR-01, regression from first verification) | `01-REVIEW-FIX.md`'s recorded `-tags=integration` run of `TestRelaySkipsPoisonRowWithoutLosingEarlierProgress` | `PASS` (per fix report; not re-run in this session — Docker testcontainers run takes longer than the spot-check budget, and `go vet`/`go build`/`make test` already confirm no compile/unit regression) | PASS (documented, not re-executed) |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` convention in this repo; the phase's equivalent runnable checks (`make proof`, `make obs-check`, `make kong-roundtrip`, `make template-smoke`) are covered under Behavioral Spot-Checks above and were re-run independently.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PLAT-01 | 01-09, 01-10 | Scaffold service via `make new-service` | SATISFIED | No change since first verification; UAT tests 52-54 (D1-D3) pass |
| PLAT-02 | 01-01, 01-04, 01-07, 01-09, 01-10 | Shared `pkg/*` sole access path | SATISFIED | No change since first verification |
| PLAT-03 | 01-01, 01-12 | `make up` full stack | SATISFIED | `docker ps` healthy this session |
| PLAT-04 | 01-02 | `make proto-gen` committed | SATISFIED | No change since first verification |
| PLAT-05 | 01-04, 01-10, 01-11, 01-12 | Exactly-once cross-service event | SATISFIED | `make proof` re-run, PASS |
| PLAT-06 | 01-04, 01-08, 01-12, 01-14 | Single trace + structured logs + trace-to-logs link | SATISFIED | `make proof` re-run PASS; G-01-7 independently reproduced-and-fixed in this session |
| PLAT-07 | 01-07 | Retry 3x → DLQ, commit-after-apply, no poison-row wedge | SATISFIED | `make test` green; CR-01 fix (`6707df5`) confirmed present with a regression test |
| PLAT-08 | 01-06, 01-13 | Jenkins CI builds+tests every push, per-commit image scoping | PARTIALLY SATISFIED — selector logic proven (unit + one live non-main build); per-commit scoping structurally requires a post-merge check |
| PLAT-09 | 01-05 | Next.js TH/EN, Thai font, mobile-first, Kong-only | SATISFIED | UAT tests 3-4 pass live (this session did not re-run curl checks; no code changed since the first verification's independent confirmation) |
| PLAT-10 | 01-01, 01-12 | Kong JWT round trip | SATISFIED | No change since first verification; UAT tests 9-14 (D1-D6) pass |

No orphaned requirements — all 10 PLAT-IDs declared across plans, present in `REQUIREMENTS.md`, and marked `[x]` complete there.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `services/gateway/.../bff.go:90` | — | `TODO(WR-01): no claims.Role check here` | Info (tracked, referenced) | Explicitly scoped to Phase 2 (AUTH-0x) per `REQUIREMENTS.md`; the `TODO` references its own review-finding ID (`WR-01`), satisfying the debt-marker gate's "formal follow-up reference" exception — not a `TBD`/`FIXME`/`XXX` and not a blocker |
| — | — | Previously-flagged `CR-01` (outbox unmarshal rollback), `WR-02` (incomplete status mapping), `WR-03` (unescaped password), `IN-01`/`WR-04` (Makefile echo-piped password) | — (all fixed) | Confirmed fixed in this session: `pkg/outbox/outbox.go` (commit `6707df5`+`425f926`), `pkg/httpx/errors.go` (comment references `WR-02` as mitigated), `deploy/postgres/init.sh` (comment references `WR-03` fix present), `Makefile` (comment/behavior matches `WR-04` fix per `01-REVIEW-FIX.md`) |

No `TBD`/`FIXME`/`XXX` debt markers found anywhere under `pkg/`, `services/`, `apps/web/src`, or `deploy/` (re-scanned this session).

### Human Verification Required

### 1. Post-merge Jenkins per-commit image scoping

**Test:** After this phase merges to `main`, push a commit under `pkg/` (should image all 4 services) then, separately, a commit only under `services/schedule/` (should image only `schedule`); watch the `boat-booking` multibranch job's scan cycles.
**Expected:** Both builds run every non-image stage; the `pkg/` build's Images stage builds `template`+`catalog`+`gateway`+`schedule`; the `schedule`-only build's Images stage builds only `schedule`; neither pushes to Harbor unless on `main`.
**Why human:** `01-UAT.md` test 8 already proved the pipeline goes green end-to-end on a live, non-main build (select-all case) — the only remaining gap is that per-commit *scoping* specifically requires a base commit on `main` to diff against, which does not exist meaningfully until this phase's own merge. This is a structural sequencing constraint of the test, not an unproven code path (`changed-services.sh`'s logic is 13/13 unit-tested).

### Gaps Summary

No gaps. UAT gap G-01-7 is closed: the fix (`e54e24b`) is present in the committed config, live in the running Grafana instance, and independently proven in this session to change Loki query results from empty (pre-fix simulation) to a correctly-scoped, correct-trace-id log line (post-fix simulation) — not just "the API returns the right JSON," but the actual query behavior the UI depends on. `make proof`, `make test`, and the Tempo API cross-check confirm no regression in the rest of the phase. Two of the three human-verification items from the first pass (mobile viewport/font/Kong routing, Grafana dashboard/trace-continuity minus the log-link defect) were closed out by the UAT session itself (tests 3, 4, 7) and are not carried forward.

The only remaining open item is live, per-commit Jenkins image scoping, which cannot be meaningfully exercised until this phase merges to `main` — this is the same structural constraint UAT test 8 already identified and is routed to `human_needed` rather than blocking the phase, consistent with the first verification's judgment on this item.

---

_Verified: 2026-09-26T19:15:00Z_
_Verifier: Claude (gsd-verifier)_
