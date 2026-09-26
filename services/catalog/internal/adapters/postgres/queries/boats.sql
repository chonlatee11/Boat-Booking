-- name: UpsertBoat :one
insert into boats (id, operator_id, name, default_capacity, status)
values ($1, $2, $3, $4, $5)
on conflict (id) do update
  set name = excluded.name,
      default_capacity = excluded.default_capacity,
      status = excluded.status,
      updated_at = now()
  where boats.operator_id = excluded.operator_id
returning *;

-- name: ListBoats :many
select * from boats order by name, id;
