---
phase: 01-platform-foundation
plan: 05
subsystem: ui
tags: [nextjs, next-intl, tanstack-query, shadcn, radix-ui, protobuf-es, tailwind, prettier, docker-compose]

requires:
  - phase: 01-platform-foundation
    provides: "gen/ts protobuf-es types (plan 01/02), Kong gateway + gateway service (plan 01)"
provides:
  - "apps/web Next.js 16 skeleton: TH/EN locale routing, IBM Plex Sans Thai, mobile-first layout"
  - "Single apiFetch() Kong-only data path, typed from gen/ts, called only from a Client Component"
  - "shadcn/ui Card + Button primitives with a hand-written cn() helper (clsx + tailwind-merge)"
  - "docker compose web profile service for `make up --profile web` on :3001"
affects: [01-11-catalog-public-api, 01-12-e2e-walking-skeleton, ui-phases]

actuals:
  tokens: 119059
  tasks: 3
  commits: 2
  plan_head_before: 95a167410fc22533a342166b47fb6e191de50df9

tech-stack:
  added:
    - "next-intl 4.14.7 — TH/EN routing, messages, navigation helpers"
    - "@tanstack/react-query 5.103.2 — client-side data fetching (staleTime 0)"
    - "@bufbuild/protobuf 2.15.0 — runtime for gen/ts protobuf-es types"
    - "shadcn 4.21.0 (radix-nova style) + radix-ui 1.6.7 — Card/Button primitives"
    - "clsx 2.1.1 + tailwind-merge 3.7.0 — hand-written cn() helper (replaces the rejected `cn` npm package)"
    - "class-variance-authority 0.7.1, lucide-react 1.48.0, tw-animate-css 1.4.0 — shadcn support libs"
    - "prettier 3.9.9 + eslint-config-prettier 10.1.8 — formatting"
  patterns:
    - "Single apiFetch<T>() helper is the only network call site; Kong base URL from NEXT_PUBLIC_API_URL, credentials 'include'"
    - "All data fetching lives in Client Components via TanStack Query; Server Components render shell only"
    - "next-intl navigation helpers (createNavigation) wrap Link/useRouter/usePathname for locale-aware routing"
    - "shadcn components import cn from '@/lib/utils', never from an npm 'cn' package"

key-files:
  created:
    - apps/web/src/lib/api.ts
    - apps/web/src/lib/utils.ts
    - apps/web/src/components/boat-list.tsx
    - apps/web/src/components/locale-switcher.tsx
    - apps/web/src/components/ui/button.tsx
    - apps/web/src/components/ui/card.tsx
    - apps/web/src/i18n/routing.ts
    - apps/web/src/i18n/request.ts
    - apps/web/src/i18n/navigation.ts
    - apps/web/src/proxy.ts
    - apps/web/messages/th.json
    - apps/web/messages/en.json
    - apps/web/components.json
    - apps/web/.prettierrc.json
    - apps/web/.prettierignore
  modified:
    - apps/web/package.json
    - apps/web/package-lock.json
    - apps/web/tsconfig.json
    - apps/web/eslint.config.mjs
    - apps/web/next.config.ts
    - apps/web/postcss.config.mjs
    - apps/web/src/app/globals.css
    - "apps/web/src/app/[locale]/layout.tsx"
    - "apps/web/src/app/[locale]/page.tsx"
    - "apps/web/src/app/[locale]/providers.tsx"
    - deploy/docker-compose.yml

key-decisions:
  - "Rejected the `cn` npm package (unaudited, flagged mid-run); hand-wrote the standard shadcn cn() helper with the already-approved clsx + tailwind-merge instead"
  - "Approved radix-ui but pinned it exact (1.6.7, --save-exact), matching Task 2's exact-pin convention"
  - "Exact-pinned class-variance-authority, lucide-react, tw-animate-css, and shadcn itself (removed all `^` ranges) for consistency with Task 2"

patterns-established:
  - "shadcn init/add commands must be followed by a grep for `from \"cn\"` — the registry templates default to importing the npm `cn` package per-component, not just at init time"

requirements-completed: [PLAT-09]

coverage:
  - id: D1
    description: "Package legitimacy gate — SUS-flagged npm packages (next-intl, @tanstack/react-query, @bufbuild/protobuf, tailwind-merge, lucide-react, prettier) confirmed by the developer before install"
    requirement: "PLAT-09"
    verification: []
    human_judgment: true
    rationale: "Human npm-registry verification is the point of this gate; the developer replied 'approved' for all six packages (see Deviations)"
  - id: D2
    description: "TH/EN locale routing, IBM Plex Sans Thai font, TanStack Query -> apiFetch() -> Kong data path, typed from gen/ts"
    requirement: "PLAT-09"
    verification:
      - kind: other
        ref: "npm --prefix apps/web run typecheck"
        status: pass
      - kind: other
        ref: "npm --prefix apps/web run build"
        status: pass
    human_judgment: true
    rationale: "Font rendering, TH/EN visual output, and Kong-only network origin need a browser; deferred to the Task 3 <human-check> harvested into the phase UAT at end of phase (public boats route lands in plan 11)"
  - id: D3
    description: "shadcn Card/Button boat list with refresh, locale switcher wired into the page header"
    requirement: "PLAT-09"
    verification:
      - kind: other
        ref: "npm --prefix apps/web run lint"
        status: pass
      - kind: other
        ref: "npm --prefix apps/web run build"
        status: pass
    human_judgment: true
    rationale: "Same end-of-phase browser check as D2 covers visual layout, touch-target size and locale switching"
  - id: D4
    description: "Prettier 3.9.9 + eslint-config-prettier formatting wired into lint/format scripts"
    verification:
      - kind: other
        ref: "npm --prefix apps/web run format:check"
        status: pass
    human_judgment: false
  - id: D5
    description: "docker-compose web profile service (node:24-alpine, bind mount, 127.0.0.1:3001)"
    verification:
      - kind: other
        ref: "docker compose -f deploy/docker-compose.yml --profile web config --format json | jq -e '.services.web.ports[0].published == \"3001\" and .services.web.image == \"node:24-alpine\"'"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-26
status: complete
---

# Phase 01 Plan 05: Next.js 16 TH/EN Skeleton with Kong-only Data Path Summary

**shadcn Card/Button boat list, next-intl locale switcher, Prettier and a docker-compose `web` profile complete the Kong-only, gen/ts-typed Next.js 16 skeleton started in Task 2.**

## Performance

- **Duration:** 55 min (across two executor sessions, separated by two checkpoints)
- **Started:** 2026-09-26T03:34:00Z (approx, Task 1/2 session)
- **Completed:** 2026-09-26T04:29:18Z
- **Tasks:** 3 (1 checkpoint, 2 code tasks)
- **Files modified:** 21 (this session) + 17 (Task 2 session)

## Accomplishments

- Package legitimacy gate cleared: developer approved next-intl, @tanstack/react-query, @bufbuild/protobuf, tailwind-merge, lucide-react, prettier at their audited versions
- `/th` and `/en` render through next-intl routing with IBM Plex Sans Thai loaded via `next/font/google`, and the font binding now actually reaches `body`/`html` (see Deviations — shadcn init had silently broken this)
- Single `apiFetch()` helper is the only network call site, pointed at Kong via `NEXT_PUBLIC_API_URL`, typed from `gen/ts` `ListBoatsResponseJson`
- Boat list renders as shadcn `Card`s with a 44px-tall retry `Button`; a `LocaleSwitcher` toggles `/th` ↔ `/en` via next-intl navigation helpers
- Removed the unaudited `cn` npm package (flagged twice — once from `shadcn init`, once again from `shadcn add card`) and replaced it with a hand-written `cn()` using the already-approved `clsx` + `tailwind-merge`
- Prettier + eslint-config-prettier wired in; `deploy/docker-compose.yml` gained a `web` profile service serving the app on `127.0.0.1:3001`

## Task Commits

1. **Task 1: Package legitimacy gate** — no commit (checkpoint; developer replied "approved" for next-intl, @tanstack/react-query, @bufbuild/protobuf, tailwind-merge, lucide-react, prettier)
2. **Task 2: /th page → Client Component → TanStack Query → apiFetch() → Kong URL, i18n, Thai font, gen/ts types** - `e95449d` (feat)
3. **Task 3: shadcn Card/Button boat cards with refresh, locale switcher, Prettier, compose web profile** - `5038232` (feat)

_Note: Task 3 was interrupted mid-run by a second blocking-human checkpoint (unaudited `cn`/`radix-ui` packages surfaced by `shadcn init`); this SUMMARY covers the full plan after the developer's checkpoint resolution was applied._

## Files Created/Modified

- `apps/web/src/lib/api.ts` - `apiFetch<T>()`, the single Kong call site
- `apps/web/src/lib/utils.ts` - hand-written `cn()` via clsx + tailwind-merge (replaces the `cn` npm package)
- `apps/web/src/components/boat-list.tsx` - Client Component: Card-based boat list, loading/error/empty states, refresh Button
- `apps/web/src/components/locale-switcher.tsx` - `/th` ↔ `/en` toggle using next-intl navigation helpers
- `apps/web/src/components/ui/button.tsx`, `card.tsx` - shadcn primitives (radix-nova style), both importing `cn` from `@/lib/utils`
- `apps/web/src/i18n/routing.ts`, `request.ts`, `navigation.ts` - next-intl routing, message loading, and `createNavigation()` helpers
- `apps/web/src/proxy.ts` - next-intl middleware (Next.js 16 `proxy.ts`)
- `apps/web/src/app/[locale]/layout.tsx` - IBM Plex Sans Thai font, `NextIntlClientProvider`, `Providers`
- `apps/web/src/app/globals.css` - shadcn theme tokens, primary retargeted to `#0B6E99`, `--font-sans`/`--font-heading` fixed to `var(--font-plex-thai)`
- `apps/web/package.json` / `package-lock.json` - all shadcn/next-intl/query/prettier deps pinned exact
- `deploy/docker-compose.yml` - `web` service (profile `web`, `node:24-alpine`, bind mount, `127.0.0.1:3001`)

## Decisions Made

- **Rejected `cn` npm package** (unaudited) in favor of a hand-written `clsx` + `tailwind-merge` helper — both libraries were already approved/OK-verdict from Task 1's audit
- **Approved `radix-ui`**, pinned exact at `1.6.7`
- **Exact-pinned** `class-variance-authority`, `lucide-react`, `tw-animate-css`, and `shadcn` itself (removed all `^` ranges) to match Task 2's `--save-exact` convention

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking, human-directed] Removed unaudited `cn` npm package, twice**
- **Found during:** Task 3 (`shadcn init` first, then again after `shadcn add card`)
- **Issue:** Both `shadcn init` and `shadcn add card` template their generated components (`button.tsx`, `card.tsx`, `utils.ts`) to `import { cn } from "cn"` — a separate npm package the legitimacy audit had not seen and the developer explicitly rejected at the first checkpoint
- **Fix:** `npm uninstall cn` (run twice — `shadcn add card` re-added it); hand-wrote `apps/web/src/lib/utils.ts` using the approved `clsx@2.1.1` + `tailwind-merge@3.7.0`; repointed `button.tsx` and `card.tsx` imports to `@/lib/utils`; grepped for any remaining `from "cn"` reference (none found)
- **Files modified:** apps/web/src/lib/utils.ts, apps/web/src/components/ui/button.tsx, apps/web/src/components/ui/card.tsx, apps/web/package.json, apps/web/package-lock.json
- **Verification:** `grep -rn 'from "cn"' apps/web/src` returns nothing; `grep -n '"cn"' apps/web/package.json` returns nothing; build/typecheck/lint all pass
- **Committed in:** `5038232` (Task 3 commit)

**2. [Rule 1 - Bug] Restored the Thai font binding after `shadcn init` overwrote it**
- **Found during:** Task 3 (`shadcn init` rewrote `globals.css`)
- **Issue:** `shadcn init`'s default `@theme inline` block set `--font-sans: var(--font-sans)` (a self-reference) and `--font-heading: var(--font-sans)`, silently discarding Task 2's `--font-sans: var(--font-plex-thai)` binding. Since `body { @apply font-sans }` resolves through this token, Thai text would have fallen back to the system sans-serif font instead of IBM Plex Sans Thai — breaking the D-34 must-have truth
- **Fix:** Changed both `--font-sans` and `--font-heading` back to `var(--font-plex-thai)` in `apps/web/src/app/globals.css`
- **Files modified:** apps/web/src/app/globals.css
- **Verification:** `grep -n "font-plex-thai" apps/web/src/app/globals.css "apps/web/src/app/[locale]/layout.tsx"` shows the variable defined in the layout and consumed in globals.css; build passes
- **Committed in:** `5038232` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking/human-directed package rejection, 1 bug from a third-party scaffold tool overwriting prior work)
**Impact on plan:** Both fixes were necessary for correctness (no unaudited runtime dependency, Thai font actually renders). No scope creep — no unrequested features added.

## Issues Encountered

None beyond the two checkpoints already documented (package legitimacy gate, and the `cn`/`radix-ui` blocking-human checkpoint resolved by the developer between sessions).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `apps/web` builds, lints, type-checks and formats cleanly; `docker compose --profile web config` validates the `web` service
- The boat list's data path (`apiFetch` → Kong → `/api/v1/public/boats`) is wired and typed, but the route itself doesn't exist until plan 11 (catalog public API) — until then the page shows its error state, which is expected per this plan's objective
- The full browser-based human check (TH/EN, Thai font rendering, 375px layout, Kong-only network calls) is deferred to the end-of-phase UAT per `workflow.human_verify_mode: end-of-phase`, to run after `make up && make proof`

---
*Phase: 01-platform-foundation*
*Completed: 2026-09-26*

## Self-Check: PASSED

All created files found on disk (api.ts, utils.ts, boat-list.tsx, locale-switcher.tsx, button.tsx, card.tsx, navigation.ts, SUMMARY.md). All commits found in git log (e95449d, 5038232, 7af9314).
