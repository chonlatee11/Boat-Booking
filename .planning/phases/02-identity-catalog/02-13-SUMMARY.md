---
phase: 02-identity-catalog
plan: 13
subsystem: admin-ui
tags: [nextjs, tanstack-query, shadcn, staff-management, d-10, admin-app]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-07)
    provides: "identity UserService (ListUsers/UpsertUser/SetUserDisabled) — super_admin-only staff/pier_admin account management with synchronous catalog pier validation"
  - phase: 02-identity-catalog (plan 02-09)
    provides: "apps/admin scaffold: rpc()/apiFetch, DataTable/ArchiveDialog pattern, useWhoami, Dialog form patterns"
provides:
  - "apps/admin's Staff page: super_admin-only list/create/edit of pier_admin/staff users with operator+pier assignment, and D-10 disable/re-enable with a 15-minute-effect confirmation"
affects: []

# Actuals (#2632)
actuals:
  tokens: 5256
  tasks: 2
  commits: 2
plan_head_before: 3a215e1a4a4947c74b70fd039b02c39ac816b9b1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pier multi-select uses native <input type=\"checkbox\"> elements inside a <label>, not a new shadcn Checkbox component — matches the plan's explicit 'no new shadcn component or package' constraint"
    - "useAllPiers() reuses the same ['piers'] cache key as the boats/routes/piers pages' own-piers fetch (02-11 pattern) — for a super_admin caller, unscoped ListPiers already returns every pier, so the cache is genuinely shareable across pages, not just conveniently named the same"
    - "useStaffUsers() calls rpc('users', 'ListUsers') without a generic type argument and casts the return via `as Promise<ListUsersResponseJson>` — matches the codebase's existing non-generic rpc() call precedent (operators/piers/boats/routes' Archive* calls) while keeping a typed response"

key-files:
  created:
    - "apps/admin/src/app/(admin)/staff/page.tsx"
    - "apps/admin/src/app/(admin)/staff/staff-dialog.tsx"
    - "apps/admin/src/app/(admin)/staff/queries.ts"
  modified: []

key-decisions:
  - "Actions cell hides BOTH edit and disable for role=super_admin rows, not just disable — UserService's updateUser (02-07) rejects any update targeting a super_admin row with FailedPrecondition, so showing an edit action that can never succeed would be a dead-end interaction; the plan text only mandated hiding disable, hiding edit too is a Rule 1/2 auto-fix (guaranteed-fail action removed)"
  - "Edit mode is supported by the same StaffDialog component used for create (user prop optional), matching the operator-dialog/boat-dialog precedent from 02-09/02-12 rather than a second component — Task 1 and Task 2's 'create Dialog' / 'Edit mode: Dialog pre-filled' descriptions are the same code path"
  - "DisableUserDialog is a small local component inside page.tsx rather than a generalized variant of the shared ArchiveDialog — the copy (title/body/confirm) is staff-specific (D-10's 15-minute wording) and the plan's files_modified list never adds a new shared-component file for this plan"

requirements-completed: [AUTH-04, AUTH-02]  # last plan declaring both (02-07's UserService backend already summarized; 02-09 also declared AUTH-02, already summarized) — requirements.ready-ids reports both ready now

coverage:
  - id: D1
    description: "super_admin creates a pier_admin or staff user (email, name, role, operator, one or more of that operator's piers) from a Dialog and the user appears in the staff list"
    requirement: AUTH-04
    verification:
      - kind: other
        ref: "npm --prefix apps/admin run build (route /staff present) + grep staff-dialog.tsx for UpsertUser + grep queries.ts for rpc('users' ListUsers"
        status: pass
    human_judgment: true
    rationale: "The actual create round-trip through Kong to UserService, and that the created user can sign in and see only their assigned piers, need a live make up stack and two browser sessions — deferred to the end-of-phase human-check (human_verify_mode=end-of-phase)."
  - id: D2
    description: "super_admin edits a user's name/role/operator/piers (email read-only) and disables/re-enables them with the D-10 confirmation copy and 15-minute-effect wording"
    requirement: AUTH-04
    verification:
      - kind: other
        ref: "grep page.tsx for the disable body copy \"จะไม่สามารถเข้าสู่ระบบได้อีก (มีผลภายใน 15 นาที)\" and SetUserDisabled; grep staff-dialog.tsx for the email-disabled-in-edit-mode input"
        status: pass
    human_judgment: true
    rationale: "Confirming the disable/re-enable round trip actually revokes/restores login (cross-session, token-expiry-dependent) needs a live stack and two browser sessions — deferred to the end-of-phase human-check."
  - id: D3
    description: "Duplicate email shows อีเมลนี้มีผู้ใช้งานแล้ว and invalid pier assignments show the backend message inline, with the Dialog kept open"
    requirement: AUTH-04
    verification:
      - kind: other
        ref: "grep staff-dialog.tsx for the already_exists->duplicate-email copy branch and the invalid_argument->apiErr.message branch; both set error state without closing the Dialog"
        status: pass
    human_judgment: false
  - id: D4
    description: "The staff page and its nav entry are shown only to super_admin (cosmetic — UserService enforces server-side)"
    requirement: AUTH-04
    verification:
      - kind: other
        ref: "page.tsx renders a no-access Card and short-circuits before the DataTable when whoami.role !== 'super_admin'; admin-shell.tsx's NAV_ITEMS already restricts /staff to super_admin (02-09)"
        status: pass
    human_judgment: false
  - id: D5
    description: "E2 zero-one-many: a staff user's 1..N pier assignments render as wrapping badges without breaking row height alignment"
    verification:
      - kind: other
        ref: "grep page.tsx for the flex flex-wrap gap-1 pier-badges cell"
        status: pass
    human_judgment: true
    rationale: "Visual row-height/wrapping behavior at real widths needs a browser — backstop verification per the UI-SPEC, deferred to the end-of-phase human-check."
  - id: D6
    description: "E5 partial: the staff form requires role and operator, and pier selection is required (at least one pier) before submit enables"
    requirement: AUTH-04
    verification:
      - kind: other
        ref: "staff-dialog.tsx's isValid computation requires emailValid && nameValid && operatorId && pierIds.length >= 1; npm run build/typecheck clean"
        status: pass
    human_judgment: false
  - id: D7
    description: "E5 empty/loading/error: create mode opens empty with submit disabled until valid; edit mode shows skeleton fields and a spinner on submit; a save failure shows บันทึกผู้ใช้งานไม่สำเร็จ กรุณาลองใหม่ with values intact"
    verification:
      - kind: other
        ref: "grep staff-dialog.tsx for the showSkeleton computation, the Spinner-in-submit-button, and the generic save-failure copy in the catch block's else branch"
        status: pass
    human_judgment: false
  - id: D8
    description: "E2 empty/error: ยังไม่มีผู้ใช้งาน / เริ่มต้นด้วยการเพิ่มผู้ใช้งานแรกของคุณ with the CTA, and โหลดข้อมูลผู้ใช้งานไม่สำเร็จ กรุณาลองใหม่ with retry"
    verification:
      - kind: other
        ref: "grep page.tsx for both empty-state strings and the errorText prop passed to DataTable"
        status: pass
    human_judgment: false

# Metrics
duration: 18min
completed: 2026-09-27
status: complete
---

# Phase 2 Plan 13: Admin Staff Users Page Summary

**Admin Staff page (`/staff`): super_admin creates, edits, disables and re-enables pier_admin/staff users with operator+pier assignment via a checkbox multi-select, on top of the 02-07 `UserService` backend and the 02-09 admin scaffold.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-09-27T02:15:00+07:00
- **Completed:** 2026-09-27T02:33:00+07:00
- **Tasks:** 2 (1 tracer, 1 auto)
- **Files modified:** 3 (3 created, 0 modified)

## Accomplishments

- `staff/queries.ts`: `useStaffUsers`, `useOperatorOptions`, `useAllPiers`, `usePiersForOperator(operatorId)` (scoped to one operator via the super_admin-only `operator_id` filter), plus `operatorName`/`pierName` display-lookup helpers — matching the per-folder hooks convention already established by boats/routes (02-11/02-12) so sibling wave-5 plans never edit the same file.
- `staff-dialog.tsx`: one Dialog component serving both create and edit — email (required, read-only on edit), name, role (`ผู้ดูแลท่าเรือ`/`พนักงาน`), operator select, and a pier checkbox list scoped to the chosen operator (changing operator clears the selection); submit stays disabled until email/name/role/operator/≥1 pier are all valid. `UpsertUser` errors map to `อีเมลนี้มีผู้ใช้งานแล้ว` (`already_exists`), the server's own message (`invalid_argument` — names the offending pier ids), or the generic save-failure copy — the Dialog always stays open on failure.
- `staff/page.tsx`: renders a no-access Card for anyone but `super_admin` (UserService is the real enforcement, this is cosmetic per T-02-13-01); `DataTable` columns for email (truncated+tooltip), name, role, operator name, wrapping pier-name badges, and status; empty/error states match the UI-SPEC copy.
- Disable/re-enable (D-10): `DisableUserDialog` (an `AlertDialog` local to this page, mirroring `ArchiveDialog`'s shape with staff-specific copy) confirms "ปิดการใช้งานผู้ใช้ '{ชื่อ}'?" / "จะไม่สามารถเข้าสู่ระบบได้อีก (มีผลภายใน 15 นาที)" before calling `SetUserDisabled(true)`; `EnableUserButton` calls `SetUserDisabled(false)`. Both actions — and edit — are hidden for `super_admin` rows (server always rejects them); disable alone is hidden for the signed-in user's own row (self-disable is rejected). A `failed_precondition` response surfaces the server's own message via a toast.

## Task Commits

1. **Task 1 (tracer): Staff user end-to-end — list → create Dialog → UpsertUser** — `f7f2816` (feat)
2. **Task 2 (auto): Edit, disable and re-enable with confirmation (D-10)** — `fa6ff30` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `apps/admin/src/app/(admin)/staff/queries.ts` — data hooks + name-lookup helpers
- `apps/admin/src/app/(admin)/staff/staff-dialog.tsx` — create/edit form
- `apps/admin/src/app/(admin)/staff/page.tsx` — list, disable/enable, no-access gate

## Decisions Made

See frontmatter `key-decisions` — hiding edit (not just disable) for `super_admin` rows, the single-component create/edit Dialog, and `DisableUserDialog` staying local to `page.tsx` rather than generalizing `ArchiveDialog`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Hid the edit action for `super_admin` rows, not just disable**
- **Found during:** Task 2 (wiring the actions cell)
- **Issue:** The plan's Task 2 action text only says to hide *disable* for `super_admin` rows. `UserService.UpdateStaffUser` (02-07) rejects **any** update targeting a `super_admin`-role row with `FailedPrecondition` — leaving the edit (pencil) action visible for those rows would let a user open the Dialog, fill it out, and always hit a save failure with no way to succeed.
- **Fix:** The actions cell returns `null` entirely for `role === 'super_admin'` rows (no edit, no disable/enable) instead of only omitting the disable button.
- **Files modified:** `apps/admin/src/app/(admin)/staff/page.tsx`
- **Verification:** Code inspection of the actions-cell branch; `npm run typecheck`/`build` clean.
- **Committed in:** `fa6ff30` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug — dead-end edit action removed for a role the backend never allows to be updated)
**Impact on plan:** Strictly a UX correctness fix consistent with the backend's own guard; no new scope, no architectural change.

## Issues Encountered

None beyond the deviation above.

## Known Stubs

None — every component is wired to a real `UserService`/`CatalogService` call; no placeholder data.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `AUTH-04` and `AUTH-02` are marked complete in this plan's frontmatter `requirements` — 02-13 is the last plan declaring both (02-07's `UserService` backend and 02-09's admin scaffold, which also declared `AUTH-02`, are already summarized), so `requirements.ready-ids` reports both ready now.
- The end-of-phase human-check (Task 2's `<human-check>`) is deferred per `human_verify_mode: end-of-phase` — creating a pier_admin, signing in as them in a private window (code from Mailpit), verifying scoped visibility, removing a pier, disabling the user, and confirming the 15-minute-effect token expiry all need a live `make up` stack and two browser sessions; harvested into the phase `UAT.md`.
- No blockers. `npm --prefix apps/admin run lint/format:check/typecheck/build` all green (build lists `/staff` alongside every other admin route).
- This was the last of the wave-5 admin plans (02-09/02-11/02-12/02-13) sharing the `apps/admin` scaffold — Phase 2's admin console now covers all five entities (operators, piers, routes, boats, staff).

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 3 created files verified present on disk (`[ -f ]`): `apps/admin/src/app/(admin)/staff/{page.tsx,staff-dialog.tsx,queries.ts}`.
- Both task commits (`f7f2816`, `fa6ff30`) verified present in `git log --oneline --all`.
- Both tasks' acceptance criteria re-run and confirmed: Task 1 — `npm run build` lists `/staff`, `staff-dialog.tsx` contains `UpsertUser` and `อีเมลนี้มีผู้ใช้งานแล้ว`, `queries.ts` contains `rpc('users'`, `page.tsx` contains `flex-wrap`, submit is gated on `pierIds.length >= 1`; Task 2 — `page.tsx` contains the disable confirmation body copy `จะไม่สามารถเข้าสู่ระบบได้อีก (มีผลภายใน 15 นาที)` and `SetUserDisabled`, `EnableUserButton` calls it with `disabled=false`, and `lint`/`format:check`/`typecheck`/`build` all exit 0.
- Plan-level `<verification>` re-run clean: `npm --prefix apps/admin run lint/format:check/typecheck/build` all green; commits measured via the plan ledger (`plan_head_before` `3a215e1a4a4947c74b70fd039b02c39ac816b9b1` → `2` commits, matching the two task commits above).
