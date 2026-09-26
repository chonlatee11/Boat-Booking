-- +goose Up
-- catalog owns boats (Phase 1) — operators/piers/routes/prices arrive in
-- Phase 2. operator_id scopes every admin-facing query (D-30).
create table boats (
  id               uuid primary key,
  operator_id      uuid not null,
  name             text not null check (char_length(name) between 1 and 100),
  default_capacity int not null check (default_capacity between 1 and 1000),
  status           text not null check (status in ('active', 'maintenance')),
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now()
);

create index boats_operator_id_idx on boats (operator_id);

-- +goose Down
drop table boats;
