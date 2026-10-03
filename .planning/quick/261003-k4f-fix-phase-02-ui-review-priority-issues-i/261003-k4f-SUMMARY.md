---
phase: quick-261003-k4f
plan: 01
subsystem: ui
tags: [nextjs, tailwind, admin, badge, form-validation, i18n]

requires:
  - phase: 02-identity-catalog
    provides: apps/admin operators/piers/routes/staff/boats list pages and create forms, UI-SPEC/UI-REVIEW
provides:
  - Semantic green active-status badge on Operators, Piers, Routes, Staff (matching Boats)
  - Per-field touched-gated required-field FieldErrors on Pier Sheet, Boat Dialog, Staff Dialog
  - Thai not-found fallback copy ("ไม่พบท่าเรือ" / "ไม่พบผู้ประกอบการ") replacing raw id-prefix fallbacks
affects: [02-ui-review, 02-ui-spec]

actuals:
  tokens: 3074
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Required-field FieldError gated behind a per-field touched flag set in onChange (not onBlur), reset on dialog/sheet open — avoids flashing errors on a blank form or during close animation"

key-files:
  modified:
    - apps/admin/src/app/(admin)/operators/page.tsx
    - apps/admin/src/app/(admin)/piers/page.tsx
    - apps/admin/src/app/(admin)/routes/page.tsx
    - apps/admin/src/app/(admin)/staff/page.tsx
    - apps/admin/src/app/(admin)/piers/pier-sheet.tsx
    - apps/admin/src/app/(admin)/boats/boat-dialog.tsx
    - apps/admin/src/app/(admin)/staff/staff-dialog.tsx
    - apps/admin/src/app/(admin)/boats/queries.ts
    - apps/admin/src/app/(admin)/staff/queries.ts
    - apps/admin/src/app/(admin)/routes/queries.ts

key-decisions:
  - "No shared ActiveBadge component or Badge variant added — one identical className string copied to four call sites (ponytail: boats' StatusBadge takes a full boat object, so it isn't directly reusable, and a new abstraction for a one-line className isn't warranted)"
  - "Touched flags set in onChange, not onBlur — blur fires on the dialog close/cancel control and would flash the error during the close animation, especially for the autoFocused boat-name input"
  - "No submit-attempted flag added — Save is already disabled={!isValid || pending}, so a submit can never be attempted while the form is invalid"
  - "Did not special-case the nil-UUID string in pierName — legacy boats with a null home_pier_id correctly render ไม่พบท่าเรือ, which is accurate and nudges the admin to assign a pier"
  - "Did not hoist a shared id-lookup helper across boats/staff/routes queries.ts — each page intentionally keeps its own queries.ts (02-11 pattern)"

patterns-established:
  - "Status badges use explicit bg-green-100/text-green-800 (active) or bg-orange-100/text-orange-800 (maintenance) Tailwind classes, never the default Badge variant, across all 5 admin list pages"

requirements-completed:
  - 02-UI-REVIEW-FIX-1
  - 02-UI-REVIEW-FIX-2
  - 02-UI-REVIEW-COPY-1

coverage:
  - id: D1
    description: "Active-status badge on Operators, Piers, Routes, Staff renders semantic green (bg-green-100 text-green-800), matching Boats"
    requirement: "02-UI-REVIEW-FIX-1"
    verification:
      - kind: other
        ref: "grep -rn '<Badge>ใช้งาน</Badge>' apps/admin/src (must find none) + grep for bg-green-100 text-green-800 in all four page.tsx files"
        status: pass
    human_judgment: true
    rationale: "Automated grep proves the className string is present and the bare-Badge pattern is gone, but the plan's own verify step also calls for a human-check of the rendered green pill color on the live stack — a visual color check is outside what a grep can prove."
  - id: D2
    description: "Blank create forms (Pier Sheet, Boat Dialog, Staff Dialog) show no required-field FieldError until the field is edited; Save stays disabled while invalid"
    requirement: "02-UI-REVIEW-FIX-2"
    verification:
      - kind: other
        ref: "negative grep for ungated FieldError conditions + touched-flag count assertions + disabled={!isValid || pending} presence, all three files"
        status: pass
    human_judgment: true
    rationale: "Grep proves the gating code exists and Save's disabled expression is unchanged, but actually opening each dialog/sheet blank, confirming zero visible errors, typing then clearing a field, and reopening to confirm reset is a UI behavior the plan's verify step assigns to a human-check."
  - id: D3
    description: "No raw UUID fragment renders in any admin name lookup; unresolved names read ไม่พบท่าเรือ / ไม่พบผู้ประกอบการ"
    requirement: "02-UI-REVIEW-COPY-1"
    verification:
      - kind: other
        ref: "grep -rn 'slice(0, 8)' apps/admin/src (must find none) + grep for the two Thai fallback strings in all three queries.ts files + slice(-4) duplicate-suffix retained in routes/queries.ts"
        status: pass
      - kind: unit
        ref: "apps/admin/src/app/(admin)/routes/money.test.ts and src/lib/api.test.ts — unrelated regression check, all 8 tests pass"
        status: pass
    human_judgment: true
    rationale: "Grep + unit tests prove the fallback string and regression safety, but confirming the Boats table actually renders ไม่พบท่าเรือ (never 00000000) for an unresolved pier on the live stack is the plan's human-check step."

duration: 25min
completed: 2026-10-03
status: complete
---

# Phase quick-261003-k4f: Fix Phase 02 UI-REVIEW Priority Issues Summary

**Semantic green status badges on 4 admin list pages, touched-gated required-field errors on 3 create forms, and Thai not-found copy replacing raw UUID fallbacks in 4 name-lookup helpers**

## Performance

- **Duration:** 25 min
- **Started:** 2026-10-03T00:00:00Z (approx)
- **Completed:** 2026-10-03
- **Tasks:** 3
- **Files modified:** 10

## Accomplishments
- Operators, Piers, Routes and Staff list pages render the ใช้งาน (active) badge with the same `bg-green-100 text-green-800` className as Boats, instead of the brand-accent default Badge variant
- Pier Sheet (operator/nameTh/nameEn), Boat Dialog (name) and Staff Dialog (name) now gate their required-field `FieldError` behind a per-field `touched` flag set in `onChange`, reset on every open — a blank create form shows zero errors; Save's `disabled={!isValid || pending}` is unchanged
- `boats/queries.ts` `pierName`, `staff/queries.ts` `pierName`/`operatorName`, and `routes/queries.ts` `pierName` all replaced their 8-character id-prefix fallback with Thai copy (`ไม่พบท่าเรือ` / `ไม่พบผู้ประกอบการ`); the routes page's G-02-9 duplicate-name id-tail suffix is kept, since it only applies to resolved piers

## Task Commits

Each task was committed atomically:

1. **Task 1: Semantic green active-status badge on Operators, Piers, Routes, Staff** - `d4e68a4` (fix)
2. **Task 2: Gate required-field FieldErrors behind per-field touched state** - `68e360f` (fix)
3. **Task 3: Human-readable Thai fallback instead of a raw id prefix in pier/operator name helpers** - `92eb70a` (fix)

_Plan metadata commit pending (handled by orchestrator per quick-task constraints)._

## Files Created/Modified
- `apps/admin/src/app/(admin)/operators/page.tsx` - active badge className
- `apps/admin/src/app/(admin)/piers/page.tsx` - active badge className
- `apps/admin/src/app/(admin)/routes/page.tsx` - active badge className
- `apps/admin/src/app/(admin)/staff/page.tsx` - active badge className
- `apps/admin/src/app/(admin)/piers/pier-sheet.tsx` - touched state for operator/nameTh/nameEn required errors
- `apps/admin/src/app/(admin)/boats/boat-dialog.tsx` - nameTouched state for name required error
- `apps/admin/src/app/(admin)/staff/staff-dialog.tsx` - nameTouched state for name required error
- `apps/admin/src/app/(admin)/boats/queries.ts` - pierName Thai not-found fallback
- `apps/admin/src/app/(admin)/staff/queries.ts` - pierName/operatorName Thai not-found fallback
- `apps/admin/src/app/(admin)/routes/queries.ts` - pierName Thai not-found fallback (duplicate-suffix logic kept)

## Decisions Made
- No shared ActiveBadge component added; one identical className string copied to four call sites (ponytail — boats' own `StatusBadge` takes a full `boat` prop, so it isn't directly reusable, and a new abstraction for a one-line className isn't warranted)
- Touched flags set in `onChange`, not `onBlur`, to avoid flashing an error during a dialog/sheet close animation
- No submit-attempted flag added — Save's existing `disabled={!isValid || pending}` already prevents submitting an invalid form
- Did not special-case the nil-UUID string in `pierName` — legacy boats with a null `home_pier_id` correctly render `ไม่พบท่าเรือ`
- Did not hoist a shared id-lookup helper across the three `queries.ts` files, preserving the existing 02-11 one-file-per-page pattern

## Deviations from Plan

None - plan executed exactly as written. All three tasks' automated verify commands (typecheck, lint, prettier, and for Task 3 the `money.test.ts`/`api.test.ts` node tests) passed on the first attempt with no auto-fixes required.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All three Phase 02 UI-REVIEW priority issues (accent-badge, ungated required errors, raw-UUID fallback) are resolved; the out-of-scope `font-medium` weight and `h1` `text-2xl` vs `text-xl` issues remain open per the plan's explicit exclusion and are not blockers.
- `git diff --stat` confirms exactly the 10 planned files changed across the three task commits, with typecheck/lint/prettier/node-tests all green.

---
*Phase: quick-261003-k4f*
*Completed: 2026-10-03*

## Self-Check: PASSED

All 10 modified files and the SUMMARY.md exist on disk; all 3 task commit hashes (d4e68a4, 68e360f, 92eb70a) exist in git log.
