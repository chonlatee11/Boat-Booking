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
