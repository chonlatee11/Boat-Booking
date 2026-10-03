-- +goose Up
-- identity owns users (D-01, D-04, D-06). email/phone are two nullable,
-- independently-unique login identifiers on the same row — at least one is
-- required; linking them is out of scope for v1
-- (assumption_delta_decision, 02-02-PLAN.md). operator_id/pier_ids are only
-- ever set for staff/pier_admin (customers get NULL/empty), enforced by the
-- role check constraints below.
create table users (
  id           uuid primary key,
  email        text unique check (email = lower(email)),
  phone        text unique,
  name         text not null default '' check (char_length(name) <= 100),
  role         text not null check (role in ('customer', 'staff', 'pier_admin', 'super_admin')),
  operator_id  uuid,
  pier_ids     uuid[] not null default '{}',
  disabled_at  timestamptz,
  created_at   timestamptz not null default now(),
  updated_at   timestamptz not null default now(),
  constraint users_has_identifier check (email is not null or phone is not null),
  constraint users_operator_id_matches_role check ((role in ('staff', 'pier_admin')) = (operator_id is not null)),
  constraint users_pier_ids_only_for_staff check (role in ('staff', 'pier_admin') or cardinality(pier_ids) = 0)
);

-- +goose Down
drop table users;
