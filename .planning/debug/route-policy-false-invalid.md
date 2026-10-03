---
status: diagnosed
trigger: "UAT test 12 (Return route, 02-12 D4): edit-route Sheet policy tiers 24/100, 2/50, 0/0 (valid) always show POLICY_VALIDATION_ERROR and Save stays disabled; delete-then-add-tier workaround fixes it; last (0h) tier also shows a delete button when it should not; create mode (test 10) validated fine."
created: "2026-09-28T00:00:00Z"
updated: "2026-09-28T00:00:00Z"
---

## Current Focus

hypothesis: CONFIRMED — see Resolution.root_cause
test: traced render vs validate defaulting in policy-editor.tsx against the wire JSON shape in gen/ts/services/catalog/v1/catalog_pb.ts and the raw-fetch client in apps/admin/src/lib/api.ts
expecting: n/a — diagnosis complete, goal is find_root_cause_only
next_action: none — return ROOT CAUSE FOUND to caller (no fix applied, per goal flag and hard-rule no-edit constraint on this parallel run)

## Symptoms

expected: A valid cancellation policy loaded from an existing route (edit) or copied via "สร้างเส้นทางย้อนกลับ" (return route) passes validation and can be saved immediately. The last tier (0 hours) has no delete button.
actual: In apps/admin "แก้ไขเส้นทาง" Sheet, policy tiers 24/100, 2/50, 0/0 (objectively valid — descending hours, includes a 0h tier) always show "นโยบายยกเลิกไม่ถูกต้อง: ต้องเรียงชั่วโมงจากมากไปน้อยและมีระดับ 0 ชั่วโมงเสมอ" and Save is disabled. Deleting a tier then re-adding it makes Save work. The last (0h) tier also shows a trash/delete button (spec says it should not). Create mode (UAT test 10) validated fine with the same-shaped default policy.
errors: "นโยบายยกเลิกไม่ถูกต้อง: ต้องเรียงชั่วโมงจากมากไปน้อยและมีระดับ 0 ชั่วโมงเสมอ" (POLICY_VALIDATION_ERROR constant, apps/admin/src/app/(admin)/routes/policy-editor.tsx:9-10)
reproduction: Open "แก้ไขเส้นทาง" (edit) Sheet on any existing route, or click "สร้างเส้นทางย้อนกลับ" on one → policy shows 24/100, 2/50, 0/0 → validation error is shown immediately and Save button is disabled, with no user edits made.
started: Reported in UAT test 12 (02-12 D4); test 10 (create mode, same phase) passed, so this is edit/copy-path-specific, not present since the editor's initial implementation for create mode.

## Eliminated

(none — root cause found on first traced hypothesis, no dead ends)

## Evidence

- timestamp: 2026-09-28T00:00:00Z
  checked: apps/admin/src/app/(admin)/routes/policy-editor.tsx (full file)
  found: validatePolicy() reads `tier.minHoursBefore ?? -1` and `tier.refundPercent ?? -1` (lines 25-26) — treats an *absent* field as a sentinel -1, which immediately fails `if (hours < 0) return POLICY_VALIDATION_ERROR`. Meanwhile the very same component's <Input value={...}> for those fields (lines 72, 90) uses `tier.minHoursBefore ?? 0` / `tier.refundPercent ?? 0` — treats an absent field as the proto3 zero-value default. Two different fallback semantics for the same "field absent" case in the same file.
  implication: If a tier object arrives with minHoursBefore/refundPercent *keys missing* (not present as literal 0), the UI will *display* "0" (render path defaults to 0) while the *validator* treats it as -1 and rejects it. This exactly matches the report: user sees 0/0 on screen (looks valid) but Save is blocked.

- timestamp: 2026-09-28T00:00:00Z
  checked: gen/ts/services/catalog/v1/catalog_pb.ts lines 1002-1043 (CancellationTier / CancellationTierJson types)
  found: `CancellationTier.minHoursBefore`/`refundPercent` are plain `int32` (proto field `int32 min_hours_before = 1` / `int32 refund_percent = 2`, no `optional` keyword). The companion wire-JSON type `CancellationTierJson` marks both as optional (`minHoursBefore?: number`, `refundPercent?: number`) — i.e. the generated types themselves document that these fields can be *absent* from the JSON wire representation.
  implication: This is exactly proto3 JSON-mapping behavior: protojson (used by connect-go's default JSON codec on the Go side, confirmed no EmitUnpopulated/EmitDefaultValues override anywhere in services/catalog or pkg) omits scalar fields that equal their zero value. A tier of `{min_hours_before: 0, refund_percent: 0}` (the mandatory 0h tier) serializes to `{}` on the wire, not `{minHoursBefore: 0, refundPercent: 0}`.

- timestamp: 2026-09-28T00:00:00Z
  checked: apps/admin/src/lib/api.ts (full file), specifically apiFetch() line 74 and rpc() lines 82-95
  found: The RPC helper does a bare `return (await res.json()) as T` — a raw JSON.parse cast straight to the generated TS type. There is no `fromJson()`/protobuf-schema-aware deserialization step anywhere in the admin app that would reconstruct proto3 zero-value defaults for keys omitted by the wire JSON.
  implication: Confirms the 0h tier really does arrive in the browser as a JS object with `minHoursBefore`/`refundPercent` literally `undefined` (missing keys), not `0`. Nothing between the wire and the component restores the default — the mismatch has to be handled (or not) purely inside policy-editor.tsx's own field access.

- timestamp: 2026-09-28T00:00:00Z
  checked: apps/admin/src/app/(admin)/routes/route-sheet.tsx lines 34-38, 60, 76, 83
  found: `DEFAULT_POLICY` (create-mode default) is a hand-written JS array literal: `{ minHoursBefore: 0, refundPercent: 0 }` — an *explicit* `0`, not an omitted key. `setPolicy(route?.cancellationPolicy ?? DEFAULT_POLICY)` (edit mode) and `setPolicy(source.cancellationPolicy ?? DEFAULT_POLICY)` (return-route copy mode) both assign the *API-sourced* array directly when `route`/`source` is defined — the `??` only guards against the whole array being null/undefined, not per-tier field omission.
  implication: Explains why create mode (test 10, pass) never hits the bug — its policy tiers always have real numeric 0 in the JS object. Edit mode and return-route mode (test 12, issue) both source `cancellationPolicy` straight from a fetched `RouteJson`/proto-JSON payload, which is exactly the path where the 0h tier's fields get dropped.

- timestamp: 2026-09-28T00:00:00Z
  checked: apps/admin/src/app/(admin)/routes/policy-editor.tsx addTier()/removeTier() (lines 52-58)
  found: `addTier()` pushes `{ minHoursBefore: 0, refundPercent: 0 }` — another explicit-0 object literal, same shape as DEFAULT_POLICY.
  implication: Explains the reported workaround exactly — deleting the broken (fields-omitted) 0h tier and clicking "เพิ่มระดับ" replaces it with a freshly constructed object that has real `0`s, which validatePolicy's `?? -1` now correctly reads as `0`, and validation passes. This is not a coincidental workaround, it's the direct mechanism.

- timestamp: 2026-09-28T00:00:00Z
  checked: apps/admin/src/app/(admin)/routes/policy-editor.tsx delete-button JSX (lines 99-111)
  found: `{tiers.length > 1 && (<Button ... onClick={() => removeTier(index)}><TrashIcon /></Button>)}` — this condition is evaluated per-tier and only checks the *total tier count*, never the tier's position/hours. It renders a delete button for every tier, including whichever one happens to be last/0-hour, whenever there is more than one tier.
  implication: Separate, unrelated-mechanism defect from the validation bug above (no shared code path — this is pure JSX with no dependency on validatePolicy or field-omission). UAT test 10 (create mode) expected "tier สุดท้ายไม่มีปุ่มลบ" (last tier has no delete button) and was marked pass, but the code as written has never implemented that guard for any tier count — the guard code (e.g. `index !== tiers.length - 1`) simply does not exist in this file. Test 12's tester caught it explicitly; test 10's tester likely didn't click that specific button. This is a missing-implementation gap, not a regression caused by the validation bug.

## Resolution

root_cause: "In apps/admin/src/app/(admin)/routes/policy-editor.tsx, validatePolicy() defaults an absent tier.minHoursBefore/refundPercent to the sentinel -1 (`?? -1`), while the same file's <Input value={...}> rendering defaults the identical absent fields to the proto3 zero-value 0 (`?? 0`). Because connect-go's default protojson JSON codec (no EmitUnpopulated override anywhere in services/catalog) omits int32 fields that equal 0 when serializing a route's cancellation_policy, and apps/admin's rpc()/apiFetch() client does a raw `res.json()` cast with no protobuf-JSON default-filling step, the mandatory 0-hour tier's fields arrive in the browser as `undefined` whenever the policy is loaded from the API (edit mode) or copied from another route (return-route mode) — never when it originates from the hardcoded DEFAULT_POLICY/addTier() object literals used in create mode. The UI therefore visibly displays '0' for that tier (render path's `?? 0`) while validatePolicy simultaneously rejects it as if hours were -1 (validate path's `?? -1`), producing a persistent false-invalid error and a disabled Save button that only clears once the tier is deleted and re-added as a fresh object with real numeric zeros."
fix: (not applied — goal: find_root_cause_only, parallel-run hard rule forbids source edits)
verification: (not applicable — no fix applied in this session)
files_changed: []
