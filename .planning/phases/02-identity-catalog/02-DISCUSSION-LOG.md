# Phase 2: Identity + Catalog - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-26
**Phase:** 02-identity-catalog
**Areas discussed:** OTP delivery, Staff login + roles, Catalog data model, Admin UI

---

## OTP delivery

| Question | Options | Selected |
|---|---|---|
| OTP channels | Email real + SMS interface / Email + SMS real / Email only | Email real + SMS interface |
| Dev OTP visibility | Mailpit / Log stdout / Fixed OTP | Mailpit |
| OTP rules | 6 digits·5 min·5 attempts, Valkey counters / same in Postgres / You decide | 6·5·5 with Valkey |
| Customer user record | Auto-create on verify / Guest JWT without user | Auto-create |

---

## Staff login + roles

| Question | Options | Selected |
|---|---|---|
| Staff auth method | Email OTP / Email + password / OTP + passkey later | Email OTP |
| Pier-level scope | Operator only, pier stored / pier_ids in JWT + scoped | pier_ids in JWT |
| What pier scope covers | Routes by pier_from, boats operator-level / Everything incl. boats | Everything incl. boats |
| Operator/pier creation + bootstrap | super_admin + env seed / pier_admin creates piers | super_admin + env seed |
| Revocation latency | ≤15 min via refresh / Immediate | ≤15 min |

**Notes:** Success criterion 4 wording conflicts with CAT-01 — resolved in favour of super_admin creating operators/piers.

---

## Catalog data model

| Question | Options | Selected |
|---|---|---|
| Route direction | Two one-way routes / Bidirectional flag | Two one-way routes |
| pier_to ownership | Any operator / Same operator only | Any operator |
| Cancellation tiers | Free tier list (jsonb) / 3 fixed columns | Free tier list |
| Price history | Current only / effective_from | effective_from |
| Price date basis | Departure date (Bangkok) / Booking date | Departure date |
| Archive with dependents | Block (FailedPrecondition) / Cascade | Block |
| Open hours | Single daily / Per weekday / Free text | Single daily |
| Bilingual names | name_th + name_en / Single | Both |

---

## Admin UI

| Question | Options | Selected |
|---|---|---|
| Admin location | /[locale]/admin in apps/web / Separate apps/admin | Separate apps/admin |
| Admin stack | Next.js 16 Thai-only / Next.js 16 + next-intl / Vite SPA | Next.js 16 Thai-only |
| Pier photo | Presigned URL → S3 (MinIO dev) / URL field / Volume via catalog | Presigned + MinIO |
| Map lib | Leaflet + OSM / MapLibre GL + vector tiles | MapLibre GL |
| Tile provider | OpenFreeMap / MapTiler / Self-host PMTiles | OpenFreeMap |

---

## Claude's Discretion

- Identity schema details, OTP hashing, Valkey keys, refresh rotation mechanics
- connect-go method shapes, BFF route names, pagination
- Admin layout details (defer to /gsd-ui-phase)
- Shared UI between apps (default copy)
- Photo URL serving strategy

## Deferred Ideas

- Real SMS provider — near launch
- Passkey login — Growth
- Geocoding in map picker
- Per-weekday open hours
- Immediate revocation
- Customer map/search screens — Phase 4
