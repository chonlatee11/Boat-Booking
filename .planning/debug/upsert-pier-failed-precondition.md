---
status: diagnosed
trigger: "UAT test 8 (G-02-8): super_admin creates a pier with a photo (photoKey) and opening hours in the admin Sheet. POST /api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertPier with pierId empty, operatorId=01a0e84b-218d-764f-ab25-442c41364895, photoKey=piers/01a0e857-9ec9-78fb-9148-7dfbe540870f.jpg returns {code:failed_precondition,message:failed precondition}; UI shows 'บันทึกท่าเรือไม่สำเร็จ กรุณาลองใหม่'. Photo upload/thumbnail succeeded before save. Also: UI time inputs showed 10:11 PM / 11:11 PM but payload sent opensAt/closesAt as empty string."
created: 2026-09-28T00:00:00Z
updated: 2026-09-28T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED (two independent root causes, both confirmed with direct evidence)
next_action: none — diagnose-only mode, returning ROOT CAUSE FOUND

## Symptoms

expected: "Creating a pier with a photo (photoKey) and opening hours in the admin Sheet saves successfully; the selected opensAt/closesAt are sent in the payload."
actual: "POST UpsertPier (super_admin, pierId empty, photoKey present after successful presign+upload) returns {code:failed_precondition,message:failed precondition}; UI shows generic save-failed toast. Separately, the time inputs visually showed 10:11 PM / 11:11 PM but the payload sent opensAt/closesAt as empty string."
errors: '{"code":"failed_precondition","message":"failed precondition"}'
reproduction: "As super_admin in apps/admin Piers page, open 'เพิ่มท่าเรือใหม่' Sheet, select operator 'ทดสอบ operators1' (01a0e84b-218d-764f-ab25-442c41364895) which was archived earlier in the same UAT session (test 4, 'archive รายที่ว่าง'), fill name/address/location, upload a photo, click บันทึก."
started: "Introduced by test 4 archiving 'ทดสอบ operators1', surfaced in test 8 when the same operator was reused for a new pier"

## Eliminated

- hypothesis: "FailedPrecondition caused by no object storage configured (02-08 D6 hint)"
  evidence: "docker inspect boatbooking-catalog-1 shows S3_PUBLIC_ENDPOINT=localhost:8333, S3_BUCKET=pier-photos, S3_ACCESS_KEY/S3_SECRET_KEY set, PHOTO_PUBLIC_BASE_URL=http://localhost:8333/pier-photos — all present. Also the symptom itself states 'Photo upload/thumbnail succeeded before save', meaning PresignPierPhoto already worked, which requires a non-nil Photos.Client. Storage is configured; this hint does not apply."
  timestamp: 2026-09-28T00:00:00Z
- hypothesis: "photo_key fails domain.Pier.Validate's photoKeyPattern regex"
  evidence: "photoKeyPattern = ^piers/[0-9a-f-]{36}\\.(jpg|png|webp)$. The observed key 'piers/01a0e857-9ec9-78fb-9148-7dfbe540870f.jpg' has a 36-char UUID body and a .jpg extension — matches. Even if it failed, Validate returns ErrInvalidArgument, not ErrFailedPrecondition, so this can't be the source of the observed code either way."
  timestamp: 2026-09-28T00:00:00Z

## Evidence

- timestamp: 2026-09-28T00:00:00Z
  checked: "docker inspect boatbooking-catalog-1 --format Config.Env"
  found: "S3_PUBLIC_ENDPOINT, S3_REGION, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY, PHOTO_PUBLIC_BASE_URL all set to dev SeaweedFS values"
  implication: "Storage is configured; PresignPierPhoto's nil-Client FailedPrecondition path (D-19) is not the cause"

- timestamp: 2026-09-28T00:00:00Z
  checked: "docker exec boatbooking-postgres-1 psql -U catalog -d catalog -c \"SELECT id, name, archived_at FROM operators WHERE id='01a0e84b-218d-764f-ab25-442c41364895';\""
  found: "Row exists: id=01a0e84b-218d-764f-ab25-442c41364895, name='ทดสอบ operators1', archived_at='2026-09-28 13:54:40.357989+00' (NOT NULL — archived)"
  implication: "The exact operator_id sent in the failing UpsertPier request is archived. services/catalog/internal/app/pier.go createPier() (lines 46-66) takes GetOperatorForShare, and 'if op.ArchivedAt.Valid { return domain.Pier{}, domain.ErrFailedPrecondition }' — this exact branch fires. Root cause of the failed_precondition confirmed."

- timestamp: 2026-09-28T00:00:00Z
  checked: "services/catalog/internal/domain/errors.go and adapters/http/scope.go toConnectErr"
  found: "ErrFailedPrecondition = errors.New(\"failed precondition\") — a bare sentinel. createPier's archived-operator branch returns this sentinel directly with zero added context (unlike ArchivePier's blockedArchiveError, which wraps '%w: active routes: ...' for a specific message). toConnectErr maps it straight to connect.CodeFailedPrecondition with err.Error() as the message, which the admin proxy passes through byte-for-byte (02-04 D1/D3 pattern)."
  implication: "The generic 'failed precondition' message on the wire is not being swallowed/mis-mapped by gateway or httpx — it originates as a deliberately terse message in catalog's own createPier code path. There is no additional detail to surface even in logs beyond what's already visible."

- timestamp: 2026-09-28T00:00:00Z
  checked: "apps/admin/src/app/(admin)/piers/pier-sheet.tsx (operator NativeSelect, lines 150-175) vs apps/admin/src/app/(admin)/piers/queries.ts useOperatorOptions() vs services/catalog/internal/adapters/postgres/queries/operators.sql ListOperatorsScoped"
  found: "ListOperatorsScoped SQL has no archived_at filter ('select * from operators where sqlc.arg(all_scope)::bool or id = sqlc.arg(operator_id) order by name, id') — it returns archived operators too. pier-sheet.tsx's operator <NativeSelectOption> maps over the full operators array with no filter and no disabled/archived indicator. Contrast: apps/admin/src/app/(admin)/operators page explicitly uses isMuted={(op) => Boolean(op.archived)} and a visible 'archived' badge (op.archived ? ... at line 81/124) to flag archived rows in its own table."
  implication: "The pier-create Sheet lets a super_admin pick an already-archived operator with zero visual cue, guaranteeing a server-side FailedPrecondition on save. This is the actual reproduction mechanism: test 4 archived 'ทดสอบ operators1', and test 8 picked that same operator (still selectable, unflagged) when creating a new pier."

- timestamp: 2026-09-28T00:00:00Z
  checked: "apps/admin/src/app/(admin)/piers/pier-sheet.tsx opensAt/closesAt state (lines 54-55, 68-69, 90-95, 116-117, 219-236) and apps/admin/src/app/layout.tsx html lang"
  found: "opensAt/closesAt are plain controlled <input type=\"time\"> bound 1:1 to onChange(e.target.value) — no debouncing, no reset-on-rerender logic, no code path that clears them after being set (handleOpenChange only resets state when the Sheet transitions closed->open, which does not re-fire while already open). html lang=\"th\" but Chromium/Firefox render <input type=\"time\"> using the browser/OS UI locale, not the page's lang attribute. hoursValid = !hoursPartial && !hoursOutOfOrder is trivially true when both opensAt and closesAt are empty strings (Boolean('')===Boolean('') => hoursPartial=false), so isValid does not require the hours fields to be filled at all."
  implication: "No application-code bug resets the state. The reported '10:11 PM / 11:11 PM shown, but payload empty' matches the well-documented native <input type=\"time\"> behavior in browsers configured with a 12-hour/AM-PM locale: the control's DOM .value (and thus the onChange event/React state) stays the empty string until the meridiem (AM/PM) segment is explicitly committed via keyboard, even though the hour/minute digits already display. If the tester typed digits but never explicitly pressed a key to commit AM/PM before clicking away, the browser shows the typed digits but never fires a valid onChange, leaving React's opensAt/closesAt state at its initial ''. Because hoursValid treats both-empty as valid, the Save button was never disabled, so the empty submission went through silently."

## Resolution

root_cause: "(1) Save failure (failed_precondition): the operator selected in the pier-create Sheet (01a0e84b-218d-764f-ab25-442c41364895, 'ทดสอบ operators1') was archived earlier in the same UAT session; catalog's createPier (services/catalog/internal/app/pier.go:57-66) correctly rejects creating a pier under an archived operator per D-08, returning the bare domain.ErrFailedPrecondition sentinel ('failed precondition', no extra detail) — this is by-design backend behavior, not a backend defect. The real gap is in apps/admin's pier-sheet.tsx: its operator dropdown (fed by ListOperators, whose ListOperatorsScoped SQL query does not filter archived_at) lists archived operators exactly like active ones, with no disabled state or 'archived' marker (unlike the operators page's own table, which already has this pattern via isMuted/op.archived). This let the super_admin pick a just-archived operator with no warning, guaranteeing the server-side rejection. ; (2) opensAt/closesAt sent empty despite visibly showing '10:11 PM'/'11:11 PM': native browser <input type=\"time\"> behavior — the control's value/onChange only commits once every segment (hour, minute, AND the AM/PM meridiem when the browser's locale is 12-hour) is explicitly set via keyboard; typing digits without explicitly committing AM/PM leaves the DOM .value (and therefore React's controlled opensAt/closesAt state, which pier-sheet.tsx binds 1:1 to onChange) as the empty string even though the visible segments show a filled-looking time. pier-sheet.tsx's hoursValid logic treats both-fields-empty as valid (Boolean('')===Boolean('')), so the Save button is never disabled by this, letting the empty submission through unnoticed."
fix: ""
verification: ""
files_changed: []
