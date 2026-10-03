-- +goose Up
-- piers belong to exactly one operator (D-07, D-17). opens_at/closes_at are
-- display-only local time-of-day (D-16) — both null or both set, opens
-- strictly before closes.
create table piers (
  id          uuid primary key,
  operator_id uuid not null references operators(id),
  name_th     text not null check (char_length(name_th) between 1 and 100),
  name_en     text not null check (char_length(name_en) between 1 and 100),
  lat         double precision not null check (lat between -90 and 90),
  lng         double precision not null check (lng between -180 and 180),
  address     text not null default '' check (char_length(address) <= 500),
  opens_at    time,
  closes_at   time,
  archived_at timestamptz,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  check ((opens_at is null) = (closes_at is null)),
  check (opens_at is null or opens_at < closes_at)
);

create index piers_operator_id_idx on piers (operator_id);

-- +goose Down
drop table piers;
