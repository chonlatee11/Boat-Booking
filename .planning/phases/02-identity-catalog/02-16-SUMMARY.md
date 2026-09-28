---
phase: 02-identity-catalog
plan: 16
subsystem: ui
tags: [maplibre-gl, react, next.js, catalog, pier-sheet]

# Dependency graph
requires:
  - phase: 02-identity-catalog
    provides: "02-11 pier Sheet + MapPicker (D-20); 02-15 backend archived-operator/HH:MM fixes for UpsertPier"
provides:
  - "MapPicker with validated, crash-proof lat/lng text drafts and a tile-failure flag that resets on success"
  - "Pier Sheet operator select filtered to non-archived operators, with an archived-operator-specific save error"
  - "Pier Sheet locale-independent 24h HH:MM hours validation"
affects: [02-UAT, admin-piers-page]

actuals:
  tokens: 3200
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Draft-string + parse-on-commit pattern for numeric inputs under a library that throws on out-of-range values (parseCoord)"
    - "\"Adjust state during render\" prop-sync pattern (compare prevValue in render, setState directly) instead of an effect+setState loop, to avoid clobbering in-progress typing"
    - "Map API-error events classified by code, not message text, to stay independent of backend wording"

key-files:
  created: []
  modified:
    - apps/admin/src/components/map-picker.tsx
    - "apps/admin/src/app/(admin)/piers/pier-sheet.tsx"

key-decisions:
  - "Switched lat/lng inputs from type=\"number\" to type=\"text\" inputMode=\"decimal\": the DOM reports a partial number like \"13.\" as an empty string on a number input, which is the root cause of both the marker jumping to 0 and out-of-range values reaching maplibre-gl's LngLat constructor"
  - "Tile-failure flag resets via map.on('sourcedata') when a tile event arrives, instead of tracking every possible non-fatal map error type, since a real load failure never produces a tile event at all"
  - "Archived-operator save error is mapped by ApiError.code === 'failed_precondition' in create mode only, not by message text, so this plan has no dependency on 02-15's backend copy"
  - "No app-wide error.tsx boundary added (explicitly out of scope per plan) — validating input at the source means nothing throws"

requirements-completed: [CAT-02]

coverage:
  - id: D1
    description: "Typing/clearing lat/lng in the pier Sheet never crashes the page; out-of-range values show an inline error and disable Save; valid pairs move the marker"
    requirement: "CAT-02"
    verification:
      - kind: unit
        ref: "npm --prefix apps/admin run build (grep gates: inputMode=decimal, parseCoord, sourcedata, no type=number)"
        status: pass
    human_judgment: true
    rationale: "Requires an actual browser to type digit-by-digit, drag the marker, and confirm no crash/dev-overlay — the plan's own <human-check> for this task; not asserted by an automated test"
  - id: D2
    description: "โหลดแผนที่ไม่สำเร็จ shows only on a genuine tile-load failure and clears once a tile loads"
    requirement: "CAT-02"
    verification:
      - kind: unit
        ref: "grep -q sourcedata apps/admin/src/components/map-picker.tsx"
        status: pass
    human_judgment: true
    rationale: "Needs a real blocked-tiles browser scenario (devtools network block) to confirm the message appears/clears correctly"
  - id: D3
    description: "Archived operators are excluded from the create-pier operator select; a race still hitting failed_precondition shows the specific Thai message"
    requirement: "CAT-02"
    verification:
      - kind: unit
        ref: "grep -q '!op.archived' apps/admin/src/app/(admin)/piers/pier-sheet.tsx"
        status: pass
    human_judgment: true
    rationale: "Needs archiving a real operator via the admin UI and confirming it disappears from the select and the specific error copy shows on a forced race"
  - id: D4
    description: "Opening/closing hours accept only 24h HH:MM text, independent of browser locale; malformed values block Save"
    requirement: "CAT-02"
    verification:
      - kind: unit
        ref: "grep -q HHMM apps/admin/src/app/(admin)/piers/pier-sheet.tsx"
        status: pass
    human_judgment: true
    rationale: "Needs a real browser to confirm the payload carries the typed HH:MM values and that a malformed value blocks Save via the UI"

duration: ~20min
completed: 2026-09-28
status: complete
---

# Phase 02 Plan 16: Piers-page UAT gap closure (G-02-7, G-02-8 UI half) Summary

**Draft-string lat/lng validation in MapPicker stops maplibre-gl's out-of-range throw, and the pier Sheet now filters archived operators and validates 24h HH:MM hours before Save.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2 completed
- **Files modified:** 2

## Accomplishments
- MapPicker now keeps a draft string per lat/lng input, parses both through `parseCoord` (range + finiteness check), and only calls `onChange` with a real `{lat, lng}` pair when both drafts are valid — no partial/empty/out-of-range value can reach maplibre-gl's `LngLat` constructor, which previously threw synchronously and crashed the page (G-02-7)
- The tile-failure flag (`โหลดแผนที่ไม่สำเร็จ`) now resets via `map.on('sourcedata')` the first time a tile actually loads, so a single transient error no longer latches the message forever (G-02-8 UI half)
- The create-pier operator select filters out archived operators (`!op.archived`), and a save that still races into `failed_precondition` shows "ผู้ประกอบการนี้ถูกเก็บถาวรแล้ว กรุณาเลือกผู้ประกอบการอื่น" mapped by `ApiError.code`, independent of backend message wording
- Opening/closing hours are now locale-independent `HH:MM` text inputs validated against a strict regex, so a malformed value blocks Save instead of silently reaching the server as an empty string

## Task Commits

1. **Task 1: MapPicker validated draft lat/lng inputs and a tile-failure flag that resets** - `fe41f8b` (fix)
2. **Task 2: Pier Sheet non-archived operators, archived-operator error copy, 24h HH:MM hours** - `c34fcae` (fix)

**Plan metadata:** committed together with STATE.md/ROADMAP.md updates (see git log)

## Files Created/Modified
- `apps/admin/src/components/map-picker.tsx` — draft-string lat/lng inputs with `parseCoord` range validation, "adjust state during render" prop sync, tile-failure flag reset on `sourcedata`
- `apps/admin/src/app/(admin)/piers/pier-sheet.tsx` — non-archived operator filter, `failed_precondition`-by-code error copy, `HHMM` regex validation replacing native `type="time"` inputs

## Decisions Made
- `type="number"` → `type="text" inputMode="decimal"` for lat/lng: the root cause of the crash was the DOM silently reporting a partial number-input value as `""` (which `Number()` coerces to `0`, a finite value that passed the old guard)
- Draft/value sync uses React's "adjust state during render" pattern (comparing a `prevValue` state during render, not inside a `useEffect`) to avoid both an effect+setState loop and clobbering in-progress typing
- Archived-operator detection in the save handler keys off `ApiError.code`, not message text, so this plan has zero coupling to 02-15's backend copy (per the plan's explicit non-dependency note)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. The plan's own grep gate `! grep -q 'type="time"'` initially caught a code comment that mentioned the removed input type in prose; reworded the comment to avoid the literal string, no logic change.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both UI halves of G-02-7 and G-02-8 are closed. Automated verification (lint, format, typecheck, build, and all plan grep gates) passes for both files.
- Remaining work is the plan's `<human-check>` steps (real-browser typing, drag, blocked-tiles, and archived-operator race scenarios) plus re-running UAT tests 7 and 8 — deferred to end-of-phase human verification per `workflow.human_verify_mode = end-of-phase`, consistent with this plan's "no separate tracer" note.
- No blockers for subsequent phase-02 plans.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-28*

## Self-Check: PASSED
- FOUND: apps/admin/src/components/map-picker.tsx
- FOUND: apps/admin/src/app/(admin)/piers/pier-sheet.tsx
- FOUND commit: fe41f8b
- FOUND commit: c34fcae
