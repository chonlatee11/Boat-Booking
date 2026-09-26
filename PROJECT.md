# Boat-Booking — Project Context & Instructions (v2: Go microservices + Kafka)

> ใช้ไฟล์นี้ 2 ที่: (1) วางใน Project Instructions ของ Project "Boat-Booking" บน claude.ai และ (2) commit เป็น `PROJECT.md` ใน repo `chonlatee11/Boat-Booking` เพื่อให้ `/gsd:new-project` ใช้เป็น seed
> Repo: https://github.com/chonlatee11/Boat-Booking (ตอนนี้ยังว่าง — เริ่มจากศูนย์)

---

## 1. Vision

ระบบจองรอบเรือข้ามฝั่งไปเกาะ (web + mobile-friendly app) ให้นักท่องเที่ยวจองล่วงหน้า จ่ายเงิน และได้ตั๋ว QR โดยไม่ต้องต่อคิวหน้าท่า ฝั่งผู้ให้บริการมีหน้าแอดมินจัดการท่าเรือ เรือ รอบ เวลา และความจุได้เอง

**เป้าหมายหลัก**
- ลดเวลาต่อคิวของนักท่องเที่ยว: จองจากมือถือ → จ่าย → โชว์ QR ที่ท่า
- ผู้ประกอบการปรับรอบ/เรือ/ความจุได้แบบ real-time โดยไม่ต้องพึ่ง dev
- รองรับหลายท่าเรือและหลายผู้ประกอบการตั้งแต่ต้น (multi-pier, multi-tenant-ready)
- UI เป็นมิตร ใช้ง่าย สไตล์ Zillow: search-first, card-based, สะอาด, mobile-first
- Backend เป็น **Go microservices, สื่อสารด้วย event ผ่าน Kafka, database-per-service (PostgreSQL)**

**ไม่อยู่ใน scope (v1)**
- Native app ใน App Store/Play Store → v1 เป็น PWA (Next.js) ก่อน
- ระบบเลือกที่นั่งแบบระบุตำแหน่ง (seat map) → v1 จองเป็น "จำนวนที่นั่ง"
- ระบบบัญชี/ใบกำกับภาษีเต็มรูปแบบ → v1 ออกใบเสร็จอย่างง่าย
- Kubernetes → v1 รันทุก service ด้วย Docker Compose บน EC2 เครื่องเดียว (ย้ายไป ECS/k8s ทีหลังได้เพราะ service แยกกันอยู่แล้ว)

---

## 2. Users & Roles

| Role | ใคร | ทำอะไร |
|---|---|---|
| `customer` | นักท่องเที่ยว (ไทย/ต่างชาติ) | ค้นหารอบ, ดูที่ว่าง, จอง, จ่าย, ดูตั๋ว, ยกเลิก (ตามนโยบาย) |
| `staff` | พนักงานหน้าท่า | สแกน QR check-in, ดูรายชื่อผู้โดยสารต่อรอบ, ปิดรอบ |
| `pier_admin` | ผู้จัดการท่าเรือ | จัดการเรือ/รอบ/ราคา/ความจุ ของท่าตัวเอง, ดูรายงาน |
| `super_admin` | เจ้าของระบบ (Chonlatee) | จัดการท่าเรือทั้งหมด, สร้าง pier_admin, ตั้งค่าระบบ |

Guest checkout: ลูกค้าจองได้โดยไม่สมัครสมาชิก (email/เบอร์โทร + OTP) — สมัครสมาชิกเป็น optional

---

## 3. Core Domain Model

```
Operator (ผู้ประกอบการ)
 └── Pier (ท่าเรือ)  — ชื่อ, พิกัด, ที่อยู่, รูป, เวลาเปิด-ปิด
      └── Route (เส้นทาง)  — pier_from → pier_to, ระยะเวลาเดินทาง, ราคาต่อ ticket type
           ├── Boat (เรือ)  — ชื่อ, ความจุ default, สถานะ (active/maintenance)
           ├── ScheduleTemplate (แม่แบบรอบ)  — วันในสัปดาห์, เวลาออก, เรือ, ความจุ, ช่วงวันที่มีผล
           └── Departure (รอบจริงในวันที่กำหนด)  — date, time, boat, capacity, status
                └── Booking  — customer, passengers ต่อ ticket type, total, status, hold_expires_at
                     ├── Payment  — provider, ref, amount, status, paid_at
                     └── Ticket (1 ต่อ booking)  — QR token, status (valid/used/voided), checked_in_at
```

**กฎสำคัญของ domain**
- `Departure.capacity` แก้ได้รายรอบ แต่ห้ามต่ำกว่าจำนวนที่จองไปแล้ว
- `available = capacity - booked - active_holds`
- Booking state machine: `pending_payment → paid → checked_in | cancelled | expired`
- Hold ที่นั่ง 10 นาทีตอนกด "จอง" — หมดเวลาแล้วปล่อยที่นั่งอัตโนมัติ
- **ห้าม overbook เด็ดขาด** — inventory ต่อรอบมีเจ้าของคนเดียวคือ booking-service และการหัก/คืนที่นั่งเป็น transaction เดียวใน Postgres (`SELECT ... FOR UPDATE`) ไม่กระจายข้าม service
- Ticket type: `adult`, `child`, `foreigner` — v1 ทำ adult/child ก่อน
- รอบที่ถูกยกเลิก → booking ที่จ่ายแล้วต้อง refund/ย้ายรอบ + แจ้งลูกค้า (saga ผ่าน Kafka)
- PDPA: เก็บข้อมูลส่วนบุคคลให้น้อยที่สุด (ชื่อ, email หรือเบอร์โทร) และเก็บใน identity/booking service เท่านั้น — event ที่วิ่งบน Kafka ใส่แค่ id ไม่ใส่ PII

---

## 4. Tech Stack

| Layer | เลือกใช้ | หมายเหตุ |
|---|---|---|
| Frontend (web + PWA) | **Next.js 15 (App Router) + TypeScript + Tailwind + shadcn/ui** | คุยกับ API Gateway เท่านั้น |
| Backend | **Go 1.23+** — chi (HTTP), connect-go/gRPC (sync ระหว่าง service), sqlc + pgx, goose (migration), franz-go (Kafka client), go-redis, slog, OpenTelemetry | 1 service = 1 binary = 1 Docker image |
| Event bus | **Kafka** — local/dev ใช้ **Redpanda** (Kafka-compatible, single binary, เบากว่ามาก), prod เริ่มด้วย Redpanda/Kafka container บน EC2, โตแล้วค่อยย้าย MSK | |
| DB | **PostgreSQL 16 — database-per-service** (schema/DB แยกกันคนละ instance หรือคนละ database บน instance เดียวใน v1) | ห้าม service ข้ามไป query DB ของกันเอง |
| Cache / Lock | **Redis** — seat hold TTL (keyspace notification), rate limit, OTP | ใช้เฉพาะ booking + identity |
| API Gateway | **Traefik** (routing, TLS) + Go **BFF** บางๆ สำหรับ verify JWT / aggregate หน้า search | หรือ Kong ถ้าต้องการ plugin |
| Event schema | **Protobuf** ใน `proto/events/` → gen Go + TS ให้ทั้ง service และ frontend (สำหรับ WebSocket/SSE ที่ว่างแบบ real-time) | |
| Payment | **PromptPay QR** + บัตร ผ่าน Opn (Omise) หรือ 2C2P | webhook เข้าที่ payment-service |
| Map | Leaflet + OSM (ฟรี) หรือ Mapbox | |
| QR | Go `skip2/go-qrcode` (gen), `html5-qrcode` (scan ฝั่ง staff) | |
| Email/Notif | Resend/SES, LINE Messaging API (phase หลัง) | |
| Infra | Docker Compose → Jenkins (build ทุก image, push ECR) → EC2 (`docker compose pull && up -d`) | |
| Observability | OpenTelemetry → Grafana Tempo/Loki/Prometheus (compose stack) หรือ Grafana Cloud free tier | จำเป็นเมื่อเป็น microservice — trace ข้าม service ต้องดูได้ตั้งแต่ phase 0 |

---

## 5. Service Architecture

### 5.1 รายชื่อ service (v1 = 7 services)

| Service | Owns (DB) | Sync API (gRPC/HTTP) | Publishes | Consumes |
|---|---|---|---|---|
| **identity** | users, roles, operator membership, OTP sessions | login/OTP, verify token, list users | `identity.UserCreated` | — |
| **catalog** | operators, piers, routes, boats, ticket prices | CRUD, `GetRoute`, `ListPiers` | `catalog.PierUpserted`, `catalog.RouteUpserted`, `catalog.BoatUpserted`, `catalog.PriceChanged` | — |
| **schedule** | schedule_templates, departures (time, boat, capacity, status) | CRUD template, generate departures, edit/cancel departure | `schedule.DepartureCreated`, `schedule.DepartureUpdated`, `schedule.DepartureCapacityChanged`, `schedule.DepartureCancelled` | `catalog.BoatUpserted` (cache capacity default) |
| **booking** | departure_inventory (projection: capacity/booked/held), bookings, passengers | `CreateBooking` (hold), `ConfirmBooking`, `CancelBooking`, `GetAvailability`, `CheckCapacityReducible` | `booking.BookingCreated`, `booking.BookingConfirmed`, `booking.BookingExpired`, `booking.BookingCancelled`, `booking.AvailabilityChanged` | `schedule.Departure*`, `payment.PaymentSucceeded`, `payment.PaymentFailed`, `payment.RefundCompleted` |
| **payment** | payment_intents, provider refs, refunds, webhook log | `CreateIntent`, provider webhook endpoint, `Refund` | `payment.PaymentSucceeded`, `payment.PaymentFailed`, `payment.RefundCompleted` | `booking.BookingCreated`, `booking.BookingExpired`, `booking.BookingCancelled` |
| **ticket** | tickets (token hash, status, check-in log) | `ValidateTicket` (scan), `GetTicket`, passenger list ต่อรอบ | `ticket.TicketIssued`, `ticket.TicketCheckedIn`, `ticket.TicketVoided` | `booking.BookingConfirmed`, `booking.BookingCancelled`, `schedule.DepartureCancelled` |
| **notification** | notification_log (idempotency), templates | resend endpoint | `notification.Sent` | `ticket.TicketIssued`, `booking.BookingCancelled`, `schedule.DepartureCancelled`, `payment.RefundCompleted` |

**Read model สำหรับหน้า search**: BFF/gateway query `catalog` (route, pier) + `booking.GetAvailability` (batch by departure ids) — ไม่ต้องมี search-service แยกใน v1; ถ้าช้าค่อยทำ `search` projection service ที่ consume ทุก event มาสร้าง denormalized table

### 5.2 หลักการ sync vs async
- **Async (Kafka)** = state change ที่ service อื่นต้องรู้ "หลังจากเกิดแล้ว" (ตั๋วออกแล้ว, จ่ายแล้ว, รอบยกเลิก)
- **Sync (gRPC)** = คำถามที่ต้องการคำตอบทันทีเพื่อตัดสินใจ (ที่ว่างเท่าไร, ลด capacity ได้ไหม, token ถูกไหม)
- กฎ: **ห้ามใช้ Kafka เป็น request/response** และห้าม sync call เป็น chain ยาวเกิน 2 hop

### 5.3 การไหลของ capacity (จุดที่พังง่ายที่สุด)
1. `schedule` เป็นเจ้าของ "ค่าที่แอดมินตั้ง" (capacity) — `booking` เป็นเจ้าของ "inventory จริง" (booked/held)
2. `schedule.DepartureCreated` → `booking` สร้าง `departure_inventory(capacity, booked=0, held=0)`
3. แอดมินลด capacity → `schedule` เรียก sync `booking.CheckCapacityReducible(departure_id, new_cap)` → ถ้า `new_cap < booked+held` ปฏิเสธทันที → ถ้าผ่าน commit แล้ว publish `DepartureCapacityChanged` → `booking` apply
4. การจอง: `booking.CreateBooking` → transaction: `SELECT ... FOR UPDATE` บน inventory row → check available → `held += n` → insert booking(pending) → outbox `BookingCreated` → set Redis key TTL 10 นาที
5. Hold หมดอายุ (Redis keyspace expiry + cron sweep กันพลาด) → transaction: `held -= n`, booking=expired → outbox `BookingExpired`
6. `PaymentSucceeded` → transaction: `held -= n; booked += n`, booking=paid → outbox `BookingConfirmed`

### 5.4 Sagas หลัก
**Booking saga (happy path)**
`BookingCreated` → payment สร้าง intent (QR) → ลูกค้าจ่าย → provider webhook → `PaymentSucceeded` → booking confirm → `BookingConfirmed` → ticket ออกตั๋ว → `TicketIssued` → notification ส่ง email

**Departure cancelled saga**
`DepartureCancelled` → booking ยกเลิกทุก booking ของรอบ → `BookingCancelled(reason=departure_cancelled)` ต่อใบ → payment refund → `RefundCompleted` → ticket void → notification แจ้งลูกค้า

**Hold expired saga**
`BookingExpired` → payment cancel intent (ถ้ายังไม่จ่าย) → ถ้าจ่ายมาหลังหมดเวลา (race) → payment publish `PaymentSucceeded` แต่ booking พบว่า expired → พยายาม re-hold ถ้าที่ยังว่าง, ไม่ว่าง → auto refund

### 5.5 Kafka conventions
- Topic ต่อ aggregate: `catalog.events`, `schedule.events`, `booking.events`, `payment.events`, `ticket.events`, `notification.events` + `*.dlq`
- **Partition key** = aggregate id (`departure_id` สำหรับ schedule/booking, `booking_id` สำหรับ payment/ticket) เพื่อรักษาลำดับต่อ entity
- Envelope: `event_id (uuid v7)`, `event_type`, `aggregate_id`, `occurred_at`, `trace_id`, `version`, `payload (proto)`
- **Transactional outbox** ทุก service: เขียน event ลงตาราง `outbox` ใน transaction เดียวกับ state → Go outbox relay (poll + publish + mark) — ไม่ต้องใช้ Debezium ใน v1
- **Consumer idempotency**: ตาราง `processed_events(event_id PK)` เช็คก่อน apply ในทุก consumer
- Retry: in-process backoff 3 ครั้ง → DLQ พร้อม error → มีหน้า admin ดู DLQ + replay
- Consumer group ต่อ service; ห้าม auto-commit — commit หลัง apply สำเร็จ

### 5.6 Shared Go packages (`pkg/`)
- `pkg/events` — proto-generated types + envelope helper
- `pkg/kafka` — producer/consumer wrapper (franz-go), otel instrumentation, DLQ
- `pkg/outbox` — outbox table schema + relay worker
- `pkg/httpx` — chi middleware (request id, auth claims, error format)
- `pkg/auth` — JWT verify, role/operator scoping
- `pkg/pgx` — connection pool, tx helper, health

---

## 6. Features แยกตามหน้าจอ

### 6.1 Customer (public)
1. **Home / Search** — hero + search bar (ท่าต้นทาง, ปลายทาง, วันที่, จำนวนคน), map แสดงท่าเรือ, card route ยอดนิยม
2. **Search Results** — card รายรอบ: เวลาออก, เรือ, ที่ว่าง (badge สี), ราคา; list ซ้าย + map ขวา (desktop), toggle บนมือถือ; ที่ว่างอัปเดต real-time ผ่าน SSE จาก `booking.AvailabilityChanged`
3. **Departure Detail** — รายละเอียด, นโยบายยกเลิก, เลือกจำนวน adult/child, ปุ่ม "จองเลย" → hold
4. **Checkout** — countdown 10 นาที, ชื่อ + email/เบอร์, สรุปราคา, PromptPay QR / บัตร, polling/SSE สถานะจ่าย
5. **Ticket** — QR ใหญ่ อ่านง่ายกลางแดด, ข้อมูลรอบ, save image, ส่ง email ซ้ำ
6. **My Bookings** — รายการจอง (login หรือ booking ref + email), ยกเลิกตามนโยบาย
7. **i18n** — ไทย/อังกฤษ ตั้งแต่ v1

### 6.2 Staff (check-in)
1. **Scanner** — กล้องสแกน QR → `ticket.ValidateTicket` → ผล valid/used/wrong departure ทันที (ใหญ่ ชัด เขียว/แดง)
2. **Passenger List** — รายชื่อต่อรอบ, check-in manual, นับคนขึ้นเรือ
3. Offline บางส่วน (cache รายชื่อวันนี้) — phase หลัง

### 6.3 Admin
1. **Dashboard** — ยอดจองวันนี้, รายได้, รอบใกล้เต็ม, รอบถัดไป
2. **Piers / Routes / Boats** — CRUD (catalog)
3. **Schedule Templates** — รอบประจำ + generate departures ล่วงหน้า N วัน (schedule)
4. **Departures** — calendar/list, แก้รายรอบ: เวลา, เรือ, ความจุ, ปิดรับจอง, ยกเลิกรอบ (trigger saga), เพิ่มรอบพิเศษ
5. **Bookings** — ค้นหา, refund, ย้ายรอบ
6. **Users & Roles** — staff/pier_admin ผูกกับท่า (identity)
7. **Reports** — ยอดขาย, load factor, export CSV
8. **System** — DLQ viewer + replay, service health

---

## 7. UI/UX Direction (Zillow-like)

- **Search-first**: หน้าแรกคือ search bar ใหญ่ตรงกลาง ไม่ใช่ landing page ยาว
- **Split layout** ผลค้นหา: card list ซ้าย + map ขวา (desktop), toggle บนมือถือ
- **Card**: รูปเรือ/ท่า, เวลาออกตัวใหญ่, ราคา bold, badge ที่ว่าง, hover ยกขึ้นเล็กน้อย
- **Palette**: พื้นขาว, น้ำเงินทะเล (~#0B6E99) primary, เขียว "ว่าง", ส้ม/แดง "ใกล้เต็ม/เต็ม", มุมมน 12–16px, เงาอ่อน
- **Typography**: font ไทยดี (IBM Plex Sans Thai / Noto Sans Thai / Prompt) ขนาดใหญ่กว่าปกติ
- **Mobile-first**: CTA sticky ล่างจอ, touch target ≥ 44px
- **Friendly copy**: "ว่างอีก 12 ที่นั่ง" ไม่ใช่ "Capacity remaining: 12"
- **Trust cues**: นโยบายยกเลิก, เบอร์ท่า, สถานะรอบชัดเจน
- Admin ใช้ shadcn/ui + data table มาตรฐาน เน้นเร็ว แก้ inline ได้

---

## 8. Roadmap (แบ่ง phase สำหรับ GSD)

แต่ละ phase จบด้วยของที่ใช้ได้จริง (vertical slice) ไม่ทำแนวนอนทีละ layer

| Phase | ชื่อ | ส่งมอบ |
|---|---|---|
| 0 | **Platform foundation** | Go workspace monorepo, service template (`make new-service`), `pkg/*` (kafka, outbox, httpx, auth, pgx), proto toolchain (buf), Docker Compose: Redpanda + Postgres + Redis + Traefik + Grafana stack, Jenkins pipeline build/push ทุก image, health/ready endpoints, trace ข้าม service เห็นใน Tempo, Next.js skeleton |
| 1 | **Identity + Catalog** | identity-service (OTP, JWT, roles, operator scope), catalog-service CRUD ครบ, admin UI piers/routes/boats + map picker, gateway routing + auth middleware |
| 2 | **Schedule** | schedule-service: templates → generate departures, แก้/ปิด/ยกเลิกรอบ, publish `schedule.*` events, admin calendar view |
| 3 | **Booking core** | booking-service: inventory projection จาก schedule events, `CreateBooking` + hold + expiry, `CheckCapacityReducible`, `GetAvailability`; public search + results + detail + checkout form (ยังไม่จ่ายจริง — mock confirm); **capacity race test** |
| 4 | **Payment + Ticket** | payment-service (PromptPay/บัตร, webhook idempotent), ticket-service (issue QR, validate), notification-service (email ตั๋ว), booking saga ครบ end-to-end, หน้า Ticket + My Bookings |
| 5 | **Check-in** | staff scanner + passenger list, `ticket.TicketCheckedIn`, admin เห็นจำนวนขึ้นเรือ real-time |
| 6 | **Refund & departure changes** | departure-cancelled saga, ลูกค้ายกเลิกตามนโยบาย, refund, ย้ายรอบ, DLQ viewer + replay |
| 7 | **Polish & launch** | i18n ครบ, PWA, SSE ที่ว่าง real-time, SEO หน้า pier/route, dashboard/reports, load test (k6) flow จอง→จ่าย, runbook deploy, backup Postgres ทุก DB |
| 8 (later) | **Growth** | LINE login/notify, foreigner ticket type, promo code, search projection service, multi-operator billing, ย้าย ECS/k8s, MSK |

---

## 9. Technical Guardrails (ให้ agent ยึด)

- **Database-per-service เด็ดขาด**: service ต่อ DB ของตัวเองเท่านั้น; ต้องการข้อมูลของคนอื่น → consume event มาทำ projection หรือ sync gRPC
- **Capacity race**: integration test ยิง concurrent 50 `CreateBooking` ใส่รอบว่าง 10 ที่ → ต้องได้ 10 booking พอดี, รันใน CI ทุกครั้ง
- **Outbox ทุก publish**: ห้าม publish Kafka ตรงจาก handler; ต้องผ่าน outbox ใน tx เดียวกับ state
- **Idempotent ทุก consumer** (`processed_events`) และ **idempotent webhook** (provider ref unique)
- **Event = fact ที่เกิดแล้ว** ตั้งชื่อ past tense (`BookingConfirmed`) ไม่ใช่คำสั่ง (`ConfirmBooking`)
- **ห้ามใส่ PII ใน event** — ส่ง id แล้วให้ปลายทาง sync call ไปขอเมื่อจำเป็น (notification ขอ email จาก booking/identity ตอนส่ง)
- **Schema evolution**: proto เพิ่ม field ได้ ห้าม reuse/ลบ field number; event มี `version`
- **Time zone**: UTC ใน DB, แสดง `Asia/Bangkok`; รอบเรือผูกกับ "วันที่ท้องถิ่น"
- **Money**: integer สตางค์
- **QR token**: random 32 bytes, เก็บ hash, ไม่มี PII
- **Auth**: JWT ใน httpOnly cookie ผ่าน gateway → forward claims ให้ service เป็น header ที่ trust เฉพาะจาก gateway; ทุก query scope ด้วย `operator_id`
- **Observability**: ทุก request/event มี `trace_id` ต่อกันข้าม Kafka (otel propagation ใน headers), structured log (slog JSON)
- **Testing**: unit (`go test`), integration ต่อ service กับ Postgres+Redpanda จริงใน testcontainers, contract test สำหรับ event (consumer เช็ค proto compatibility), e2e Playwright เฉพาะ flow จอง→จ่าย→ตั๋ว
- **Secrets**: `.env` ไม่ commit, `.env.example` ต่อ service
- **Commits**: conventional commits, scope = service name (`feat(booking): ...`)

---

## 10. Open Decisions (ให้ GSD ถามหรือตัดสินใจตอน planning)

1. Payment provider: Opn (Omise) vs 2C2P
2. Postgres v1: instance เดียวหลาย database (ถูก, ง่าย) vs container ต่อ service (แยกจริง) — แนะนำอย่างแรกใน v1 แต่ config ให้สลับได้ด้วย DSN
3. Internal sync API: connect-go (gRPC+HTTP/JSON ได้ทั้งคู่, debug ง่าย) vs gRPC ล้วน
4. Gateway: Traefik + Go BFF vs Kong
5. นโยบายยกเลิก/คืนเงิน default — ตั้งได้ต่อ route
6. Search read model: query สด (BFF aggregate) ใน v1 หรือทำ projection ตั้งแต่ phase 3
7. Event serialization: protobuf (แนะนำ) vs JSON + JSON schema

---

## 11. วิธีเริ่มงานด้วย GSD บนเครื่อง

```bash
git clone https://github.com/chonlatee11/Boat-Booking
cd Boat-Booking
cp /path/to/BOAT_BOOKING_PROJECT_CONTEXT.md PROJECT.md
claude
```

ใน Claude Code:
1. `/gsd:new-project` — ให้ agent อ่าน PROJECT.md ก่อนตอบคำถาม → ได้ REQUIREMENTS.md + ROADMAP.md
2. ตรวจ ROADMAP ให้ตรง Section 8; ถ้า agent แตก phase แบบทีละ service (แนวนอน) ให้ดึงกลับเป็น vertical slice
3. `/gsd:discuss-phase 0` → `/gsd:plan-phase 0` → `/gsd:execute-phase 0` — phase 0 สำคัญมาก เพราะ service template + `pkg/*` จะถูก copy ไปทุก service; review ให้ดีก่อนไปต่อ
4. Model routing เดิม: Haiku research/verify, Sonnet เขียนโค้ด; งานที่กินโค้ดหลาย service พร้อมกัน (saga) ให้ Opus plan แล้ว Sonnet execute ทีละ service
5. ใส่ `CLAUDE.md` ย่อยใน `services/<name>/` ระบุ: owns อะไร, publish/consume อะไร, ห้ามแตะ DB อื่น — ลด context ที่ subagent ต้องโหลด
6. หลังจบทุก phase: `make test-integration` (รวม capacity race) ก่อน commit

---

## 12. Repo Structure เป้าหมาย

```
Boat-Booking/
├── PROJECT.md                 ← ไฟล์นี้
├── .planning/                 ← GSD สร้างให้
├── go.work
├── proto/
│   ├── events/                ← *.proto ของทุก event
│   └── services/              ← gRPC/connect definitions
├── gen/                       ← generated Go + TS (commit หรือ gen ใน CI)
├── pkg/                       ← shared Go packages (events, kafka, outbox, httpx, auth, pgx)
├── services/
│   ├── identity/
│   ├── catalog/
│   ├── schedule/
│   ├── booking/
│   ├── payment/
│   ├── ticket/
│   ├── notification/
│   └── gateway/               ← Go BFF
│       (แต่ละ service: cmd/, internal/{domain,app,adapters}, migrations/, CLAUDE.md, Dockerfile)
├── apps/
│   └── web/                   ← Next.js
├── deploy/
│   ├── docker-compose.yml     ← dev: redpanda, postgres, redis, traefik, grafana stack, ทุก service
│   ├── docker-compose.prod.yml
│   └── jenkins/Jenkinsfile
├── Makefile                   ← new-service, proto-gen, test-integration, up/down
└── .env.example
```
