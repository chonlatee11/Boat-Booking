-- +goose Up
-- Transactional outbox (D-05, D-10, D-11) and idempotent-consumer ledger
-- (D-13). Every service copies this file verbatim from services/_template;
-- goose owns this DDL, pkg/outbox and pkg/kafka never migrate anything
-- themselves.
create table outbox (
  id           bigserial primary key,
  event_id     uuid not null unique,
  topic        text not null,
  aggregate_id text not null,
  event_type   text not null,
  payload      bytea not null,
  traceparent  text not null default '',
  created_at   timestamptz not null default now(),
  published_at timestamptz
);

-- Partial index so the relay's "unpublished rows" scan never touches
-- already-delivered history, no matter how large the table grows.
create index outbox_unpublished_idx on outbox (id) where published_at is null;

create table processed_events (
  event_id     uuid primary key,
  event_type   text not null,
  processed_at timestamptz not null default now()
);

-- +goose Down
drop table processed_events;
drop table outbox;
