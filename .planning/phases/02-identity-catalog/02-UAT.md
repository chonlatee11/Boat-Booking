---
status: complete
phase: 02-identity-catalog
source: [02-01-SUMMARY.md, 02-02-SUMMARY.md, 02-03-SUMMARY.md, 02-04-SUMMARY.md, 02-05-SUMMARY.md, 02-06-SUMMARY.md, 02-07-SUMMARY.md, 02-08-SUMMARY.md, 02-09-SUMMARY.md, 02-10-SUMMARY.md, 02-11-SUMMARY.md, 02-12-SUMMARY.md, 02-13-SUMMARY.md]
started: 2026-09-28T11:30:02Z
updated: 2026-10-03T06:57:30Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: ล้าง state: `make down` + ลบ volume ของ project (ครบทุก profile) แล้ว `make up` — stack บูตจากศูนย์ไม่มี error, migrate-identity / migrate-catalog exit 0, identity + catalog + gateway + seaweedfs healthy, GET http://localhost:8000/api/v1/public/piers ตอบ 200 (JSON)
result: pass

### 2. Human decisions ที่บันทึกไว้ (02-01 D1, D2)
expected: ยืนยันอีกครั้งว่ายังโอเคกับ: dev object storage = SeaweedFS chrislusf/seaweedfs:4.47 และแพ็กเกจ go-redis v9.22.0, minio-go v7.3.0, maplibre-gl 6.11.2 ที่คุณเคย approve ไว้
result: pass

### 3. Admin login super_admin (02-09 D2, D3)
expected: เปิด http://localhost:3002/login ขอ OTP ด้วย SUPER_ADMIN_EMAIL แล้วเปิดเมลใน Mailpit (http://localhost:8025) → body อ่านเป็น "รหัสของคุณ / Your code: ......" ภาษาไทยถูกต้อง (ไม่ใช่ mojibake) และ Subject ภาษาไทยยังถูกต้อง
result: pass
retest_of: ""ผ่านหมด แต่ใน Mailpit มันแสดงผลภาษาที่อ่านไม่ออก — Subject แสดงไทยถูก แต่ body ภาษาไทยเป็น mojibake (เช่น 'à¸£à¸«à¸±à¸ª...'); ส่วนภาษาอังกฤษ/รหัสอ่านได้""
retest_after: 02-14

### 4. Operators CRUD (02-09 D4)
expected: หน้า operators: สร้าง operator 2 ราย, แก้ชื่อ 1 ราย, archive รายที่ว่าง (มี confirm dialog + toast), เห็น skeleton ตอนโหลด/empty state ตอนว่าง; ย่อจอ 768px และ 360px ตาราง scroll ภายในกรอบ ไม่ทำให้ทั้งหน้าเลื่อนแนวนอน
result: pass

### 5. Customer web OTP login (02-10 D4)
expected: เปิด http://localhost:3001/th/login ที่ 360px, ล็อกอินด้วยอีเมลยาว 40+ ตัว (รหัสจาก Mailpit): ใส่รหัสผิด 2 ครั้งเห็นครั้งที่เหลือ 4 แล้ว 3, ขอรหัสใหม่ทันทีเห็นข้อความ cooldown/นับถอยหลัง 60 วิ, ใส่รหัสถูก header เปลี่ยนเป็น signed-in, /en/login ข้อความอังกฤษ, sign out แล้วกลับเป็นลิงก์ sign-in; devtools Local Storage ไม่มี token (มีแค่ httpOnly cookie)
result: pass

### 6. Phone login ผ่าน dev SMS (02-02 D7)
expected: ที่หน้า login ของ web ใส่เบอร์ไทยแบบ local (เช่น 0812345678) → ใน Mailpit มีเมลถึง <เลข E.164>@sms.local พร้อมรหัส, ใส่รหัสแล้วล็อกอินได้เป็น customer
result: pass

### 7. Piers: สร้าง/แก้ไขพร้อม map picker (02-11 D1)
expected: super_admin หน้า /piers → "เพิ่มท่าเรือใหม่": พิมพ์ lat/lng ทีละตัว (รวมค่าค้าง เช่น "13." และค่าเกินช่วง เช่น "137"), ลาก marker → ไม่มี crash/dev overlay เลย; marker ย้ายตามคู่ lat/lng ที่ valid และตามการลาก; ค่าเกินช่วงขึ้น error inline + ปุ่มบันทึก disabled โดย marker ไม่ขยับ
result: pass
retest_of: ""กำลังพิม latitude แล้วหน้าพัง — Next.js dev overlay: Console Error 'Worker failed to load. Check that the worker URL is correct.' (2 issues, หน้าเบื้องหลังเป็น error page); และลองพิมพ์ lat/lng แล้ว marker ไม่ย้าย""
retest_after: 02-16

### 8. Pier photo upload + map ล้มเหลว
expected: archive operator ที่ไม่มีท่า แล้วเปิด Sheet สร้าง pier; สร้าง pier พร้อมรูป + เวลา "08:00"/"17:30" (ดู payload ใน devtools Network); ลองพิมพ์ "8:0" ในเวลาเปิด; block tiles.openfreemap.org ใน devtools แล้วเปิด Sheet ใหม่ → operator ที่ archive แล้วไม่อยู่ใน select; บันทึกสำเร็จและ payload UpsertPier มี opensAt/closesAt ตรงตามที่พิมพ์ (ไม่ว่าง); "8:0" ขึ้น error inline + บันทึก disabled; "โหลดแผนที่ไม่สำเร็จ" ขึ้นเฉพาะตอน block tiles และยังพิมพ์ lat/lng เองได้
result: pass
retest_of: ""ลาก marker บนแผนที่ได้ แต่บันทึก pier ไม่ได้: POST /api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertPier (super_admin, pierId ว่าง, มี photoKey piers/<uuid>.jpg หลังอัปโหลด thumbnail ขึ้นแล้ว) ตอบ {code:failed_precondition, message:failed precondition} → UI แสดง บันทึกท่าเรือไม่สำเร็จ กรุณาลองใหม่. สังเกตเพิ่ม: UI เวลาเปิด/ปิด แสดง 10:11 PM / 11:11 PM แต่ payload ส่ง opensAt/closesAt เป็น string ว่าง; ขึ้น โหลดแผนที่ไม่สำเร็จ ทั้งที่ tile แสดงบางส่วน""
retest_after: 02-15, 02-16

### 9. Routes: สร้าง/แก้ไข + ชื่อ derive (02-12 D1)
expected: หน้า /routes ที่มีท่าชื่อ "Proof Pier" ซ้ำกันหลายท่า: ดูรายการ routes, archive dialog, และ select ท่าต้นทาง/ปลายทางใน route Sheet; แล้วลองสร้างคู่ (pier_from, pier_to) เดิมซ้ำ → ท่าชื่อซ้ำแสดง label มี suffix id (เช่น "Proof Pier (xxxx)") ทุกที่ แยกออกได้; ท่าชื่อไม่ซ้ำไม่มี suffix; สร้างคู่ซ้ำขึ้น "มีเส้นทางนี้อยู่แล้ว"
result: pass
retest_of: ""ไม่ผ่าน มันสร้างซ้ำได้ — รายการ routes มี 2 แถว Proof Pier → Proof Pier (60 นาที, ใช้งาน) ไม่ขึ้น มีเส้นทางนี้อยู่แล้ว""
retest_after: 02-17

### 10. Cancellation policy editor (02-12 D2)
expected: ตอนสร้าง route นโยบายเริ่มต้นเป็น >24ชม. 100% / 2-24ชม. 50% / <2ชม. 0%; เพิ่ม/ลบ tier ได้ (tier สุดท้ายไม่มีปุ่มลบ); ทำให้ชั่วโมงไม่เรียงลดลง หรือไม่มี tier 0 ชม. → ปุ่มบันทึกถูกบล็อกพร้อมข้อความตาม UI-SPEC, แก้กลับแล้วบันทึกได้
result: pass

### 11. Route prices (02-12 D3)
expected: เพิ่มราคา adult 150.50 / child 80 มีผลวันนี้ และอีกราคาที่มีผลในอนาคต → ประวัติราคาเรียงใหม่สุดก่อน, ราคาปัจจุบันแสดง ฿150.50, เลือกวันที่ก่อนวันนี้ไม่ได้, ไม่มีปุ่มแก้ราคา (เพิ่มได้อย่างเดียว)
result: pass

### 12. Return route (02-12 D4)
expected: เปิด "แก้ไขเส้นทาง" ของ route ที่มี policy 24/100, 2/50, 0/0 และแยกกันกด "สร้างเส้นทางย้อนกลับ" → ทั้งสอง Sheet ไม่ขึ้น error policy และปุ่มบันทึก enabled ทันที (ไม่ต้องลบ-เพิ่ม tier); tier สุดท้าย (0 ชม.) ไม่มีปุ่มลบ แม้ลบ tier จนเหลืออันเดียว; เส้นทางย้อนกลับเติมท่าสลับกัน + duration/policy เดิม
result: pass
retest_of: ""ข้อก่อนหน้ากดบันทึกไม่ได้ ขึ้นแบบนี้ตลอด และต้องกดลบเพิ่มระดับก่อน แล้วกดเพิ่มใหม่ถึงจะบันทึกได้ — Sheet แก้ไขเส้นทาง: policy 24/100, 2/50, 0/0 (ถูกต้อง) แต่ขึ้น นโยบายยกเลิกไม่ถูกต้อง: ต้องเรียงชั่วโมงจากมากไปน้อยและมีระดับ 0 ชั่วโมงเสมอ และปุ่มบันทึก disabled; tier สุดท้ายมีปุ่มลบด้วย""
retest_after: 02-17

### 13. Pier archive ถูกบล็อกเมื่อมี route (02-11 D6)
expected: archive pier ที่มี route active อยู่ → ขึ้น error inline แสดงรายการ route ที่ขวางอยู่ (ไม่ใช่ confirm dialog ที่สอง); ล็อกอินเป็น staff แล้วไม่เห็นปุ่มเขียน/แก้ไขใดๆ
result: pass

### 14. Boats (02-12 D5)
expected: ในฐานะ pier_admin หน้า boats: สร้าง/แก้ไข/archive เรือ (ชื่อ, ความจุ, สถานะ, home pier เลือกได้เฉพาะท่าของตัวเอง); badge สถานะ ใช้งาน=เขียว / ซ่อมบำรุง=ส้ม; ที่ 768px ตาราง scroll ในกรอบ, ชื่อยาวถูกตัดพร้อม tooltip
result: pass

### 15. Staff: สร้าง user + scope (02-13 D1)
expected: ในฐานะ super_admin หน้า staff: สร้าง pier_admin ของ operator A ผูก 2 ท่า → ขึ้นในรายการ; เปิด private window ล็อกอินเป็น user นั้น (รหัสจาก Mailpit) → /piers /routes /boats เห็นเฉพาะของท่าที่ได้รับมอบหมาย, อีเมลซ้ำขึ้น "อีเมลนี้มีผู้ใช้งานแล้ว"
result: pass

### 16. Staff: แก้ไข/disable (02-13 D2)
expected: กลับมาเป็น super_admin: ถอดท่าออก 1 ท่า (อีเมลแก้ไม่ได้) → private window refresh แล้วเห็นท่าเดียว; disable user (ข้อความยืนยันพูดถึงมีผลภายใน 15 นาที) → ใน private window ลบ cookie access_token (หรือรอหมดอายุ) แล้วนำทาง → refresh ล้มเหลวและถูกส่งไป /login; enable กลับแล้วล็อกอินได้อีก
result: pass

### 17. Pier badges wrap (02-13 D5)
expected: ในตาราง staff user ที่มีหลายท่า badge ตัดบรรทัดได้โดยแถวไม่เบี้ยว/ไม่ misalign
result: pass

### 18. Concurrent UpsertBoat (02-08 D8, backend)
expected: (backend — ไม่มี UI ให้ทดสอบตรงๆ) ยอมรับได้ไหมว่า UpsertBoat ใช้ pattern UPDATE ... WHERE id=$1 แถวเดียวเหมือน routes ที่พิสูจน์แล้วว่า race-safe → concurrent upsert เรือเดียวกันได้ 1 แถว + 1 outbox ต่อ write; ตอบ pass / skip หรือขอให้เขียน test
result: pass
source: automated
note: "TestConcurrentUpsertBoatSameID (services/catalog/cmd/boats_photo_integration_test.go, plan 02-15) — PASS"

### 19. pier_ids is a signed JWT claim that round-trips Issue -> Verify; nil PierIDs verifies to a (02-01 D3)
expected: pier_ids is a signed JWT claim that round-trips Issue -> Verify; nil PierIDs verifies to an empty (non-nil) slice; role constants defined once in pkg/auth
result: pass
source: automated
coverage_id: D3

### 20. httpx.RequireInternal parses/validates X-Pier-Ids (uuid.Parse per part, 401 on any invalid (02-01 D4)
expected: httpx.RequireInternal parses/validates X-Pier-Ids (uuid.Parse per part, 401 on any invalid part, next never called); httpx.ForwardClaims (moved from gateway) deletes all five trusted headers first and sets X-Pier-Ids only when non-empty
result: pass
source: automated
coverage_id: D4

### 21. GET /api/v1/whoami returns pier_ids as a JSON array end-to-end through Kong, for a devtoke (02-01 D5)
expected: GET /api/v1/whoami returns pier_ids as a JSON array end-to-end through Kong, for a devtoken minted with -pier-ids
result: pass
source: automated
coverage_id: D5

### 22. identity service scaffolded (services/identity, deploy/services.txt, go.work), own Postgre (02-02 D1)
expected: identity service scaffolded (services/identity, deploy/services.txt, go.work), own Postgres DB, identity.events topic, /healthz + /readyz reporting db/kafka/valkey
result: pass
source: automated
coverage_id: D1

### 23. RequestOtp -> VerifyOtp full email happy path: crypto/rand 6-digit code, HMAC-hashed in Va (02-02 D2)
expected: RequestOtp -> VerifyOtp full email happy path: crypto/rand 6-digit code, HMAC-hashed in Valkey with 300s TTL, delivered via SMTP to Mailpit, single-use via atomic DEL, auto-creates a customer row + identity.UserCreated outbox event on first login
result: pass
source: automated
coverage_id: D2

### 24. An existing user (staff/pier_admin) logging in via OTP gets their stored role/operator_id/ (02-02 D3)
expected: An existing user (staff/pier_admin) logging in via OTP gets their stored role/operator_id/pier_ids in the issued JWT, with no duplicate row or event created
result: pass
source: automated
coverage_id: D3

### 25. Wrong-code lockout: Attempts-Left counts down 4,3,2,1 across 4 wrong codes; the 5th wrong  (02-02 D4)
expected: Wrong-code lockout: Attempts-Left counts down 4,3,2,1 across 4 wrong codes; the 5th wrong code (and the correct code afterwards) return FailedPrecondition
result: pass
source: automated
coverage_id: D4

### 26. Send-rate limits: an immediate resend is rejected (60s cooldown); a 6th send to the same d (02-02 D5)
expected: Send-rate limits: an immediate resend is rejected (60s cooldown); a 6th send to the same destination within an hour is rejected
result: pass
source: automated
coverage_id: D5

### 27. Concurrent VerifyOtp calls with the same correct code: exactly one succeeds, the other 9 g (02-02 D6)
expected: Concurrent VerifyOtp calls with the same correct code: exactly one succeeds, the other 9 get FailedPrecondition, exactly one customer row is created
result: pass
source: automated
coverage_id: D6

### 28. No OTP code ever reaches stdout, including on the wrong-code mismatch path (captured proce (02-02 D8)
expected: No OTP code ever reaches stdout, including on the wrong-code mismatch path (captured process stdout, since httpx.NewLogger doesn't route through slog.SetDefault)
result: pass
source: automated
coverage_id: D8

### 29. Every identity RPC rejects a caller without the internal token with 401 (both RPCs share o (02-02 D9)
expected: Every identity RPC rejects a caller without the internal token with 401 (both RPCs share one httpx.RequireInternal-gated chi route group in cmd/main.go, so the one tested RPC's rejection generalises structurally to the other)
result: pass
source: automated
coverage_id: D9

### 30. NormalizeDestination and ResendSender unit behavior: phone/email classification table, and (02-02 D10)
expected: NormalizeDestination and ResendSender unit behavior: phone/email classification table, and the Resend REST client posts the expected JSON/auth header and never leaks the code into a delivery-failure error
result: pass
source: automated
coverage_id: D10

### 31. Operators end-to-end: super_admin creates/renames via CatalogService.UpsertOperator, every (02-03 D1)
expected: Operators end-to-end: super_admin creates/renames via CatalogService.UpsertOperator, every other role (pier_admin, staff, customer) gets PermissionDenied, no claims gets Unauthenticated; ListOperators applies the Scope rule, ordered by name then id with an id tiebreak
result: pass
source: automated
coverage_id: D1

### 32. Piers scoped by (operator_id, pier_ids): super_admin unrestricted (optionally filtered by  (02-03 D2)
expected: Piers scoped by (operator_id, pier_ids): super_admin unrestricted (optionally filtered by operator_id), pier_admin/staff scoped to their claims, an empty pier_ids scope sees nothing; updates can never move a pier to a different operator even when the request tries; out-of-scope/cross-operator ids answer NotFound, never PermissionDenied, never the row; PierUpserted carries no address field
result: pass
source: automated
coverage_id: D2

### 33. Public (no-claims) ListPiers returns only non-archived piers with full projection (ids, na (02-03 D3)
expected: Public (no-claims) ListPiers returns only non-archived piers with full projection (ids, names, coordinates, address, hours) — CAT-06
result: pass
source: automated
coverage_id: D3

### 34. ArchiveOperator is FailedPrecondition while the operator owns any non-archived pier (no ca (02-03 D4)
expected: ArchiveOperator is FailedPrecondition while the operator owns any non-archived pier (no cascade, D-15); archiving an operator with none succeeds and is idempotent; creating a pier for an archived operator fails FailedPrecondition
result: pass
source: automated
coverage_id: D4

### 35. domain.Pier.Validate: Unicode-code-point name/address length limits (including combining m (02-03 D5)
expected: domain.Pier.Validate: Unicode-code-point name/address length limits (including combining marks), lat/lng range and (0,0) rejection, opens_at/closes_at both-empty-or-both-HH:MM-with-opens<closes
result: pass
source: automated
coverage_id: D5

### 36. POST /api/v1/admin/{service}/{method} allow-lists CatalogService/UserService by exact name (02-04 D1)
expected: POST /api/v1/admin/{service}/{method} allow-lists CatalogService/UserService by exact name, 404s any other service or malformed method, requires a valid staff/pier_admin/super_admin access token (401/403 otherwise), rejects non-JSON content-type and bodies over 64 KiB before any upstream contact, and passes the upstream's connect response (status + body) through unchanged
result: pass
source: automated
coverage_id: D1

### 37. Every client-supplied X-User-Id/X-Operator-Id/X-Role/X-Pier-Ids/X-Internal-Token, Cookie a (02-04 D2)
expected: Every client-supplied X-User-Id/X-Operator-Id/X-Role/X-Pier-Ids/X-Internal-Token, Cookie and Authorization is deleted before httpx.ForwardClaims sets verified values on the outbound admin-proxy request (Anti-Pattern 2)
result: pass
source: automated
coverage_id: D2

### 38. GET /api/v1/public/{boats|piers|routes} calls the mapped CatalogService RPC with body {} a (02-04 D3)
expected: GET /api/v1/public/{boats|piers|routes} calls the mapped CatalogService RPC with body {} and only X-Internal-Token — never any claim header or cookie, even when the inbound request carries spoofed ones — and passes the upstream response through byte-for-byte; an unknown resource is 404 without contacting the upstream
result: pass
source: automated
coverage_id: D3

### 39. Legacy POST /api/v1/boats REST route and its typed catalogv1connect client are removed fro (02-04 D4)
expected: Legacy POST /api/v1/boats REST route and its typed catalogv1connect client are removed from the gateway; deploy/proof.sh, deploy/kong/roundtrip.sh and bruno/Boat-Booking/boats.bru all work against the new proxy paths, and make kong-roundtrip + make proof stay green
result: pass
source: automated
coverage_id: D4

### 40. Browser OTP login through Kong's api-auth route (no JWT plugin, 20/min rate limit, CORS fr (02-05 D1)
expected: Browser OTP login through Kong's api-auth route (no JWT plugin, 20/min rate limit, CORS from localhost:3001/3002): otp/request -> otp/verify with the real code read from Mailpit sets httpOnly/Secure/SameSite=Lax access_token+refresh_token cookies and returns {userId, role, operatorId, pierIds} with no token in the body
result: pass
source: automated
coverage_id: D1

### 41. Wrong-code, cooldown/hourly, and unmapped-code error shapes: 400 {code:invalid_argument, a (02-05 D2)
expected: Wrong-code, cooldown/hourly, and unmapped-code error shapes: 400 {code:invalid_argument, attemptsLeft:n} on a wrong code, 429 on cooldown/hourly, pkg/httpx maps ResourceExhausted->429 and FailedPrecondition->400 with the message preserved
result: pass
source: automated
coverage_id: D2

### 42. No client-supplied claim header ever reaches identity through the auth routes, and an over (02-05 D3)
expected: No client-supplied claim header ever reaches identity through the auth routes, and an oversized body is rejected before any upstream contact
result: pass
source: automated
coverage_id: D3

### 43. Refresh(A) rotates to a fresh refresh token, revokes A, and re-reads the user's current ro (02-05 D4)
expected: Refresh(A) rotates to a fresh refresh token, revokes A, and re-reads the user's current role/operator_id/pier_ids on every call (D-10); a role/pier change reaches the very next access token
result: pass
source: automated
coverage_id: D4

### 44. Replaying an already-rotated refresh token revokes the whole session family (both the repl (02-05 D5)
expected: Replaying an already-rotated refresh token revokes the whole session family (both the replayed token and the one issued after it are rejected); an expired or unknown token is rejected; a disabled user's refresh is rejected
result: pass
source: automated
coverage_id: D5

### 45. Gateway refresh/logout cookie shaping: missing/invalid refresh cookie or any identity erro (02-05 D6)
expected: Gateway refresh/logout cookie shaping: missing/invalid refresh cookie or any identity error clears both cookies; logout always returns 204 and clears cookies even when identity errors
result: pass
source: automated
coverage_id: D6

### 46. identity startup bootstrap (D-09): SUPER_ADMIN_EMAIL becomes a non-disabled super_admin id (02-05 D7)
expected: identity startup bootstrap (D-09): SUPER_ADMIN_EMAIL becomes a non-disabled super_admin idempotently — a repeat call, a pre-existing non-super-admin row, and a disabled super_admin all converge to the same state with at most one identity.UserCreated ever published
result: pass
source: automated
coverage_id: D7

### 47. Logging in through Kong with SUPER_ADMIN_EMAIL yields a whoami with role super_admin and e (02-05 D8)
expected: Logging in through Kong with SUPER_ADMIN_EMAIL yields a whoami with role super_admin and empty pier_ids
result: pass
source: automated
coverage_id: D8

### 48. Routes end-to-end: pier_admin creates/edits a one-way route only when pier_from is in thei (02-06 D1)
expected: Routes end-to-end: pier_admin creates/edits a one-way route only when pier_from is in their (operator_id, pier_ids) scope; pier_to may be any non-archived pier of any operator; route.operator_id always derived from pier_from; a second active route for the same pair is AlreadyExists; update-by-route_id updates the same row
result: pass
source: automated
coverage_id: D1

### 49. Effective-dated prices (D-14): AddRoutePrice validates amount/ticket_type/not-before-today (02-06 D2)
expected: Effective-dated prices (D-14): AddRoutePrice validates amount/ticket_type/not-before-today, publishes catalog.PriceChanged, and re-adding the same (route, ticket_type, effective_from) replaces the amount in one row; ListRoutePrices orders effective_from desc then ticket_type
result: pass
source: automated
coverage_id: D2

### 50. Public (no-claims) ListRoutes serves CAT-06: non-archived routes between non-archived pier (02-06 D3)
expected: Public (no-claims) ListRoutes serves CAT-06: non-archived routes between non-archived piers, each with current_prices reflecting the price in effect for today's Asia/Bangkok date (inclusive effective_from boundary), absent (never zero) when no price is set
result: pass
source: automated
coverage_id: D3

### 51. Archive rules (D-15): ArchivePier is rejected with FailedPrecondition while any non-archiv (02-06 D4)
expected: Archive rules (D-15): ArchivePier is rejected with FailedPrecondition while any non-archived route uses it as pier_from/pier_to, naming only in-scope routes and counting the rest as other operators; ArchiveRoute/ArchivePier are idempotent; archived piers/routes disappear from public lists and reject further writes
result: pass
source: automated
coverage_id: D4

### 52. Only super_admin can call UserService (ListUsers, UpsertUser, SetUserDisabled); pier_admin (02-07 D1)
expected: Only super_admin can call UserService (ListUsers, UpsertUser, SetUserDisabled); pier_admin/staff get PermissionDenied, no claims gets Unauthenticated
result: pass
source: automated
coverage_id: D1

### 53. UpsertUser only assigns role staff or pier_admin; customer and super_admin are rejected wi (02-07 D2)
expected: UpsertUser only assigns role staff or pier_admin; customer and super_admin are rejected with InvalidArgument (super_admin only via SUPER_ADMIN_EMAIL, D-09)
result: pass
source: automated
coverage_id: D2

### 54. Before persisting, UpsertUser calls catalog ListPiers (forwarding caller claims + internal (02-07 D3)
expected: Before persisting, UpsertUser calls catalog ListPiers (forwarding caller claims + internal token, operator_id filter) and rejects with InvalidArgument naming any pier that is missing, archived, or owned by another operator
result: pass
source: automated
coverage_id: D3

### 55. A created staff user who signs in via OTP receives a JWT with that role, operator_id, and  (02-07 D4)
expected: A created staff user who signs in via OTP receives a JWT with that role, operator_id, and pier_ids
result: pass
source: automated
coverage_id: D4

### 56. Creating a user whose email already belongs to staff/pier_admin/super_admin returns Alread (02-07 D5)
expected: Creating a user whose email already belongs to staff/pier_admin/super_admin returns AlreadyExists and changes nothing; an existing customer email is promoted in place (same user id), with no second identity.UserCreated
result: pass
source: automated
coverage_id: D5

### 57. Five concurrent UpsertUser creates for the same new email yield exactly one success and fo (02-07 D6)
expected: Five concurrent UpsertUser creates for the same new email yield exactly one success and four AlreadyExists, with one users row
result: pass
source: automated
coverage_id: D6

### 58. SetUserDisabled(true) sets disabled_at and revokes all refresh tokens (refresh -> Unauthen (02-07 D7)
expected: SetUserDisabled(true) sets disabled_at and revokes all refresh tokens (refresh -> Unauthenticated, OTP login -> PermissionDenied); disabling yourself or a super_admin is FailedPrecondition; SetUserDisabled(false) re-enables login
result: pass
source: automated
coverage_id: D7

### 59. ListUsers returns non-customer users ordered by email then id, optionally filtered by oper (02-07 D8)
expected: ListUsers returns non-customer users ordered by email then id, optionally filtered by operator_id; customers are never listed; updating by user_id changes name/role/operator/piers with catalog re-validation, email is immutable, and a customer/super_admin target row is FailedPrecondition
result: pass
source: automated
coverage_id: D8

### 60. Every boat write requires home_pier_id; boat.operator_id is always its home pier's operato (02-08 D1)
expected: Every boat write requires home_pier_id; boat.operator_id is always its home pier's operator; pier_admin may create/edit/archive a boat only when home_pier_id is in their pier_ids (and operator); out-of-scope boats answer NotFound; staff/customer writes are denied
result: pass
source: automated
coverage_id: D1

### 61. catalog.BoatUpserted gains home_pier_id (field 6) and archived (field 7) as additive-only  (02-08 D2)
expected: catalog.BoatUpserted gains home_pier_id (field 6) and archived (field 7) as additive-only fields; schedule's existing consumer keeps applying the event unchanged
result: pass
source: automated
coverage_id: D2

### 62. ListBoats with pier_admin/staff claims returns only boats whose home pier is in scope; emp (02-08 D3)
expected: ListBoats with pier_admin/staff claims returns only boats whose home pier is in scope; empty pier_ids -> empty list; super_admin -> all; no claims -> every non-archived boat
result: pass
source: automated
coverage_id: D3

### 63. Missing home pier / empty name -> InvalidArgument; UpsertBoat with existing boat_id update (02-08 D4)
expected: Missing home pier / empty name -> InvalidArgument; UpsertBoat with existing boat_id updates in place; ArchiveBoat is idempotent and archived boats reject edits
result: pass
source: automated
coverage_id: D4

### 64. PresignPierPhoto (pier_admin/super_admin) allow-lists image/jpeg|png|webp and 1..5,242,880 (02-08 D5)
expected: PresignPierPhoto (pier_admin/super_admin) allow-lists image/jpeg|png|webp and 1..5,242,880 bytes, generates piers/<uuidv7>.<ext> server-side, returns a 10-minute PUT URL signed over Content-Type and Content-Length
result: pass
source: automated
coverage_id: D5

### 65. UpsertPier accepts photo_key only as piers/<uuid>.(jpg|png|webp); ListPiers returns photo_ (02-08 D6)
expected: UpsertPier accepts photo_key only as piers/<uuid>.(jpg|png|webp); ListPiers returns photo_url = PHOTO_PUBLIC_BASE_URL + '/' + photo_key; FailedPrecondition with no storage configured and catalog still starts
result: pass
source: automated
coverage_id: D6

### 66. make proof and make kong-roundtrip pass with a super_admin creating operator -> pier -> bo (02-08 D7)
expected: make proof and make kong-roundtrip pass with a super_admin creating operator -> pier -> boat through the admin proxy; roundtrip also checks public piers/routes return 200
result: pass
source: automated
coverage_id: D7

### 67. Every boat write requires home_pier_id; the boat's operator_id is always its home pier's o (02-08 D9)
expected: Every boat write requires home_pier_id; the boat's operator_id is always its home pier's operator (upload validation prohibitions: server-generated key, signed Content-Type/Content-Length)
result: pass
source: automated
coverage_id: D9

### 68. apps/admin scaffolded as a separate Thai-only Next.js 16 app (no next-intl, no locale pref (02-09 D1)
expected: apps/admin scaffolded as a separate Thai-only Next.js 16 app (no next-intl, no locale prefix) on :3002, with byte-identical shadcn/Tailwind tokens to apps/web and no @tanstack/react-table dependency
result: pass
source: automated
coverage_id: D1

### 69. apps/admin is gated by the same make lint/web-check checks as apps/web, the Jenkins Tools  (02-09 D5)
expected: apps/admin is gated by the same make lint/web-check checks as apps/web, the Jenkins Tools stage installs its dependencies, and a compose admin service serves it on 127.0.0.1:3002 under the web profile
result: pass
source: automated
coverage_id: D5

### 70. auth.* TH/EN message keys match the UI-SPEC Copywriting Contract exactly (destination labe (02-10 D1)
expected: auth.* TH/EN message keys match the UI-SPEC Copywriting Contract exactly (destination label, send/verify CTAs, resend countdown, wrong-code/expired/rate-limited errors)
result: pass
source: automated
coverage_id: D1

### 71. apps/web builds with the /[locale]/login route present and TypeScript/ESLint/Prettier clea (02-10 D2)
expected: apps/web builds with the /[locale]/login route present and TypeScript/ESLint/Prettier clean
result: pass
source: automated
coverage_id: D2

### 72. apiFetch retries once after POST /api/v1/auth/refresh on a 401 for non-auth paths via one  (02-10 D3)
expected: apiFetch retries once after POST /api/v1/auth/refresh on a 401 for non-auth paths via one shared in-flight promise; non-2xx responses carry status/code/message/attemptsLeft
result: pass
source: automated
coverage_id: D3

### 73. Map style URL comes from NEXT_PUBLIC_MAP_STYLE_URL (default OpenFreeMap); no API key, no g (02-11 D2)
expected: Map style URL comes from NEXT_PUBLIC_MAP_STYLE_URL (default OpenFreeMap); no API key, no geocoding (D-20)
result: pass
source: automated
coverage_id: D2

### 74. Pier photo (jpeg/png/webp <=5MB) uploads directly from the browser to object storage via P (02-11 D3)
expected: Pier photo (jpeg/png/webp <=5MB) uploads directly from the browser to object storage via PresignPierPhoto; pier saves only photo_key; list/public API show photo_url (D-19, CAT-06)
result: pass
source: automated
coverage_id: D3

### 75. Dev object storage (SeaweedFS) runs in Compose on 127.0.0.1:8333 with a public-read pier-p (02-11 D4)
expected: Dev object storage (SeaweedFS) runs in Compose on 127.0.0.1:8333 with a public-read pier-photos bucket, catalog-scoped read/write identity, and CORS allowing PUT from http://localhost:3002; catalog gets S3_*/PHOTO_PUBLIC_BASE_URL env
result: pass
source: automated
coverage_id: D4

### 76. A PUT whose Content-Length differs from the presigned size is rejected with 403 (upload in (02-11 D5)
expected: A PUT whose Content-Length differs from the presigned size is rejected with 403 (upload integrity, T-02-11-01)
result: pass
source: automated
coverage_id: D5

### 77. Per route, the admin sees price history newest-effective-first (server-ordered) and adds a (02-12 D3)
expected: Per route, the admin sees price history newest-effective-first (server-ordered) and adds adult/child prices in baht converted to integer satang without floating-point arithmetic (bahtToSatang), with an effective-from date that cannot be before today (Asia/Bangkok); there is no edit-price action, only new effective-dated rows (D-14)
result: pass
source: automated
coverage_id: D3

### 78. Duplicate email shows อีเมลนี้มีผู้ใช้งานแล้ว and invalid pier assignments show the backen (02-13 D3)
expected: Duplicate email shows อีเมลนี้มีผู้ใช้งานแล้ว and invalid pier assignments show the backend message inline, with the Dialog kept open
result: pass
source: automated
coverage_id: D3

### 79. The staff page and its nav entry are shown only to super_admin (cosmetic — UserService enf (02-13 D4)
expected: The staff page and its nav entry are shown only to super_admin (cosmetic — UserService enforces server-side)
result: pass
source: automated
coverage_id: D4

### 80. E5 partial: the staff form requires role and operator, and pier selection is required (at  (02-13 D6)
expected: E5 partial: the staff form requires role and operator, and pier selection is required (at least one pier) before submit enables
result: pass
source: automated
coverage_id: D6

### 81. E5 empty/loading/error: create mode opens empty with submit disabled until valid; edit mod (02-13 D7)
expected: E5 empty/loading/error: create mode opens empty with submit disabled until valid; edit mode shows skeleton fields and a spinner on submit; a save failure shows บันทึกผู้ใช้งานไม่สำเร็จ กรุณาลองใหม่ with values intact
result: pass
source: automated
coverage_id: D7

### 82. E2 empty/error: ยังไม่มีผู้ใช้งาน / เริ่มต้นด้วยการเพิ่มผู้ใช้งานแรกของคุณ with the CTA, a (02-13 D8)
expected: E2 empty/error: ยังไม่มีผู้ใช้งาน / เริ่มต้นด้วยการเพิ่มผู้ใช้งานแรกของคุณ with the CTA, and โหลดข้อมูลผู้ใช้งานไม่สำเร็จ กรุณาลองใหม่ with retry
result: pass
source: automated
coverage_id: D8

## Summary

total: 82
passed: 82
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-02-3
  truth: "OTP email body แสดงภาษาไทยถูกต้องใน Mailpit (UTF-8), ไม่ใช่ mojibake"
  status: resolved
  resolved_by: 02-14-PLAN.md
  resolved_at: 2026-09-29
  reason: "User reported: ผ่านหมด แต่ใน Mailpit มันแสดงผลภาษาที่อ่านไม่ออก — Subject แสดงไทยถูก แต่ body ภาษาไทยเป็น mojibake (เช่น 'à¸£à¸«à¸±à¸ª...'); ส่วนภาษาอังกฤษ/รหัสอ่านได้"
  severity: minor
  test: 3
  root_cause: "sendSMTPMessage builds raw message with no MIME-Version/Content-Type/Content-Transfer-Encoding; UTF-8 Thai body defaults to us-ascii and Mailpit renders it as Latin-1 mojibake; Subject is raw UTF-8 (not RFC 2047), renders only via Mailpit leniency. Shared by SMTPSender and DevSMSSender."
  artifacts:
    - path: "services/identity/internal/adapters/notify/notify.go"
      issue: "sendSMTPMessage emits no MIME/charset headers; Subject not RFC 2047 encoded"
  missing:
    - "Add MIME-Version: 1.0, Content-Type: text/plain; charset=UTF-8, Content-Transfer-Encoding: 8bit headers"
    - "Encode Subject with mime.QEncoding/BEncoding (stdlib mime)"
  debug_session: ".planning/debug/otp-email-thai-mojibake.md"
- gap_id: G-02-7
  truth: "พิมพ์ lat/lng ใน pier Sheet แล้ว marker ย้ายตาม โดยหน้าไม่ crash (map worker โหลดได้)"
  status: resolved
  resolved_by: 02-16-PLAN.md
  resolved_at: 2026-09-29
  reason: "User reported: กำลังพิม latitude แล้วหน้าพัง — Next.js dev overlay: Console Error 'Worker failed to load. Check that the worker URL is correct.' (2 issues, หน้าเบื้องหลังเป็น error page); และลองพิมพ์ lat/lng แล้ว marker ไม่ย้าย"
  severity: blocker
  test: 7
  root_cause: "map-picker handleLatChange/handleLngChange only guard Number.isFinite: partial input ('' from '13.' → 0) and out-of-range values (e.g. 137 while typing) reach Marker.setLngLat, which throws for |lat|>90; thrown in a React effect with no error boundary (no error.tsx) → Next dev overlay replaces page. 'Worker failed to load' is maplibre generic mislabel. Also map.on(error) latches tilesFailed for any error with no reset (causes spurious โหลดแผนที่ไม่สำเร็จ in test 8)."
  artifacts:
    - path: "apps/admin/src/components/map-picker.tsx"
      issue: "unguarded lat/lng parse (L107-117) feeds setLngLat (L50-63, L87-105) without range check; map.on(error) L73 treats every error as fatal, never resets"
  missing:
    - "Ignore empty/partial input and reject |lat|>90 / |lng|>180 before onChange/setLngLat"
    - "Only latch tilesFailed for tile/source errors and reset on successful load"
  debug_session: ".planning/debug/pier-map-worker-crash.md"
- gap_id: G-02-8
  truth: "สร้าง pier พร้อมรูป (photoKey) และเวลาทำการใน admin Sheet แล้วบันทึกสำเร็จ; opensAt/closesAt ที่เลือกถูกส่งไปใน payload"
  status: resolved
  resolved_by: 02-15-PLAN.md, 02-16-PLAN.md
  resolved_at: 2026-09-29
  reason: "User reported: ลาก marker บนแผนที่ได้ แต่บันทึก pier ไม่ได้: POST /api/v1/admin/boatbooking.catalog.v1.CatalogService/UpsertPier (super_admin, pierId ว่าง, มี photoKey piers/<uuid>.jpg หลังอัปโหลด thumbnail ขึ้นแล้ว) ตอบ {code:failed_precondition, message:failed precondition} → UI แสดง บันทึกท่าเรือไม่สำเร็จ กรุณาลองใหม่. สังเกตเพิ่ม: UI เวลาเปิด/ปิด แสดง 10:11 PM / 11:11 PM แต่ payload ส่ง opensAt/closesAt เป็น string ว่าง; ขึ้น โหลดแผนที่ไม่สำเร็จ ทั้งที่ tile แสดงบางส่วน"
  severity: blocker
  test: 8
  root_cause: "(1) Selected operator 01a0e84b… was archived earlier in UAT test 4; createPier correctly rejects (D-08) but pier-sheet operator select lists archived operators unmarked (ListOperatorsScoped has no archived filter), and catalog returns bare ErrFailedPrecondition with no detail. (2) Empty opensAt/closesAt: native <input type=time> in 12h browser locale keeps value '' until AM/PM segment is committed; hoursValid accepts both-empty so save is not blocked — not a state-binding bug."
  artifacts:
    - path: "apps/admin/src/app/(admin)/piers/pier-sheet.tsx"
      issue: "operator select shows archived operators; hoursValid silently accepts incomplete native time entry"
    - path: "services/catalog/internal/app/pier.go"
      issue: "archived-operator rejection returns undetailed failed precondition"
    - path: "services/catalog/internal/adapters/postgres/queries/operators.sql"
      issue: "ListOperatorsScoped has no archived filter"
  missing:
    - "Exclude/disable archived operators in pier-sheet operator select (reuse operators page isMuted/archived pattern)"
    - "Wrap archived-operator FailedPrecondition with a specific message"
    - "Make time entry unambiguous (e.g. 24h HH:MM input or detect incomplete entry) so hours are not silently dropped"
  debug_session: ".planning/debug/upsert-pier-failed-precondition.md"
- gap_id: G-02-9
  truth: "สร้าง route คู่ (pier_from, pier_to) ที่ active อยู่แล้วซ้ำไม่ได้ — ขึ้น มีเส้นทางนี้อยู่แล้ว; รายการแยกท่าที่ชื่อเหมือนกันได้"
  status: resolved
  resolved_by: 02-17-PLAN.md
  resolved_at: 2026-09-29
  reason: "User reported: ไม่ผ่าน มันสร้างซ้ำได้ — รายการ routes มี 2 แถว Proof Pier → Proof Pier (60 นาที, ใช้งาน) ไม่ขึ้น มีเส้นทางนี้อยู่แล้ว"
  severity: major
  test: 9
  diagnosis_hint: "ตรวจว่า 2 แถวเป็น pier_id คู่เดียวกันจริง หรือเป็นท่าชื่อ Proof Pier หลายท่าจาก make proof; และ pier_from == pier_to ถูกปฏิเสธหรือไม่"
  root_cause: "Backend correct (routes_active_pair_uq partial unique + CHECK pier_from<>pier_to). The two rows are a route and its legitimate reverse between two distinct piers both named 'Proof Pier' (deploy/proof.sh hardcodes pier name, no run suffix). Admin pierName() and route-sheet selects show only name_th, so same-named piers are indistinguishable."
  artifacts:
    - path: "apps/admin/src/app/(admin)/routes/queries.ts"
      issue: "pierName() has no disambiguation on name collision"
    - path: "apps/admin/src/app/(admin)/routes/route-sheet.tsx"
      issue: "pier selects render only nameTh"
    - path: "deploy/proof.sh"
      issue: "pier name 'Proof Pier' has no unique suffix (operator name does)"
  missing:
    - "Disambiguate pier labels when names collide (append operator name or short id) in list and selects"
    - "Add timestamp suffix to proof.sh pier name"
  debug_session: ".planning/debug/duplicate-route-created.md"
- gap_id: G-02-12
  truth: "policy ที่ถูกต้องซึ่งโหลดมา/คัดลอกมา (แก้ไข route, สร้างเส้นทางย้อนกลับ) ผ่าน validation และบันทึกได้ทันที; tier สุดท้าย (0 ชม.) ไม่มีปุ่มลบ"
  status: resolved
  resolved_by: 02-17-PLAN.md
  resolved_at: 2026-09-29
  reason: "User reported: ข้อก่อนหน้ากดบันทึกไม่ได้ ขึ้นแบบนี้ตลอด และต้องกดลบเพิ่มระดับก่อน แล้วกดเพิ่มใหม่ถึงจะบันทึกได้ — Sheet แก้ไขเส้นทาง: policy 24/100, 2/50, 0/0 (ถูกต้อง) แต่ขึ้น นโยบายยกเลิกไม่ถูกต้อง: ต้องเรียงชั่วโมงจากมากไปน้อยและมีระดับ 0 ชั่วโมงเสมอ และปุ่มบันทึก disabled; tier สุดท้ายมีปุ่มลบด้วย"
  severity: major
  test: 12
  root_cause: "protojson omits int32 zero fields, so the 0h tier arrives from the API (edit/return-route) with minHoursBefore/refundPercent undefined; validatePolicy defaults absent fields to -1 while inputs render ?? 0 → false invalid until tier re-added via addTier(). Separately, delete button renders for every tier when >1 (no last-tier guard) — test 10 pass was incorrect on that point."
  artifacts:
    - path: "apps/admin/src/app/(admin)/routes/policy-editor.tsx"
      issue: "validatePolicy uses ?? -1 (L25-26) vs render ?? 0 (L72/90); delete button has no last-tier guard (L99-111)"
  missing:
    - "Default absent tier fields to 0 in validatePolicy (or normalize policy on load)"
    - "Hide delete button on the last (0h) tier"
  debug_session: ".planning/debug/route-policy-false-invalid.md"
- gap_id: G-02-18
  truth: "มี integration test (testcontainers) พิสูจน์ว่า concurrent UpsertBoat บนเรือลำเดียวกันได้ 1 แถว boats และ 1 outbox row ต่อ write ที่สำเร็จ"
  status: resolved
  resolved_by: 02-15-PLAN.md
  resolved_at: 2026-09-29
  reason: "User reported: เขียน test — ผู้ใช้ต้องการ integration test จริงสำหรับ concurrent UpsertBoat แทนการอ้างอิง pattern ของ routes"
  severity: minor
  test: 18
  root_cause: "Not a defect — concurrency test for UpsertBoat was never written; user requested one instead of relying on the routes pattern."
  artifacts:
    - path: "services/catalog"
      issue: "missing concurrent UpsertBoat integration test"
  missing:
    - "Add testcontainers integration test: N concurrent UpsertBoat on same boat_id → 1 boats row, 1 outbox row per successful write"
  debug_session: ""
