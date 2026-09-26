-- name: UpsertBoatProjection :exec
insert into boats (boat_id, operator_id, default_capacity, status)
values ($1, $2, $3, $4)
on conflict (boat_id) do update
  set operator_id      = excluded.operator_id,
      default_capacity = excluded.default_capacity,
      status           = excluded.status,
      updated_at       = now();

-- name: GetBoat :one
select * from boats where boat_id = $1;
