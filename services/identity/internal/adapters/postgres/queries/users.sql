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
