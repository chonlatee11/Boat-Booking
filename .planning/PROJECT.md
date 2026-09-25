# Boat-Booking

## What This Is

ระบบจองรอบเรือข้ามฝั่งไปเกาะ (web + mobile-friendly PWA) ให้นักท่องเที่ยวค้นหารอบ จองล่วงหน้า จ่ายเงิน และได้ตั๋ว QR โดยไม่ต้องต่อคิวหน้าท่า ฝั่งผู้ให้บริการมีหน้าแอดมินจัดการท่าเรือ เรือ รอบ เวลา และความจุได้เอง รองรับหลายท่าเรือ/หลายผู้ประกอบการตั้งแต่ต้น (multi-pier, multi-tenant-ready)

Backend เป็น **Go microservices** สื่อสารด้วย event ผ่าน **Kafka** (Redpanda ใน dev/prod v1), **database-per-service (PostgreSQL)**; frontend เป็น Next.js PWA สไตล์ Zillow (search-first, card-based, mobile-first)

Seed document: `PROJECT.md` ที่ root ของ repo (v2: Go microservices + Kafka) — ไฟล์นี้สังเคราะห์จากที่นั่น + คำตอบตอน questioning

## Core Value

**ลูกค้าจองจากมือถือ → จ่าย → โชว์ QR ที่ท่าได้ โดยระบบไม่ overbook เด็ดขาด** — inventory ต่อรอบมีเจ้าของคนเดียว (booking-service) และการหัก/คืนที่นั่งเป็น Postgres transaction เดียว

## Business Context

- **Customer**: นักท่องเที่ยว (ไทย/ต่างชาติ) เป็นผู้ใช้; ผู้ประกอบการเรือ/ท่าเรือเป็นผู้ให้บริการ; เจ้าของระบบ = Chonlatee (super_admin)
- **Revenue model**: ยังไม่กำหนดใน v1 (multi-operator billing อยู่ใน Growth phase) — v1 เน้นใช้งานได้จริงกับผู้ประกอบการ
- **Success metric**: จำนวน booking ที่จ่ายสำเร็จและ check-in ด้วย QR โดยไม่มี overbook
- **Strategy notes**: `PROJECT.md` (root) section 1, 8

## Users & Roles

| Role | ใคร | ทำอะไร |
|---|---|---|
| `customer` | นักท่องเที่ยว (ไทย/ต่างชาติ) | ค้นหารอบ, ดูที่ว่าง, จอง, จ่าย, ดูตั๋ว, ยกเลิก (ตามนโยบาย) |
| `staff` | พนักงานหน้าท่า | สแกน QR check-in, ดูรายชื่อผู้โดยสารต่อรอบ, ปิดรอบ |
| `pier_admin` | ผู้จัดการท่าเรือ | จัดการเรือ/รอบ/ราคา/ความจุของท่าตัวเอง, ดูรายงาน |
| `super_admin` | เจ้าของระบบ | จัดการท่าเรือทั้งหมด, สร้าง pier_admin, ตั้งค่าระบบ |

Guest checkout: ลูกค้าจองได้โดยไม่สมัครสมาชิก (email/เบอร์โทร + OTP) — สมัครสมาชิกเป็น optional

## Requirements

### Validated

(None yet — ship to validate)

### Active

**Milestone 1 (v1) = Phase 0–4 ของ seed roadmap: จอง → จ่าย → ตั๋ว ครบ end-to-end**

Platform foundation
- [ ] Go workspace monorepo + service template (`make new-service`) + shared `pkg/*` (events, kafka, outbox, httpx, auth, pgx)
- [ ] Proto toolchain (buf) gen Go + TS จาก `proto/events/` และ `proto/services/`
- [ ] Docker Compose dev stack: Redpanda + Postgres + Redis + Kong + Grafana (Tempo/Loki/Prometheus)
- [ ] Jenkins pipeline build/push ทุก image → ECR → EC2 `docker compose pull && up -d`
- [ ] health/ready endpoints ทุก service; trace ข้าม service (HTTP + Kafka) เห็นใน Tempo
- [ ] Next.js 15 skeleton (App Router, TS, Tailwind, shadcn/ui, i18n ไทย/อังกฤษ)

Identity + Catalog
- [ ] identity-service: OTP login (email/เบอร์), JWT ใน httpOnly cookie, roles, operator scoping
- [ ] catalog-service: CRUD operators, piers, routes, boats, ticket prices (adult/child)
- [ ] Admin UI: piers / routes / boats + map picker
- [ ] Kong routing + JWT verify; Go BFF บางๆ forward claims เป็น trusted header

Schedule
- [ ] schedule-service: schedule templates → generate departures ล่วงหน้า N วัน
- [ ] แก้/ปิดรับจอง/ยกเลิกรอบ/เพิ่มรอบพิเศษ; publish `schedule.*` events
- [ ] Admin calendar/list view ของ departures

Booking core
- [ ] booking-service: `departure_inventory` projection จาก schedule events
- [ ] `CreateBooking` (hold 10 นาที) + expiry (Redis TTL + cron sweep), `GetAvailability`, `CheckCapacityReducible`
- [ ] Public: Home/Search → Results → Departure Detail → Checkout form (mock confirm)
- [ ] Capacity race integration test (50 concurrent → 10 ที่ → ได้ 10 booking พอดี) รันใน CI

Payment + Ticket + Notification
- [ ] payment-service: provider interface + mock provider; PromptPay QR / บัตร ผ่าน provider จริง (Opn หรือ 2C2P — ตัดสินใจตอน plan Phase 4); webhook idempotent
- [ ] ticket-service: issue QR (token hash), `ValidateTicket`, passenger list ต่อรอบ
- [ ] notification-service: email ตั๋ว (Resend/SES), idempotent ด้วย notification_log
- [ ] Booking saga ครบ: BookingCreated → PaymentSucceeded → BookingConfirmed → TicketIssued → email
- [ ] Hold-expired saga (รวม race จ่ายหลังหมดเวลา → re-hold หรือ auto refund)
- [ ] หน้า Ticket (QR ใหญ่, save image, ส่ง email ซ้ำ) + My Bookings (login หรือ booking ref + email)

### Deferred to later milestones (จาก seed roadmap Phase 5–8)

- Check-in: staff scanner + passenger list, `TicketCheckedIn`, admin เห็นจำนวนขึ้นเรือ real-time
- Refund & departure changes: departure-cancelled saga, ลูกค้ายกเลิกตามนโยบาย, refund, ย้ายรอบ, DLQ viewer + replay
- Polish & launch: PWA, SSE ที่ว่าง real-time, SEO, dashboard/reports/export CSV, load test k6, runbook deploy, backup Postgres
- Growth: LINE login/notify, foreigner ticket type, promo code, search projection service, multi-operator billing, ECS/k8s, MSK, staff offline mode

### Out of Scope

- Native app ใน App Store/Play Store — v1 เป็น PWA (Next.js) ก่อน
- Seat map เลือกที่นั่งระบุตำแหน่ง — v1 จองเป็น "จำนวนที่นั่ง"
- ระบบบัญชี/ใบกำกับภาษีเต็มรูปแบบ — v1 ออกใบเสร็จอย่างง่าย
- Kubernetes — v1 รันทุก service ด้วย Docker Compose บน EC2 เครื่องเดียว (service แยกกันแล้ว ย้ายทีหลังได้)
- Debezium/CDC — ใช้ Go outbox relay (poll + publish) เพียงพอใน v1
- Search projection service — query สดผ่าน BFF ใน v1 (Key Decision #6)

## Context

### Core Domain Model

```
Operator (ผู้ประกอบการ)
 └── Pier (ท่าเรือ)  — ชื่อ, พิกัด, ที่อยู่, รูป, เวลาเปิด-ปิด
      └── Route (เส้นทาง)  — pier_from → pier_to, ระยะเวลาเดินทาง, ราคาต่อ ticket type, นโยบายยกเลิก
           ├── Boat (เรือ)  — ชื่อ, ความจุ default, สถานะ (active/maintenance)
           ├── ScheduleTemplate (แม่แบบรอบ)  — วันในสัปดาห์, เวลาออก, เรือ, ความจุ, ช่วงวันที่มีผล
           └── Departure (รอบจริงในวันที่กำหนด)  — date, time, boat, capacity, status
                └── Booking  — customer, passengers ต่อ ticket type, total, status, hold_expires_at
                     ├── Payment  — provider, ref, amount, status, paid_at
                     └── Ticket (1 ต่อ booking)  — QR token, status (valid/used/voided), checked_in_at
```

กฎสำคัญ:
- `Departure.capacity` แก้ได้รายรอบ แต่ห้ามต่ำกว่าจำนวนที่จองไปแล้ว
- `available = capacity - booked - active_holds`
- Booking state machine: `pending_payment → paid → checked_in | cancelled | expired`
- Hold ที่นั่ง 10 นาทีตอนกด "จอง" — หมดเวลาแล้วปล่อยที่นั่งอัตโนมัติ
- **ห้าม overbook เด็ดขาด** — `SELECT ... FOR UPDATE` บน inventory row ใน booking-service เท่านั้น
- Ticket type: `adult`, `child`, `foreigner` — v1 ทำ adult/child ก่อน
- รอบที่ถูกยกเลิก → booking ที่จ่ายแล้วต้อง refund/ย้ายรอบ + แจ้งลูกค้า (saga ผ่าน Kafka)
- PDPA: เก็บ PII ให้น้อยที่สุด (ชื่อ, email หรือเบอร์โทร) เฉพาะใน identity/booking service — event บน Kafka ใส่แค่ id

### Service Architecture (v1 = 7 services + BFF)

| Service | Owns (DB) | Sync API (connect-go) | Publishes | Consumes |
|---|---|---|---|---|
| **identity** | users, roles, operator membership, OTP sessions | login/OTP, verify token, list users | `identity.UserCreated` | — |
| **catalog** | operators, piers, routes, boats, ticket prices | CRUD, `GetRoute`, `ListPiers` | `catalog.PierUpserted`, `catalog.RouteUpserted`, `catalog.BoatUpserted`, `catalog.PriceChanged` | — |
| **schedule** | schedule_templates, departures | CRUD template, generate departures, edit/cancel departure | `schedule.DepartureCreated`, `DepartureUpdated`, `DepartureCapacityChanged`, `DepartureCancelled` | `catalog.BoatUpserted` |
| **booking** | departure_inventory (projection), bookings, passengers | `CreateBooking`, `ConfirmBooking`, `CancelBooking`, `GetAvailability`, `CheckCapacityReducible` | `booking.BookingCreated`, `BookingConfirmed`, `BookingExpired`, `BookingCancelled`, `AvailabilityChanged` | `schedule.Departure*`, `payment.PaymentSucceeded`, `PaymentFailed`, `RefundCompleted` |
| **payment** | payment_intents, provider refs, refunds, webhook log | `CreateIntent`, provider webhook, `Refund` | `payment.PaymentSucceeded`, `PaymentFailed`, `RefundCompleted` | `booking.BookingCreated`, `BookingExpired`, `BookingCancelled` |
| **ticket** | tickets (token hash, status, check-in log) | `ValidateTicket`, `GetTicket`, passenger list | `ticket.TicketIssued`, `TicketCheckedIn`, `TicketVoided` | `booking.BookingConfirmed`, `BookingCancelled`, `schedule.DepartureCancelled` |
| **notification** | notification_log (idempotency), templates | resend endpoint | `notification.Sent` | `ticket.TicketIssued`, `booking.BookingCancelled`, `schedule.DepartureCancelled`, `payment.RefundCompleted` |
| **gateway (BFF)** | — | aggregate search (catalog + `booking.GetAvailability` batch), SSE availability | — | `booking.AvailabilityChanged` (SSE fan-out) |

Sync vs async:
- **Async (Kafka)** = state change ที่ service อื่นต้องรู้หลังเกิดแล้ว
- **Sync (connect-go)** = คำถามที่ต้องการคำตอบทันที (ที่ว่าง, ลด capacity ได้ไหม, token ถูกไหม)
- ห้ามใช้ Kafka เป็น request/response; ห้าม sync chain ยาวเกิน 2 hop

Capacity flow (จุดที่พังง่ายที่สุด):
1. `schedule` เป็นเจ้าของ capacity ที่แอดมินตั้ง; `booking` เป็นเจ้าของ inventory จริง (booked/held)
2. `DepartureCreated` → booking สร้าง `departure_inventory(capacity, booked=0, held=0)`
3. ลด capacity → schedule เรียก sync `CheckCapacityReducible` → ผ่านแล้ว commit → publish `DepartureCapacityChanged` → booking apply
4. `CreateBooking` → tx: `SELECT ... FOR UPDATE` → check available → `held += n` → insert booking(pending) → outbox `BookingCreated` → Redis key TTL 10 นาที
5. Hold หมดอายุ (Redis keyspace expiry + cron sweep) → tx: `held -= n`, booking=expired → outbox `BookingExpired`
6. `PaymentSucceeded` → tx: `held -= n; booked += n`, booking=paid → outbox `BookingConfirmed`

Sagas: Booking (happy path), Departure cancelled, Hold expired — รายละเอียดใน root `PROJECT.md` §5.4

Kafka conventions:
- Topic ต่อ aggregate: `catalog.events`, `schedule.events`, `booking.events`, `payment.events`, `ticket.events`, `notification.events` + `*.dlq`
- Partition key = aggregate id (`departure_id` / `booking_id`) รักษาลำดับต่อ entity
- Envelope: `event_id (uuid v7)`, `event_type`, `aggregate_id`, `occurred_at`, `trace_id`, `version`, `payload (proto)`
- Transactional outbox ทุก service; consumer idempotency ด้วย `processed_events(event_id PK)`
- Retry in-process 3 ครั้ง → DLQ; consumer group ต่อ service; commit หลัง apply สำเร็จ

Shared `pkg/`: `events`, `kafka` (franz-go + otel + DLQ), `outbox`, `httpx` (chi middleware), `auth` (JWT, role/operator scoping), `pgx`

### Features by screen (v1 scope)

Customer: Home/Search (hero + search bar + map + popular routes), Search Results (card list + map split, badge ที่ว่าง), Departure Detail (เลือก adult/child → hold), Checkout (countdown 10 นาที, PromptPay QR/บัตร, polling สถานะจ่าย), Ticket (QR ใหญ่, save image, resend email), My Bookings, i18n ไทย/อังกฤษ

Admin (v1): Piers/Routes/Boats CRUD, Schedule Templates + generate departures, Departures calendar/list + edit/close/cancel/add special, Users & Roles (staff/pier_admin ผูกท่า)

Staff/Reports/Dashboard/DLQ viewer → milestone ถัดไป

### UI/UX Direction (Zillow-like)

- Search-first: หน้าแรกคือ search bar ใหญ่ ไม่ใช่ landing page ยาว
- Split layout: card list ซ้าย + map ขวา (desktop), toggle บนมือถือ
- Card: รูปเรือ/ท่า, เวลาออกตัวใหญ่, ราคา bold, badge ที่ว่าง, hover ยกขึ้น
- Palette: พื้นขาว, น้ำเงินทะเล ~#0B6E99 primary, เขียว "ว่าง", ส้ม/แดง "ใกล้เต็ม/เต็ม", มุมมน 12–16px
- Typography: IBM Plex Sans Thai / Noto Sans Thai / Prompt ขนาดใหญ่กว่าปกติ
- Mobile-first: CTA sticky ล่างจอ, touch target ≥ 44px
- Friendly copy: "ว่างอีก 12 ที่นั่ง" ไม่ใช่ "Capacity remaining: 12"
- Admin: shadcn/ui + data table, แก้ inline ได้

### Target repo structure

```
Boat-Booking/
├── PROJECT.md, .planning/, go.work
├── proto/{events,services}/     gen/
├── pkg/                          ← events, kafka, outbox, httpx, auth, pgx
├── services/{identity,catalog,schedule,booking,payment,ticket,notification,gateway}/
│     (cmd/, internal/{domain,app,adapters}, migrations/, CLAUDE.md, Dockerfile)
├── apps/web/                     ← Next.js
├── deploy/{docker-compose.yml, docker-compose.prod.yml, jenkins/Jenkinsfile}
├── Makefile                      ← new-service, proto-gen, test-integration, up/down
└── .env.example
```

### Working notes

- Phase 0 สำคัญมาก: service template + `pkg/*` ถูก copy ไปทุก service — review ให้ดีก่อนไปต่อ
- ใส่ `CLAUDE.md` ย่อยใน `services/<name>/` ระบุ owns / publish / consume / ห้ามแตะ DB อื่น
- หลังจบทุก phase: `make test-integration` (รวม capacity race) ก่อน commit
- Roadmap ต้องเป็น vertical slice — ถ้าแตกทีละ service (แนวนอน) ให้ดึงกลับ

## Constraints

- **Tech stack**: Go 1.23+ (chi, connect-go, sqlc + pgx, goose, franz-go, go-redis, slog, OpenTelemetry), PostgreSQL 16, Redis, Kafka/Redpanda, Kong, Next.js 15 + TS + Tailwind + shadcn/ui, Protobuf (buf) — กำหนดไว้แล้วใน seed
- **Architecture**: Database-per-service เด็ดขาด; outbox ทุก publish; idempotent ทุก consumer/webhook; event = past-tense fact; ห้าม PII ใน event; proto schema evolution (เพิ่ม field ได้ ห้าม reuse/ลบ number, มี `version`)
- **Infra**: Docker Compose → Jenkins → ECR → EC2 เครื่องเดียว; Kubernetes เป็น non-goal v1
- **Data**: UTC ใน DB แสดง `Asia/Bangkok`, รอบผูกกับวันที่ท้องถิ่น; เงินเป็น integer สตางค์; QR token random 32 bytes เก็บ hash
- **Security**: JWT httpOnly cookie ผ่าน Kong → claims เป็น header ที่ trust เฉพาะจาก gateway; ทุก query scope ด้วย `operator_id`; PDPA minimal PII; `.env` ไม่ commit
- **Observability**: `trace_id` ต่อกันข้าม HTTP + Kafka (otel propagation), structured log slog JSON — ตั้งแต่ Phase 0
- **Testing**: unit `go test`; integration ต่อ service กับ Postgres+Redpanda จริง (testcontainers); contract test event; e2e Playwright เฉพาะ flow จอง→จ่าย→ตั๋ว; capacity race test ใน CI ทุกครั้ง
- **Commits**: conventional commits, scope = service name (`feat(booking): ...`)
- **Team**: solo developer — ปรับ scope ต่อ phase ให้จบได้จริง

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Milestone 1 = Phase 0–4 (foundation → identity/catalog → schedule → booking core → payment/ticket) | ได้ flow หลักครบ end-to-end ใช้จริงได้; เก็บ 5–7 ไว้ milestone ถัดไปเพราะอาจมี requirement เพิ่ม | — Pending |
| Payment provider: ยังไม่เลือก — ทำ provider interface + mock ก่อน | Opn (Omise) vs 2C2P ตัดสินใจตอน plan Phase 4; interface ทำให้สลับได้ | — Pending |
| Internal sync API: connect-go | gRPC + HTTP/JSON ในตัวเดียว, curl debug ได้, gen TS ให้ frontend | — Pending |
| Gateway: Kong + Go BFF บางๆ | Kong ทำ routing/TLS/JWT plugin/rate limit; BFF ทำเฉพาะ aggregate search + SSE | — Pending |
| Postgres v1: instance เดียว หลาย database, สลับด้วย DSN | ถูกและง่าย; database-per-service ยังคงอยู่ระดับ logical | — Pending |
| Search read model: query สดผ่าน BFF (catalog + availability batch) | ไม่ต้องมี projection service ใน v1; ทำเมื่อช้า | — Pending |
| Event serialization: Protobuf | typed, gen Go + TS, schema evolution ชัด | — Pending |
| Cancel/refund policy default: ขั้นบันได (>24 ชม. คืน 100%, 2–24 ชม. คืน 50%, <2 ชม. ไม่คืน) ตั้งได้ต่อ route | ยุติธรรมทั้งสองฝั่ง; override ต่อ route ได้ | — Pending |
| Kafka dev/prod v1: Redpanda | Kafka-compatible, single binary, เบา; ย้าย MSK เมื่อโต | — Pending |
| Outbox relay ใน Go (poll + publish + mark), ไม่ใช้ Debezium | ลด moving parts ใน v1 | — Pending |
| Project structure: vertical slices ต่อ phase | แต่ละ phase จบด้วยของที่ใช้ได้จริง | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-09-25 after initialization*
