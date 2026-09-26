---
phase: 01-platform-foundation
plan: 14
subsystem: infra
tags: [grafana, tempo, loki, observability, provisioning]

# Dependency graph
requires:
  - phase: 01-platform-foundation
    provides: Grafana/Tempo/Loki observability stack with trace-to-logs derived field (D-20, D-51)
provides:
  - Tempo → Loki trace-to-logs link with a ±1m span time window, closing UAT gap G-01-7
affects: []

# Actuals (#2632)
actuals:
  tokens: 340
  tasks: 1
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Grafana provisioning-file config changes are applied by restarting only the grafana container (docker restart), not a full compose up/down"

key-files:
  created: []
  modified:
    - deploy/observability/grafana/provisioning/datasources/datasources.yaml

key-decisions:
  - "Widened Tempo's tracesToLogsV2 window by spanStartTimeShift: '-1m' / spanEndTimeShift: '1m' instead of moving where request logs are emitted, per D-51 — keeps filterByTraceID: true so the wider window stays scoped to one trace"

patterns-established: []

requirements-completed: [PLAT-06]

coverage:
  - id: D1
    description: "Grafana's provisioned Tempo datasource reports tracesToLogsV2 with spanStartTimeShift='-1m', spanEndTimeShift='1m', filterByTraceID=true, datasourceUid='loki'"
    requirement: "PLAT-06"
    verification:
      - kind: other
        ref: "docker restart boatbooking-grafana-1 && curl -sf http://localhost:3000/api/datasources/uid/tempo | jq -e '.jsonData.tracesToLogsV2 | .spanStartTimeShift == \"-1m\" and .spanEndTimeShift == \"1m\" and .filterByTraceID == true and .datasourceUid == \"loki\"' (exited 0)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Clicking \"Logs for this span\" on the catalog UpsertBoat span (and the schedule consume span) of a make proof trace opens Loki and shows that service's http request log line with the matching trace_id, with no lines from other traces"
    requirement: "PLAT-06"
    verification: []
    human_judgment: true
    rationale: "Requires visually confirming Grafana Explore UI behavior (clicking a span, inspecting the resulting Loki panel) — not automatable from this session"

# Metrics
duration: 5min
completed: 2026-09-26
status: complete
---

# Phase 01 Plan 14: Widen Tempo trace-to-logs window (G-01-7) Summary

**Added `spanStartTimeShift: '-1m'` / `spanEndTimeShift: '1m'` to Grafana's Tempo→Loki `tracesToLogsV2` config so "Logs for this span" resolves for inner spans, not just the outer request span**

## Performance

- **Duration:** 5 min
- **Started:** 2026-09-26T11:18:00Z
- **Completed:** 2026-09-26T11:21:17Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Root cause from 01-UAT.md gap G-01-7 fixed: `tracesToLogsV2` had no time shift, so "Logs for this span" only searched Loki over the clicked span's exact [start, end] window — inner spans (e.g. catalog UpsertBoat) end before the outer request log line is written, so the query returned nothing.
- `spanStartTimeShift: '-1m'` and `spanEndTimeShift: '1m'` added under the Tempo datasource's `tracesToLogsV2`, widening the search window by one minute on each side while `filterByTraceID: true` keeps results scoped to the clicked trace.
- Verified via the Grafana API after restarting only the `boatbooking-grafana-1` container (config re-read from the bind-mounted provisioning directory, no full compose cycle needed).

## Task Commits

Each task was committed atomically:

1. **Task 1: Add a ±1m span time shift to the Tempo → Loki trace-to-logs link** - `e54e24b` (fix)

**Plan metadata:** (this commit)

## Files Created/Modified
- `deploy/observability/grafana/provisioning/datasources/datasources.yaml` - Tempo datasource `tracesToLogsV2` gained `spanStartTimeShift: '-1m'` and `spanEndTimeShift: '1m'`; no other lines changed (verified via `git diff`, exactly 2 lines added)

## Decisions Made
- Fixed via widening the Grafana-side query window (D-51) rather than changing where `pkg/httpx/middleware.go` emits the request log — the log's position at the end of the outer otelhttp span is correct/expected behavior, so the query window is what needed to move, not the app code.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - config-only fix, no stubs introduced.

## Next Phase Readiness
- G-01-7 automated verification closed: the Grafana API confirms the provisioned `tracesToLogsV2` shift values and `filterByTraceID` remains `true`.
- Remaining human-check step (clicking through Grafana Explore on a live `make proof` trace to visually confirm "Logs for this span" now returns the catalog/schedule log lines) is deferred to end-of-phase UAT per `workflow.human_verify_mode: end-of-phase` — see coverage item D2 above.
- No blockers for phase completion from this plan.

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*

## Self-Check: PASSED
- FOUND: deploy/observability/grafana/provisioning/datasources/datasources.yaml
- FOUND: .planning/phases/01-platform-foundation/01-14-SUMMARY.md
- FOUND commit: e54e24b
