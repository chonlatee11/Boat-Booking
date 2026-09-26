-- name: InsertPier :one
insert into piers (id, operator_id, name_th, name_en, lat, lng, address, opens_at, closes_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
returning *;

-- name: UpdatePier :one
update piers
set name_th = $2,
    name_en = $3,
    lat = $4,
    lng = $5,
    address = $6,
    opens_at = $7,
    closes_at = $8,
    updated_at = now()
where id = $1
returning *;

-- name: GetPierForUpdateScoped :one
select * from piers
where id = $1
  and (sqlc.arg(all_scope)::bool or (operator_id = sqlc.arg(operator_id) and id = any(sqlc.arg(pier_ids)::uuid[])))
for update;

-- name: ListPiersAdmin :many
select * from piers
where (sqlc.arg(all_scope)::bool and (sqlc.narg(filter_operator_id)::uuid is null or operator_id = sqlc.narg(filter_operator_id)::uuid))
   or (not sqlc.arg(all_scope)::bool and operator_id = sqlc.arg(operator_id) and id = any(sqlc.arg(pier_ids)::uuid[]))
order by name_th, id;

-- name: ListPiersPublic :many
select * from piers
where archived_at is null
order by name_th, id;

-- name: GetPierForShareScoped :one
select * from piers
where id = $1
  and (sqlc.arg(all_scope)::bool or (operator_id = sqlc.arg(operator_id) and id = any(sqlc.arg(pier_ids)::uuid[])))
for share;

-- name: GetPierForShare :one
select * from piers
where id = $1
for share;
