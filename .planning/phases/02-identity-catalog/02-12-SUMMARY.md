---
phase: 02-identity-catalog
plan: 12
subsystem: admin-ui
tags: [nextjs, tanstack-query, shadcn, cancellation-policy, effective-dated-pricing, admin-app, d-11, d-13, d-14]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-06)
    provides: "CatalogService.UpsertRoute/ListRoutes/ArchiveRoute/AddRoutePrice/ListRoutePrices RPCs, D-13 default policy, D-14 latest-effective-from price lookup, D-12 shared pier_to"
  - phase: 02-identity-catalog (plan 02-08)
    provides: "CatalogService.UpsertBoat/ListBoats/ArchiveBoat scoped by home_pier_id (D-07)"
  - phase: 02-identity-catalog (plan 02-09)
    provides: "apps/admin scaffold: rpc()/apiFetch, DataTable/ArchiveDialog, useWhoami, Dialog/Sheet form patterns"
provides:
  - "apps/admin's Routes page: one-way route CRUD with derived 'ท่า A → ท่า B' names, a D-13 cancellation-policy editor, effective-dated adult/child prices (add-only), a one-click return-route Sheet, and archive"
  - "apps/admin's Boats page: CRUD with home-pier scope, semantic active/maintenance status badges, and archive"
  - "money.ts: bahtToSatang/formatSatang — the one baht<->satang conversion this admin app uses for money input"
affects: [04]

# Actuals (#2632)
actuals:
  tokens: 11150
  tasks: 3
  commits: 3
plan_head_before: ea54afb9e6ec662b008de7315ce4e3adb5557978

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "RouteSheet accepts an optional `returnOf` prop and renders itself recursively for the 'return route' shortcut instead of a second component — one form definition, pre-filled with swapped piers (D-11)"
    - "money.ts imports nothing and uses only string/BigInt arithmetic (never parseFloat/Number on the amount) so baht<->satang conversion is provably float-free, mirroring pkg/money's Go convention"
    - "Boat status badges use explicit green/orange Tailwind classes on top of the Badge primitive rather than its default/secondary variants, since the UI-SPEC reserves the brand accent color for CTAs, never status"
    - "price-section.tsx trusts ListRoutePrices' server-side ordering (effective_from desc, 02-06) instead of re-sorting client-side"

key-files:
  created:
    - "apps/admin/src/app/(admin)/routes/page.tsx"
    - "apps/admin/src/app/(admin)/routes/route-sheet.tsx"
    - "apps/admin/src/app/(admin)/routes/queries.ts"
    - "apps/admin/src/app/(admin)/routes/policy-editor.tsx"
    - "apps/admin/src/app/(admin)/routes/price-section.tsx"
    - "apps/admin/src/app/(admin)/routes/money.ts"
    - "apps/admin/src/app/(admin)/boats/page.tsx"
    - "apps/admin/src/app/(admin)/boats/boat-dialog.tsx"
    - "apps/admin/src/app/(admin)/boats/queries.ts"
  modified: []

key-decisions:
  - "Return route implemented as RouteSheet(returnOf=route) rather than a separate component — the same create-mode form, pre-filled with pierFromId/pierToId swapped and the original duration/policy; recursion guards itself since a returnOf Sheet has isEdit=false and therefore never renders its own nested return-route button"
  - "A swapped return-route origin outside the caller's own piers is caught client-side (inline hint, save blocked) before the backend's NotFound round-trip, since the select can silently hold a value with no matching visible option"
  - "Route archive has no D-15-style blocked-state — ArchiveRoute has no analogous FailedPrecondition condition (routes are the leaf entity, nothing else references them) — so failures fall through to a plain toast like operators/boats"

patterns-established:
  - "money.ts: baht-string -> satang-string conversion via regex + BigInt, and satang-string -> ฿-formatted display, both with zero imports so the file compiles standalone and is trivially unit-testable"

requirements-completed: [CAT-03, CAT-04, CAT-05]

coverage:
  - id: D1
    description: "pier_admin creates/edits a one-way route from a Sheet (pier_from from own piers, pier_to from public piers spanning any operator, D-12); the list shows the derived 'ท่า A → ท่า B' name with no stored route name (D-17); already_exists on a duplicate pair renders 'มีเส้นทางนี้อยู่แล้ว'"
    requirement: CAT-03
    verification:
      - kind: other
        ref: "npm --prefix apps/admin run build (route /routes present) + grep route-sheet.tsx for UpsertRoute/already_exists + grep queries.ts for /api/v1/public/piers"
        status: pass
    human_judgment: true
    rationale: "The actual create/edit round-trip through Kong, pier_to spanning another operator, and the derived-name rendering need a live stack and a browser — deferred to Task 3's end-of-phase human-check (human_verify_mode=end-of-phase)."
  - id: D2
    description: "Cancellation-policy editor starts from the D-13 default (>24h 100% / 2-24h 50% / <2h 0%), lets tiers be added/removed (remove hidden on the last tier), and blocks save with the exact UI-SPEC copy when hours don't strictly descend or the 0-hour tier is missing — validatePolicy mirrors services/catalog's ValidateCancellationPolicy exactly"
    requirement: CAT-03
    verification:
      - kind: other
        ref: "grep policy-editor.tsx for the validation copy and `tiers.length > 1` remove-button guard; npm run build/typecheck/lint/format:check all clean"
        status: pass
    human_judgment: true
    rationale: "Triggering the invalid-order and missing-0-tier states interactively, and confirming the disabled-save UX, need a live browser session — deferred to the end-of-phase human-check."
  - id: D3
    description: "Per route, the admin sees price history newest-effective-first (server-ordered) and adds adult/child prices in baht converted to integer satang without floating-point arithmetic (bahtToSatang), with an effective-from date that cannot be before today (Asia/Bangkok); there is no edit-price action, only new effective-dated rows (D-14)"
    requirement: CAT-05
    verification:
      - kind: unit
        ref: "standalone tsc --target es2020 compile of money.ts + node assertions: bahtToSatang('150.5')==='15050', bahtToSatang('150.555')===null, bahtToSatang('100000.01')===null (exceeds 10,000,000 satang), formatSatang('15050')==='฿150.50'"
        status: pass
      - kind: other
        ref: "grep price-section.tsx for AddRoutePrice/ยังไม่ได้ตั้งราคา and absence of any edit-price action"
        status: pass
    human_judgment: false
  - id: D4
    description: "'สร้างเส้นทางย้อนกลับ' opens a new-route Sheet pre-filled with pier_from/pier_to swapped and the same duration and policy (prices are not copied); if the swapped origin is outside the caller's own piers, the origin select shows an inline hint and blocks save"
    requirement: CAT-03
    verification:
      - kind: other
        ref: "grep route-sheet.tsx/page.tsx for สร้างเส้นทางย้อนกลับ and the originOutOfScope guard"
        status: pass
    human_judgment: true
    rationale: "Confirming the swapped Sheet opens correctly and the out-of-scope hint appears needs a live stack with two operators' piers — deferred to the end-of-phase human-check."
  - id: D5
    description: "pier_admin lists, creates, edits and archives boats (name, default capacity, status active/maintenance, home pier restricted to their own piers, D-07); status shows badge ใช้งาน (green) / ซ่อมบำรุง (orange) using semantic colors, never the brand accent"
    requirement: CAT-04
    verification:
      - kind: other
        ref: "npm --prefix apps/admin run build (route /boats present) + grep boat-dialog.tsx for homePierId/UpsertBoat/save-failure copy + grep page.tsx for ซ่อมบำรุง/ArchiveBoat"
        status: pass
    human_judgment: true
    rationale: "The create/edit/archive round-trip through Kong and the visual badge colors at 768px need a live stack and a browser — deferred to the end-of-phase human-check."

# Metrics
duration: 17min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 12: Admin Routes and Boats Pages Summary

**Admin Routes page (one-way routes, D-13 cancellation-policy editor, D-14 effective-dated prices, D-11 return-route shortcut, archive) and Boats page (home-pier scope, semantic status badges, archive), on top of the 02-06/02-08 catalog backend and the 02-09 admin scaffold.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-09-26T20:53:00Z
- **Completed:** 2026-09-26T21:10:00Z
- **Tasks:** 3 (1 tracer, 2 auto)
- **Files modified:** 9 (9 created, 0 modified)

## Accomplishments

- `routes/queries.ts` + `route-sheet.tsx` + `page.tsx`: full route CRUD Sheet — `pier_from` restricted to the caller's own non-archived piers (`ListPiers` via the admin proxy), `pier_to` open to any non-archived pier of any operator via the public `/api/v1/public/piers` projection (D-12); the list derives `ท่า A → ท่า B` names from both pier lists (D-17), shows duration, current adult/child prices, and status; `already_exists` on a duplicate pair renders "มีเส้นทางนี้อยู่แล้ว"
- `policy-editor.tsx`: tier editor starting from the D-13 default `[{24,100},{2,50},{0,0}]`, add/remove tiers (remove hidden on the last remaining one), and `validatePolicy` mirroring `services/catalog/internal/domain/route.go`'s `ValidateCancellationPolicy` exactly (1-10 tiers, strictly descending hours ≥ 0, a 0-hour tier required, 0-100%) — invalid input shows the exact UI-SPEC copy and disables save
- `money.ts`: `bahtToSatang`/`formatSatang` using only regex + `BigInt` string arithmetic (no `parseFloat`/`Number()` on the amount, no imports) — bounded to the same 0-10,000,000 satang range `services/catalog`'s `RoutePrice.Validate` enforces server-side
- `price-section.tsx`: price history rendered in the server's own newest-first order (02-06's `ListRoutePrices`), an add-only form (`AddRoutePrice`) with a ticket-type select, baht amount converted through `bahtToSatang`, and an effective-from date whose `min` is today's Asia/Bangkok date — no edit or delete action exists anywhere in this file
- Return route: `RouteSheet` accepts an optional `returnOf` prop and renders itself recursively — a create-mode Sheet pre-filled with `pierFromId`/`pierToId` swapped and the original duration/policy (prices are never copied); if the swapped origin isn't one of the caller's own piers, an inline hint blocks save before the backend's `NotFound` round-trip
- Route archive: `ArchiveDialog` (entity "เส้นทาง") wired to `ArchiveRoute`, with a dedicated row-level "สร้างเส้นทางย้อนกลับ" action alongside edit/archive
- `boats/queries.ts` + `boat-dialog.tsx` + `page.tsx`: boat CRUD Dialog (name 1-100, capacity 1-1000, status, home pier restricted to the caller's own non-archived piers per D-07) with skeleton fields on edit and the exact save-failure copy; the list shows semantic green (`ใช้งาน`)/orange (`ซ่อมบำรุง`) status badges — explicit Tailwind classes, not the Badge primitive's brand-colored default variant — plus archive wiring (`ArchiveBoat`) and write actions hidden for `staff`

## Task Commits

1. **Task 1 (tracer): Routes page end-to-end (list, Sheet, UpsertRoute)** — `49cadcc` (feat)
2. **Task 2 (auto, tdd): Policy editor, prices, return route, archive** — `f9119a3` (feat)
3. **Task 3 (auto): Boats page (home pier, status, archive)** — `a68e2ef` (feat)

**Plan metadata:** committed separately after this SUMMARY.

_Note: `workflow.tdd_mode` is disabled for this project, so Task 2's tests (money.ts's conversion cases, policy-editor.tsx's validation cases) and implementation were written and verified together in one pass, matching the process note already established in 02-06/02-08/02-09/02-11's SUMMARYs._

## Files Created/Modified

- `apps/admin/src/app/(admin)/routes/{page,route-sheet,queries}.tsx` — list, form, hooks
- `apps/admin/src/app/(admin)/routes/policy-editor.tsx` — tier editor + `validatePolicy`
- `apps/admin/src/app/(admin)/routes/price-section.tsx` — price history + `AddRoutePrice` form
- `apps/admin/src/app/(admin)/routes/money.ts` — `bahtToSatang`/`formatSatang`
- `apps/admin/src/app/(admin)/boats/{page,boat-dialog,queries}.tsx` — list, form, hooks

## Decisions Made

See frontmatter `key-decisions` — the recursive `RouteSheet(returnOf=...)` shape for return routes, the client-side out-of-scope-origin guard, and routes having no D-15-style blocked-archive state (nothing references a route, unlike a pier's routes).

## Deviations from Plan

None — plan executed as written. All `must_haves.truths`, `key_links`, and `prohibitions` from the plan frontmatter are satisfied by the implementation; the `must_haves.artifacts[].contains` literals that landed in a composed dialog/section file rather than the listed page file (e.g. `UpsertBoat` lives in `boat-dialog.tsx`, not `boats/page.tsx`) follow the exact same page/dialog split already established and accepted in 02-09's `operators/page.tsx` + `operator-dialog.tsx` and 02-11's `piers/page.tsx` + `pier-sheet.tsx` — not a new pattern, and not flagged as a deviation there either.

## Issues Encountered

- `apps/admin`'s `tsconfig.json` targets `ES2017`, which rejects BigInt literal syntax (`100n`) even though `lib` includes `esnext` — every BigInt value in `money.ts` and `page.tsx` is constructed via `BigInt(100)` instead of a literal, satisfying both the app-wide `tsc --noEmit` (ES2017) and the plan's own standalone `tsc --target es2020` verify command for `money.ts`.

## Known Stubs

None — every component wired to a real backend call; no placeholder data.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `CAT-03`, `CAT-04`, `CAT-05` are marked complete in this plan's frontmatter `requirements`; `CAT-03`/`CAT-05` were also declared by sibling plan `02-06` (already summarized) and `CAT-04` by `02-08` (already summarized) — `requirements.ready-ids` marks them complete now that every declaring plan has a SUMMARY.
- The end-of-phase human-check (Task 3's `<human-check>`) is deferred per `human_verify_mode: end-of-phase` — the full round-trip (create a route to another operator's pier, invalid-then-fixed policy, adult/child prices, return route, archive; create/maintain/archive a boat; 768px layout) needs a live `make up` stack and a browser; harvested into the phase `UAT.md`.
- No blockers. `npm --prefix apps/admin run lint/format:check/typecheck/build` all green; the standalone `money.ts` conversion check passes all specified cases including the two boundary rejections (`150.555` — 3 fraction digits, `100000.01` — exceeds 10,000,000 satang).
- This was the last of the three wave-5 admin plans (02-09/02-11/02-12) sharing the `apps/admin` scaffold — 02-13 (staff users) is the remaining plan in this phase.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-26*

## Self-Check: PASSED

- All 9 key created files verified present on disk (`[ -f ]`): `routes/{page,route-sheet,queries,policy-editor,price-section,money}.tsx/.ts`, `boats/{page,boat-dialog,queries}.tsx/.ts`.
- All 3 task commits (`49cadcc`, `f9119a3`, `a68e2ef`) verified present in `git log --oneline --all`.
- All three tasks' acceptance criteria re-run and confirmed: Task 1 — `npm run build` lists `/routes`, `queries.ts` contains `/api/v1/public/piers`, `route-sheet.tsx` contains `UpsertRoute`, `page.tsx` contains `สร้างเส้นทางใหม่` and renders `→`; Task 2 — `money.ts` contains `bahtToSatang` with no `parseFloat`/`Number(` conversion of the amount (standalone `tsc --target es2020` compile + node assertions all pass), `policy-editor.tsx` contains the validation copy and the `tiers.length > 1` remove-button guard, `price-section.tsx` contains `AddRoutePrice`/`ยังไม่ได้ตั้งราคา` with no edit-price action, `route-sheet.tsx` contains `สร้างเส้นทางย้อนกลับ`; Task 3 — `npm run build` lists `/boats`, `boat-dialog.tsx` contains `homePierId`/`UpsertBoat`/the save-failure copy, `page.tsx` contains `ซ่อมบำรุง`/`ArchiveBoat`.
- Plan-level `<verification>` re-run: `npm --prefix apps/admin run lint/format:check/typecheck/build` all green (routes lists `/boats`, `/routes`, `/login`, `/operators`, `/piers`); the money conversion check is green.
