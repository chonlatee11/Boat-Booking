-- +goose Up
-- schedule's own projection of catalog boats (database-per-service: schedule
-- never reads catalog's database directly). Populated exclusively by the
-- catalog.BoatUpserted consumer (D-01, PLAT-05).
create table boats (
  boat_id          uuid primary key,
  operator_id      uuid not null,
  default_capacity int not null,
  status           text not null check (status in ('active', 'maintenance')),
  updated_at       timestamptz not null default now()
);

-- +goose Down
drop table boats;
