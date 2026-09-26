# proto/

Single buf module (`buf.yaml`, `version: v2`) for every service's wire
contracts. Layout: `proto/events/<svc>/v1/*.proto` (Kafka payloads) and
`proto/services/<svc>/v1/*.proto` (sync connect-go APIs).

## Workflow

```bash
make proto-gen    # buf generate -> gen/go, gen/ts (committed, not gitignored)
make proto-check  # lint + breaking + drift + PII gate (CI runs this)
```

`make proto-gen` must be run — and its output committed — after every proto
change. `make proto-check`'s drift step fails the build if `gen/` is stale.

## Review checklist (read before adding or changing any `.proto`)

- **Events are past-tense facts.** Name messages `BoatUpserted`,
  `BookingConfirmed` — never a command (`ConfirmBooking`).
- **No personal data in `proto/events/**`.** Event payloads carry ids only
  (`boat_id`, `operator_id`, ...). A consumer that needs an email, phone
  number, or name fetches it via a sync call to the owning service —
  never denormalize PII into an event for convenience. `make proto-check`
  runs `proto/pii-check.sh proto/events` to catch PII-shaped field names
  (`email`, `phone`, `first_name`, `address`, `dob`, ...) mechanically, but
  it is a backstop, not a substitute for reviewing new event fields.
- **Money is `int64` satang.** Never `float`/`double` for any monetary
  amount.
- **Never reuse or delete a field number.** Removing a field? Mark the
  number `reserved` instead of deleting or recycling it — this is the only
  way `buf breaking` can protect old-record decoding.
- **Bump `Envelope.version`** whenever a payload's semantics change in a way
  that changes how a consumer must interpret it (not on every additive
  field — additive fields are backward compatible by construction).
- **Naming conventions:** `event_type` = `<svc>.<MessageName>` (e.g.
  `catalog.BoatUpserted`); Kafka topic `<svc>.events`; DLQ topic
  `<svc>.events.dlq`.
- **After any change:** run `make proto-gen` and commit the regenerated
  `gen/go` and `gen/ts` output alongside the `.proto` edit — never commit a
  `.proto` change without also committing its generated code.
