---
status: testing
phase: 01-platform-foundation
source: [01-VERIFICATION.md]
started: 2026-09-26T08:00:00Z
updated: 2026-09-26T08:00:00Z
---

## Current Test

number: 1
name: Live Jenkins changed-services image scoping (PLAT-08)
expected: |
  Push a commit under pkg/ then a commit only under services/schedule/, and watch two Jenkins multibranch scan cycles.
  First build images all 4 services, second images only schedule; both run every non-image stage; neither pushes off main.
awaiting: user response

## Tests

### 1. Live Jenkins changed-services image scoping (PLAT-08)
expected: Push a commit under pkg/ then a commit only under services/schedule/, and watch two Jenkins multibranch scan cycles. First build images all 4 services, second images only schedule; both run every non-image stage; neither pushes off main.
result: [pending]

### 2. Grafana trace continuity across the Kafka hop + platform dashboard (PLAT-06)
expected: With the stack up after `make proof`, open http://localhost:3000 (Grafana) -> Explore -> Tempo, search `{ span.aggregate_id = "<boat id printed by make proof>" }`, open the trace. One trace shows gateway HTTP span -> connect call to catalog -> Kafka publish span -> schedule consumer span, same trace id, no gap at the Kafka hop; "Logs for this span" jumps to the matching Loki lines; dashboard "platform" shows HTTP rate / consumer lag / processed-events / outbox-backlog panels with real data.
result: [pending]

### 3. Web skeleton on a 375px viewport (PLAT-09)
expected: Open http://localhost:3001 in a 375px-wide viewport, then /en; press refresh; open devtools Network. / redirects to /th; boat cards list the proof boat; Thai renders in IBM Plex Sans Thai (no system-font fallback); /en shows English strings; every XHR goes to localhost:8000 (Kong) not directly to gateway/catalog; refresh refetches.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
