-- +goose Up
-- Only sha256(token) is ever stored (D-09) — the opaque 32-byte refresh
-- token itself never touches Postgres.
create table refresh_tokens (
  id          uuid primary key,
  user_id     uuid not null references users(id),
  token_hash  bytea not null unique,
  expires_at  timestamptz not null,
  revoked_at  timestamptz,
  created_at  timestamptz not null default now()
);

create index refresh_tokens_user_id_idx on refresh_tokens (user_id);

-- +goose Down
drop table refresh_tokens;
