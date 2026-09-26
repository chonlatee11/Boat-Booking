-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: GetUserByPhone :one
select * from users where phone = $1;

-- name: GetUser :one
select * from users where id = $1;

-- name: InsertCustomerByEmail :one
insert into users (id, email, role)
values ($1, $2, 'customer')
on conflict (email) do nothing
returning *;

-- name: InsertCustomerByPhone :one
insert into users (id, phone, role)
values ($1, $2, 'customer')
on conflict (phone) do nothing
returning *;

-- name: UpsertSuperAdmin :one
insert into users (id, email, role)
values ($1, $2, 'super_admin')
on conflict (email) do update set
  role = 'super_admin', operator_id = null, pier_ids = '{}', disabled_at = null, updated_at = now()
returning id, (xmax = 0) as inserted;

-- name: InsertStaffUser :one
insert into users (id, email, name, role, operator_id, pier_ids)
values ($1, $2, $3, $4, $5, $6)
on conflict (email) do nothing
returning *;

-- name: PromoteCustomerToStaff :one
update users
set role = $2, operator_id = $3, pier_ids = $4, name = $5, updated_at = now()
where email = $1 and role = 'customer'
returning *;

-- name: UpdateStaffUser :one
update users
set name = $2, role = $3, operator_id = $4, pier_ids = $5, updated_at = now()
where id = $1 and role in ('staff', 'pier_admin')
returning *;

-- name: SetUserDisabledAt :one
update users
set disabled_at = case when sqlc.arg(disabled)::bool then now() else null end, updated_at = now()
where id = $1 and role in ('staff', 'pier_admin')
returning *;

-- name: ListStaffUsers :many
select * from users
where role <> 'customer'
  and (sqlc.narg(operator_id)::uuid is null or operator_id = sqlc.narg(operator_id)::uuid)
order by email, id;
