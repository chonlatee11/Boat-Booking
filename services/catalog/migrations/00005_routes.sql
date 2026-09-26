-- +goose Up
-- Routes are one-way (D-11): pier_from -> pier_to, each with its own
-- cancellation policy. pier_to may belong to any operator (D-12), so it is
-- a bare uuid FK to piers, not scoped to operator_id at the DB level.
create table routes (
  id                  uuid primary key,
  operator_id         uuid not null references operators(id),
  pier_from_id        uuid not null references piers(id),
  pier_to_id          uuid not null references piers(id),
  duration_minutes    int not null check (duration_minutes between 1 and 1440),
  cancellation_policy jsonb not null,
  archived_at         timestamptz,
  created_at          timestamptz not null default now(),
  updated_at          timestamptz not null default now(),
  check (pier_from_id <> pier_to_id)
);

create index routes_operator_id_idx on routes (operator_id);
create index routes_pier_from_id_idx on routes (pier_from_id);
create index routes_pier_to_id_idx on routes (pier_to_id);

-- Only one active (pier_from, pier_to) pair at a time (CAT-03 idempotency
-- edge) — a second create for the same pair is AlreadyExists.
create unique index routes_active_pair_uq on routes (pier_from_id, pier_to_id) where archived_at is null;

-- +goose Down
drop table routes;
