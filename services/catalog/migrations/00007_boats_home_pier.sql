-- +goose Up
-- Boats come under pier-level scoping (D-07): a boat's operator is always
-- its home pier's operator. home_pier_id is nullable so Phase-1 dev rows
-- survive the migration — the app requires it on every write (UpsertBoat
-- rejects a missing home pier with InvalidArgument), so it only stays null
-- until the row is next edited.
alter table boats add column home_pier_id uuid references piers(id);
alter table boats add column archived_at timestamptz;

create index boats_home_pier_id_idx on boats (home_pier_id);

-- +goose Down
drop index boats_home_pier_id_idx;
alter table boats drop column archived_at;
alter table boats drop column home_pier_id;
