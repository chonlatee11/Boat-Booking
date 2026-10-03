---
phase: 02-identity-catalog
plan: 10
subsystem: auth
tags: [next-intl, react-query, input-otp, apps-web, otp-login, session-refresh]

# Dependency graph
requires:
  - phase: 02-identity-catalog (plan 02-05)
    provides: gateway cookie routes POST /api/v1/auth/{otp/request,otp/verify,refresh,logout}, GET /api/v1/whoami
provides:
  - "apps/web /[locale]/login route: two-step OTP sign-in (destination -> 6-digit code) in TH/EN"
  - "apps/web AccountStatus header component: whoami-driven signed-in/sign-out vs sign-in link"
  - "apps/web apiFetch: one-shot refresh-on-401 retry with a single shared in-flight refresh promise, parsed status/code/message/attemptsLeft on thrown errors"
affects: [02-11, 02-13]

# Actuals (#2632)
actuals:
  tokens: 5575
  tasks: 2
  commits: 2
plan_head_before: 3aae7bbd4f0e2636a86a422a7356a1305e7f41a8

# Tech tracking
tech-stack:
  added:
    - "input-otp@1.5.0 (apps/web, exact pin) — 6-digit OTP entry, same version already vetted in apps/admin"
  patterns:
    - "apps/web's apiFetch is now byte-for-byte the same refresh-retry/parsed-error shape as apps/admin/src/lib/api.ts — no shared package (CONTEXT discretion, per-app copy), but the contract (ApiError with status/code/attemptsLeft, one shared in-flight refresh promise, one retry per request) is identical across both frontends"
    - "shadcn add in apps/web pulls the same unaudited 'cn' npm package apps/admin's 02-09 already rejected — every generated ui/*.tsx import rewritten to @/lib/utils, 'cn' dropped from package.json before installing, lockfile regenerated clean"

key-files:
  created:
    - apps/web/src/components/otp-login.tsx
    - apps/web/src/components/account-status.tsx
    - apps/web/src/app/[locale]/login/page.tsx
    - apps/web/src/components/ui/input-otp.tsx
    - apps/web/src/components/ui/input.tsx
    - apps/web/src/components/ui/label.tsx
    - apps/web/src/components/ui/spinner.tsx
  modified:
    - apps/web/src/lib/api.ts
    - apps/web/src/app/[locale]/page.tsx
    - apps/web/messages/th.json
    - apps/web/messages/en.json
    - apps/web/package.json
    - apps/web/package-lock.json

key-decisions:
  - "otp-login.tsx and api.ts follow apps/admin's exact proven shapes (otpErrorMessage mapping, refreshSession with a shared in-flight promise) rather than inventing a parallel implementation — the two apps' auth UX is now consistent"
  - "A failed_precondition (expired/locked code) returns the user to the destination step instead of leaving them stuck re-typing a dead code"
  - "All auth.* message keys (TH/EN) were added in Task 1 rather than split across Task 1/2 as the plan's file list implied, since the full copy set was already known from the UI-SPEC Copywriting Contract — Task 2 needed no message file changes"

requirements-completed: [AUTH-01]

coverage:
  - id: D1
    description: "auth.* TH/EN message keys match the UI-SPEC Copywriting Contract exactly (destination label, send/verify CTAs, resend countdown, wrong-code/expired/rate-limited errors)"
    requirement: AUTH-01
    verification:
      - kind: other
        ref: "node -e message-key-presence check (see PLAN.md Task 1 <verify>)"
        status: pass
    human_judgment: false
  - id: D2
    description: "apps/web builds with the /[locale]/login route present and TypeScript/ESLint/Prettier clean"
    requirement: AUTH-01
    verification:
      - kind: other
        ref: "npm --prefix apps/web run lint && run typecheck && run build (route /[locale]/login listed in output)"
        status: pass
    human_judgment: false
  - id: D3
    description: "apiFetch retries once after POST /api/v1/auth/refresh on a 401 for non-auth paths via one shared in-flight promise; non-2xx responses carry status/code/message/attemptsLeft"
    requirement: AUTH-01
    verification:
      - kind: other
        ref: "grep for /api/v1/auth/refresh, credentials: 'include', attemptsLeft, role=\"alert\" across apps/web/src/lib/api.ts and otp-login.tsx (PLAN.md Task 2 <verify>)"
        status: pass
    human_judgment: false
  - id: D4
    description: "A customer can sign in end-to-end at /th/login or /en/login with a real OTP code (Mailpit), see wrong-code attempts-left count down, get rate-limited/expired copy, use the 60s resend countdown, wrap a long email at 360px without overflow, and see the home header flip from sign-in link to signed-in+sign-out"
    requirement: AUTH-01
    verification: []
    human_judgment: true
    rationale: "Requires the live stack (make up), a real Mailpit-delivered code, a real browser at 360px width, and devtools inspection to confirm no token in Local Storage — exactly the plan's own Task 2 human-check, deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase"

# Metrics
duration: 7min
completed: 2026-09-27
status: complete
---

# Phase 2 Plan 10: Customer OTP Sign-In (apps/web) Summary

**A `/[locale]/login` route with a two-step OTP form (`input-otp`, TH/EN), a whoami-driven `AccountStatus` header, and an `apiFetch` upgraded to retry once after a single shared in-flight `/api/v1/auth/refresh` — the customer-facing half of AUTH-01, mirroring the already-proven `apps/admin` implementation.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-09-27T03:09:38+07:00
- **Completed:** 2026-09-27T03:16:40+07:00
- **Tasks:** 2 (1 tracer, 1 auto)
- **Files modified:** 12 (7 created, 5 modified)

## Accomplishments

- `apps/web/src/components/otp-login.tsx`: two-step destination → 6-digit `input-otp` flow using `apiFetch('/api/v1/auth/otp/request'|'/otp/verify')`. Send/verify buttons show an inline `Spinner` and stay disabled until non-empty / 6 digits filled. Errors render under the input with `role="alert"`, mapped to `errors.wrongCode` (with `attemptsLeft`), `errors.expired` (also resets to the destination step), `errors.rateLimited` (429), or `errors.generic`. A 60s resend countdown gates the resend link after a successful send.
- `apps/web/src/components/account-status.tsx`: `useQuery(['whoami'])` drives a signed-in indicator + sign-out button (`POST /api/v1/auth/logout` then invalidates `whoami`), or a locale-aware `Link` to `/login` when unauthenticated. Wired into the home page header next to `LocaleSwitcher`.
- `apps/web/src/app/[locale]/login/page.tsx`: renders `OtpLogin` inside a `max-w-screen-sm` Card.
- `apps/web/src/lib/api.ts`: `apiFetch<T>` now retries once after a successful shared in-flight `POST /api/v1/auth/refresh` on a 401 for any non-`/api/v1/auth/` path, and attaches `status`/`code`/`message`/`attemptsLeft` to thrown errors (never reads/stores a token itself); `204` returns `undefined`. `BoatList` (existing consumer) is unaffected.
- shadcn `input-otp`, `input`, `label`, `spinner` added via `npx shadcn@4.21.0 add` (pinned exact); the unaudited `cn` package it pulls transitively was stripped from `package.json`/`package-lock.json` and every generated import rewritten to `@/lib/utils`, matching the fix 02-09 already established for `apps/admin`.
- `auth.*` message keys added to both `messages/th.json` and `messages/en.json` verbatim from the UI-SPEC Copywriting Contract.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): Customer sign-in end-to-end** - `e5a7e51` (feat)
2. **Task 2 (auto): Session renewal and error states** - `eb5ca08` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `apps/web/src/components/otp-login.tsx` — two-step OTP form, error mapping, resend countdown
- `apps/web/src/components/account-status.tsx` — whoami-driven header indicator
- `apps/web/src/app/[locale]/login/page.tsx` — `/[locale]/login` route
- `apps/web/src/app/[locale]/page.tsx` — `AccountStatus` added to the home header
- `apps/web/src/lib/api.ts` — refresh-retry + parsed `ApiError`
- `apps/web/src/components/ui/{input-otp,input,label,spinner}.tsx` — shadcn components, `cn` import fixed
- `apps/web/messages/{th,en}.json` — `auth.*` keys
- `apps/web/package.json`, `apps/web/package-lock.json` — `input-otp@1.5.0` exact, no `cn`

## Decisions Made

- Reused `apps/admin`'s exact `apiFetch`/`otp-login.tsx` shapes (proven in 02-09) instead of a fresh design — consistent auth UX and error handling across both frontends, smallest possible diff.
- A `failed_precondition` (expired/locked code) resets the flow to the destination step rather than leaving a dead code entry visible.
- All `auth.*` message keys were written once in Task 1 (the full set was already known from the UI-SPEC), so Task 2 needed no message-file changes despite being listed as a Task 2 file in the plan frontmatter.

## Deviations from Plan

None - plan executed exactly as written (the `cn`-package strip was already called out as required handling in the plan's own context notes, not an unplanned deviation).

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- AUTH-01 is now fully covered end-to-end (gateway routes from 02-05 + this plan's UI); `requirements.ready-ids` will mark it complete once every sibling plan declaring it (02-07, 02-09, 02-13) has a SUMMARY.
- `apps/web`'s `apiFetch` and `apps/admin`'s are now identical in contract — any future shared-package extraction (if ever needed) has zero shape mismatch to reconcile.
- The Task 2 human-check (live OTP via Mailpit, 360px wrap, attempts-left countdown, TH/EN switch, cookie-only storage) is deferred to end-of-phase UAT per `workflow.human_verify_mode=end-of-phase` — no blocker, just not yet human-verified.
- No blockers. `npm --prefix apps/web run lint/format:check/typecheck/build` all green; message-key and grep-based acceptance checks all pass.

---
*Phase: 02-identity-catalog*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 7 created files verified present on disk: `otp-login.tsx`, `account-status.tsx`, `login/page.tsx`, `ui/input-otp.tsx`, `ui/input.tsx`, `ui/label.tsx`, `ui/spinner.tsx`.
- Both task commits (`e5a7e51`, `eb5ca08`) verified present in `git log --oneline --all`.
- Both tasks' acceptance criteria re-run and confirmed: build lists `/[locale]/login`; `th.json`/`en.json` contain the exact contract strings; `otp-login.tsx` contains `InputOTP`, `maxLength={6}`, `attemptsLeft`, `role="alert"`; `api.ts` contains `/api/v1/auth/refresh` and `credentials: 'include'`.
- Plan-level `<verification>` re-run clean: `npm --prefix apps/web run lint`, `format:check`, `typecheck`, `build` all exit 0.
