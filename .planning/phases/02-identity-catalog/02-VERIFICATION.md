---
phase: 02-identity-catalog
verified: 2026-09-28T23:40:00Z
status: passed
score: 5/5 must-haves verified
covered_files:

  - ".planning/REQUIREMENTS.md"
  - ".planning/phases/02-identity-catalog/02-01-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-01-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-02-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-02-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-03-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-03-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-04-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-04-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-05-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-05-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-06-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-06-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-07-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-07-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-08-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-08-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-09-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-09-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-10-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-10-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-11-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-11-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-12-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-12-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-13-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-13-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-14-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-14-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-15-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-15-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-16-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-16-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-17-PLAN.md"
  - ".planning/phases/02-identity-catalog/02-17-SUMMARY.md"
  - ".planning/phases/02-identity-catalog/02-UAT.md"
  - "apps/admin/src/app/(admin)/piers/pier-sheet.tsx"
  - "apps/admin/src/app/(admin)/routes/policy-editor.tsx"
  - "apps/admin/src/app/(admin)/routes/queries.ts"
  - "apps/admin/src/app/(admin)/routes/route-sheet.tsx"
  - "apps/admin/src/components/map-picker.tsx"
  - "deploy/proof.sh"
  - "services/catalog/cmd/boats_photo_integration_test.go"
  - "services/catalog/cmd/main_integration_test.go"
  - "services/catalog/cmd/operators_piers_integration_test.go"
  - "services/catalog/internal/app/boat.go"
  - "services/catalog/internal/app/pier.go"
  - "services/identity/cmd/main_integration_test.go"
  - "services/identity/internal/adapters/notify/notify.go"
  - "services/identity/internal/adapters/notify/notify_test.go"
  - "services/identity/internal/app/otp.go"

covered_digest: "v1:sha256:5517a715ceb6d9e649df0cd27e2a730937aa9eea8b9f13481169543505a85bf0"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/5
  gaps_closed:
    - "SC1/AUTH-01/D-03: OTP wrong-code lockout holds under concurrent guessing (CR-01) — fixed by services/identity/internal/app/otp.go's atomic TxPipelined(HGetAll+HIncrBy); proven by TestOtpAttemptsLockoutConcurrent (re-run, PASS)"
    - "SC3/SC4/CAT-04/AUTH-05: boat.operator_id stays correct after a cross-operator home-pier move (CR-02) — fixed by services/catalog/internal/app/boat.go's updateBoat persisting b.OperatorID; proven by TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID (re-run, PASS)"
    - "G-02-3: OTP/dev-SMS SMTP messages declare UTF-8 MIME + RFC 2047 Subject (notify.go otpMessage); TestOtpMessageIsUTF8MIME re-run, PASS"
    - "G-02-8 backend half: createPier's archived-operator rejection names the reason (\"operator is archived\"); TestArchiveOperatorBlockedByPiers re-run, PASS"
    - "G-02-18: 10 concurrent UpsertBoat calls on one boat_id serialize with no torn write; TestConcurrentUpsertBoatSameID re-run, PASS"
    - "G-02-7: MapPicker lat/lng typing/dragging never crashes; draft-string parseCoord range guard + tile-failure reset confirmed in apps/admin/src/components/map-picker.tsx"
    - "G-02-8 UI half: pier Sheet operator select filters archived operators; opening/closing hours validated as locale-independent HH:MM text, confirmed in pier-sheet.tsx"
    - "G-02-9: same-named piers disambiguated with an id-tail suffix in routes list/archive dialog/route Sheet selects (pierName in queries.ts, used by route-sheet.tsx); deploy/proof.sh gives each run's pier a unique name"
    - "G-02-12: policy-editor validatePolicy defaults absent tier fields to 0 (not -1) so a loaded/copied valid policy passes immediately; last-tier delete button is hidden"
  gaps_remaining: []
  regressions: []
deferred: []
advisory: []
human_verification:

  - test: "G-02-3 (02-14): with `make up` running, request an OTP at the admin/web login page and open the message in Mailpit (http://localhost:8025)"
    expected: "The body reads \"รหัสของคุณ / Your code: ......\" in correct Thai (not mojibake), and the subject still reads correctly"
    why_human: "Mailpit's rendering of the raw MIME message can only be confirmed visually in its UI; the unit/integration tests prove the message bytes are correctly formed but not how the mail client renders them"
  - test: "G-02-7 (02-16): as super_admin on /piers, open \"เพิ่มท่าเรือใหม่\" and type latitude/longitude one keystroke at a time (including a partial value like \"13.\" and an out-of-range value like \"137\"); drag the marker; block tiles.openfreemap.org in devtools and reopen the Sheet"
    expected: "No crash/dev-overlay at any point; the marker follows valid typed pairs and drag; an out-of-range value shows an inline range error and disables Save without moving the marker; \"โหลดแผนที่ไม่สำเร็จ\" appears only when tiles are blocked and the manual lat/lng inputs still work"
    why_human: "Real keystroke-by-keystroke typing, marker drag, and a blocked-network scenario need a live browser; not exercised by an automated test"
  - test: "G-02-8 UI half (02-16): archive an operator with no piers, confirm it is absent from the create-pier operator select; create a pier with hours \"08:00\"/\"17:30\" and confirm the network payload carries those exact strings; type \"8:0\" in opening hours"
    expected: "Archived operator is not selectable; a valid save's UpsertPier payload has opensAt/closesAt exactly as typed (never empty); a malformed HH:MM value shows the inline error and disables Save"
    why_human: "Confirming the actual network payload and archived-operator-select filtering in a live session needs a browser + devtools"
  - test: "G-02-9 (02-17): on /routes with the dev DB's two piers both named \"Proof Pier\", confirm the routes list, archive dialog, and both route Sheet pier selects show an id-suffixed label (e.g. \"Proof Pier (xxxx)\"); create the same (pier_from, pier_to) pair again"
    expected: "Same-named piers are visually distinguishable everywhere a pier name is shown; unique-named piers show no suffix; the duplicate-pair create attempt still shows \"มีเส้นทางนี้อยู่แล้ว\""
    why_human: "Visual confirmation of label rendering across three UI surfaces needs a live browser session against the dev DB's actual duplicate-named piers"
  - test: "G-02-12 (02-17): open \"แก้ไขเส้นทาง\" on an existing route with policy 24/100, 2/50, 0/0, and separately click \"สร้างเส้นทางย้อนกลับ\" to copy a policy"
    expected: "Both Sheets show no policy validation error and Save is enabled immediately (no delete-and-re-add workaround needed); the last (0h) tier has no delete button in either Sheet, including after deleting tiers down to one"
    why_human: "Confirming the editor renders with Save already enabled on load, not just that the validation function is correct in isolation, needs a live browser"
---

# Phase 02: Identity + Catalog Verification Report

**Phase Goal:** Customers and staff can authenticate with correct roles/scoping, and pier_admin can manage the catalog data (piers/routes/boats/prices) that customers browse publicly
**Verified:** 2026-09-28T23:40:00Z
**Status:** human_needed
**Re-verification:** Yes — after gap closure plans 02-14 through 02-17 (and two out-of-band review-fix commits, CR-01/CR-02)

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Customer requests OTP via email/phone and receives a session (JWT httpOnly cookie) without a password account | ✓ VERIFIED | `services/gateway/internal/adapters/http/auth.go` implements `/api/v1/auth/otp/request`/`otp/verify`; `services/identity/internal/app/otp.go` auto-creates a `role=customer` row with no password field. The CR-01 concurrency gap from the initial verification is closed: `VerifyOtp` now runs `HGetAll` + `HIncrBy` in one `TxPipelined` round trip (lines ~145-152), so every verify attempt — right or wrong — consumes one attempt slot atomically before comparison. Re-ran `TestOtpAttemptsLockoutConcurrent` (20 concurrent wrong guesses): **PASS**. `TestOtpMessageIsUTF8MIME` (G-02-3 fix, MIME/charset headers + RFC 2047 Subject): **PASS**. |
| 2 | staff/pier_admin/super_admin log in and receive `role`+`operator_id` claims; Kong verifies JWT; BFF forwards claims as trusted headers; requests missing headers rejected | ✓ VERIFIED | Unchanged since initial verification: `pkg/auth/auth.go` issues RS256 JWTs with role/operator_id/pier_ids; `httpx.ForwardClaims` strips inbound spoofed `X-*` headers before setting verified ones; `httpx.RequireInternal` gates both `services/catalog/cmd/main.go` and `services/identity/cmd/main.go`. No regression found in the re-verified files. |
| 3 | super_admin creates pier_admin/staff users assigned to operator+pier; every admin query scoped by `operator_id` so a pier_admin never sees another operator's data | ✓ VERIFIED | `services/identity/internal/app/users.go` UpsertUser validates piers against catalog before persisting; scoped SQL ANDs `operator_id` with `pier_ids` throughout. The CR-02 gap (boat cross-operator move leaving a stale `operator_id`) is closed: `updateBoat` in `services/catalog/internal/app/boat.go` now includes `OperatorID: toPgUUID(b.OperatorID)` in `UpdateBoatParams`, and the SQL `UpdateBoat` query sets `operator_id = $2`. Re-ran `TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID`: **PASS** — moving a boat to another operator's pier now correctly re-derives and persists the new `operator_id`, hides it from the old operator's scoped list, and surfaces it to the new operator's. |
| 4 | super_admin creates/edits operators, creates piers; pier_admin edits/archives assigned piers (map picker), creates/edits/archives routes (tiered cancellation policy), boats, and per-route ticket prices (adult/child, integer satang) via admin UI | ✓ VERIFIED, remaining polish confirmed by human check | Backend CRUD/archive/pricing logic unchanged from initial verification (still solid). All five UAT-found UI/backend defects in this area are closed at the code level: G-02-7 (map-picker crash on partial/out-of-range lat/lng — `apps/admin/src/components/map-picker.tsx` now uses draft-string `parseCoord` range validation and a `sourcedata`-reset tile-failure flag), G-02-8 (archived operators filtered from the pier-create select, specific Thai error copy, locale-independent 24h `HH:MM` hours validation in `pier-sheet.tsx`; backend wraps the archived-operator rejection with a named reason in `pier.go`), G-02-9 (same-named piers disambiguated with an id-tail suffix via `pierName()` in `routes/queries.ts`, used by `route-sheet.tsx`'s two pier selects; `deploy/proof.sh` now gives each run's pier a unique name), G-02-12 (`policy-editor.tsx`'s `validatePolicy` now defaults absent tier fields to `0` instead of `-1`, and the delete button is hidden on the last tier), G-02-18 (`TestConcurrentUpsertBoatSameID` proves 10 concurrent `UpsertBoat` calls on one `boat_id` serialize with exactly 1+10 outbox rows and no torn write). `npx tsc --noEmit` on `apps/admin` is clean; `gofmt -l`/`go vet` clean on all re-verified Go files. Visual/interaction confirmation of each fix deferred to human verification (see below). |
| 5 | Public search lists piers and routes with coordinates for the map, without authentication | ✓ VERIFIED | Unchanged since initial verification: `publicHandler` calls CatalogService with only the internal token; `ListPiersPublic`/`ListRoutesPublic` return non-archived rows with coordinates. No regression found. |

**Score:** 5/5 truths fully verified (0 present-but-behavior-unverified)

### Gaps Closed Since Initial Verification

Both **critical** findings from the initial verification pass are fixed and re-tested:

1. **CR-01 (OTP attempt-limit race).** `services/identity/internal/app/otp.go` `VerifyOtp` now increments the attempt counter atomically in the same Valkey pipeline as the hash read, so the D-03 5-attempt lockout holds under concurrent guessing. Verified by re-running `TestOtpAttemptsLockoutConcurrent` — **PASS**.
2. **CR-02 (boat cross-operator move stale operator_id).** `services/catalog/internal/app/boat.go` `updateBoat` now persists the derived `operator_id` on every update. Verified by re-running `TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID` — **PASS**.

All six UAT gaps targeted by plans 02-14 through 02-17 (G-02-3, G-02-7, G-02-8, G-02-9, G-02-12, G-02-18) are closed at the code level, each corroborated by re-running the specific test or grepping the specific fix described in the plan's `must_haves`:

| Gap | Fix location | Automated evidence |
|-----|--------------|---------------------|
| G-02-3 | `services/identity/internal/adapters/notify/notify.go` (`otpMessage`) | `TestOtpMessageIsUTF8MIME` — PASS |
| G-02-8 (backend) | `services/catalog/internal/app/pier.go` | `TestArchiveOperatorBlockedByPiers` — PASS |
| G-02-18 | `services/catalog/cmd/boats_photo_integration_test.go` | `TestConcurrentUpsertBoatSameID` — PASS |
| G-02-7 | `apps/admin/src/components/map-picker.tsx` | grep-confirmed `parseCoord`, `sourcedata` reset; `tsc --noEmit` clean |
| G-02-8 (UI) | `apps/admin/src/app/(admin)/piers/pier-sheet.tsx` | grep-confirmed `!op.archived`, `HHMM` regex, `failed_precondition` handling |
| G-02-9 | `apps/admin/src/app/(admin)/routes/queries.ts`, `route-sheet.tsx`, `deploy/proof.sh` | grep-confirmed `pierName` id-tail suffix, `pierName(...)` used in both selects, `SUFFIX` in proof.sh |
| G-02-12 | `apps/admin/src/app/(admin)/routes/policy-editor.tsx` | grep-confirmed `?? 0` fallback, `index < tiers.length - 1` delete guard |

No gaps remain unaddressed. No regressions found in any re-verified file.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/identity/internal/app/otp.go` | Atomic attempt-limiting VerifyOtp | ✓ VERIFIED | `TxPipelined(HGetAll+HIncrBy)` confirmed; `TestOtpAttemptsLockoutConcurrent` passes |
| `services/identity/internal/adapters/notify/notify.go` | UTF-8 MIME OTP/dev-SMS messages | ✓ VERIFIED | `otpMessage` builder with MIME headers + RFC 2047 Subject; `TestOtpMessageIsUTF8MIME` passes |
| `services/catalog/internal/app/boat.go` | `operator_id` always derived from home pier, persisted on update | ✓ VERIFIED | `updateBoat` writes `OperatorID`; `TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID` passes |
| `services/catalog/internal/app/pier.go` | Archived-operator create rejection names the reason | ✓ VERIFIED | `%w: operator is archived`; `TestArchiveOperatorBlockedByPiers` passes |
| `services/catalog/cmd/boats_photo_integration_test.go` | Concurrent UpsertBoat regression test | ✓ VERIFIED | `TestConcurrentUpsertBoatSameID` present and passing |
| `apps/admin/src/components/map-picker.tsx` | Crash-proof lat/lng inputs, tile-failure reset | ✓ VERIFIED (wiring; visual confirmation pending) | `parseCoord`, draft-string inputs, `sourcedata` reset confirmed by read |
| `apps/admin/src/app/(admin)/piers/pier-sheet.tsx` | Archived-operator filter, HH:MM hours validation | ✓ VERIFIED (wiring; visual confirmation pending) | `!op.archived`, `HHMM` regex, `failed_precondition` mapping confirmed by read |
| `apps/admin/src/app/(admin)/routes/policy-editor.tsx` | 0-default validation, last-tier delete guard | ✓ VERIFIED | `?? 0` fallback and `index < tiers.length - 1` guard confirmed by read |
| `apps/admin/src/app/(admin)/routes/queries.ts` + `route-sheet.tsx` | Same-named pier disambiguation | ✓ VERIFIED (wiring; visual confirmation pending) | `pierName` suffix logic confirmed; both route-sheet selects call `pierName(...)` |
| `deploy/proof.sh` | Run-unique pier name | ✓ VERIFIED | `Proof Pier %s` with `SUFFIX=$(date +%s)` confirmed |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `services/identity/internal/app/otp.go` VerifyOtp | Valkey `TxPipelined` | atomic HGetAll+HIncrBy before comparison | ✓ WIRED (fixes CR-01) |
| `services/catalog/internal/app/boat.go` updateBoat | `postgres.UpdateBoatParams.OperatorID` | `b.OperatorID` (derived from home pier) now flows into the UPDATE | ✓ WIRED (fixes CR-02) |
| `apps/admin/.../map-picker.tsx` lat/lng inputs | `onChange(LatLng \| undefined)` | `parseCoord` range guard, only fires with two valid drafts | ✓ WIRED |
| `apps/admin/.../pier-sheet.tsx` operator select | catalog `ListOperators` result | `.filter((op) => !op.archived)` | ✓ WIRED |
| `apps/admin/.../routes/route-sheet.tsx` pier selects | `pierName()` in `queries.ts` | both `NativeSelect`s call `pierName(pier.pierId, ownPiers, publicPiers)` | ✓ WIRED |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| AUTH-01 | 02-02, 02-05, 02-10, 02-14 | Customer OTP login, no password, secure lockout, correct MIME | ✓ SATISFIED | otp.go (CR-01 fixed), notify.go (G-02-3 fixed) |
| AUTH-02 | 02-02, 02-05, 02-07, 02-09, 02-13, 02-14 | staff/pier_admin/super_admin login with role+operator_id claims | ✓ SATISFIED | session.go, bootstrap.go, users.go |
| AUTH-03 | 02-01, 02-04, 02-05 | Kong verifies JWT; BFF forwards trusted headers; missing headers rejected | ✓ SATISFIED | claims.go, proxy.go, RequireInternal wiring (unchanged, no regression) |
| AUTH-04 | 02-07, 02-13 | super_admin creates pier_admin/staff assigned to operator+pier | ✓ SATISFIED | users.go UpsertUser, staff-dialog.tsx (unchanged, no regression) |
| AUTH-05 | 02-01, 02-03, 02-06, 02-07, 02-08, 02-15 | Every admin query scoped by operator_id | ✓ SATISFIED | scope.go; boat.go's operator_id invariant now holds after update (CR-02 fixed) |
| CAT-01 | 02-03, 02-09, 02-15, 02-16 | super_admin creates/edits operators | ✓ SATISFIED | operator.go, operator-dialog.tsx; archived-operator UX fixed (G-02-8) |
| CAT-02 | 02-03, 02-06, 02-08, 02-11, 02-15, 02-16 | pier_admin CRUD/archive piers (map picker) | ✓ SATISFIED | pier.go, map-picker.tsx (G-02-7, G-02-8 fixed) |
| CAT-03 | 02-06, 02-12, 02-17 | pier_admin CRUD/archive routes (tiered cancellation policy) | ✓ SATISFIED | route.go, policy-editor.tsx (G-02-9, G-02-12 fixed) |
| CAT-04 | 02-08, 02-12, 02-15 | pier_admin creates/edits boats | ✓ SATISFIED | boat.go — create and update paths both correct (CR-02, G-02-18 fixed) |
| CAT-05 | 02-06, 02-12 | Per-route ticket prices, integer satang | ✓ SATISFIED | price.go, price-section.tsx, money.ts (unchanged, no regression) |
| CAT-06 | 02-03, 02-04, 02-06, 02-08, 02-11 | Public search lists piers/routes with coordinates, no auth | ✓ SATISFIED | publicHandler, ListPiersPublic/ListRoutesPublic (unchanged, no regression) |

No orphaned requirements — REQUIREMENTS.md's Phase 2 mapping (AUTH-01..05, CAT-01..06) exactly matches the union of `requirements:` fields across all 17 plans.

**Note (documentation staleness, not a code gap):** `.planning/REQUIREMENTS.md`'s checkbox list and traceability table still show AUTH-03/AUTH-04/AUTH-05/CAT-01/CAT-05/CAT-06 as unchecked/"Gaps Found", even though the *initial* 02-VERIFICATION.md (2026-09-27) already marked all six ✓ SATISFIED and this re-verification found no regression in any of them. This file was not updated after the initial verification pass or after this gap-closure round. It should be synced to reflect Phase 2 = Complete for all 11 requirements before/at milestone completion — flagged as ℹ️ Info, not a blocker, since the underlying code and tests are evidenced independently above.

### Anti-Patterns Found

No unresolved `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers in any file modified by plans 02-14 through 02-17 or in the CR-01/CR-02 fix commits. The `placeholder="08:00"` / `placeholder="17:30"` matches in `pier-sheet.tsx` are HTML input placeholder attributes, not debt markers.

ℹ️ Info: `.planning/REQUIREMENTS.md` traceability staleness (see note above).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CR-01 fix holds under concurrency | `go test -tags=integration -run TestOtpAttemptsLockoutConcurrent ./services/identity/cmd/...` | `--- PASS: TestOtpAttemptsLockoutConcurrent (1.00s)` | ✓ PASS |
| CR-02 fix holds for cross-operator move | `go test -tags=integration -run TestUpdateBoatMovedToAnotherOperatorPierUpdatesOperatorID ./services/catalog/cmd/...` | `--- PASS (0.55s)` | ✓ PASS |
| G-02-18 concurrent UpsertBoat, no torn write | `go test -tags=integration -run TestConcurrentUpsertBoatSameID ./services/catalog/cmd/...` | `--- PASS (1.43s)` | ✓ PASS |
| G-02-8 backend archived-operator message | `go test -tags=integration -run TestArchiveOperatorBlockedByPiers ./services/catalog/cmd/...` | `--- PASS (0.45s)` | ✓ PASS |
| G-02-3 OTP MIME/UTF-8 message format | `go test -run TestOtpMessageIsUTF8MIME ./services/identity/internal/adapters/notify/...` | `--- PASS (0.00s)` | ✓ PASS |
| Go formatting clean on all re-verified files | `gofmt -l <files>` | no output | ✓ PASS |
| Admin TypeScript compiles clean | `npx tsc --noEmit -p apps/admin` | no output/errors | ✓ PASS |

All five integration tests above were run in this verification pass against real testcontainers Postgres + Redpanda/Valkey (Docker available), not taken from SUMMARY.md claims.

### Human Verification Required

Five items, one per closed UAT gap that involves visual rendering or live browser interaction that presence/grep checks and unit/integration tests cannot exercise. See the frontmatter `human_verification` section for full test/expected/why_human detail:

1. **G-02-3** — Mailpit renders the OTP email body in correct Thai (not mojibake).
2. **G-02-7** — Typing/dragging lat/lng in the pier Sheet never crashes the page; range errors and the tile-failure message behave correctly.
3. **G-02-8 (UI)** — Archived operators are unselectable; typed hours reach the actual network payload; malformed hours block Save.
4. **G-02-9** — Same-named piers show a disambiguating suffix everywhere the routes page displays a pier name.
5. **G-02-12** — A loaded or copied cancellation policy validates immediately with Save enabled; the last tier never shows a delete button.

Automated checks (existing tests re-run, new regression tests, static grep/read confirmation, `tsc`/`gofmt`/`go vet`) all passed. Awaiting human visual/interaction confirmation of these five items before the phase can be marked fully `passed`.

---

_Verified: 2026-09-28T23:40:00Z_
_Verifier: Claude (gsd-verifier)_

## Post-UAT Re-check (2026-10-03)

Quick task 261003-k4f (UI-REVIEW fixes) touched two covered files after this report: `apps/admin/src/app/(admin)/piers/pier-sheet.tsx` (required-field `FieldError` now gated on a per-field `touched` flag) and `apps/admin/src/app/(admin)/routes/queries.ts` (`pierName` not-found fallback is now `ไม่พบท่าเรือ` instead of `id.slice(0, 8)`). Re-checked the G-02-8 / G-02-9 must-haves by grep: `.filter((op) => !op.archived)`, the `HHMM` regex + Save gate, `failed_precondition` handling, and the `id.slice(-4)` duplicate-name suffix are all unchanged. `typecheck`/`lint`/`prettier` clean; the admin UI was confirmed visually by the user. Status remains `passed`; `covered_digest` recomputed via `verification.fingerprint`.
