-- name: InsertRoute :one
insert into routes (id, operator_id, pier_from_id, pier_to_id, duration_minutes, cancellation_policy)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: UpdateRoute :one
update routes
set pier_to_id = $2,
    duration_minutes = $3,
    cancellation_policy = $4,
    updated_at = now()
where id = $1
returning *;

-- name: GetRouteForUpdateScoped :one
select routes.* from routes
where routes.id = $1
  and (sqlc.arg(all_scope)::bool or (routes.operator_id = sqlc.arg(operator_id) and routes.pier_from_id = any(sqlc.arg(pier_ids)::uuid[])))
for update of routes;

-- name: GetRouteScoped :one
select routes.* from routes
where routes.id = $1
  and (sqlc.arg(all_scope)::bool or (routes.operator_id = sqlc.arg(operator_id) and routes.pier_from_id = any(sqlc.arg(pier_ids)::uuid[])));

-- name: ListRoutesAdmin :many
select routes.* from routes
join piers pf on pf.id = routes.pier_from_id
join piers pt on pt.id = routes.pier_to_id
where (sqlc.arg(all_scope)::bool and (sqlc.narg(filter_operator_id)::uuid is null or routes.operator_id = sqlc.narg(filter_operator_id)::uuid))
   or (not sqlc.arg(all_scope)::bool and routes.operator_id = sqlc.arg(operator_id) and routes.pier_from_id = any(sqlc.arg(pier_ids)::uuid[]))
order by pf.name_th, pt.name_th, routes.id;

-- name: ListRoutesPublic :many
select routes.* from routes
join piers pf on pf.id = routes.pier_from_id
join piers pt on pt.id = routes.pier_to_id
where routes.archived_at is null and pf.archived_at is null and pt.archived_at is null
order by pf.name_th, pt.name_th, routes.id;

-- name: ArchiveRoute :one
update routes
set archived_at = coalesce(archived_at, now()),
    updated_at = now()
where id = $1
returning *;

-- name: ListActiveRoutesForPier :many
select
  routes.id as route_id,
  routes.operator_id as operator_id,
  pf.name_th as pier_from_name_th,
  pt.name_th as pier_to_name_th
from routes
join piers pf on pf.id = routes.pier_from_id
join piers pt on pt.id = routes.pier_to_id
where routes.archived_at is null
  and (routes.pier_from_id = $1 or routes.pier_to_id = $1)
order by pf.name_th, pt.name_th, routes.id;
