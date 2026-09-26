-- name: UpsertRoutePrice :one
insert into route_prices (route_id, ticket_type, amount_satang, effective_from)
values ($1, $2, $3, $4)
on conflict (route_id, ticket_type, effective_from) do update
  set amount_satang = excluded.amount_satang,
      updated_at = now()
returning *;

-- name: ListRoutePrices :many
select * from route_prices
where route_id = $1
order by effective_from desc, ticket_type;

-- name: ListCurrentPrices :many
select distinct on (route_id, ticket_type)
  route_id, ticket_type, amount_satang, effective_from
from route_prices
where route_id = any(sqlc.arg(route_ids)::uuid[])
  and effective_from <= sqlc.arg(on_date)::date
order by route_id, ticket_type, effective_from desc;
