---
gsd_state_version: "1.0"
current_phase: 02
current_phase_name: Identity + Catalog
status: executing
stopped_at: Completed quick-261003-k4f (Phase 02 UI-REVIEW priority fixes)
last_updated: "2026-10-03T07:40:15.541Z"
last_activity: 2026-09-28
last_activity_desc: Phase 02 execution started
state_head: 92eb70a92e2d7d7744067b1258fc6950ded4507c
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 31
  completed_plans: 31
  percent: 20
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-25)

**Core value:** ลูกค้าจองจากมือถือ → จ่าย → โชว์ QR ที่ท่าได้ โดยระบบไม่ overbook เด็ดขาด
**Current focus:** Phase 02 — Identity + Catalog

## Current Position

Phase: 02 (Identity + Catalog) — EXECUTING
Plan: 5 of 17
Status: Ready to execute
Last activity: 2026-10-03 - Completed quick task 261003-k4f: Fix Phase 02 UI-REVIEW priority issues in apps/admin

Progress: [██░░░░░░░░] 20%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: - min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01 | 40min | 3 tasks | 23 files |
| Phase 01 P02 | 18min | 2 tasks | 20 files |
| Phase 01 P03 | 8min | 2 tasks | 4 files |
| Phase 01 P04 | 45min | 2 tasks | 9 files |
| Phase 01 P06 | 71min | 2 tasks | 14 files |
| Phase 01 P05 | 55min | 3 tasks | 21 files |
| Phase 01 P07 | 24min | 2 tasks | 5 files |
| Phase 01 P08 | 32min | 2 tasks | 11 files |
| Phase 01 P09 | 45min | 2 tasks | 28 files |
| Phase 01 P10 | 27min | 2 tasks | 28 files |
| Phase 01 P11 | ~30min | 2 tasks | 25 files |
| Phase 01 P12 | 32min | 2 tasks | 12 files |
| Phase 01 P13 | 25min | 2 tasks | 5 files |
| Phase 01 P14 | 5min | 1 tasks | 1 files |
| Phase 02 P01 | 11min | 3 tasks | 9 files |
| Phase 02 P02 | 35min | 3 tasks | 42 files |
| Phase 02 P03 | 23min | 2 tasks | 32 files |
| Phase 02 P04 | 21min | 2 tasks | 10 files |
| Phase 02 P05 | 30min | 3 tasks | 27 files |
| Phase 02 P06 | 33min | 3 tasks | 23 files |
| Phase 02 P07 | 20min | 2 tasks | 15 files |
| Phase 02 P08 | 48min | 2 tasks | 32 files |
| Phase 02 P09 | 35min | 3 tasks | 51 files |
| Phase 02 P10 | 7min | 2 tasks | 12 files |
| Phase 02 P11 | 33min | 3 tasks | 13 files |
| Phase 02 P12 | 17min | 3 tasks | 9 files |
| Phase 02-identity-catalog P13 | 18min | 2 tasks | 3 files |
| Phase 02 P14 | 12min | 1 tasks | 2 files |
| Phase 02 P15 | 18 min | 2 tasks | 3 files |
| Phase 02 P16 | ~20min | 2 tasks | 2 files |
| Phase 02 P17 | 10 min | 3 tasks | 4 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Roadmap: Milestone 1 = Phases 1-5, dependency-forced chain (Foundation → Identity+Catalog → Schedule → Booking Core → Payment+Ticket+Notification), matches seed Phase 0-4 and research SUMMARY.md
- Payment provider (Opn vs 2C2P) deferred to a sandbox spike at Phase 5 planning
- Kong 3.9.1 DB-less JWT sufficiency deferred to a spike at Phase 1 planning (Traefik fallback if it fails)
- [Phase 01]: Kong 3.9.1 DB-less passed the D-28 spike on the first attempt (all 8 roundtrip checks) — no Traefik fallback needed
- [Phase 01]: buf toolchain: pinned protocolbuffers/go v1.36.12, connectrpc/go v1.18.1, bufbuild/es v2.15.0 remote plugins; gen/go and gen/ts committed — Exact tags verified against proxy.golang.org rather than trusting illustrative versions from planning
- [Phase 01]: pkg/clock/pkg/money TDD RED phase used a genuine Go build failure (undefined symbols) for a brand-new package, not a compiling-but-wrong stub — Idiomatic Go TDD for greenfield packages; confirmed intentional (target symbols only, no unrelated errors) before GREEN
- [Phase 01]: franz-go pinned to v1.21.7 and goose to v3.27.3 instead of the plan's illustrative v1.22.0/v3.28.0 (both require go1.26.0, breaking the Go 1.25.x pin).
- [Phase 01]: deploy/redpanda/topics.sh default RPK_BROKERS changed to 127.0.0.1:9093 (Redpanda's internal listener) instead of localhost:9092 — the external listener advertises the host-mapped port, unreachable from inside the same container.
- [Phase 01]: JCasC config kept outside JENKINS_HOME (/usr/local/jenkins-casc.yaml) to survive image rebuilds; agent docker-group membership fixed in ENTRYPOINT against the live docker.sock GID since compose group_add doesn't survive sshd's PAM user switch (Pitfall 12)
- [Phase 01]: Rejected unaudited 'cn' npm package; replaced with hand-written clsx+tailwind-merge cn() helper — shadcn init/add commands template components to import a separate 'cn' package not covered by the legitimacy audit; developer rejected it at the blocking-human checkpoint
- [Phase 01]: Approved radix-ui, pinned exact at 1.6.7 — Legitimate shadcn dependency with strong download/repo signals; pinned exact to match Task 2's --save-exact convention
- [Phase 01]: kgo BlockRebalanceOnPoll requires AllowRebalance() on every PollFetches iteration, including the ctx-cancelled fake-fetch path — Skipping AllowRebalance on early ctx-done return left the poller count non-zero, deadlocking Client.Close()'s graceful group-leave forever - found and fixed before the first commit
- [Phase 01]: Loki compactor.delete_request_store must be set whenever limits_config.retention_period is non-zero, even though the plan text didn't call it out — added delete_request_store: filesystem
- [Phase 01]: Grafana's Loki derived field keeps the literal double-dollar '$${__value.raw}' — Grafana's provisioning-file env-var expansion would otherwise consume a single $ before Loki's own derived-field macro sees it
- [Phase 01]: [Phase 01] golang.org/x/sync pinned to v0.22.0 (not the plan's v0.23.0) — v0.23.0 requires go1.26, breaking the repo's go1.25.x toolchain pin
- [Phase 01]: [Phase 01] sqlc's pgx/v5 codegen maps postgres uuid columns to pgtype.UUID; internal/app owns small toPgUUID/fromPgUUID converters so pgx-specific types never leak into domain/app signatures
- [Phase 01]: [Phase 01] otelconnect pinned to v0.10.0; GOWORK=off go mod tidy re-run for pkg/services/_template/services/catalog/services/gateway after pkg gained an otelconnect dependency
- [Phase 01]: [Phase 01] Kafka-consume proof for TestUpsertBoatPublishesBoatUpserted reuses pkg/kafka.Consumer (fresh consumer group reads from earliest offset by default) instead of new raw-consumer test infra
- [Phase 01]: gateway re-scaffolded onto the exact template run(ctx) shape (no DB special-casing needed) with routes mounted directly, not behind httpx.RequireInternal — gateway is the origin of the internal-token trust boundary, not a consumer of it (D-29, D-30)
- [Phase 01]: goose pinned to v3.27.3, not v3.28.0 (go1.26.0 minimum breaks the repo's go1.25.x pin)
- [Phase 01]: docker compose up --wait replaced with a Makefile wait_ready loop (--wait cannot express a by-design exited-0 one-shot container as success)
- [Phase 01]: pkg/kafka.Consumer log calls switched to ctx-aware slog *Context methods so trace_id reaches Loki (PLAT-06 proof requirement)
- [Phase 01]: LC_ALL=C pinned on every sort in changed-services.sh for cross-locale determinism between dev host and CI agent
- [Phase 01]: smoke.sh fixed for two Rule 1 bugs found during Task 2 verification: pipefail killing the poll loop on any in-progress build, and a Jenkins result-vs-console-flush race in the new test-integration/template-smoke console assertion
- [Phase 01]: [Phase 01] Widened Tempo's tracesToLogsV2 window by spanStartTimeShift: '-1m' / spanEndTimeShift: '1m' (D-51) instead of moving request-log emission, closing UAT gap G-01-7 while filterByTraceID keeps results scoped to one trace
- [Phase 02]: SeaweedFS chrislusf/seaweedfs:4.47 chosen for dev object storage (D-19 detail) — MinIO unpullable from Docker Hub (404, archived) — Task 1 checkpoint:decision resolved before dispatch
- [Phase 02]: go-redis v9.22.0, minio-go v7.3.0, maplibre-gl 6.11.2 approved by developer at Task 2 package-legitimacy checkpoint — Task 2 checkpoint:human-verify (gate=blocking-human) resolved before dispatch
- [Phase 02]: ForwardClaims moved from gateway package into pkg/httpx — single implementation shared by every future internal caller — Task 3 GREEN phase implementation choice
- [Phase 02]: [Phase 02] identity's OTP hashing (HMAC-SHA256 pepper for both destination and code) implemented exactly as the plan's Context block specified — no deviation
- [Phase 02]: [Phase 02] services/identity/go.mod needed its own standalone GOWORK=off go mod tidy beyond a workspace-mode go get — the Dockerfile builds each service as an isolated module, which needs every transitive dep's go.sum entry (golang-jwt/jwt/v5 via pkg/auth) that workspace-mode go get does not populate
- [Phase 02]: app.Scope + http.scopeFrom/toConnectErr established as the one operator/pier scoping rule (Pitfall 6) — routes/boats/prices reuse it, not reimplement it
- [Phase 02]: UpsertOperator kept boat.go's single ON CONFLICT upsert shape; UpsertPier uses an explicit create/update branch since pier updates need GetPierForUpdateScoped's (operator_id, pier_ids) filter
- [Phase 02]: Routes takes one shared *http.Client instead of a typed catalog client + separate transport param
- [Phase 02]: adminProxy buffers the bounded request body upfront (io.ReadAll behind MaxBytesReader) instead of streaming through httputil.ReverseProxy, guaranteeing the upstream is never contacted for an oversized body
- [Phase 02]: [Phase 02] Kong global cors plugin gained http://localhost:3002 directly (not a second route plugin) for the admin app origin (D-18)
- [Phase 02]: [Phase 02] identity EnsureSuperAdmin promote-existing-row path publishes no identity.UserCreated -- only a fresh insert announces the identity; promotion's next refresh re-reads the new role via D-10
- [Phase 02]: All Task 1-3 proto (route/price events + catalog.proto RPCs) generated in one buf generate pass during Task 1 to keep gen/ internally consistent, since price.proto's TicketType is imported by catalog.proto's Route.current_prices field added in Task 2. — Avoids a partial/inconsistent intermediate proto generation; each task's commit still adds only that task's Go implementation.
- [Phase 02]: [Phase 02] identity's UserService validates pier ownership via a synchronous catalog.ListPiers call (forwarding caller claims + internal token) instead of a foreign key -- database-per-service forbids cross-service FKs; ForwardClaims reused for service-to-service calls, not just gateway-to-service
- [Phase 02]: [Phase 02] Boats reuse routes' exact GetPierForShareScoped/GetBoatForUpdateScoped scope pattern (D-07) instead of a boat-specific variant -- one scoping shape applied consistently across entities
- [Phase 02]: [Phase 02] app.Photos.PresignPierPhoto uses minio-go's PresignHeader (not PresignedPutObject) since only PresignHeader can sign extra headers -- required to bind Content-Type and Content-Length into the URL signature (T-02-08-02)
- [Phase 02]: [Phase 02] catalog's newPhotos() treats an unset S3_PUBLIC_ENDPOINT as optional dev infrastructure (nil Client, no startup error) rather than a required config -- PresignPierPhoto returns FailedPrecondition instead, matching the existing outbox-relay/consumer optionality pattern
- [Phase 02]: [Phase 02] shadcn add pulled every generated ui/*.tsx import from an unaudited 'cn' npm package (same issue Phase 1 rejected) -- rewrote every import to @/lib/utils and dropped 'cn' from package.json/lockfile before writing any app code
- [Phase 02]: [Phase 02] operator-dialog.tsx skips a separate GetOperator fetch/skeleton for edit mode since the row is already in memory from the ListOperators query backing the table
- [Phase 02]: [Phase 02] AdminShell's "collapsible to top bar on narrow screens" is a Tailwind hidden md:flex / md:hidden sidebar-vs-top-nav pair, not a Sheet-based hamburger drawer
- [Phase 02]: [Phase 02] apps/web otp-login.tsx and api.ts mirror apps/admin's exact refresh-retry/parsed-error and OTP error-mapping shapes rather than a fresh design — Consistent auth UX across both frontends, smallest diff, reuses code already proven in 02-09
- [Phase 02]: [Phase 02] apps/web's shadcn add pulled the same unaudited 'cn' npm package 02-09 already rejected -- rewrote every generated ui/*.tsx import to @/lib/utils and dropped 'cn' from package.json/lockfile before writing app code — Matches the established mitigation from 02-09; keeps the dependency tree auditable
- [Phase 02]: [Phase 02] storage-init retries s3.configure up to 15x/2s instead of trusting storage's healthcheck alone -- the master HTTP healthcheck can pass before the filer's gRPC listener (which weed shell dials) is ready under load; a cold make up hit this race
- [Phase 02]: [Phase 02] maplibre-gl v6 has no default export -- import { Map as MapLibreMap, Marker } from 'maplibre-gl', not the v1-v3 default `maplibregl` import
- [Phase 02]: RouteSheet(returnOf=route) recursive self-render for the D-11 return-route shortcut instead of a second component — One form definition; guarded from infinite nesting since a returnOf Sheet has isEdit=false
- [Phase 02]: money.ts uses only regex+BigInt string arithmetic (no parseFloat/Number on the amount, zero imports) — Mirrors pkg/money's Go convention and the 10,000,000 satang server bound; compiles/tests standalone
- [Phase 02]: Boat status badges use explicit green/orange Tailwind classes instead of the Badge default/secondary variants — UI-SPEC reserves the brand accent color for CTAs, never status badges
- [Phase 02]: Hid edit action for super_admin rows in staff page, not just disable, since UserService rejects any update to a super_admin row — Prevents a guaranteed-fail edit action; server-side guard already existed in 02-07
- [Phase 02]: [Phase 02] otpMessage extracted as the single RFC 5322 builder for SMTP-delivered OTP messages, adding MIME-Version/Content-Type/Content-Transfer-Encoding headers and an RFC 2047-encoded Subject — Closes gap G-02-3: Mailpit decoded the undeclared-charset UTF-8 Thai body as Latin-1; 8bit encoding chosen since this path only talks to Mailpit in dev, ResendSender (prod, JSON) is unaffected
- [Phase 02]: [Phase 02] createPier's archived-operator rejection wraps domain.ErrFailedPrecondition with "operator is archived" (%w) -- toConnectErr's errors.Is switch is unaffected, wire code unchanged — Closes G-02-8 backend half: the bare sentinel gave no reason on the wire
- [Phase 02]: [Phase 02] TestConcurrentUpsertBoatSameID reuses boats_photo_integration_test.go's existing helpers, no new helper file — Closes G-02-18: proves the existing FOR UPDATE lock (D-07) serializes 10 concurrent UpsertBoat writes with no torn write
- [Phase 02]: type=number lat/lng inputs switched to text+inputMode=decimal to stop partial values silently becoming 0/out-of-range and crashing maplibre-gl — DOM reports a partial number input like '13.' as empty string, which Number() coerces to 0 (finite, passes old guard); a text draft parsed via parseCoord fixes both the crash and the marker jump
- [Phase 02]: Archived-operator save error mapped by ApiError.code === failed_precondition, not message text — Keeps this plan independent of 02-15's backend copy per the plan's explicit non-dependency note
- [Phase 02]: pierName's duplicate check flattens the given pier lists once and compares nameTh across all entries except the id being rendered, rather than adding a separate duplicate-count pass — One loop, same signature every caller already uses
- [Phase 02]: [quick-261003-k4f]: Fixed 3 Phase 02 UI-REVIEW priority issues in apps/admin — semantic green status badges, touched-gated required-field errors, Thai not-found copy replacing raw UUID fallbacks

### Pending Todos

None yet.

### Blockers/Concerns

- Phase 1 is highest-leverage and highest-risk: shared `pkg/*` template is copied into every later service — a mistake here compounds across all 7 services (per research/SUMMARY.md)
- Phase 5 combines payment + ticket + notification + deploy intentionally (no useful partial-completion state for the saga) — largest phase by requirement count (15)

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 261003-k4f | Fix Phase 02 UI-REVIEW priority issues in apps/admin | 2026-10-03 | 92eb70a | [261003-k4f-fix-phase-02-ui-review-priority-issues-i](./quick/261003-k4f-fix-phase-02-ui-review-priority-issues-i/) |

## Deferred Items

Items acknowledged and deferred at milestone close, most recent first:

| Category | Item | Status | Deferred At | Milestone |
|----------|------|--------|-------------|-----------|
| *(none)* | | | | |

## Session Continuity

Last session: 2026-10-03T07:40:15.451Z
Stopped at: Completed quick-261003-k4f (Phase 02 UI-REVIEW priority fixes)
Resume file: None
