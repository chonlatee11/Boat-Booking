-- +goose Up
-- Effective-dated ticket prices (D-14): the price in effect on date D is
-- the row with the latest effective_from <= D. Re-adding the same
-- (route, ticket_type, effective_from) replaces the amount in place — the
-- primary key makes same-date re-adds an explicit upsert, not a new row.
create table route_prices (
  route_id       uuid not null references routes(id),
  ticket_type    text not null check (ticket_type in ('adult', 'child')),
  amount_satang  bigint not null check (amount_satang between 0 and 10000000),
  effective_from date not null,
  created_at     timestamptz not null default now(),
  updated_at     timestamptz not null default now(),
  primary key (route_id, ticket_type, effective_from)
);

-- +goose Down
drop table route_prices;
