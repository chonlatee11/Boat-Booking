---
status: complete
phase: 01-platform-foundation
source: [01-01-SUMMARY.md, 01-02-SUMMARY.md, 01-03-SUMMARY.md, 01-04-SUMMARY.md, 01-05-SUMMARY.md, 01-06-SUMMARY.md, 01-07-SUMMARY.md, 01-08-SUMMARY.md, 01-09-SUMMARY.md, 01-10-SUMMARY.md, 01-11-SUMMARY.md, 01-12-SUMMARY.md, 01-13-SUMMARY.md]
started: 2026-09-26T10:08:25Z
updated: 2026-09-26T12:30:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: ล้าง state: `make down` แล้วลบ volume ของ project (compose `down -v` ด้วย $(COMPOSE) ตัวเดียวกับใน Makefile, ครบทุก profile) จากนั้น `make up` และ `make proof` — stack บูตจากศูนย์ไม่มี error, migrate-<svc> ทุกตัว exit 0, ทุก service healthy, `make proof` PASS และ GET /api/v1/public/boats เห็น boat จริง
result: pass

### 2. Package legitimacy gate (01-05 D1)
expected: npm packages ที่ถูก flag (next-intl, @tanstack/react-query, @bufbuild/protobuf, tailwind-merge, lucide-react, prettier) คุณเคยยืนยันแล้วว่าเป็นแพ็กเกจจริงก่อนติดตั้ง — ยืนยันอีกครั้งว่ายังโอเคกับรายการนี้ใน apps/web/package.json
result: pass

### 3. Web skeleton TH/EN + font + Kong-only data path (01-05 D2)
expected: เปิด http://localhost:3001 ใน viewport 375px: / redirect ไป /th, ภาษาไทยเรนเดอร์ด้วย IBM Plex Sans Thai (ไม่ fallback เป็น system font), /en แสดงข้อความอังกฤษ, devtools Network ทุก XHR ไปที่ localhost:8000 (Kong) ไม่ยิงตรงไป gateway/catalog
result: pass

### 4. Boat card list + refresh + locale switcher (01-05 D3)
expected: ที่ 375px หน้า /th แสดง boat ของ make proof เป็น shadcn Card, ปุ่ม refresh กดแล้ว refetch (เห็น request ใหม่ใน Network), locale switcher ใน header สลับ TH/EN ได้, ปุ่มขนาดพอดีนิ้ว
result: pass

### 5. Consumer with no handlers starts no Kafka client (01-07 D5)
expected: gateway (relay/consumer off) บูตแล้ว log ไม่มีการต่อ Kafka/ไม่มี consumer group และ stop gateway แล้วปิดสะอาดไม่มี error — หรือยืนยันจากโค้ด pkg/kafka ว่ามี early-return เมื่อไม่มี handler
result: pass

### 6. Gateway re-scaffolded runtime boot/shutdown (01-11 D3)
expected: หลัง `make up` container gateway เป็น healthy; stop gateway แล้ว log ออกตามลำดับ readyz 503 -> stop HTTP -> OTel shutdown ภายใน 15s, exit code 0
result: pass

### 7. Grafana trace continuity across Kafka hop + platform dashboard (01-12 D9)
expected: หลัง `make proof` เปิด http://localhost:3000 -> Explore -> Tempo ค้น { span.aggregate_id = "<boat id จาก make proof>" }: trace เดียวมี gateway HTTP -> catalog connect -> Kafka publish -> schedule consume span, trace id เดียวกัน ไม่ขาดตรง Kafka; 'Logs for this span' กระโดดไป Loki ได้; dashboard 'platform' มี panel HTTP rate / consumer lag / processed-events / outbox backlog ที่มีข้อมูลจริง
result: pass
retest: "pass (2026-09-26, after 01-14 fix for G-01-7)"
prior_issue: resolved by 01-14-PLAN.md

### 8. Live Jenkins changed-services image scoping (01-13 D5)
expected: push commit ที่แก้ใต้ pkg/ แล้ว commit ที่แก้เฉพาะ services/schedule/ ดู Jenkins multibranch scan 2 รอบ: รอบแรก build image ทั้ง 4 service, รอบสอง build แค่ schedule; ทั้งคู่รันทุก stage ที่ไม่ใช่ image; ไม่มีรอบไหน push ถ้าไม่ใช่ main
result: pass
retest: "pass (2026-09-26, user confirmed live Jenkins per-commit scoping)"
prior_note: "Partially verified live 2026-09-26: throwaway branch uat-scope-pkg (commit de1bf7d, pkg/clock comment-only change) built SUCCESS on real Jenkins — Diff base origin/main, every non-image stage ran (Lint/Proto/Unit/Integration/Migrations/Template PASS/Web), Images built all 4 (template, catalog, gateway, schedule), Push skipped due to when conditional; branch, worktree and images removed afterwards. Per-commit scoping (services/schedule/-only change -> only schedule) cannot be exercised off main: non-main builds diff against origin/main, which is docs-only until phase 1 merges, so every branch build selects all. Re-check on main after merge (base = GIT_PREVIOUS_SUCCESSFUL_COMMIT)."

### 9. [01-01 D1] Kong 3.9.1 DB-less JWT edge spike: reject-bad-token (foreign key, expired, wrong kind), routing, CORS preflight, rate-limiting 429
expected: Kong 3.9.1 DB-less JWT edge spike: reject-bad-token (foreign key, expired, wrong kind), routing, CORS preflight, rate-limiting 429
result: pass
source: automated
coverage_id: D1

### 10. [01-01 D2] pkg/auth RS256 Issuer/Verifier: round-trip, TTLs (900s/2592000s), foreign-key/alg-confusion/expired/wrong-issuer/wrong-kind rejection, cookie attributes
expected: pkg/auth RS256 Issuer/Verifier: round-trip, TTLs (900s/2592000s), foreign-key/alg-confusion/expired/wrong-issuer/wrong-kind rejection, cookie attributes
result: pass
source: automated
coverage_id: D2

### 11. [01-01 D3] pkg/httpx trust-boundary middleware: RequireInternal constant-time token check + claim propagation, RequireClaims gate, FromContext/WithClaims
expected: pkg/httpx trust-boundary middleware: RequireInternal constant-time token check + claim propagation, RequireClaims gate, FromContext/WithClaims
result: pass
source: automated
coverage_id: D3

### 12. [01-01 D4] Gateway BFF: GET /api/v1/whoami verifies access cookie/bearer token; ForwardClaims deletes then sets trusted headers, closing the header-spoof vector
expected: Gateway BFF: GET /api/v1/whoami verifies access cookie/bearer token; ForwardClaims deletes then sets trusted headers, closing the header-spoof vector
result: pass
source: automated
coverage_id: D4

### 13. [01-01 D5] Root Dockerfile (distroless nonroot) + docker-compose.yml: Kong pinned 3.9.1, gateway publishes no host port, private key never reaches gateway env
expected: Root Dockerfile (distroless nonroot) + docker-compose.yml: Kong pinned 3.9.1, gateway publishes no host port, private key never reaches gateway env
result: pass
source: automated
coverage_id: D5

### 14. [01-01 D6] golangci-lint v2 + lefthook baseline: forbidigo bans time.Now/fmt.Print*/kgo.NewClient/pgxpool.New in app code, proven to actually fire
expected: golangci-lint v2 + lefthook baseline: forbidigo bans time.Now/fmt.Print*/kgo.NewClient/pgxpool.New in app code, proven to actually fire
result: pass
source: automated
coverage_id: D6

### 15. [01-02 D1] make proto-gen regenerates committed gen/go (protoc-gen-go + connect-go) and gen/ts (protoc-gen-es, JSON types) from proto/ in one command
expected: make proto-gen regenerates committed gen/go (protoc-gen-go + connect-go) and gen/ts (protoc-gen-es, JSON types) from proto/ in one command
result: pass
source: automated
coverage_id: D1

### 16. [01-02 D2] Envelope{event_id, event_type, aggregate_id, occurred_at, version, Any payload} and catalog.BoatUpserted{boat_id, operator_id, name, default_capacity, status} exist as the real wire contract
expected: Envelope{event_id, event_type, aggregate_id, occurred_at, version, Any payload} and catalog.BoatUpserted{boat_id, operator_id, name, default_capacity, status} exist as the real wire contract
result: pass
source: automated
coverage_id: D2

### 17. [01-02 D3] CatalogService{UpsertBoat, ListBoats} generates a connect-go handler/client package and TS JSON types
expected: CatalogService{UpsertBoat, ListBoats} generates a connect-go handler/client package and TS JSON types
result: pass
source: automated
coverage_id: D3

### 18. [01-02 D4] make proto-check fails on buf lint errors, gen/ drift, breaking changes against main (skipped with notice while main has no buf.yaml), and PII-shaped field names under proto/events
expected: make proto-check fails on buf lint errors, gen/ drift, breaking changes against main (skipped with notice while main has no buf.yaml), and PII-shaped field names under proto/events
result: pass
source: automated
coverage_id: D4

### 19. [01-02 D5] Root Dockerfile copies gen/go so service images build with GOWORK=off
expected: Root Dockerfile copies gen/go so service images build with GOWORK=off
result: pass
source: automated
coverage_id: D5

### 20. [01-03 D1] clock.LocalDate maps UTC instants to the Asia/Bangkok calendar date at both midnight boundaries (23:59:59/00:00:00, and the 00:00-01:00 / 23:00-00:00 windows), returned as midnight UTC of that date
expected: clock.LocalDate maps UTC instants to the Asia/Bangkok calendar date at both midnight boundaries (23:59:59/00:00:00, and the 00:00-01:00 / 23:00-00:00 windows), returned as midnight UTC of that date
result: pass
source: automated
coverage_id: D1

### 21. [01-03 D2] clock.Now is overridable in tests and restorable to wall-clock time afterward
expected: clock.Now is overridable in tests and restorable to wall-clock time afterward
result: pass
source: automated
coverage_id: D2

### 22. [01-03 D3] Asia/Bangkok resolves via embedded time/tzdata (no system zoneinfo dependency, required for distroless images)
expected: Asia/Bangkok resolves via embedded time/tzdata (no system zoneinfo dependency, required for distroless images)
result: pass
source: automated
coverage_id: D3

### 23. [01-03 D4] money.PercentOf truncates fractional satang toward zero (12345,50->6172; 1,50->0; 333,33->109) and rejects pct<0/>100 or amount<0
expected: money.PercentOf truncates fractional satang toward zero (12345,50->6172; 1,50->0; 333,33->109) and rejects pct<0/>100 or amount<0
result: pass
source: automated
coverage_id: D4

### 24. [01-03 D5] money.FormatBaht renders integer satang as a thousands-separated ฿ display string, with negative amounts prefixed before the symbol, using integer arithmetic only (zero float identifiers in the package)
expected: money.FormatBaht renders integer satang as a thousands-separated ฿ display string, with negative amounts prefixed before the symbol, using integer arithmetic only (zero float identifiers in the package)
result: pass
source: automated
coverage_id: D5

### 25. [01-03 D6] make lint passes with pkg/clock excluded from the forbidigo time.Now ban and pkg/money containing zero forbidigo violations
expected: make lint passes with pkg/clock excluded from the forbidigo time.Now ban and pkg/money containing zero forbidigo violations
result: pass
source: automated
coverage_id: D6

### 26. [01-04 D1] State change written via transactional outbox in the same Postgres tx; the relay publishes it to <svc>.events with key=aggregate_id and headers event_type/event_id/traceparent
expected: State change written via transactional outbox in the same Postgres tx; the relay publishes it to <svc>.events with key=aggregate_id and headers event_type/event_id/traceparent
result: pass
source: automated
coverage_id: D1

### 27. [01-04 D2] The trace id from the originating HTTP-equivalent span survives the outbox row -> relay -> Kafka record traceparent header, unbroken across the async hop
expected: The trace id from the originating HTTP-equivalent span survives the outbox row -> relay -> Kafka record traceparent header, unbroken across the async hop
result: pass
source: automated
coverage_id: D2

### 28. [01-04 D3] Relay reads at most 100 unpublished rows (FOR UPDATE SKIP LOCKED, ORDER BY id), publishes with acks=all, stops the batch at the first publish failure so per-aggregate ordering is never violated, and increments a publish_errors counter on that path
expected: Relay reads at most 100 unpublished rows (FOR UPDATE SKIP LOCKED, ORDER BY id), publishes with acks=all, stops the batch at the first publish failure so per-aggregate ordering is never violated, and increments a publish_errors counter on that path
result: pass
source: automated
coverage_id: D3

### 29. [01-04 D4] Relay wakes on PollInterval or immediately on Nudge(), sweeps rows published >7 days ago hourly (never touching unpublished rows), and flushes once more before Run returns on shutdown
expected: Relay wakes on PollInterval or immediately on Nudge(), sweeps rows published >7 days ago hourly (never touching unpublished rows), and flushes once more before Run returns on shutdown
result: pass
source: automated
coverage_id: D4

### 30. [01-04 D5] pkg/testenv gives every future service integration suite a Postgres 17 + Redpanda testcontainers helper, reusing deploy/redpanda/topics.sh for topic provisioning; goose applies services/_template/migrations against a fresh per-test database
expected: pkg/testenv gives every future service integration suite a Postgres 17 + Redpanda testcontainers helper, reusing deploy/redpanda/topics.sh for topic provisioning; goose applies services/_template/migrations against a fresh per-test database
result: pass
source: automated
coverage_id: D5

### 31. [01-04 D6] make test, make lint, and both GOWORK=off module builds (services/gateway, pkg) stay green with the new dependencies wired in
expected: make test, make lint, and both GOWORK=off module builds (services/gateway, pkg) stay green with the new dependencies wired in
result: pass
source: automated
coverage_id: D6

### 32. [01-05 D4] Prettier 3.9.9 + eslint-config-prettier formatting wired into lint/format scripts
expected: Prettier 3.9.9 + eslint-config-prettier formatting wired into lint/format scripts
result: pass
source: automated
coverage_id: D4

### 33. [01-05 D5] docker-compose web profile service (node:24-alpine, bind mount, 127.0.0.1:3001)
expected: docker-compose web profile service (node:24-alpine, bind mount, 127.0.0.1:3001)
result: pass
source: automated
coverage_id: D5

### 34. [01-06 D1] Jenkins controller + SSH build agent (docker.sock mounted) come up healthy from `make ci-up`, JCasC applies with no anonymous access
expected: Jenkins controller + SSH build agent (docker.sock mounted) come up healthy from `make ci-up`, JCasC applies with no anonymous access
result: pass
source: automated
coverage_id: D1

### 35. [01-06 D2] A commit on the current branch is picked up by the boat-booking multibranch job and Jenkinsfile runs make dev-tools/lint/test/test-integration/images to a green SUCCESS build, with the agent able to run testcontainers via host.docker.internal
expected: A commit on the current branch is picked up by the boat-booking multibranch job and Jenkinsfile runs make dev-tools/lint/test/test-integration/images to a green SUCCESS build, with the agent able to run testcontainers via host.docker.internal
result: pass
source: automated
coverage_id: D2

### 36. [01-06 D3] Harbor v2.15.2 runs from the same compose file with a private `boatbooking` project; make push tags/pushes images there and the Jenkinsfile Push stage only runs on main
expected: Harbor v2.15.2 runs from the same compose file with a private `boatbooking` project; make push tags/pushes images there and the Jenkinsfile Push stage only runs on main
result: pass
source: automated
coverage_id: D3

### 37. [01-07 D1] Consumer.Handle(eventType, fn) applies fn exactly once per unique event_id — a duplicate delivery of the same event_id is a no-op — inside one Postgres tx with processed_events, and manual offset commit happens only after that tx commits
expected: Consumer.Handle(eventType, fn) applies fn exactly once per unique event_id — a duplicate delivery of the same event_id is a no-op — inside one Postgres tx with processed_events, and manual offset commit happens only after that tx commits
result: pass
source: automated
coverage_id: D1

### 38. [01-07 D2] The consumer's process span continues the originating producer span's trace id across the Kafka hop (kotel WithProcessSpan, D-06)
expected: The consumer's process span continues the originating producer span's trace id across the Kafka hop (kotel WithProcessSpan, D-06)
result: pass
source: automated
coverage_id: D2

### 39. [01-07 D3] A handler that fails is retried in-process 3 times (1s/5s/25s default backoff, shortened in tests) then the original envelope is produced to <topic>.dlq with error/consumer_group/attempts/failed_at/source_topic/source_partition/source_offset headers, an ERROR log line, and a dlq counter increment; the source offset commits so the partition is not blocked
expected: A handler that fails is retried in-process 3 times (1s/5s/25s default backoff, shortened in tests) then the original envelope is produced to <topic>.dlq with error/consumer_group/attempts/failed_at/source_topic/source_partition/source_offset headers, an ERROR log line, and a dlq counter increment; the source offset commits so the partition is not blocked
result: pass
source: automated
coverage_id: D3

### 40. [01-07 D4] If the consumer stops before committing an offset (mid-backoff or otherwise), the record is redelivered to the next group member and applied exactly once
expected: If the consumer stops before committing an offset (mid-backoff or otherwise), the record is redelivered to the next group member and applied exactly once
result: pass
source: automated
coverage_id: D4

### 41. [01-07 D6] make test-integration and make lint stay green across the whole repo with the new consumer wired in
expected: make test-integration and make lint stay green across the whole repo with the new consumer wired in
result: pass
source: automated
coverage_id: D6

### 42. [01-08 D1] Synthetic OTLP span sent to the Collector is retrievable from Tempo through Grafana's Tempo datasource proxy by trace id (make up-infra, make obs-check traces)
expected: Synthetic OTLP span sent to the Collector is retrievable from Tempo through Grafana's Tempo datasource proxy by trace id (make up-infra, make obs-check traces)
result: pass
source: automated
coverage_id: D1

### 43. [01-08 D2] Logs to Loki and metrics to Prometheus over OTLP, trace<->log derived-field links, platform.json dashboard provisioned with per-service HTTP/consumer-lag/outbox/DLQ panels
expected: Logs to Loki and metrics to Prometheus over OTLP, trace<->log derived-field links, platform.json dashboard provisioned with per-service HTTP/consumer-lag/outbox/DLQ panels
result: pass
source: automated
coverage_id: D2

### 44. [01-09 D1] services/_template boots HTTP+relay+consumer as errgroup goroutines, reports /healthz 200 and /readyz 200 {db,kafka} once dependencies answer, and shuts down cleanly within 15s of SIGTERM/ctx-cancel in the D-40 order (readyz 503 -> stop HTTP -> flush relay after HTTP drains -> close Kafka+DB -> OTel shutdown)
expected: services/_template boots HTTP+relay+consumer as errgroup goroutines, reports /healthz 200 and /readyz 200 {db,kafka} once dependencies answer, and shuts down cleanly within 15s of SIGTERM/ctx-cancel in the D-40 order (readyz 503 -> stop HTTP -> flush relay after HTTP drains -> close Kafka+DB -> OTel shutdown)
result: pass
source: automated
coverage_id: D1

### 45. [01-09 D2] Every non-health route sits behind httpx.RequireInternal; POST /v1/pings without X-Internal-Token, or with the token but no claim headers, returns 401 — operator id is taken from claims, never the request body
expected: Every non-health route sits behind httpx.RequireInternal; POST /v1/pings without X-Internal-Token, or with the token but no claim headers, returns 401 — operator id is taken from claims, never the request body
result: pass
source: automated
coverage_id: D2

### 46. [01-09 D3] POST /v1/pings writes a pings row and a __NAME__.PingRecorded outbox row in one tx (empty payload, ids only — D-45); the service's own consumer acks it exactly once via processed_events, and replaying the same envelope leaves processed_events at exactly one row for that event_id
expected: POST /v1/pings writes a pings row and a __NAME__.PingRecorded outbox row in one tx (empty payload, ids only — D-45); the service's own consumer acks it exactly once via processed_events, and replaying the same envelope leaves processed_events at exactly one row for that event_id
result: pass
source: automated
coverage_id: D3

### 47. [01-09 D4] Note validation: empty note and a note over 280 characters both return 400 invalid_argument (domain.ValidateNote, also enforced by the pings table's check constraint)
expected: Note validation: empty note and a note over 280 characters both return 400 invalid_argument (domain.ValidateNote, also enforced by the pings table's check constraint)
result: pass
source: automated
coverage_id: D4

### 48. [01-09 D5] Exported spans carry only allowlisted attributes (D-45) — client.address, user_agent.original, url.query are dropped before export, http.route survives
expected: Exported spans carry only allowlisted attributes (D-45) — client.address, user_agent.original, url.query are dropped before export, http.route survives
result: pass
source: automated
coverage_id: D5

### 49. [01-09 D6] Logs are slog JSON on stdout with service/env (+trace_id/span_id when a span is active), fanned out through the otelslog bridge; LOG_FORMAT=text switches stdout to text; /readyz caches results for 1s and times out each check at 2s; SetShuttingDown forces 503
expected: Logs are slog JSON on stdout with service/env (+trace_id/span_id when a span is active), fanned out through the otelslog bridge; LOG_FORMAT=text switches stdout to text; /readyz caches results for 1s and times out each check at 2s; SetShuttingDown forces 503
result: pass
source: automated
coverage_id: D6

### 50. [01-09 D7] With OTEL_EXPORTER_OTLP_ENDPOINT unset the service still runs (propagator set, no exporters constructed)
expected: With OTEL_EXPORTER_OTLP_ENDPOINT unset the service still runs (propagator set, no exporters constructed)
result: pass
source: automated
coverage_id: D7

### 51. [01-09 D8] make sqlc-gen (pinned sqlc/sqlc:1.31.1) produces committed code with no diff on regeneration; make test, make test-integration, and make lint stay green repo-wide with the template wired into go.work
expected: make sqlc-gen (pinned sqlc/sqlc:1.31.1) produces committed code with no diff on regeneration; make test, make test-integration, and make lint stay green repo-wide with the template wired into go.work
result: pass
source: automated
coverage_id: D8

### 52. [01-10 D1] make new-service name=<svc> scaffolds a working service from services/_template: renames every __NAME__ token, wires go.work + deploy/services.txt, and the result builds and its own template-inherited tests run
expected: make new-service name=<svc> scaffolds a working service from services/_template: renames every __NAME__ token, wires go.work + deploy/services.txt, and the result builds and its own template-inherited tests run
result: pass
source: automated
coverage_id: D1

### 53. [01-10 D2] make new-service guards reject a missing name, a name not matching ^[a-z][a-z0-9]*$, and an existing service name, each exiting non-zero with the filesystem left untouched (git status --porcelain unchanged)
expected: make new-service guards reject a missing name, a name not matching ^[a-z][a-z0-9]*$, and an existing service name, each exiting non-zero with the filesystem left untouched (git status --porcelain unchanged)
result: pass
source: automated
coverage_id: D2

### 54. [01-10 D3] make template-smoke builds a freshly scaffolded service into a container image reporting Docker health status healthy, then removes every trace (container, dir, go.work entry, services.txt line)
expected: make template-smoke builds a freshly scaffolded service into a container image reporting Docker health status healthy, then removes every trace (container, dir, go.work entry, services.txt line)
result: pass
source: automated
coverage_id: D3

### 55. [01-10 D4] CatalogService.UpsertBoat (connect-go) writes the boats row and a catalog.BoatUpserted outbox row in one tx, operator_id from trusted claims only; the relay publishes it to catalog.events keyed by boat_id
expected: CatalogService.UpsertBoat (connect-go) writes the boats row and a catalog.BoatUpserted outbox row in one tx, operator_id from trusted claims only; the relay publishes it to catalog.events keyed by boat_id
result: pass
source: automated
coverage_id: D4

### 56. [01-10 D5] UpsertBoat rejects missing claims (Unauthenticated) and invalid name/capacity/status (InvalidArgument); reusing an existing boat_id under a different operator returns NotFound and leaves the stored boat unchanged
expected: UpsertBoat rejects missing claims (Unauthenticated) and invalid name/capacity/status (InvalidArgument); reusing an existing boat_id under a different operator returns NotFound and leaves the stored boat unchanged
result: pass
source: automated
coverage_id: D5

### 57. [01-10 D6] ListBoats returns every boat ordered by name then id (stable order) and needs only the internal token, no claims
expected: ListBoats returns every boat ordered by name then id (stable order) and needs only the internal token, no claims
result: pass
source: automated
coverage_id: D6

### 58. [01-10 D7] make test and make lint stay green repo-wide with catalog wired into go.work, including the standalone (GOWORK=off) build every module needs for the root Dockerfile
expected: make test and make lint stay green repo-wide with catalog wired into go.work, including the standalone (GOWORK=off) build every module needs for the root Dockerfile
result: pass
source: automated
coverage_id: D7

### 59. [01-11 D1] schedule scaffolded via make new-service and boots/shuts down cleanly through the same template-inherited health/readiness/shutdown lifecycle as every other service
expected: schedule scaffolded via make new-service and boots/shuts down cleanly through the same template-inherited health/readiness/shutdown lifecycle as every other service
result: pass
source: automated
coverage_id: D1

### 60. [01-11 D2] schedule consumes catalog.BoatUpserted from catalog.events and upserts its own boats projection exactly once; a duplicate delivery of the same event_id is a no-op and a later event for the same boat overwrites the projection
expected: schedule consumes catalog.BoatUpserted from catalog.events and upserts its own boats projection exactly once; a duplicate delivery of the same event_id is a no-op and a later event for the same boat overwrites the projection
result: pass
source: automated
coverage_id: D2

### 61. [01-11 D4] GET /api/v1/public/boats calls catalog ListBoats with only X-Internal-Token and returns protojson {\\"boats\\": [...]} (empty list rendered as [], not null)
expected: GET /api/v1/public/boats calls catalog ListBoats with only X-Internal-Token and returns protojson {\\"boats\\": [...]} (empty list rendered as [], not null)
result: pass
source: automated
coverage_id: D4

### 62. [01-11 D5] POST /api/v1/boats verifies the access_token cookie, strips any client-supplied X-User-Id/X-Operator-Id/X-Role/X-Internal-Token and sets them from verified claims + the internal token, and calls catalog UpsertBoat over connect-go; 201 on success, 401 without a valid access token (missing/expired/refresh-kind), and catalog's CodeInvalidArgument maps to HTTP 400 {\\"code\\":\\"invalid_argument\\"}
expected: POST /api/v1/boats verifies the access_token cookie, strips any client-supplied X-User-Id/X-Operator-Id/X-Role/X-Internal-Token and sets them from verified claims + the internal token, and calls catalog UpsertBoat over connect-go; 201 on success, 401 without a valid access token (missing/expired/refresh-kind), and catalog's CodeInvalidArgument maps to HTTP 400 {\\"code\\":\\"invalid_argument\\"}
result: pass
source: automated
coverage_id: D5

### 63. [01-11 D6] make test, make test-integration, and make lint all stay green repo-wide with schedule and the re-scaffolded gateway in the workspace
expected: make test, make test-integration, and make lint all stay green repo-wide with schedule and the re-scaffolded gateway in the workspace
result: pass
source: automated
coverage_id: D6

### 64. [01-12 D1] make up starts Postgres 17, Redpanda (auto-create off), Valkey, the observability stack, migrate-<svc> one-shots, catalog, schedule, gateway, Kong and the web profile; every long-running service reports healthy and no Go service publishes a host port
expected: make up starts Postgres 17, Redpanda (auto-create off), Valkey, the observability stack, migrate-<svc> one-shots, catalog, schedule, gateway, Kong and the web profile; every long-running service reports healthy and no Go service publishes a host port
result: pass
source: automated
coverage_id: D1

### 65. [01-12 D2] make proof: POST /api/v1/boats through Kong with a dev token creates a boat in catalog, schedule's own database holds it with the same capacity within 30s, the catalog outbox event_id appears exactly once in schedule.processed_events, and GET /api/v1/public/boats lists it
expected: make proof: POST /api/v1/boats through Kong with a dev token creates a boat in catalog, schedule's own database holds it with the same capacity within 30s, the catalog outbox event_id appears exactly once in schedule.processed_events, and GET /api/v1/public/boats lists it
result: pass
source: automated
coverage_id: D2

### 66. [01-12 D3] make proof finds one Tempo trace (by span attribute aggregate_id = boat id) whose spans come from gateway, catalog and schedule, and Loki holds a schedule log line carrying that trace_id
expected: make proof finds one Tempo trace (by span attribute aggregate_id = boat id) whose spans come from gateway, catalog and schedule, and Loki holds a schedule log line carrying that trace_id
result: pass
source: automated
coverage_id: D3

### 67. [01-12 D4] make kong-roundtrip additionally proves the catalog hop: public boats 200 without a token, POST /api/v1/boats 201 with a valid token
expected: make kong-roundtrip additionally proves the catalog hop: public boats 200 without a token, POST /api/v1/boats 201 with a valid token
result: pass
source: automated
coverage_id: D4

### 68. [01-12 D5] deploy/postgres/init.sh enforces database-per-service at the Postgres level (PUBLIC CONNECT revoked, only the owning role granted); isolation-check.sh proves every cross-service CONNECT is denied and every own-database CONNECT succeeds
expected: deploy/postgres/init.sh enforces database-per-service at the Postgres level (PUBLIC CONNECT revoked, only the owning role granted); isolation-check.sh proves every cross-service CONNECT is denied and every own-database CONNECT succeeds
result: pass
source: automated
coverage_id: D5

### 69. [01-12 D6] Developer inner loop works from the host against up-infra: make migrate-catalog (idempotent goose up) and make run-catalog (go run, hosts overridden to localhost) both succeed, catalog reports /readyz 200
expected: Developer inner loop works from the host against up-infra: make migrate-catalog (idempotent goose up) and make run-catalog (go run, hosts overridden to localhost) both succeed, catalog reports /readyz 200
result: pass
source: automated
coverage_id: D6

### 70. [01-12 D7] pkg/testenv.TestImagesMatchCompose keeps the testcontainers image tags and deploy/docker-compose.yml's tags in lockstep, no Docker required
expected: pkg/testenv.TestImagesMatchCompose keeps the testcontainers image tags and deploy/docker-compose.yml's tags in lockstep, no Docker required
result: pass
source: automated
coverage_id: D7

### 71. [01-12 D8] make test, make test-integration, and make lint stay green repo-wide, including the pkg/kafka logging fix's effect on every existing consumer test
expected: make test, make test-integration, and make lint stay green repo-wide, including the pkg/kafka logging fix's effect on every existing consumer test
result: pass
source: automated
coverage_id: D8

### 72. [01-13 D1] deploy/ci/changed-services.sh selects services by exact path segment (services/catalogx/ never selects catalog) and treats pkg/proto/gen/_template/go.work(.sum)/Makefile/Dockerfile changes as select-all; docs-only and web-only changes select nothing
expected: deploy/ci/changed-services.sh selects services by exact path segment (services/catalogx/ never selects catalog) and treats pkg/proto/gen/_template/go.work(.sum)/Makefile/Dockerfile changes as select-all; docs-only and web-only changes select nothing
result: pass
source: automated
coverage_id: D1

### 73. [01-13 D2] make ci reproduces the full Jenkins pipeline locally in order (lint incl. web eslint+tsc, proto-check, test, test-integration, migrate-validate, template-smoke, web-check, images), never skippable on test-integration
expected: make ci reproduces the full Jenkins pipeline locally in order (lint incl. web eslint+tsc, proto-check, test, test-integration, migrate-validate, template-smoke, web-check, images), never skippable on test-integration
result: pass
source: automated
coverage_id: D2

### 74. [01-13 D3] Final Jenkinsfile stages mirror make ci exactly; only the Push stage is conditional and gated to branch 'main'
expected: Final Jenkinsfile stages mirror make ci exactly; only the Push stage is conditional and gated to branch 'main'
result: pass
source: automated
coverage_id: D3

### 75. [01-13 D4] A real Jenkins build of the current branch's HEAD goes green (Push correctly skipped, not on main) and deploy/ci/smoke.sh confirms the console shows both make test-integration and PASS template-smoke actually ran
expected: A real Jenkins build of the current branch's HEAD goes green (Push correctly skipped, not on main) and deploy/ci/smoke.sh confirms the console shows both make test-integration and PASS template-smoke actually ran
result: pass
source: automated
coverage_id: D4

## Summary

total: 75
passed: 75
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-01-7
  truth: "หลัง `make proof` เปิด http://localhost:3000 -> Explore -> Tempo ค้น { span.aggregate_id = \"<boat id จาก make proof>\" }: trace เดียวมี gateway HTTP -> catalog connect -> Kafka publish -> schedule consume span, trace id เดียวกัน ไม่ขาดตรง Kafka; 'Logs for this span' กระโดดไป Loki ได้; dashboard 'platform' มี panel HTTP rate / consumer lag / processed-events / outbox backlog ที่มีข้อมูลจริง"
  status: resolved
  resolved_by: 01-14-PLAN.md
  resolved_at: 2026-09-26
  reason: "User reported: กด Logs for this span กระโดดไป Loki ได้ แต่ไม่มีข้อมูลอะไรเลย แบบนี้ถูกแล้วไหม แต่กดจาก gateway มีข้อมูล dashboard 'platform' กดได้ มีข้อมูล HTTP rate Processed events แต่ consumer lag Outbox backlog Outbox oldest unpublished age ว่างเปล่า"
  severity: minor
  test: 7
  root_cause: "Grafana Tempo datasource tracesToLogsV2 sets no spanStartTimeShift/spanEndTimeShift, so 'Logs for this span' queries Loki over exactly the clicked span's [start,end]. Service request logs are emitted at the end of the outer otelhttp server span (pkg/httpx/middleware.go requestLog), which falls outside inner child spans: trace 3feeb126 catalog log at ..934.32ms vs catalog UpsertBoat connect span ending ..933.82ms and catalog.events publish span starting ..934.92ms -> empty result. Logs are present in Loki with the right trace_id (verified by direct Loki query for gateway, catalog, schedule). Dashboard part is not a defect: consumer-lag/outbox-backlog/oldest-age series exist and are flat 0 on an idle healthy system."
  artifacts:
    - path: "deploy/observability/grafana/provisioning/datasources/datasources.yaml"
      issue: "tracesToLogsV2 missing spanStartTimeShift/spanEndTimeShift — log window equals span duration"
  missing:
    - "Add spanStartTimeShift: '-1m' and spanEndTimeShift: '1m' under tracesToLogsV2 (filterByTraceID: true keeps results scoped to the trace)"
  debug_session: "inline — diagnosed during UAT session (Loki + Tempo API queries on trace 3feeb12697817011487fd893ff324dd3)"
