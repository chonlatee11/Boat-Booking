-- name: InsertBoat :one
insert into boats (id, operator_id, home_pier_id, name, default_capacity, status)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: UpdateBoat :one
update boats
set operator_id = $2,
    home_pier_id = $3,
    name = $4,
    default_capacity = $5,
    status = $6,
    updated_at = now()
where id = $1
returning *;

-- name: GetBoatForUpdateScoped :one
select * from boats
where id = $1
  and (sqlc.arg(all_scope)::bool or (operator_id = sqlc.arg(operator_id) and home_pier_id = any(sqlc.arg(pier_ids)::uuid[])))
for update;

-- name: ListBoatsAdmin :many
select * from boats
where sqlc.arg(all_scope)::bool or (operator_id = sqlc.arg(operator_id) and home_pier_id = any(sqlc.arg(pier_ids)::uuid[]))
order by name, id;

-- name: ListBoatsPublic :many
select * from boats
where archived_at is null
order by name, id;

-- name: ArchiveBoat :one
update boats
set archived_at = coalesce(archived_at, now()),
    updated_at = now()
where id = $1
returning *;
