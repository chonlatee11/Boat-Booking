-- +goose Up
-- Sample aggregate exercising every platform layer for this template's
-- POST /v1/pings slice (D-03). Real services drop this migration and their
-- own copy of internal/{domain,app,adapters/postgres/pings.sql.go...}.
create table pings (
  id           uuid primary key,
  operator_id  uuid not null,
  note         text not null check (char_length(note) between 1 and 280),
  created_at   timestamptz not null default now(),
  acked_at     timestamptz
);

-- +goose Down
drop table pings;
