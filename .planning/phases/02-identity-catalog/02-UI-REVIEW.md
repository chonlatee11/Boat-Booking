# Phase 2 — UI Review

**Audited:** 2026-10-03
**Baseline:** `.planning/phases/02-identity-catalog/02-UI-SPEC.md` (approved design contract)
**Screenshots:** captured — live `make up` stack, authenticated as `super_admin` via a real Mailpit OTP round-trip. Desktop (1440×900), tablet (768×1024), mobile (375×812). Stored in `.planning/ui-reviews/02-20261003-141456/` (gitignored).

Surfaces audited: `apps/web` login (TH/EN) + home header; `apps/admin` login, and all 5 authenticated entity pages (Operators, Piers, Routes, Boats, Staff) including their Create/Edit Sheet/Dialog forms (Pier map-picker Sheet, Route policy-editor + price-history Sheet, Boat/Staff/Operator Dialogs).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | All CTA/empty/error copy matches the contract verbatim across both apps; one leak of a raw UUID fragment to the admin when a boat's home pier is out of scope |
| 2. Visuals | 2/4 | Create dialogs for Piers, Boats, and Staff show red required-field errors the instant they open, before any user input |
| 3. Color | 2/4 | Active-status `Badge` uses the brand accent (`bg-primary`) instead of semantic green on 4 of 5 list pages — direct, repeated Color-contract violation |
| 4. Typography | 2/4 | `font-medium` (500 weight) is baked into nearly every shadcn primitive (buttons, labels, badges, dialog titles, table headers) despite the contract explicitly forbidding a third weight |
| 5. Spacing | 4/4 | Every icon-only button is a verified 44×44px (`size-11`) target with a correct Thai `aria-label`; spacing scale is clean, no arbitrary values, table overflow correctly contained |
| 6. Experience Design | 2/4 | Loading/empty/error states are comprehensive and copy-correct, but the premature-validation-error pattern is a repeatable, high-frequency UX defect (hits every "create" flow for 3 of 5 entities) |

**Overall: 15/24**

---

## Top 3 Priority Fixes

1. **Status badges render in brand-accent blue instead of semantic green, on 4 of 5 admin list pages** — Confirmed in `apps/admin/src/app/(admin)/operators/page.tsx:84`, `piers/page.tsx:115`, `routes/page.tsx:103`, `staff/page.tsx:223`: all do `<Badge>ใช้งาน</Badge>`, which resolves to the `Badge` component's `default` variant (`bg-primary text-primary-foreground` → `#0b6e99`). The UI-SPEC's Color section is explicit: the accent is "Never used for status badges (those use semantic green/orange, not the brand accent)." The Boats page (`boats/page.tsx`) already does this correctly with explicit green/orange Tailwind classes (per its own 02-12 SUMMARY decision log) — the other four pages missed the same rule. **Fix:** replace `<Badge>ใช้งาน</Badge>` with the same explicit `bg-green-*`/`text-green-*` pattern already used in `boats/page.tsx`, in all four files.

2. **Three of five Create dialogs greet the admin with a red validation error before they've typed anything** — Confirmed visually (screenshots `dialog-boat-create.png`, `dialog-staff-create.png`, `sheet-pier-create.png`) and in code: `pier-sheet.tsx:193/209/224` (`{!operatorValid/!trimmedNameTh/!trimmedNameEn && <FieldError>...}`), `boat-dialog.tsx:129` (`{!trimmedName && <FieldError>กรุณากรอกชื่อเรือ</FieldError>}`), `staff-dialog.tsx:183` (`{!trimmedName && <FieldError>กรุณากรอกชื่อ</FieldError>}`) all render unconditionally off current validity with zero touched/submitted gating. Opening "เพิ่มเรือใหม่" or "เพิ่มผู้ใช้งานใหม่" shows a required-field error on a completely blank form. This is a confirmed regression against the project's own `route-sheet.tsx` (duration/origin errors correctly gated with `&& durationMinutes` / a derived flag) and `operator-dialog.tsx` (no premature error at all — silently disables Save instead). **Fix:** add a `touched`/`submitAttempted` boolean per offending field (or one dialog-level flag set on first blur/submit) and gate every unconditional `FieldError` in `pier-sheet.tsx`, `boat-dialog.tsx`, and `staff-dialog.tsx` behind it, matching `route-sheet.tsx`'s existing pattern.

3. **`font-medium` (500 weight) is used throughout the app despite the contract's explicit "only 400/700" rule** — The UI-SPEC Typography section states: "Only the two font weights already loaded by `IBM_Plex_Sans_Thai` are used (400, 700) — the loaded 500 weight is not part of this contract and should not be introduced as a third weight." `font-medium` ships baked into the shadcn "nova" preset's own primitives: `components/ui/button.tsx`, `label.tsx`, `field.tsx`, `dialog.tsx`, `card.tsx`, `table.tsx`, `badge.tsx`, `alert-dialog.tsx`, `sheet.tsx`, plus three app files (`admin-shell.tsx` nav links, `price-section.tsx`, `route-sheet.tsx`). That means nearly every button, label, dialog title, table header, and nav link in the app renders at 500 weight — the exact weight the contract says must not be introduced. **Fix:** either override `font-medium` → `font-normal` (body-weight UI chrome, matching most admin-table conventions) in the primitives, or get the UI-SPEC checker to formally amend the Typography contract to permit 500 — the current state is a direct, unaddressed contradiction between the approved spec and the shipped code.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

**Strengths:** every copy string in the Copywriting Contract table was grep-verified present verbatim: OTP flow copy (`apps/web/src/components/otp-login.tsx`, `apps/admin/src/components/otp-login.tsx`), all 5 entities' primary CTAs, all 5 entities' empty-state heading/body, all 5 entities' list-load-failure/save-failure copy, the D-15 blocked-archive inline error, the D-10 disable-user confirmation (`staff/page.tsx:85/87/101`), and the D-13 cancellation-policy validation message (`routes/policy-editor.tsx:10`). No generic "Submit"/"Cancel"/"OK" labels found anywhere in app code (`grep` for `>Submit<`/`>Cancel<`/`"Submit"` returns nothing outside `ยกเลิก`/shadcn primitives).

**Finding:** `boats/queries.ts:29` (`pierName()`) falls back to `id.slice(0, 8)` — the raw UUID's first 8 hex characters — when a boat's `home_pier_id` isn't present in the admin's own `ListPiers` result (e.g. an archived or out-of-scope pier). This is visible in the live Boats table screenshot (`admin-boats-desktop.png`): the "ท่าประจำ" column repeatedly shows the literal string `00000000` for many rows instead of a pier name or an explicit "pier not found" label. Exposing a raw technical identifier fragment to a Thai-only admin UI is inconsistent with the polish of every other fallback string in the contract (which are all human-readable Thai sentences). Not covered explicitly by the Copywriting Contract table, but out of step with its spirit.

### Pillar 2: Visuals (2/4)

**Strengths:** login card focal point is correctly centered at both 1440×900 and 375×812 (`min-h-svh flex items-center justify-center`, confirmed via screenshot pixel measurement). Active sidebar nav item uses the accent correctly (`admin-shell.tsx:87`: `bg-primary text-primary-foreground`). Edit-mode skeleton states render correctly on Pier/Route/Boat/Staff Sheets per their SUMMARYs' grep verification.

**Finding (see Priority Fix #2):** `dialog-boat-create.png` and `dialog-staff-create.png` show a bright red `FieldError` ("กรุณากรอกชื่อเรือ" / "กรุณากรอกชื่อ") directly beneath an empty, untouched required field, immediately below a focused, empty input — the very first thing an admin sees when opening "เพิ่มเรือใหม่" or "เพิ่มผู้ใช้งานใหม่" is an error message. `sheet-pier-create.png` shows three simultaneous errors (operator/name-TH/name-EN) on a form that has had zero interaction. This reads as a broken/already-failed form on first paint, which undermines the "clear focal point" and visual-hierarchy pillar criteria — the red error text competes with the Save CTA for attention before the user has done anything wrong.

### Pillar 3: Color (2/4, see Priority Fix #1)

**Strengths:** `--primary: #0b6e99` is hardcoded correctly in both apps' `globals.css`, byte-identical between `apps/web` and `apps/admin` (confirmed by `02-09-SUMMARY.md`'s own `cmp` check). No hardcoded hex/rgb colors found outside the shadcn `ui/` primitives (`grep -rnE "#[0-9a-fA-F]{3,8}|rgb\("` over app code returns nothing). Destructive actions correctly use the `destructive` token, never the accent.

**Finding:** the default `Badge` variant (`bg-primary`) is used for the "ใช้งาน" (active) status indicator on **Operators, Piers, Routes, and Staff** list pages — all four render the brand-accent blue for a status badge, which the Color contract explicitly prohibits ("Never used for status badges"). The **Boats** page is the sole correct implementation, using explicit `green-*`/`orange-*` Tailwind classes specifically because (per its own 02-12 SUMMARY) "the UI-SPEC reserves the brand accent color for CTAs, never status" — meaning the project already identified and solved this exact problem once, but the fix wasn't applied to the four pages built in earlier/sibling plans. This is visually confirmed in `admin-operators-desktop.png`, `admin-piers-desktop.png`, `admin-routes-desktop.png`, `admin-staff-desktop.png` (blue pill badges) vs. `admin-boats-desktop.png` (correct green pill badges).

### Pillar 4: Typography (2/4, see Priority Fix #3)

**Strengths:** text size usage is disciplined — only 4 distinct Tailwind size classes in use app-wide (`text-xs`, `text-sm`, `text-base`, `text-2xl`), well within the "flag if >4 sizes" abstract threshold, and no arbitrary `text-[Npx]` values found in app code.

**Finding — weight:** `grep` across `apps/admin/src` finds `font-medium` (Tailwind's 500 weight) in 11 files — 8 of them shadcn-generated primitives (`button.tsx`, `label.tsx`, `field.tsx`, `dialog.tsx`, `card.tsx`, `table.tsx`, `badge.tsx`, `alert-dialog.tsx`, `sheet.tsx`) plus 3 app files (`admin-shell.tsx`, `price-section.tsx`, `route-sheet.tsx`). Since every button, label, field error, dialog title, table header, and badge in the app is built from these primitives, 500 weight is effectively the dominant UI-chrome weight throughout both admin and (via `boat-list.tsx`) web — directly contradicting the contract's explicit two-weight (400/700) rule.

**Finding — size, minor:** all 5 admin list-page `<h1>` titles use `text-2xl` (24px: `operators/page.tsx:64`, `piers/page.tsx:81`, `routes/page.tsx:75`, `boats/page.tsx:69`, `staff/page.tsx:186`) rather than the contract's declared Heading token (20px, i.e. Tailwind's `text-xl`). Consistently applied everywhere (not a one-off), so this reads as a coherent-but-off-spec system rather than visual inconsistency — lower severity than the weight issue, noted for completeness.

### Pillar 5: Spacing (4/4)

**Strengths:** every icon-only interactive element audited carries an explicit `size-11` (44px) override plus a correct Thai `aria-label` naming its action — confirmed by file/line for: operator edit (`operators/page.tsx:99-101`), pier edit (`piers/page.tsx:132-134`), boat edit (`boats/page.tsx:105-107`), route edit + return-route (`routes/page.tsx:119-121, 132-134`), staff edit/disable (`staff/page.tsx:75-77, 245-247`), shared archive action (`archive-dialog.tsx:47-49`), policy-tier remove (`policy-editor.tsx:103-105`), and photo remove (`photo-upload.tsx:90-92`, with `aria-label="ลบรูปภาพ"` exactly matching the spec's named example). This is a 100%-hit rate on a specific, easy-to-miss accessibility requirement and is a genuine strength worth calling out. No arbitrary spacing values found outside 3 primitive-internal uses (focus-ring width, tooltip arrow offset) and one deliberate `max-w-[220px]` truncation width in `TruncatedCell`. The shared `DataTable` wraps its `<Table>` in `overflow-x-auto rounded-lg border`, correctly satisfying the UI-SPEC's E2-overflow backstop ("table scrolls horizontally inside its container... page body never scrolls sideways") — verified in code, and consistent with the tablet-width screenshot showing clipped-not-reflowed table content.

### Pillar 6: Experience Design (2/4)

**Strengths:** every one of the 5 entity list pages implements the full loading/error/empty state set from the UI-SPEC with matching copy (`skeleton` rows, `errorText` + retry, `Empty`/`EmptyTitle`/`EmptyDescription` with inline CTA) — verified by grep across all 5 `page.tsx` files. Destructive confirmations (archive, disable-user) use the exact contract copy and correctly block instead of cascading (D-15's blocked-pier-archive inline error, confirmed in `piers/page.tsx:70`). Save-in-flight states show a `Spinner` and disable the submit button consistently across every Dialog/Sheet.

**Finding (see Priority Fix #2):** the premature-validation-error pattern is an Experience Design defect, not just a Visuals one — it means the single most common first action an admin takes (opening "add new X" for 3 of 5 entities) is met with an error state before any input, which is the opposite of the intended "no issues until the user does something wrong" validation UX. Given these are the entity-creation flows a `pier_admin`/`super_admin` would use most frequently (piers, boats, staff), this is a repeatable degradation, not an edge case.

**Finding, minor:** the "ใช้งาน"/archived-row color issue (Pillar 3) also has an Experience Design angle — an admin scanning a list for "what's currently usable" loses the at-a-glance red/amber/green status cue that semantic coloring would otherwise provide, since active rows render in brand blue indistinguishable in hue-category from any other blue UI chrome (nav, CTAs, links) on the same screen.

---

## Files Audited

- `apps/web/src/app/[locale]/login/page.tsx`, `apps/web/src/components/otp-login.tsx`, `apps/web/src/components/account-status.tsx`, `apps/web/src/app/[locale]/page.tsx`
- `apps/admin/src/app/login/page.tsx`, `apps/admin/src/components/otp-login.tsx`, `apps/admin/src/components/admin-shell.tsx`
- `apps/admin/src/app/(admin)/operators/page.tsx`, `apps/admin/src/components/operator-dialog.tsx`
- `apps/admin/src/app/(admin)/piers/page.tsx`, `pier-sheet.tsx`, `queries.ts`, `apps/admin/src/components/map-picker.tsx`, `photo-upload.tsx`
- `apps/admin/src/app/(admin)/routes/page.tsx`, `route-sheet.tsx`, `policy-editor.tsx`, `price-section.tsx`, `money.ts`, `queries.ts`
- `apps/admin/src/app/(admin)/boats/page.tsx`, `boat-dialog.tsx`, `queries.ts`
- `apps/admin/src/app/(admin)/staff/page.tsx`, `staff-dialog.tsx`, `queries.ts`
- `apps/admin/src/components/data-table.tsx`, `archive-dialog.tsx`
- `apps/admin/src/components/ui/{button,badge,label,field,dialog,card,table,alert-dialog,sheet,tooltip}.tsx`
- `apps/admin/src/app/globals.css`, `apps/web/src/app/globals.css`
- Live screenshots: `.planning/ui-reviews/02-20261003-141456/*.png` (24 files — login states, all 5 authenticated list pages at desktop/tablet/mobile, and 6 Create/Edit Sheet/Dialog states), captured via an authenticated Playwright session (real OTP round-trip through Mailpit against the running `make up` stack).
