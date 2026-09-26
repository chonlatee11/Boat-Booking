package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/clock"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// EventRouteUpserted is the past-tense fact published when a route is
// created, edited, or archived (D-01, D-11).
const EventRouteUpserted = "catalog.RouteUpserted"

// pgUniqueViolation is Postgres's error code for a unique-constraint
// violation (23505), used here for routes_active_pair_uq (CAT-03
// idempotency edge).
const pgUniqueViolation = "23505"

// UpsertRoute validates r and writes the routes row plus its RouteUpserted
// outbox row in the same tx, returning the stored route.
//
// r.PierFromID must be in scope (Scope.All() or (OperatorID, PierIDs) via
// GetPierForShareScoped, D-07) and non-archived; r.PierToID may be any
// non-archived pier of any operator (D-12) via the unscoped GetPierForShare.
// r.OperatorID is always derived from the stored pier_from row, never taken
// from the request (D-30). An empty CancellationPolicy on create uses the
// default schedule (D-13). Create (r.ID zero) requires scope.CanWrite();
// update requires the route already be in scope via
// GetRouteForUpdateScoped — out-of-scope or missing ids return
// domain.ErrNotFound. A second active route for the same
// (pier_from, pier_to) pair returns domain.ErrAlreadyExists.
func UpsertRoute(ctx context.Context, tx pgx.Tx, scope Scope, r domain.Route) (domain.Route, error) {
	if !scope.CanWrite() {
		return domain.Route{}, domain.ErrPermissionDenied
	}

	q := postgres.New(tx)

	pierFrom, err := q.GetPierForShareScoped(ctx, postgres.GetPierForShareScopedParams{
		ID:         toPgUUID(r.PierFromID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Route{}, domain.ErrNotFound
		}
		return domain.Route{}, fmt.Errorf("app: get pier_from: %w", err)
	}
	if pierFrom.ArchivedAt.Valid {
		return domain.Route{}, domain.ErrFailedPrecondition
	}

	pierTo, err := q.GetPierForShare(ctx, toPgUUID(r.PierToID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Route{}, fmt.Errorf("%w: unknown pier_to", domain.ErrInvalidArgument)
		}
		return domain.Route{}, fmt.Errorf("app: get pier_to: %w", err)
	}
	if pierTo.ArchivedAt.Valid {
		return domain.Route{}, domain.ErrFailedPrecondition
	}

	r.OperatorID = fromPgUUID(pierFrom.OperatorID)
	if len(r.CancellationPolicy) == 0 {
		r.CancellationPolicy = domain.DefaultCancellationPolicy()
	}
	if err := r.Validate(); err != nil {
		return domain.Route{}, err
	}

	policyJSON, err := json.Marshal(r.CancellationPolicy)
	if err != nil {
		return domain.Route{}, fmt.Errorf("app: marshal cancellation policy: %w", err)
	}

	if r.ID == uuid.Nil {
		return createRoute(ctx, tx, q, r, policyJSON)
	}
	return updateRoute(ctx, tx, q, scope, r, policyJSON)
}

func createRoute(ctx context.Context, tx pgx.Tx, q *postgres.Queries, r domain.Route, policyJSON []byte) (domain.Route, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return domain.Route{}, fmt.Errorf("app: new route id: %w", err)
	}

	row, err := q.InsertRoute(ctx, postgres.InsertRouteParams{
		ID:                 toPgUUID(id),
		OperatorID:         toPgUUID(r.OperatorID),
		PierFromID:         toPgUUID(r.PierFromID),
		PierToID:           toPgUUID(r.PierToID),
		DurationMinutes:    r.DurationMinutes,
		CancellationPolicy: policyJSON,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Route{}, domain.ErrAlreadyExists
		}
		return domain.Route{}, fmt.Errorf("app: insert route: %w", err)
	}

	stored, err := routeFromRow(row)
	if err != nil {
		return domain.Route{}, err
	}
	return publishRouteUpserted(ctx, tx, stored)
}

func updateRoute(ctx context.Context, tx pgx.Tx, q *postgres.Queries, scope Scope, r domain.Route, policyJSON []byte) (domain.Route, error) {
	stored, err := q.GetRouteForUpdateScoped(ctx, postgres.GetRouteForUpdateScopedParams{
		ID:         toPgUUID(r.ID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Route{}, domain.ErrNotFound
		}
		return domain.Route{}, fmt.Errorf("app: get route for update: %w", err)
	}
	if stored.ArchivedAt.Valid {
		return domain.Route{}, domain.ErrFailedPrecondition
	}

	row, err := q.UpdateRoute(ctx, postgres.UpdateRouteParams{
		ID:                 toPgUUID(r.ID),
		PierToID:           toPgUUID(r.PierToID),
		DurationMinutes:    r.DurationMinutes,
		CancellationPolicy: policyJSON,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Route{}, domain.ErrAlreadyExists
		}
		return domain.Route{}, fmt.Errorf("app: update route: %w", err)
	}

	updated, err := routeFromRow(row)
	if err != nil {
		return domain.Route{}, err
	}
	return publishRouteUpserted(ctx, tx, updated)
}

// ArchiveRoute soft-deletes id (super_admin or pier_admin in scope, D-15).
// Out-of-scope or missing ids return domain.ErrNotFound. Archiving an
// already-archived route is a successful no-op — it returns the route
// unchanged and publishes no new event.
func ArchiveRoute(ctx context.Context, tx pgx.Tx, scope Scope, id uuid.UUID) (domain.Route, error) {
	if !scope.CanWrite() {
		return domain.Route{}, domain.ErrPermissionDenied
	}
	q := postgres.New(tx)

	stored, err := q.GetRouteForUpdateScoped(ctx, postgres.GetRouteForUpdateScopedParams{
		ID:         toPgUUID(id),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Route{}, domain.ErrNotFound
		}
		return domain.Route{}, fmt.Errorf("app: get route for archive: %w", err)
	}
	if stored.ArchivedAt.Valid {
		return routeFromRow(stored)
	}

	row, err := q.ArchiveRoute(ctx, toPgUUID(id))
	if err != nil {
		return domain.Route{}, fmt.Errorf("app: archive route: %w", err)
	}
	archived, err := routeFromRow(row)
	if err != nil {
		return domain.Route{}, err
	}
	return publishRouteUpserted(ctx, tx, archived)
}

// ListRoutes returns routes ordered by pier_from name_th, pier_to name_th,
// then id, each with CurrentPrices attached for today's Asia/Bangkok date
// (D-14). public=true (CAT-06, no claims) always returns the non-archived
// public projection (both endpoints non-archived), regardless of scope.
// Otherwise super_admin sees every route (optionally filtered by
// filterOperatorID), pier_admin/staff see only routes in
// (OperatorID, PierIDs) (AUTH-05).
func ListRoutes(ctx context.Context, q *postgres.Queries, scope Scope, public bool, filterOperatorID *uuid.UUID) ([]domain.Route, error) {
	var (
		routes []domain.Route
		err    error
	)
	if public {
		rows, listErr := q.ListRoutesPublic(ctx)
		if listErr != nil {
			return nil, fmt.Errorf("app: list public routes: %w", listErr)
		}
		routes, err = routesFromRows(rows)
	} else {
		var filter pgtype.UUID
		if scope.All() && filterOperatorID != nil {
			filter = pgtype.UUID{Bytes: *filterOperatorID, Valid: true}
		}

		rows, listErr := q.ListRoutesAdmin(ctx, postgres.ListRoutesAdminParams{
			AllScope:         scope.All(),
			FilterOperatorID: filter,
			OperatorID:       toPgUUID(scope.OperatorID),
			PierIds:          toPgUUIDs(scope.PierIDArray()),
		})
		if listErr != nil {
			return nil, fmt.Errorf("app: list routes: %w", listErr)
		}
		routes, err = routesFromRows(rows)
	}
	if err != nil {
		return nil, err
	}
	return attachCurrentPrices(ctx, q, routes, clock.LocalDate(clock.Now()))
}

func publishRouteUpserted(ctx context.Context, tx pgx.Tx, r domain.Route) (domain.Route, error) {
	env, err := events.New(EventRouteUpserted, r.ID.String(), &catalogv1.RouteUpserted{
		RouteId:         r.ID.String(),
		OperatorId:      r.OperatorID.String(),
		PierFromId:      r.PierFromID.String(),
		PierToId:        r.PierToID.String(),
		DurationMinutes: r.DurationMinutes,
		Archived:        r.Archived,
	})
	if err != nil {
		return domain.Route{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Route{}, fmt.Errorf("app: outbox insert: %w", err)
	}
	return r, nil
}

func routeFromRow(row postgres.Route) (domain.Route, error) {
	var policy []domain.CancellationTier
	if err := json.Unmarshal(row.CancellationPolicy, &policy); err != nil {
		return domain.Route{}, fmt.Errorf("app: unmarshal cancellation policy: %w", err)
	}
	return domain.Route{
		ID:                 fromPgUUID(row.ID),
		OperatorID:         fromPgUUID(row.OperatorID),
		PierFromID:         fromPgUUID(row.PierFromID),
		PierToID:           fromPgUUID(row.PierToID),
		DurationMinutes:    row.DurationMinutes,
		CancellationPolicy: policy,
		Archived:           row.ArchivedAt.Valid,
	}, nil
}

func routesFromRows(rows []postgres.Route) ([]domain.Route, error) {
	routes := make([]domain.Route, len(rows))
	for i, row := range rows {
		r, err := routeFromRow(row)
		if err != nil {
			return nil, err
		}
		routes[i] = r
	}
	return routes, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (pgcode 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
