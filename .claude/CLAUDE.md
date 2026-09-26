<!-- GSD:project-start source:PROJECT.md -->

## Project

**Boat-Booking**

ระบบจองรอบเรือข้ามฝั่งไปเกาะ (web + mobile-friendly PWA) ให้นักท่องเที่ยวค้นหารอบ จองล่วงหน้า จ่ายเงิน และได้ตั๋ว QR โดยไม่ต้องต่อคิวหน้าท่า ฝั่งผู้ให้บริการมีหน้าแอดมินจัดการท่าเรือ เรือ รอบ เวลา และความจุได้เอง รองรับหลายท่าเรือ/หลายผู้ประกอบการตั้งแต่ต้น (multi-pier, multi-tenant-ready)

Backend เป็น **Go microservices** สื่อสารด้วย event ผ่าน **Kafka** (Redpanda ใน dev/prod v1), **database-per-service (PostgreSQL)**; frontend เป็น Next.js PWA สไตล์ Zillow (search-first, card-based, mobile-first)

Seed document: `PROJECT.md` ที่ root ของ repo (v2: Go microservices + Kafka) — ไฟล์นี้สังเคราะห์จากที่นั่น + คำตอบตอน questioning

**Core Value:** **ลูกค้าจองจากมือถือ → จ่าย → โชว์ QR ที่ท่าได้ โดยระบบไม่ overbook เด็ดขาด** — inventory ต่อรอบมีเจ้าของคนเดียว (booking-service) และการหัก/คืนที่นั่งเป็น Postgres transaction เดียว

### Constraints

- **Tech stack**: Go 1.23+ (chi, connect-go, sqlc + pgx, goose, franz-go, go-redis, slog, OpenTelemetry), PostgreSQL 16, Redis, Kafka/Redpanda, Kong, Next.js 15 + TS + Tailwind + shadcn/ui, Protobuf (buf) — กำหนดไว้แล้วใน seed
- **Architecture**: Database-per-service เด็ดขาด; outbox ทุก publish; idempotent ทุก consumer/webhook; event = past-tense fact; ห้าม PII ใน event; proto schema evolution (เพิ่ม field ได้ ห้าม reuse/ลบ number, มี `version`)
- **Infra**: Docker Compose → Jenkins → ECR → EC2 เครื่องเดียว; Kubernetes เป็น non-goal v1
- **Data**: UTC ใน DB แสดง `Asia/Bangkok`, รอบผูกกับวันที่ท้องถิ่น; เงินเป็น integer สตางค์; QR token random 32 bytes เก็บ hash
- **Security**: JWT httpOnly cookie ผ่าน Kong → claims เป็น header ที่ trust เฉพาะจาก gateway; ทุก query scope ด้วย `operator_id`; PDPA minimal PII; `.env` ไม่ commit
- **Observability**: `trace_id` ต่อกันข้าม HTTP + Kafka (otel propagation), structured log slog JSON — ตั้งแต่ Phase 0
- **Testing**: unit `go test`; integration ต่อ service กับ Postgres+Redpanda จริง (testcontainers); contract test event; e2e Playwright เฉพาะ flow จอง→จ่าย→ตั๋ว; capacity race test ใน CI ทุกครั้ง
- **Commits**: conventional commits, scope = service name (`feat(booking): ...`)
- **Team**: solo developer — ปรับ scope ต่อ phase ให้จบได้จริง

<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->

## Technology Stack

## Verdict on the Prescribed Stack

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | **1.25.x** (module `go 1.25`) | Backend language, all 7 services + BFF | 1.25 is current stable (1.25.7 as of Feb 2026); connect-go v1.20+ already requires Go ≥1.25 as its minimum, so "1.23+" in the seed is now a floor you can't actually build on with current connect-go — bump the workspace `go.work`/`go.mod` to 1.25. Confidence: HIGH. |
| chi | **v5.3.2** (`github.com/go-chi/chi/v5`) | HTTP router + middleware for `pkg/httpx` and BFF | Actively maintained (last publish Aug 2026), zero deps, idiomatic `net/http` handler chain — fits the "thin BFF" role. Note: Go 1.22+ stdlib `net/http.ServeMux` now does method+wildcard routing natively, so chi's *routing* value is smaller than in 2022; you're really keeping it for its middleware ecosystem (`middleware.RequestID`, `Recoverer`, `Timeout`, compress). Confidence: HIGH. |
| connect-go | **v1.20.0** (`connectrpc.com/connect`) | Sync inter-service RPC (gRPC+HTTP/JSON+Connect in one handler) | Current stable; v1 module is supported indefinitely (v2 is beta on a separate module path, don't use it yet — API not final). Requires Go ≥1.25 (see above). Gives curl-able JSON debugging + generated Go/TS clients from the same proto, exactly the "sync API you can debug without grpcurl" requirement. Confidence: HIGH. |
| sqlc | **v1.31.x** | Generate typed Go from SQL + schema, per service | pgx/v5 support has been stable since sqlc 1.18 (years ago, not new/risky). Set `sql_package: "pgx/v5"` in `sqlc.yaml`. Confidence: HIGH. |
| pgx | **v5.x** (`github.com/jackc/pgx/v5`) | Postgres driver + pool | De facto standard high-perf Postgres driver for Go; use `pgxpool` per service, one pool per service DB/DSN. Confidence: HIGH. |
| goose | **v3.x** (`github.com/pressly/goose/v3`) | SQL + Go migrations per service | v3 is the actively tagged major; supports plain-SQL and Go-func migrations, works per-service with each service's own `migrations/` dir as the repo structure already plans. Confidence: HIGH. |
| franz-go | **v1.22.x** (`github.com/twmb/franz-go`) | Kafka/Redpanda client (producer + consumer) | Fastest, most CPU/memory-efficient pure-Go Kafka client; actively released (v1.22.0 shipped Sept 18 2026, weekly cadence). Confluent's `confluent-kafka-go` (cgo, wraps librdkafka) and `segmentio/kafka-go` (unmaintained-adjacent, no transactions) are both worse fits — see "What NOT to Use". Confidence: HIGH. |
| go-redis | **v9.19.x** (`github.com/redis/go-redis/v9`) | Redis client for seat-hold TTL, rate limit, OTP | v9 is the only maintained line; protocol-compatible with both Redis and Valkey (see Cache section — pick the server separately from the client). Confidence: HIGH. |
| slog | stdlib (Go 1.21+) | Structured logging, JSON handler | Already in stdlib, zero dependency, exactly what `pkg/httpx`/`pkg/kafka` need for `trace_id`-correlated JSON logs. Confidence: HIGH. |
| OpenTelemetry Go | **v1.47.0** (traces+metrics API/SDK), Logs SDK **now stable as of v1.47.0 (Sept 24 2026)** | Distributed tracing across HTTP + Kafka | Traces and metrics have been stable for a long time; Logs SDK just went stable literally this week (Sept 2026) — you can now use the OTel Go logs bridge with slog instead of a separate log shipper config, simplifying the Grafana Loki pipeline. Confidence: HIGH. |
| PostgreSQL | **17.x** (recommend over the seed's "16") | Primary datastore, database-per-service | 16 still fully supported, but greenfield + self-managed on your own EC2 (no RDS version lag to worry about) means there's no reason to start one major behind. 17 is the mature choice (18 shipped Sept 2025, is only ~1yr old — async I/O rework is real but newer/less battle-tested for a solo-maintained prod system). If you want the 18 read-throughput win, 18.6 is also fine; either way pin the exact minor and test `pg_upgrade` path before v1 launch. Confidence: HIGH on 17 being safe; MEDIUM on "18 vs 17" being purely a risk-tolerance call. |
| Redis (or Valkey) | Redis **8.x** (AGPLv3) or **Valkey 9.1** (BSD-3) | Seat-hold TTL + keyspace notifications, rate limit, OTP | See "Cache/Lock licensing" note below — both work unmodified with `go-redis/v9`. Confidence: HIGH. |
| Redpanda | **v25.x/26.x** current release, pin exact tag (e.g. `redpandadata/redpanda:v25.x.x`) | Kafka-compatible event bus, dev + prod v1 | Single binary, no ZooKeeper/KRaft split-brain to manage, Kafka-protocol compatible so franz-go needs zero code changes if you ever move to MSK. For single-node dev/prod-v1, run with `--overprovisioned --smp 1 --memory 512M-1G` per Redpanda's own single-broker lab config. Confidence: HIGH. |
| Kong Gateway | **kong:3.9.1 (OSS, pinned)** | API gateway: routing, TLS, JWT plugin, rate limit | **Critical gotcha, read the Gateway section below before Phase 0.** Confidence: HIGH on the licensing fact, MEDIUM-risk on whether Kong OSS is still the right pick given the freeze. |
| Next.js | **16.x** (App Router) — seed says 15 | Frontend PWA | Next.js 16.3 is current stable (Sept 2026); starting a greenfield project now, build on 16 not 15. React 19.2 features, faster dev-mode memory, `"use cache"` directive for the search/results pages that need to feel instant. If you specifically need 15's stability window for a reason not stated in the seed, 15 is still fine — just know you're one major behind day one. Confidence: MEDIUM (recommendation, not a hard fact — team may have reasons to stay on 15). |
| buf CLI | **v1.73.x** | Protobuf lint/breaking-check/codegen orchestration | Drives `protoc-gen-go`, `protoc-gen-connect-go` (Go, v1.18.1 via buf remote plugins), `@bufbuild/protoc-gen-es` (v2.14.1, TS message types) and `@connectrpc/protoc-gen-connect-es` (v1.7.x, TS Connect clients — **use the `@connectrpc/*` package, not the archived `@bufbuild/protoc-gen-connect-es`**, see below). Confidence: HIGH. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@connectrpc/protoc-gen-connect-es` | v1.7.x | Generate TS Connect clients for Next.js | Use this, not `@bufbuild/protoc-gen-connect-es` (last published ~3 years ago, effectively abandoned — the org moved to `@connectrpc/*` scope). |
| `@bufbuild/protoc-gen-es` | v2.14.x | Generate TS message types from proto | Pairs with the connect-es plugin above; both driven from one `buf.gen.yaml`. |
| `testcontainers-go` + `modules/postgres`, `modules/redpanda` | v0.44.x (lockstep) | Integration tests against real Postgres + Redpanda in CI | Exactly matches the seed's "integration ต่อ service กับ Postgres+Redpanda จริง" requirement — no fakes/mocks for the capacity-race test. Pin `postgres:17-alpine` and a specific `redpandadata/redpanda:vX.Y.Z` tag in the module `Run()` calls so CI doesn't silently pick up a new major. |
| `skip2/go-qrcode` | latest tag | Server-side QR generation for tickets | Still fine for v1 — pure Go, no deps, does exactly one job. If profiling later shows it's a hot path, `piglig/go-qr` is a 3-4x faster, zero-dependency drop-in with lower allocs — not needed at v1 scale (one QR per paid booking). |
| `qr-scanner` (nimiq) or native `BarcodeDetector` | latest | Staff-side browser QR scanning (Phase 5 check-in) | **Do not use `html5-qrcode`** — unmaintained since 2023, wraps the also-unmaintained `zxing-js`. `qr-scanner` is lighter, worker-based, actively used; fall back further to the native `BarcodeDetector` Web API (supported in Chrome/Edge/Android WebView, i.e. exactly the staff-tablet environment you'd deploy to at a pier) with `qr-scanner` as the polyfill path. Not urgent for v1 (check-in is Phase 5), but don't let the Phase 0 template wire up the dead library. |
| `resend-go/v3` (`github.com/resend/resend-go/v3`) | v3 (v4 also exists) | Transactional email (ticket, OTP) | Zero third-party runtime deps, simple `Emails.Send` API matches the seed's "email ตั๋ว (Resend/SES)" requirement directly. |
| `aws-sdk-go-v2/service/ses` | latest | Alternative/fallback email sender | Only if you need SES specifically (e.g. already on AWS SES sending reputation) — otherwise Resend's DX is strictly better for a solo dev. |
| `omise/omise-go` | latest (pub. Mar 2026) | Opn (Omise) payment provider Go client | See Payment Provider section — has a maintained webhook HTTP handler helper and native PromptPay charge support. |
| Leaflet + `react-leaflet` + OSM tiles | Leaflet 1.9.x | Map picker (admin) + route map (public search) | Free, no API key, matches "Leaflet/OSM ฟรี" in the seed. Use Mapbox only if you need styled/branded tiles or better Thai-language label rendering — OSM's Thai POI/label coverage is decent in tourist areas but patchier in smaller piers. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `buf generate` + `buf.gen.yaml` | Single command generates Go (service+events) and TS (web app) from `proto/` | Wire `make proto-gen` to this; commit generated code per the seed's repo structure (`gen/`) so CI doesn't need network access to buf's remote plugins on every run — or vendor the plugin binaries. |
| `testcontainers-go` | Spins up real Postgres/Redpanda per test run | Requires Docker-in-CI (Jenkins agent needs Docker socket access) — confirm the Jenkins agent AMI has Docker before Phase 0 is "done". |
| `golangci-lint` | Go linting across the workspace | Standard choice for multi-module Go workspaces; not in the seed explicitly but expected tooling — add in Phase 0 alongside `make new-service`. |
| Grafana Tempo/Loki/Prometheus (LGTM-minus-Grafana-Cloud) | Tracing/logs/metrics, self-hosted in Compose | Current LGTM stack images are fine to pin at latest-stable tags; no breaking gotchas found for this combination in 2026. The OTel Go Logs SDK going stable this month (see above) means Loki ingestion can go through the same OTel Collector pipeline as traces, instead of a separate log-shipping sidecar — simplifies the Compose stack by one moving part. |

## Gateway: Kong Licensing Gotcha (read before Phase 0)

- If you build against `kong:latest` in `docker-compose.yml`, you will eventually pull a licensed-only image and things will silently misbehave or lock features.
- **Pin `kong:3.9.1`** (or whatever the final OSS tag turns out to be — verify against `Kong/kong` GitHub discussions before Phase 0) explicitly in the compose file, and treat any Kong upgrade as a deliberate decision, not `docker compose pull`.
- Kong OSS 3.9.1 still has DB-less mode and the JWT plugin (both used since v1 of Kong OSS, not new/enterprise features), so **the prescribed architecture (Kong + thin Go BFF, JWT plugin, DB-less declarative config) still works on the frozen OSS tag.** This is a pin-and-monitor issue, not a redesign.
- Because the seed's own root `PROJECT.md` §4 already lists **Traefik + Go BFF** as an equally-considered alternative ("หรือ Kong ถ้าต้องการ plugin"), and Traefik has no equivalent licensing cliff (Apache-2.0, no OSS/Enterprise split), it's worth a 30-minute spike before Phase 0 to confirm Kong 3.9.1's DB-less JWT plugin does everything you need — if it does, ship it pinned; if you hit an OSS-only limitation, Traefik + a slightly thicker Go BFF (JWT verification moves from Kong plugin into the BFF, which you're already writing) is a clean fallback with no rewrite of the service layer.
- Concrete Phase 0 config: DB-less Kong reads a single `kong.yml` declarative file (services, routes, `jwt` plugin per consumer/route, `cors`, `rate-limiting`) — no `kong-postgres` container needed, which also removes one moving part from the dev Compose stack (the seed's dev stack doesn't currently list a separate Kong DB, confirming DB-less was already the implicit intent).

## franz-go Patterns (per downstream request)

- **Producer side (outbox relay):** one `kgo.Client` per service configured with `kgo.ProducerBatchMaxBytes`, `kgo.RequiredAcks(kgo.AllISRAcks())`, and idempotent producer enabled by default in franz-go — the relay polls the `outbox` table, produces, and only marks rows sent after a successful `ProduceSync` (or batched `Produce` + wait), matching the "outbox in same tx as state, relay does the actual publish" design already in the seed.
- **Consumer side:** use `kgo.ConsumerGroup(serviceName)`, `kgo.ConsumeTopics(...)`, and **`kgo.DisableAutoCommit()`** — franz-go auto-commits by default, which the seed's guardrail explicitly forbids ("ห้าม auto-commit — commit หลัง apply สำเร็จ"). Call `cl.CommitRecords(ctx, records...)` (or `CommitUncommittedOffsets`) only after the consumer has written to `processed_events` and applied the projection in the same DB transaction. This is the standard "process-then-commit" pattern and is exactly what franz-go's manual-commit API is designed for.
- **DLQ:** on exhausting in-process retries (3, per the seed), produce the failed record + error metadata to `<topic>.dlq` using the same client, then commit the original offset anyway (don't block the partition) — franz-go doesn't have DLQ as a built-in feature, it's an app-level pattern of "produce to dead-letter topic, then commit forward."
- **Ordering:** partition key = aggregate id is a producer-side concern (`kgo.KeyPartitioner` uses the record key you set — set `Record.Key = []byte(departureID)` or `[]byte(bookingID)` per the seed's convention); franz-go doesn't need special config for this beyond setting the key.

## connect-go + buf Codegen for TypeScript (per downstream request)

## testcontainers-go for Postgres + Redpanda (per downstream request)

- `go get github.com/testcontainers/testcontainers-go` + `.../modules/postgres` + `.../modules/redpanda`, all released in lockstep at v0.44.x — keep them on the same version to avoid API-mismatch surprises.
- Postgres module: `postgres.Run(ctx, "postgres:17-alpine", postgres.WithDatabase(...), postgres.WithUsername(...), postgres.WithPassword(...))`, then run `goose` migrations against the returned DSN before each service's integration tests — this gives every service test its own throwaway Postgres 17 instance matching prod.
- Redpanda module: `redpanda.Run(ctx, "docker.redpandadata/redpanda:v25.x.x")` (pin to match your Compose dev-stack tag) — exposes a real Kafka-protocol broker for franz-go producer/consumer round-trip tests, including the capacity-race integration test's async side (if any event assertions are part of that test).
- Run these in CI (Jenkins) — confirm the Jenkins build agent has Docker socket access; this is a Phase-0 infra checkbox, not a code concern.

## Payment Provider: Opn (Omise) vs 2C2P

| Criterion | Opn (Omise) | 2C2P |
|---|---|---|
| Go SDK | `github.com/omise/omise-go` — actively published (last release Mar 2026), idiomatic Go client, includes a ready-made `WebhookHTTPHandler` that unmarshals `Event` objects from Omise's webhook POSTs | No first-party Go SDK found; integration is via their REST API (redirect-hosted checkout, embedded UI, or direct API) — you'd hand-roll the Go client and webhook verification yourself |
| PromptPay QR | First-class `Source`/`Charge` API for PromptPay, documented specifically for this flow (`docs.opn.ooo/promptpay`) | Also supports PromptPay (interoperable with regional QR too), but documentation is JS/mobile-SDK-first, not Go-first |
| Webhook maturity | Named `Event` object + signature-friendly handler helper in the SDK — meaningfully less code for the "webhook idempotent" requirement | Webhook exists but you're implementing verification/parsing from scratch against their API docs |
| Sandbox | Omise/Opn has a standard test-mode dashboard + test API keys, well-trodden path for SEA fintech integrations | 2C2P has a sandbox too but is oriented more toward enterprise/redirect-checkout integration flows |
| **Recommendation** | **Prefer Opn (Omise) for v1.** The existence of a maintained Go SDK with a webhook handler directly reduces the highest-risk part of Phase 4 (idempotent webhook handling) from "write and test HMAC verification + JSON parsing yourself" to "use the library's `WebhookHTTPHandler` and verify against test events." 2C2P may still be worth it later for enterprise/corporate-card volume or if the operator relationships specifically require it, but that's a Growth-phase (§8, phase 8) decision, not v1. | — |

## Cache/Lock: Redis License Note

## Installation

# Go workspace (per service go.mod, go.work at root)

# sqlc + goose CLIs (dev tools, not go.mod deps)

# testcontainers

# QR + email + payment

# buf (proto toolchain, local install or Homebrew)

# Next.js frontend

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| chi | stdlib `net/http.ServeMux` (Go 1.22+ pattern routing) | If you want zero router deps at all — Go 1.22+'s mux now does `GET /pier/{id}` style routing natively. You'd lose chi's middleware chain conventions (Recoverer, RequestID, Timeout), which you'd then hand-roll. Not worth it for 8 services; chi stays. |
| Kong (pinned OSS 3.9.1) | Traefik + Go BFF | If the Kong OSS licensing freeze turns out to block something you need (a plugin now Enterprise-only), or if you'd rather avoid tracking a frozen fork at all — the seed itself lists this as an equal option. Apache-2.0, no license cliff, but you move JWT verification logic into your own BFF code instead of a declarative plugin. |
| franz-go | `segmentio/kafka-go` | Never for this project — no transactional/exactly-once support, slower, and maintenance has been inconsistent; franz-go is strictly better here. |
| franz-go | `confluent-kafka-go` | If you need Confluent-specific enterprise features (Schema Registry deep integration) — but it's a cgo wrapper around librdkafka, meaning cross-compilation/Docker multi-stage builds get heavier. Not worth it when your events are already Protobuf via buf, not Confluent Schema Registry. |
| Redis 8 (AGPLv3) | Valkey 9.x (BSD-3) | Default recommendation is actually Valkey (see Cache section) — listed here as "alternative" only because the seed named "Redis" specifically. |
| PostgreSQL 17 | PostgreSQL 18 | If you want the new async I/O subsystem's read-throughput win (up to 3x on storage-bound reads) and are comfortable running a ~1-year-old major in solo-maintained prod. Either is fine; 17 is the more conservative pick. |
| skip2/go-qrcode | piglig/go-qr | If ticket QR generation ever becomes a measured hot path (unlikely at v1 booking volumes) — 3-4x faster, far fewer allocations, zero deps either way. |
| Opn (Omise) | 2C2P | If operator/business relationships specifically require 2C2P, or you need enterprise redirect-checkout flows at scale later — not a v1 concern. |
| Next.js 16 | Next.js 15 | If there's a hard reason (dependency not yet updated for 16, team familiarity) to stay one major behind — 15 is still fine, just not the current default for a fresh Sept 2026 project. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `html5-qrcode` (for the staff scanner) | Unmaintained since 2023 (no releases since April 2023), wraps the also-unmaintained `zxing-js` | `qr-scanner` (nimiq), with native `BarcodeDetector` API as first choice where supported (Chrome/Edge/Android WebView — the staff-tablet environment) |
| `kong:latest` in Compose/prod | Will eventually resolve to a licensed-only Enterprise image once 3.10+ becomes "latest" | Explicitly pin `kong:3.9.1` (OSS) and upgrade deliberately, tracking Kong's OSS support discussions |
| `@bufbuild/protoc-gen-connect-es` | Effectively abandoned npm package (last publish ~3 years ago), superseded by a namespace move | `@connectrpc/protoc-gen-connect-es` |
| `segmentio/kafka-go` | No exactly-once/transactional support, weaker fit for the outbox+idempotent-consumer architecture already designed | `franz-go` |
| Debezium/CDC for outbox relay | Explicitly out of scope per the seed already — extra moving part (Kafka Connect cluster) not needed at this scale | Go outbox relay (poll+publish+mark), already the seed's decision — this research confirms it's still the right call for v1 |
| Kubernetes for v1 | Already out of scope per the seed; confirmed still correct — Docker Compose on one EC2 is the right complexity level for a solo dev at this stage, and the service-per-container boundary already makes a later ECS/EKS move mechanical, not a rewrite | Docker Compose → Jenkins → ECR → EC2 (as decided) |
| connect-go v2 module (`connectrpc.com/connect/v2`) | Still beta as of Sept 2026, API not final | v1 module (`connectrpc.com/connect`), supported indefinitely |

## Stack Patterns by Variant

- Fall back to Traefik (Apache-2.0, file/label-based dynamic config) + move JWT verification into the Go BFF middleware.
- Because the BFF already needs to "forward claims as trusted headers" per the seed's guardrail, this fallback is additive work in one place (BFF), not a service-layer rewrite.
- Stay on Redis proper (AGPLv3, fine for self-hosted) rather than Valkey, since Redis 8 has the more integrated vector/search story — this is a v8+ (Growth) decision, not v1.
- Use 18.6 instead of 17.x; the BFF's live-query search pattern (no projection service in v1) is read-heavy, so this is a legitimate reason to take the newer major if you want it — otherwise 17 is the safer default.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `connectrpc.com/connect@v1.20.0` | Go ≥1.25 | Minimum bumped in this release — do not try to pair with Go 1.23/1.24 toolchains; update `go.work`'s `go` directive accordingly. |
| `sqlc` (sql_package: pgx/v5) | `github.com/jackc/pgx/v5` | Stable pairing since sqlc 1.18; no known issues at current versions. |
| `testcontainers-go/modules/postgres` + `/modules/redpanda` | `testcontainers-go` core, same v0.44.x line | Released in lockstep — upgrade all three together, don't pin core and modules to different minors. |
| `franz-go` | Kafka protocol 0.8.0 through 4.4+, and Redpanda (Kafka-protocol-compatible) | No Redpanda-specific client needed; same franz-go code targets both Redpanda now and MSK later. |
| `go-redis/v9` | Both Redis (incl. AGPLv3 Redis 8) and Valkey (RESP-protocol compatible) | Client doesn't care which server you run; the license question is a deployment/legal decision, not a code one. |
| `@connectrpc/protoc-gen-connect-es` + `@bufbuild/protoc-gen-es` | Same `buf.gen.yaml`, both plugins needed | connect-es generates the *service* client, protoc-gen-es generates the *message* types they depend on — you need both, not one or the other. |
| Kong `kong:3.9.1` OSS | Postgres or DB-less | Use DB-less per the seed's dev-stack design (no separate `kong-database` container listed) — confirm the declarative `kong.yml` covers JWT plugin config, which it does in OSS. |

## Sources

- [Go 1.25 Release Notes](https://go.dev/doc/go1.25) — confirmed current Go stable line, HIGH confidence (official docs)
- [go-chi/chi GitHub](https://github.com/go-chi/chi), [pkg.go.dev/github.com/go-chi/chi/v5](https://pkg.go.dev/github.com/go-chi/chi/v5) — v5.3.2, activity confirmed Aug 2026, HIGH
- [connectrpc/connect-go GitHub](https://github.com/connectrpc/connect-go), [connectrpc.com/docs/go/getting-started](https://connectrpc.com/docs/go/getting-started/) — v1.20.0, Go≥1.25 requirement, v2 beta status, HIGH
- [sqlc docs — Using Go and pgx](https://docs.sqlc.dev/en/stable/guides/using-go-and-pgx.html), [sqlc changelog](https://docs.sqlc.dev/en/stable/reference/changelog.html) — v1.31.x, pgx/v5 stability since 1.18, HIGH
- [pressly/goose GitHub](https://github.com/pressly/goose) — v3 current major, HIGH
- [twmb/franz-go GitHub](https://github.com/twmb/franz-go), CHANGELOG.md — v1.22.0 (Sept 18 2026), active weekly releases, HIGH
- [redis/go-redis GitHub releases](https://github.com/redis/go-redis/releases) — v9.19.x stable line, HIGH
- [PostgreSQL 18 Released](https://www.postgresql.org/about/news/postgresql-18-released-3142/), [endoflife.date/postgresql](https://endoflife.date/postgresql) — version/support timeline, HIGH
- [Redpanda Docker Compose single-broker lab](https://docs.redpanda.com/labs/docker-compose/single-broker/) — official single-node dev config pattern, HIGH
- [Kong Gateway version support policy](https://developer.konghq.com/gateway/version-support-policy/), [Kong/kong GitHub Discussion #14642 — OSS 3.9.1](https://github.com/Kong/kong/discussions/14642), [Kong/kong Discussion #14628](https://github.com/Kong/kong/discussions/14628) — OSS freeze at 3.9.1 starting 3.10, HIGH (multiple corroborating official/community sources)
- [Kong JWT plugin docs](https://developer.konghq.com/plugins/jwt/), [Kong DB-less mode docs](https://developer.konghq.com/gateway/db-less-mode/) — confirms JWT + DB-less are OSS-tier features, HIGH
- [Next.js 16 blog](https://nextjs.org/blog/next-16), [Next.js 16.3 blog](https://nextjs.org/blog/next-16-3), [Upgrading to v16 guide](https://nextjs.org/docs/app/guides/upgrading/version-16) — 16.3 current stable Sept 2026, HIGH
- [buf CLI npm/GitHub](https://github.com/bufbuild/buf) — v1.73.x, HIGH
- [@bufbuild/protoc-gen-es npm](https://www.npmjs.com/package/@bufbuild/protoc-gen-es), [@connectrpc/protoc-gen-connect-es npm](https://www.npmjs.com/package/@connectrpc/protoc-gen-connect-es), [@bufbuild/protoc-gen-connect-es npm (deprecated path)](https://www.npmjs.com/package/@bufbuild/protoc-gen-connect-es) — version + abandonment evidence, HIGH
- [Kong Gateway pricing/architecture analysis 2026](https://www.truefoundry.com/blog/kong-gateway-pricing-architecture-an-analysis-for-ai-teams-2026-edition) — third-party corroboration of licensing split, MEDIUM (secondary source, cross-checked against official Kong discussions)
- [testcontainers-go GitHub releases](https://github.com/testcontainers/testcontainers-go/releases), [Testcontainers Redpanda module](https://testcontainers.com/modules/redpanda/), [Postgres module docs](https://golang.testcontainers.org/modules/postgres/) — v0.44.x lockstep versioning, HIGH
- [Omise Go SDK — pkg.go.dev](https://pkg.go.dev/github.com/omise/omise-go), [Omise PromptPay docs](https://docs.opn.ooo/promptpay) — SDK freshness (pub. Mar 2026), webhook handler, HIGH
- [2C2P developer docs](https://developer.2c2p.com/docs/general), [2C2P SDK APIs](https://developer.2c2p.com/v4.0.2/docs/sdk-apis) — no first-party Go SDK found, integration model, MEDIUM (docs reviewed, no hands-on sandbox test)
- [skip2/go-qrcode GitHub](https://github.com/skip2/go-qrcode), [piglig/go-qr GitHub](https://github.com/piglig/go-qr) — status + faster alternative, HIGH
- [mebjas/html5-qrcode GitHub](https://github.com/mebjas/html5-qrcode), [ScanApp blog on html5-qrcode](https://scanapp.org/blog/2026/05/24/is-html5-qrcode-the-best-javascript-scanner-library.html) — unmaintained status confirmed (no releases since Apr 2023) via primary repo + third-party corroboration, HIGH
- [resend/resend-go GitHub](https://github.com/resend/resend-go), [Resend Go docs](https://resend.com/docs/send-with-go) — SDK current, HIGH
- [Redis AGPLv3 announcement](https://redis.io/blog/agplv3/), [Percona — Redis license change explainer](https://www.percona.com/blog/the-redis-license-has-changed-what-you-need-to-know/), [Valkey vs Redis 2026 fork analysis](https://dev.to/synsun/redis-vs-valkey-in-2026-what-the-license-fork-actually-changed-1kni) — licensing timeline + Valkey positioning, HIGH (official Redis blog + cross-checked secondary sources)
- [OpenTelemetry Go Logs API/SDK RC blog](https://opentelemetry.io/blog/2026/go-logs-api-sdk-rc/), [opentelemetry-go GitHub releases](https://github.com/open-telemetry/opentelemetry-go/releases) — Logs SDK went stable in v1.47.0 (Sept 24 2026, literally days before this research), HIGH

<!-- GSD:stack-end -->

## User Preferences

- **ภาษา**: ตอบคำถามและถามคำถามผู้ใช้เป็นภาษาไทยเสมอ (technical terms, code, file paths, commit messages, subagent prompts คงเป็นภาษาอังกฤษ)
- **การเขียนโค้ด**: ใช้ skill `ponytail` (full) ทุกครั้งที่เขียน/แก้/รีวิวโค้ด — เลือกวิธีที่สั้นและง่ายที่สุดที่ใช้งานได้จริง, YAGNI, stdlib/native ก่อน dependency ใหม่, ห้ามเพิ่ม abstraction ที่ไม่ได้ขอ

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
