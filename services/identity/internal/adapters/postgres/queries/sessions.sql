-- name: InsertRefreshToken :exec
insert into refresh_tokens (id, user_id, token_hash, expires_at)
values ($1, $2, $3, $4);
