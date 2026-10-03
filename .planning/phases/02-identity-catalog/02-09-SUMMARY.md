---
phase: 02-identity-catalog
plan: 09
subsystem: admin-ui
tags: [nextjs, tanstack-query, shadcn, otp, kong, admin-app]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-05)
    provides: "Browser cookie session routes (POST /api/v1/auth/otp/request, otp/verify, refresh, logout) and GET /api/v1/whoami"
  - phase: 02-identity-catalog (plan 02-04)
    provides: "POST /api/v1/admin/{service}/{method} allow-listed reverse proxy (CatalogService/UserService) that apps/admin's rpc() calls through"
  - phase: 02-identity-catalog (plan 02-03)
    provides: "CatalogService UpsertOperator/ListOperators/ArchiveOperator RPCs the operators page renders and mutates"
provides:
  - "apps/admin: Thai-only Next.js 16 admin app on :3002 (D-18), identical shadcn/Tailwind tokens to apps/web"
  - "apiFetch/rpc client with one-shot POST /api/v1/auth/refresh retry on 401 and a typed {code, message, attemptsLeft} error shape"
  - "OtpLogin two-step flow, (admin) whoami-guarded layout + role-filtered AdminShell nav, and a shared DataTable/ArchiveDialog pattern every later entity page reuses"
  - "First complete entity page: operators (list/create/edit/archive) proving the whole admin pattern end-to-end"
affects: [02-11, 02-12, 02-13]

# Actuals (#2632)
actuals:
  tokens: 25899
  tasks: 3
  commits: 3
plan_head_before: 92d1bc61795253bc2b43273961a00cb1146e790c

# Tech tracking
tech-stack:
  added:
    - "input-otp 1.5.0 (admin OTP entry)"
    - "sonner 2.0.8 (admin toasts)"
    - "next-themes 0.4.6 (transitive dep of shadcn's sonner block, no dark-mode toggle used)"
  patterns:
    - "apps/admin copies apps/web's shadcn preset/tokens byte-for-byte instead of re-deriving them from the b2fA preset a second time (globals.css is cmp-identical)"
    - "One shared module-level refresh promise in lib/api.ts collapses concurrent 401s into a single POST /api/v1/auth/refresh, then retries each caller's original request once"
    - "DataTable<T> + ArchiveDialog in src/components/ are the one reusable list/confirm pair every future entity page (piers, routes, boats, staff) wires against, not a per-entity reimplementation"

key-files:
  created:
    - apps/admin/package.json
    - apps/admin/src/lib/api.ts
    - apps/admin/src/lib/queries.ts
    - apps/admin/src/components/otp-login.tsx
    - apps/admin/src/components/admin-shell.tsx
    - apps/admin/src/components/data-table.tsx
    - apps/admin/src/components/archive-dialog.tsx
    - apps/admin/src/components/operator-dialog.tsx
    - apps/admin/src/app/login/page.tsx
    - "apps/admin/src/app/(admin)/layout.tsx"
    - "apps/admin/src/app/(admin)/operators/page.tsx"
    - apps/admin/src/components/ui/*.tsx (input-otp, field, label, input, textarea, select, native-select, dialog, sheet, alert-dialog, table, badge, empty, skeleton, sonner, separator, tooltip, spinner)
  modified:
    - Makefile
    - Jenkinsfile
    - deploy/docker-compose.yml

key-decisions:
  - "shadcn add pulled every generated ui/*.tsx component's cn import from a separate, unaudited 'cn' npm package (same failure mode Phase 1 hit and rejected at a blocking-human checkpoint) — rewrote every import to @/lib/utils and removed 'cn' from package.json/package-lock.json before continuing; no checkpoint needed since the fix reuses Phase 1's already-approved pattern, not a new package decision"
  - "operator-dialog.tsx skips a separate GetOperator fetch/skeleton for edit mode: the row data is already in memory from the ListOperators query that rendered the table, so there is no async gap to cover — later entity dialogs (piers/routes) that DO need their own fetch can add the skeleton state then"
  - "AdminShell's 'collapsible to top bar on narrow screens' is a Tailwind hidden md:flex / md:hidden pair (sidebar vs. horizontal top nav), not a Sheet-based hamburger drawer — no acceptance criterion asked for toggle state, and both nav renderings share the same NavLinks list"

requirements-completed: [CAT-01]  # AUTH-01/AUTH-02 also declared by sibling plans 02-10/02-13 without a SUMMARY yet; requirements.ready-ids gates them until those plans finish

coverage:
  - id: D1
    description: "apps/admin scaffolded as a separate Thai-only Next.js 16 app (no next-intl, no locale prefix) on :3002, with byte-identical shadcn/Tailwind tokens to apps/web and no @tanstack/react-table dependency"
    requirement: AUTH-02
    verification:
      - kind: other
        ref: "npm --prefix apps/admin run build (routes /login, /operators listed) + cmp apps/web/src/app/globals.css apps/admin/src/app/globals.css"
        status: pass
    human_judgment: false
  - id: D2
    description: "OtpLogin two-step flow (destination -> input-otp) with UI-SPEC copy for wrong-code/expired/rate-limited errors, resend cooldown, and disabled-until-valid buttons at every step"
    requirement: AUTH-02
    verification:
      - kind: other
        ref: "grep-verified copy strings in otp-login.tsx (all four error/cooldown strings present) + npm run build/typecheck clean"
        status: pass
    human_judgment: true
    rationale: "Actual OTP round-trip (code from Mailpit, cookie issuance, redirect) requires the live docker compose stack and a browser — deferred to Task 3's end-of-phase human-check per human_verify_mode=end-of-phase"
  - id: D3
    description: "(admin) layout guards on useWhoami: redirects to /login on a 401, shows a no-access card for role=customer, otherwise renders the role-filtered AdminShell (super_admin sees all 5 nav items, pier_admin/staff see 3)"
    requirement: AUTH-02
    verification:
      - kind: other
        ref: "npm --prefix apps/admin run typecheck/build clean; role-filter logic in admin-shell.tsx NAV_ITEMS"
        status: pass
    human_judgment: true
    rationale: "Role-based nav rendering and the 401-redirect need a live whoami response from Kong/gateway to observe — end-of-phase human-check"
  - id: D4
    description: "super_admin lists, creates, edits and archives operators through the shared DataTable + OperatorDialog + ArchiveDialog; a failed_precondition ArchiveOperator response renders inline under the table instead of a toast; write actions are hidden for non-super_admin roles"
    requirement: CAT-01
    verification:
      - kind: other
        ref: "grep-verified rpc(...UpsertOperator/ArchiveOperator...) call sites + UI-SPEC copy strings (archive/save-failure/empty-state text) present in the respective components; npm run build/typecheck/lint/format:check all clean"
        status: pass
    human_judgment: true
    rationale: "The actual CRUD round-trip against catalog through Kong, and the visual states (skeleton/empty/error/archive-confirm) at 768px/360px, need a live stack and a browser — end-of-phase human-check (Task 3)"
  - id: D5
    description: "apps/admin is gated by the same make lint/web-check checks as apps/web, the Jenkins Tools stage installs its dependencies, and a compose admin service serves it on 127.0.0.1:3002 under the web profile"
    requirement: AUTH-02
    verification:
      - kind: other
        ref: "make web-check (apps/web + apps/admin format:check/build all green); isolated compose-config check on the extracted admin service block confirms ports[0].published==3002 and image==node:24-alpine"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-09-27
status: complete
---

# Phase 2 Plan 9: Admin App Scaffold, OTP Login and Operators CRUD Summary

**A Thai-only Next.js 16 admin app on :3002 with a two-step OTP login, a whoami-guarded role-filtered shell, and the first complete entity page (operators) built on a shared DataTable/ArchiveDialog pattern every later admin page reuses.**

## Performance

- **Duration:** 35 min
- **Started:** 2026-09-26T19:31:00Z
- **Completed:** 2026-09-26T20:06:24Z
- **Tasks:** 3 (1 tracer, 2 auto)
- **Files modified:** 51 (48 created, 3 modified)

## Accomplishments

- `apps/admin` scaffolded by copying `apps/web`'s config/tokens (not `create-next-app`): `package.json` named `admin` with `dev`/`start` on `:3002`, dependencies identical minus `next-intl` (Thai only, D-18), `next.config.ts` without the i18n plugin wrapper, `tsconfig.json`/eslint/postcss/prettier copied, `globals.css` byte-identical (`cmp` clean) to `apps/web`. 18 shadcn components installed (`input-otp`, `field`, `label`, `input`, `textarea`, `select`, `native-select`, `dialog`, `sheet`, `alert-dialog`, `table`, `badge`, `empty`, `skeleton`, `sonner`, `separator`, `tooltip`, `spinner`) so plans 02-11..13 never touch `package.json` except for `maplibre-gl`.
- `lib/api.ts`: `apiFetch<T>` adds credentialed fetch + a single shared in-flight `POST /api/v1/auth/refresh` retried once on any 401 outside `/api/v1/auth/*`, and throws a typed `{status, code, message, attemptsLeft}` error from a non-2xx body. `rpc<Res>(service, method, body)` posts JSON to `POST /api/v1/admin/{connectServiceName}/{method}`.
- `otp-login.tsx`: destination step ("อีเมลหรือเบอร์โทรศัพท์", disabled-until-filled "ส่งรหัส OTP") -> code step (6-slot `input-otp`, digits-only pattern, 60s resend cooldown, "ยืนยันรหัส" disabled until 6 digits) with the UI-SPEC's wrong-code/expired-locked/rate-limited copy mapped from the error's `code`/`status`.
- `(admin)/layout.tsx` + `admin-shell.tsx`: `useWhoami()`-guarded layout redirects to `/login` on 401, shows a no-access card for `role=customer`, otherwise renders `AdminShell` — a role-filtered nav (super_admin: all 5 entities; pier_admin/staff: piers/routes/boats only) that collapses from a left sidebar to a horizontal top bar under `md:`.
- `data-table.tsx` + `archive-dialog.tsx` + `operator-dialog.tsx`: the shared `DataTable<T>` (skeleton rows, error+retry, empty slot, muted archived rows, `TruncatedCell`/`Tooltip` for long text) and a reusable 44×44px `ArchiveDialog` confirm, both written once here for every later entity page. `operators/page.tsx` wires `ListOperators`/`UpsertOperator`/`ArchiveOperator` end-to-end, with a `failed_precondition` archive response rendered as an inline error under the table (never a toast) and write actions hidden for non-super_admin.
- Makefile `lint`/`web-check`, the Jenkins `Tools` stage, and a new `admin` compose service (profile `web`, `127.0.0.1:3002`, mirroring `web`'s bind-mount/`npm run dev` pattern) all gate/serve `apps/admin` the same way `apps/web` already is.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): Admin sign-in end-to-end scaffold — OTP login, whoami guard, read-only operators list** - `54af79f` (feat)
2. **Task 2 (auto): Operators CRUD with the shared DataTable, Dialog form and AlertDialog archive** - `040b396` (feat)
3. **Task 3 (auto): Wire apps/admin into CI and Compose** - `9f6a0a5` (chore)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `apps/admin/**` — new Next.js 16 admin app (config, `src/app`, `src/components`, `src/components/ui`, `src/lib`) — see frontmatter `key-files` for the non-boilerplate list
- `Makefile` — `lint`/`web-check` targets gain apps/admin's eslint/tsc/format/build steps
- `Jenkinsfile` — `Tools` stage runs `npm --prefix apps/admin ci`
- `deploy/docker-compose.yml` — new `admin` service (profile `web`, `127.0.0.1:3002`)

## Decisions Made

- `shadcn add` generated every `ui/*.tsx` component importing `cn` from a separate, unaudited npm package (`cn@0.2.6`, pulled transitively by the `shadcn` CLI itself) instead of `@/lib/utils` — the exact same failure Phase 1 hit and rejected at a blocking-human checkpoint. Rewrote every generated component's import to `@/lib/utils` and removed `cn` from `package.json`/`package-lock.json` before writing any app code. Not treated as a new checkpoint: this reuses Phase 1's already-approved resolution (hand-written `cn()` via `clsx`+`tailwind-merge`), not a new package-legitimacy decision.
- `operator-dialog.tsx` has no separate fetch-by-id for edit mode — the row is already in memory from the `ListOperators` query backing the table, so there's no async gap requiring a skeleton state. Later entity dialogs that need their own fetch (e.g. a pier Sheet with a map picker) can add that state then.
- `AdminShell`'s "collapsible to top bar on narrow screens" requirement is implemented as a `hidden md:flex` sidebar / `md:hidden` horizontal top nav pair sharing one `NavLinks` list, not a `Sheet`-based hamburger drawer — no acceptance criterion required toggle state.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed the unaudited 'cn' npm package pulled in by every `shadcn add` component**
- **Found during:** Task 1 (`npx shadcn@4.21.0 add ...`)
- **Issue:** Every generated `src/components/ui/*.tsx` file imported `cn` from a bare `cn` npm package (added to `package.json`/`package-lock.json` by the CLI itself), not the project's own `@/lib/utils` helper — the same supply-chain gap Phase 1 already identified and rejected via a blocking-human checkpoint (`STATE.md` decision log).
- **Fix:** `sed`-replaced every `from "cn"` import with `from "@/lib/utils"` across all 18 newly generated components, ran `npm uninstall cn`, and reformatted with `prettier --write` to match project style (single quotes, semicolons).
- **Files modified:** all files under `apps/admin/src/components/ui/`, `apps/admin/package.json`, `apps/admin/package-lock.json`
- **Verification:** `grep -rn 'from "cn"' apps/admin/src` returns nothing; `npm --prefix apps/admin run lint/typecheck/build` all clean
- **Committed in:** `54af79f` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 3 blocking/supply-chain fix)
**Impact on plan:** Necessary to avoid reintroducing a supply-chain risk the project already rejected once; no scope creep — the fix only touches shadcn-generated files and package metadata.

## Issues Encountered

- The harness's secret-file read guard blocks any Bash command that references `.env` by name, which is exactly how the Makefile's `$(COMPOSE)` variable invokes `docker compose` (`--env-file .env`). Running the plan's literal Task 3 compose-config check against the full `deploy/docker-compose.yml` was not possible in this session. Worked around it by extracting just the new `admin` service block (which has no `${VAR}` interpolation — its `ports`/`image`/`environment` are all literals) into an isolated compose file under the scratch directory and validating that in place: `docker compose -f <isolated-file> --profile web config --format json` confirms `services.admin.ports[0].published == "3002"` and `services.admin.image == "node:24-alpine"`. This proves the added YAML is syntactically correct and resolves as intended; it does not prove the full multi-service file still composes cleanly with `.env` populated, which the human should confirm via `make up` at the Task 3 end-of-phase check.

## User Setup Required

None — no new external service configuration required. `NEXT_PUBLIC_API_URL` for `apps/admin` reuses the same Kong URL already configured for `apps/web`.

## Next Phase Readiness

- CAT-01 is ready and marked complete in this plan's frontmatter `requirements` (both plans declaring it — 02-03 and 02-09 — now have a SUMMARY).
- AUTH-01 and AUTH-02 are NOT yet marked complete: AUTH-01 is also declared by 02-10 (no SUMMARY yet), AUTH-02 by 02-13 (no SUMMARY yet). `requirements.ready-ids` will mark them once those plans finish.
- The end-of-phase human-check (Task 3's `<human-check>`) is deferred per `human_verify_mode: end-of-phase` — sign-in via Mailpit, operators CRUD, and the 768px/360px responsive check all need a live `make up` stack and a browser; harvested into the phase `UAT.md`.
- 02-11/02-12/02-13 (piers, routes/boats, staff pages) can now build directly on `DataTable`, `ArchiveDialog`, `apiFetch`/`rpc`, `useWhoami`, and `AdminShell`'s nav — no further scaffold work needed, only new page folders and dialogs per the plan's own file-list note.
- No blockers. `npm --prefix apps/admin run lint/typecheck/format:check/build` and `make web-check` (apps/web + apps/admin) are all green. `make lint`'s pre-existing gosec findings in `services/gateway/internal/adapters/http/{auth,proxy}_test.go` (from plans 02-04/02-05) are unrelated to this plan's changes and out of scope per the deviation scope-boundary rule — not fixed here.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All key created files verified present on disk (`[ -f ]`): `apps/admin/package.json`, `apps/admin/src/lib/api.ts`, `apps/admin/src/lib/queries.ts`, `apps/admin/src/components/otp-login.tsx`, `apps/admin/src/components/admin-shell.tsx`, `apps/admin/src/components/data-table.tsx`, `apps/admin/src/components/archive-dialog.tsx`, `apps/admin/src/components/operator-dialog.tsx`, `apps/admin/src/app/login/page.tsx`, `apps/admin/src/app/(admin)/layout.tsx`, `apps/admin/src/app/(admin)/operators/page.tsx`.
- All 3 task commits (`54af79f`, `040b396`, `9f6a0a5`) verified present in `git log --oneline --all`.
- All three tasks' acceptance criteria re-run and confirmed: Task 1 — build lists `/login`/`/operators`, `package.json` has `"dev": "next dev -p 3002"` and no `@tanstack/react-table`, all named `ui/*.tsx` files exist, `layout.tsx` contains `lang="th"` and `IBM_Plex_Sans_Thai`; Task 2 — `operators/page.tsx` calls `rpc(` for `UpsertOperator`/`ArchiveOperator`, `archive-dialog.tsx` contains the archive body copy, `operator-dialog.tsx` contains the save-failure copy, lint/format:check/typecheck/build all exit 0; Task 3 — `Makefile` contains the admin typecheck/build lines, `Jenkinsfile` contains `npm --prefix apps/admin ci`, the isolated compose-config check prints `true`.
- Plan-level `<verification>` re-run clean: `npm --prefix apps/admin run lint/typecheck/build` and `make web-check` (apps/web + apps/admin) all green. The plan's own `make lint` Go-lint step surfaces pre-existing gosec findings in unrelated 02-04/02-05 test files (documented above, out of scope).
