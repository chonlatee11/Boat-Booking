-- name: UpsertOperator :one
insert into operators (id, name)
values ($1, $2)
on conflict (id) do update
  set name = excluded.name,
      updated_at = now()
  where operators.archived_at is null
returning *;

-- name: ListOperatorsScoped :many
select * from operators
where sqlc.arg(all_scope)::bool or id = sqlc.arg(operator_id)
order by name, id;

-- name: GetOperator :one
select * from operators where id = $1;

-- name: ArchiveOperator :one
update operators
set archived_at = coalesce(archived_at, now()),
    updated_at = now()
where id = $1
returning *;

-- name: CountActivePiersForOperator :one
select count(*) from piers where operator_id = $1 and archived_at is null;
