package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/clock"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/money"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// EventPriceChanged is the past-tense fact published when a route's
// effective-dated ticket price is added or replaced (D-01, D-14).
const EventPriceChanged = "catalog.PriceChanged"

// AddRoutePrice validates p against today's Asia/Bangkok local date and
// writes the route_prices row plus its PriceChanged outbox row in the same
// tx, returning the stored price. p.RouteID must be in scope
// (Scope.All() or (OperatorID, PierIDs) via GetRouteForUpdateScoped) and
// non-archived; re-adding the same (route, ticket_type, effective_from)
// replaces the amount in place (one row, D-14).
func AddRoutePrice(ctx context.Context, tx pgx.Tx, scope Scope, p domain.RoutePrice) (domain.RoutePrice, error) {
	if !scope.CanWrite() {
		return domain.RoutePrice{}, domain.ErrPermissionDenied
	}

	q := postgres.New(tx)

	route, err := q.GetRouteForUpdateScoped(ctx, postgres.GetRouteForUpdateScopedParams{
		ID:         toPgUUID(p.RouteID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RoutePrice{}, domain.ErrNotFound
		}
		return domain.RoutePrice{}, fmt.Errorf("app: get route for price update: %w", err)
	}
	if route.ArchivedAt.Valid {
		return domain.RoutePrice{}, domain.ErrFailedPrecondition
	}

	today := clock.LocalDate(clock.Now())
	if err := p.Validate(today); err != nil {
		return domain.RoutePrice{}, err
	}

	row, err := q.UpsertRoutePrice(ctx, postgres.UpsertRoutePriceParams{
		RouteID:       toPgUUID(p.RouteID),
		TicketType:    string(p.TicketType),
		AmountSatang:  int64(p.AmountSatang),
		EffectiveFrom: toPgDate(p.EffectiveFrom),
	})
	if err != nil {
		return domain.RoutePrice{}, fmt.Errorf("app: upsert route price: %w", err)
	}
	stored := routePriceFromRow(row)

	env, err := events.New(EventPriceChanged, p.RouteID.String(), &catalogv1.PriceChanged{
		RouteId:       stored.RouteID.String(),
		OperatorId:    fromPgUUID(route.OperatorID).String(),
		TicketType:    ticketTypeToProto(stored.TicketType),
		AmountSatang:  int64(stored.AmountSatang),
		EffectiveFrom: stored.EffectiveFrom.Format("2006-01-02"),
	})
	if err != nil {
		return domain.RoutePrice{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.RoutePrice{}, fmt.Errorf("app: outbox insert: %w", err)
	}

	return stored, nil
}

// ListRoutePrices returns routeID's prices ordered by effective_from
// descending then ticket_type. The route must be readable in the caller's
// scope (staff may read, unlike AddRoutePrice which requires CanWrite) —
// out-of-scope or missing routeID returns domain.ErrNotFound.
func ListRoutePrices(ctx context.Context, q *postgres.Queries, scope Scope, routeID uuid.UUID) ([]domain.RoutePrice, error) {
	// IN-02: a plain read, so use the non-locking GetRouteScoped rather
	// than GetRouteForUpdateScoped's FOR UPDATE, which would otherwise
	// take a row lock on this read-only path and contend with concurrent
	// AddRoutePrice/ArchiveRoute writers for no reason.
	_, err := q.GetRouteScoped(ctx, postgres.GetRouteScopedParams{
		ID:         toPgUUID(routeID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("app: get route for price list: %w", err)
	}

	rows, err := q.ListRoutePrices(ctx, toPgUUID(routeID))
	if err != nil {
		return nil, fmt.Errorf("app: list route prices: %w", err)
	}
	prices := make([]domain.RoutePrice, len(rows))
	for i, row := range rows {
		prices[i] = routePriceFromRow(row)
	}
	return prices, nil
}

// attachCurrentPrices populates each route's CurrentPrices with the price
// in effect on onDate (D-14) — a route with no prices set keeps an empty
// CurrentPrices, never a zero-amount entry.
func attachCurrentPrices(ctx context.Context, q *postgres.Queries, routes []domain.Route, onDate time.Time) ([]domain.Route, error) {
	if len(routes) == 0 {
		return routes, nil
	}
	ids := make([]pgtype.UUID, len(routes))
	for i, r := range routes {
		ids[i] = toPgUUID(r.ID)
	}
	rows, err := q.ListCurrentPrices(ctx, postgres.ListCurrentPricesParams{
		RouteIds: ids,
		OnDate:   toPgDate(onDate),
	})
	if err != nil {
		return nil, fmt.Errorf("app: list current prices: %w", err)
	}

	byRoute := make(map[uuid.UUID][]domain.RoutePrice, len(routes))
	for _, row := range rows {
		id := fromPgUUID(row.RouteID)
		byRoute[id] = append(byRoute[id], domain.RoutePrice{
			RouteID:       id,
			TicketType:    domain.TicketType(row.TicketType),
			AmountSatang:  money.Satang(row.AmountSatang),
			EffectiveFrom: fromPgDate(row.EffectiveFrom),
		})
	}
	for i := range routes {
		routes[i].CurrentPrices = byRoute[routes[i].ID]
	}
	return routes, nil
}

func routePriceFromRow(row postgres.RoutePrice) domain.RoutePrice {
	return domain.RoutePrice{
		RouteID:       fromPgUUID(row.RouteID),
		TicketType:    domain.TicketType(row.TicketType),
		AmountSatang:  money.Satang(row.AmountSatang),
		EffectiveFrom: fromPgDate(row.EffectiveFrom),
	}
}

func ticketTypeToProto(t domain.TicketType) catalogv1.TicketType {
	switch t {
	case domain.TicketTypeAdult:
		return catalogv1.TicketType_TICKET_TYPE_ADULT
	case domain.TicketTypeChild:
		return catalogv1.TicketType_TICKET_TYPE_CHILD
	default:
		return catalogv1.TicketType_TICKET_TYPE_UNSPECIFIED
	}
}

// toPgDate converts a UTC-midnight local calendar date (pkg/clock.LocalDate
// convention) to pgtype.Date.
func toPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// fromPgDate is toPgDate's inverse.
func fromPgDate(d pgtype.Date) time.Time {
	return d.Time
}
