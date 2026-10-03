-- +goose Up
-- operators own piers, routes, boats, and prices (Phase 2). Only super_admin
-- may create/edit operators (D-08).
create table operators (
  id          uuid primary key,
  name        text not null check (char_length(name) between 1 and 100),
  archived_at timestamptz,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now()
);

-- +goose Down
drop table operators;
