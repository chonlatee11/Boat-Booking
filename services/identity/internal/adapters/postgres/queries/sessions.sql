-- name: InsertRefreshToken :exec
insert into refresh_tokens (id, user_id, token_hash, expires_at)
values ($1, $2, $3, $4);

-- name: GetRefreshTokenForUpdate :one
select * from refresh_tokens where token_hash = $1 for update;

-- name: RevokeRefreshToken :exec
update refresh_tokens set revoked_at = now() where id = $1 and revoked_at is null;

-- name: RevokeAllRefreshTokens :exec
update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null;

-- name: RevokeRefreshTokenByHash :exec
update refresh_tokens set revoked_at = now() where token_hash = $1 and revoked_at is null;
