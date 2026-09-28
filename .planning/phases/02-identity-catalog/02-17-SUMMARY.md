---
phase: 02-identity-catalog
plan: 17
subsystem: ui
tags: [admin, routes, catalog, validation, proof-script]

# Dependency graph
requires:
  - phase: 02-identity-catalog
    provides: routes/policy-editor.tsx, routes/queries.ts, routes/route-sheet.tsx, deploy/proof.sh (02-13/02-14/02-15 wave-5 admin routes plans)
provides:
  - validatePolicy treats absent protojson-omitted zero fields (minHoursBefore, refundPercent) as 0, not a negative sentinel
  - Policy editor delete button hidden on the last (mandatory 0-hour) tier and the single-tier case
  - pierName disambiguates same-named piers with the id's last 4 characters, applied to the routes list, archive dialog and both route Sheet pier selects
  - deploy/proof.sh's proof pier is named "Proof Pier <SUFFIX>", matching the already-unique operator/boat names
affects: []

# Actuals (#2632)
actuals:
  tokens: 1336
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "validatePolicy/pierName remain the single functions every caller (editor, route Sheet, routes list, archive dialog) goes through — gap fixes land once, not per-caller"

key-files:
  created: []
  modified:
    - apps/admin/src/app/(admin)/routes/policy-editor.tsx
    - apps/admin/src/app/(admin)/routes/queries.ts
    - apps/admin/src/app/(admin)/routes/route-sheet.tsx
    - deploy/proof.sh

key-decisions:
  - "pierName's duplicate check flattens the given pier lists once and compares nameTh across all entries except the id being rendered, rather than adding a separate 'is duplicate' pass — one loop, same signature every caller already uses"

requirements-completed: [CAT-03]

coverage:
  - id: D1
    description: "A valid cancellation policy loaded from the API or copied via 'สร้างเส้นทางย้อนกลับ' passes validation immediately, including the 0-hour tier"
    requirement: "CAT-03"
    verification:
      - kind: unit
        ref: "grep: no '?? -1' fallback remains in policy-editor.tsx; tsc/build pass with the 0-fallback"
        status: pass
      - kind: manual_procedural
        ref: "UAT test 10 (deferred to end-of-phase per human_verify_mode: end-of-phase)"
        status: unknown
    human_judgment: true
    rationale: "Config sets human_verify_mode: end-of-phase — the plan's <verify><human-check> for real-browser confirmation is deferred to the phase-level UAT consolidation, not run mid-flight"
  - id: D2
    description: "The policy editor's delete button never renders on the last tier (or the single-tier case)"
    requirement: "CAT-03"
    verification:
      - kind: unit
        ref: "grep: 'tiers.length - 1' guard present in policy-editor.tsx"
        status: pass
    human_judgment: false
  - id: D3
    description: "Same-named piers are distinguishable in the routes list, archive dialog and both route Sheet pier selects via an id suffix"
    verification:
      - kind: unit
        ref: "grep: pierName uses slice(-4); route-sheet.tsx selects call pierName(...) instead of rendering p.nameTh directly; admin build/typecheck/lint/format pass"
        status: pass
      - kind: manual_procedural
        ref: "UAT test 9 (deferred to end-of-phase per human_verify_mode: end-of-phase)"
        status: unknown
    human_judgment: true
    rationale: "Config sets human_verify_mode: end-of-phase — real-browser confirmation with the dev DB's two same-named 'Proof Pier' rows is deferred to phase-level UAT"
  - id: D4
    description: "make proof creates a run-unique 'Proof Pier <SUFFIX>' pier instead of a fixed 'Proof Pier' name"
    verification:
      - kind: integration
        ref: "make up && make proof — PASS applied, PASS exactly-once, PASS public-list, PASS single-trace, PASS logs-correlated"
        status: pass
    human_judgment: false

# Metrics
duration: ~10min
completed: 2026-09-28
status: complete
---

# Phase 02 Plan 17: Routes UAT Gap Closure (G-02-12, G-02-9) Summary

**validatePolicy now treats protojson-omitted zero fields as 0 (not -1), the policy editor never shows a delete button on the last tier, `pierName` appends an id suffix to disambiguate same-named piers everywhere routes displays a pier name, and `make proof`'s pier gets a run-unique name.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-09-28T16:04:00Z (approx)
- **Completed:** 2026-09-28T16:14:21Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Fixed `validatePolicy`'s `?? -1` fallback to `?? 0` for both `minHoursBefore` and `refundPercent`, so a loaded or copied valid policy (whose mandatory 0-hour tier arrives from protojson with both fields omitted) no longer fails client-side validation and disables Save
- Hid the policy editor's per-tier delete button on the last tier (`index < tiers.length - 1`), which also covers the single-tier case, matching UI-SPEC E4
- Rewrote `pierName` in `routes/queries.ts` to flatten the given pier lists and append the pier id's last 4 characters in parentheses whenever another pier in the lists shares the same Thai name, keeping the existing 8-char-prefix fallback for missing/unnamed piers
- Wired `route-sheet.tsx`'s two `NativeSelect`s (ท่าต้นทาง, ท่าปลายทาง) to render `pierName(...)` labels instead of raw `nameTh`; `routes/page.tsx`'s list label and archive dialog already went through `pierName` and needed no change
- Gave `deploy/proof.sh`'s `UpsertPier` call a run-unique name (`Proof Pier <SUFFIX>`), matching the operator and boat names it already suffixes, so repeated `make proof` runs stop producing identically named piers

## Task Commits

Each task was committed atomically:

1. **Task 1: Policy editor absent-field fallback and last-tier delete guard** - `d27e0c6` (fix)
2. **Task 2: Disambiguate same-named piers in routes list and Sheet selects** - `0259f57` (fix)
3. **Task 3: Run-unique pier name in make proof** - `2a26bae` (fix)

_No TDD tasks in this plan; each is a single fix commit._

## Files Created/Modified
- `apps/admin/src/app/(admin)/routes/policy-editor.tsx` - `validatePolicy` fallback 0 not -1; delete button hidden on the last tier
- `apps/admin/src/app/(admin)/routes/queries.ts` - `pierName` appends `(last-4-of-id)` suffix on Thai-name collision
- `apps/admin/src/app/(admin)/routes/route-sheet.tsx` - both pier selects render `pierName(...)` labels
- `deploy/proof.sh` - `UpsertPier` JSON body suffixes `nameTh`/`nameEn` with `SUFFIX`

## Decisions Made
- `pierName`'s duplicate check flattens the id's own pier lists once (`pierLists.flatMap(...)`) and scans for any *other* pier (`p.pierId !== id`) sharing the same `nameTh`, rather than pre-building a name→count map — the pier list is small (a few hundred at most per admin page, per the plan's accepted DoS threat T-02-17-03), so an O(n) scan per label is fine and keeps the diff to one function

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Task 2's route-sheet.tsx edit initially exceeded the Prettier line-length limit (multi-arg `pierName(...)` call on one line); ran `prettier --write` on the file to reformat before re-running `format:check`, which then passed. Not a deviation from the plan's intended change, just a formatting pass.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- G-02-12 and G-02-9 are both closed: policies validate immediately on load/copy, the last tier can't be deleted, same-named piers are distinguishable everywhere the routes page shows a pier name, and `make proof` stops creating duplicate-named piers.
- The plan's `<verification>` end-of-phase human check (UAT tests 9, 10, 12 re-run clean) is deferred per `workflow.human_verify_mode: end-of-phase` — it will be consolidated with other phase 02 human-verify items at phase-level UAT, not run mid-flight for this plan.
- This was the last plan (17 of 17) in phase 02-identity-catalog. Phase is ready for `/gsd-verify-work 02` and end-of-phase UAT.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-28*

## Self-Check: PASSED

All key files verified present on disk (policy-editor.tsx, queries.ts, route-sheet.tsx, proof.sh, this SUMMARY.md). All task commits (`d27e0c6`, `0259f57`, `2a26bae`) and the metadata commit (`45c6290`) verified present in git log.
