# API Coverage — Phase 02 Identity + Catalog

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

External APIs this phase integrates: Resend REST API (prod OTP email, plan 02-02), an S3-compatible object-storage API (pier photos, plans 02-08 and 02-11), OpenFreeMap vector tiles via MapLibre GL (admin map picker, plan 02-11), and the Mailpit HTTP/SMTP API (dev and test OTP capture only, plans 02-02 and 02-05).

## Resend (https://api.resend.com)

| capability | decision | reason |
|---|---|---|
| emails.send (POST /emails) | INTEGRATE | |
| emails.batch (POST /emails/batch) | OPT-OUT | not needed: one OTP per request, never a batch |
| emails.get (GET /emails/{id}) | OPT-OUT | not needed yet: delivery status tracking is not a v1 requirement |
| emails.update / emails.cancel (scheduled sends) | OPT-OUT | not needed: OTPs are sent immediately, never scheduled |
| domains (create/verify/list/update/delete) | OPT-OUT | done once by a human in the Resend dashboard (user_setup in 02-02), not from code |
| api-keys (create/list/delete) | OPT-OUT | the key is provisioned by a human and injected via env |
| audiences / contacts / broadcasts | OPT-OUT | explicitly out of scope: no marketing email |
| webhooks (delivery events) | OPT-OUT | not needed yet: revisit with ticket emails (NOTF-01, Phase 5) |

## S3-compatible object storage (dev container chosen in 02-01; S3 in prod)

| capability | decision | reason |
|---|---|---|
| presigned PutObject (Content-Type + Content-Length signed) | INTEGRATE | |
| anonymous GetObject on the pier-photos bucket | INTEGRATE | |
| CreateBucket / bucket policy / bucket CORS (dev init) | INTEGRATE | |
| DeleteObject | OPT-OUT | not needed yet: archive is soft delete (D-15); orphaned photo cleanup deferred |
| ListObjects | OPT-OUT | explicitly denied: public bucket must not be listable |
| HeadObject (verify upload before save) | OPT-OUT | not needed yet: keys are server-generated and format-validated; a missing object only yields a broken image |
| multipart upload | OPT-OUT | not needed: photos are at most 5 MB, a single PUT |
| presigned POST policy | OPT-OUT | D-19 specifies presigned PUT |

## OpenFreeMap tiles via MapLibre GL

| capability | decision | reason |
|---|---|---|
| vector style + tiles (NEXT_PUBLIC_MAP_STYLE_URL) | INTEGRATE | |
| geocoding / address search | OPT-OUT | deferred per CONTEXT (Deferred Ideas) |
| routing / directions | OPT-OUT | explicitly out of scope: no navigation feature |
| static map images | OPT-OUT | not needed: the admin map is interactive; customer map is Phase 4 |

## Mailpit (dev and test only)

| capability | decision | reason |
|---|---|---|
| SMTP receive (port 1025) | INTEGRATE | |
| search messages (GET /api/v1/search) | INTEGRATE | |
| get message (GET /api/v1/message/{ID}) | INTEGRATE | |
| delete messages / tags / release | OPT-OUT | not needed: tests search by unique recipient addresses |
