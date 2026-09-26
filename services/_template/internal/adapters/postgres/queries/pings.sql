-- name: InsertPing :one
insert into pings (id, operator_id, note)
values ($1, $2, $3)
returning *;

-- name: AckPing :execrows
update pings set acked_at = now() where id = $1 and acked_at is null;

-- name: GetPing :one
select * from pings where id = $1;
