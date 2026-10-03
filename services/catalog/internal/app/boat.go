// Package app holds thin use-case functions taking pgx.Tx directly — no
// repository interfaces, no mocks (D-14).
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// EventBoatUpserted is the past-tense fact published when a boat is created,
// updated, or archived (D-01). home_pier_id and archived are additive
// fields (D-07) — the schedule-service consumer ignores them and keeps
// applying the event unchanged. The payload holds ids, name, capacity and
// status only — no personal data (D-45).
const EventBoatUpserted = "catalog.BoatUpserted"

// UpsertBoat validates b and writes the boats row plus its BoatUpserted
// outbox row in the same tx, returning the stored boat.
//
// b.HomePierID must be in scope (Scope.All() or (OperatorID, PierIDs) via
// GetPierForShareScoped, D-07) and non-archived; b.OperatorID is always
// derived from the stored home pier, never taken from the request (D-30).
// Create (b.ID zero) requires scope.CanWrite(); update requires the boat
// already be in scope via GetBoatForUpdateScoped — out-of-scope or missing
// ids return domain.ErrNotFound. A home pier that is missing, out of scope,
// or archived returns domain.ErrNotFound/domain.ErrFailedPrecondition
// respectively; editing an already-archived boat also returns
// domain.ErrFailedPrecondition.
func UpsertBoat(ctx context.Context, tx pgx.Tx, scope Scope, b domain.Boat) (domain.Boat, error) {
	if !scope.CanWrite() {
		return domain.Boat{}, domain.ErrPermissionDenied
	}
	if b.HomePierID == uuid.Nil {
		return domain.Boat{}, fmt.Errorf("%w: home_pier_id is required", domain.ErrInvalidArgument)
	}

	q := postgres.New(tx)

	pier, err := q.GetPierForShareScoped(ctx, postgres.GetPierForShareScopedParams{
		ID:         toPgUUID(b.HomePierID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Boat{}, domain.ErrNotFound
		}
		return domain.Boat{}, fmt.Errorf("app: get home pier: %w", err)
	}
	if pier.ArchivedAt.Valid {
		return domain.Boat{}, domain.ErrFailedPrecondition
	}

	b.OperatorID = fromPgUUID(pier.OperatorID)
	b.Name = strings.TrimSpace(b.Name)
	if err := b.Validate(); err != nil {
		return domain.Boat{}, err
	}

	if b.ID == uuid.Nil {
		return createBoat(ctx, tx, q, b)
	}
	return updateBoat(ctx, tx, q, scope, b)
}

func createBoat(ctx context.Context, tx pgx.Tx, q *postgres.Queries, b domain.Boat) (domain.Boat, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return domain.Boat{}, fmt.Errorf("app: new boat id: %w", err)
	}

	row, err := q.InsertBoat(ctx, postgres.InsertBoatParams{
		ID:              toPgUUID(id),
		OperatorID:      toPgUUID(b.OperatorID),
		HomePierID:      toPgUUID(b.HomePierID),
		Name:            b.Name,
		DefaultCapacity: b.DefaultCapacity,
		Status:          string(b.Status),
	})
	if err != nil {
		return domain.Boat{}, fmt.Errorf("app: insert boat: %w", err)
	}
	return publishBoatUpserted(ctx, tx, boatFromRow(row))
}

func updateBoat(ctx context.Context, tx pgx.Tx, q *postgres.Queries, scope Scope, b domain.Boat) (domain.Boat, error) {
	stored, err := q.GetBoatForUpdateScoped(ctx, postgres.GetBoatForUpdateScopedParams{
		ID:         toPgUUID(b.ID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Boat{}, domain.ErrNotFound
		}
		return domain.Boat{}, fmt.Errorf("app: get boat for update: %w", err)
	}
	if stored.ArchivedAt.Valid {
		return domain.Boat{}, domain.ErrFailedPrecondition
	}

	row, err := q.UpdateBoat(ctx, postgres.UpdateBoatParams{
		ID:              toPgUUID(b.ID),
		OperatorID:      toPgUUID(b.OperatorID),
		HomePierID:      toPgUUID(b.HomePierID),
		Name:            b.Name,
		DefaultCapacity: b.DefaultCapacity,
		Status:          string(b.Status),
	})
	if err != nil {
		return domain.Boat{}, fmt.Errorf("app: update boat: %w", err)
	}
	return publishBoatUpserted(ctx, tx, boatFromRow(row))
}

// ListBoats returns boats ordered by name then id. public=true (no claims)
// returns only non-archived boats. Otherwise super_admin sees every boat;
// pier_admin/staff see only boats whose home pier is in
// (OperatorID, PierIDs) (AUTH-05, CAT-04) — a pier_admin with no assigned
// piers gets an empty list.
func ListBoats(ctx context.Context, q *postgres.Queries, scope Scope, public bool) ([]domain.Boat, error) {
	if public {
		rows, err := q.ListBoatsPublic(ctx)
		if err != nil {
			return nil, fmt.Errorf("app: list public boats: %w", err)
		}
		return boatsFromRows(rows), nil
	}

	rows, err := q.ListBoatsAdmin(ctx, postgres.ListBoatsAdminParams{
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		return nil, fmt.Errorf("app: list boats: %w", err)
	}
	return boatsFromRows(rows), nil
}

// ArchiveBoat soft-deletes id (super_admin or pier_admin in scope, D-07).
// Out-of-scope or missing ids return domain.ErrNotFound. Archiving an
// already-archived boat is a successful no-op — it returns the boat
// unchanged and publishes no new event.
func ArchiveBoat(ctx context.Context, tx pgx.Tx, scope Scope, id uuid.UUID) (domain.Boat, error) {
	if !scope.CanWrite() {
		return domain.Boat{}, domain.ErrPermissionDenied
	}
	q := postgres.New(tx)

	stored, err := q.GetBoatForUpdateScoped(ctx, postgres.GetBoatForUpdateScopedParams{
		ID:         toPgUUID(id),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Boat{}, domain.ErrNotFound
		}
		return domain.Boat{}, fmt.Errorf("app: get boat for archive: %w", err)
	}
	if stored.ArchivedAt.Valid {
		return boatFromRow(stored), nil
	}

	row, err := q.ArchiveBoat(ctx, toPgUUID(id))
	if err != nil {
		return domain.Boat{}, fmt.Errorf("app: archive boat: %w", err)
	}
	return publishBoatUpserted(ctx, tx, boatFromRow(row))
}

func publishBoatUpserted(ctx context.Context, tx pgx.Tx, b domain.Boat) (domain.Boat, error) {
	env, err := events.New(EventBoatUpserted, b.ID.String(), &catalogv1.BoatUpserted{
		BoatId:          b.ID.String(),
		OperatorId:      b.OperatorID.String(),
		Name:            b.Name,
		DefaultCapacity: b.DefaultCapacity,
		Status:          statusToProto(b.Status),
		HomePierId:      b.HomePierID.String(),
		Archived:        b.Archived,
	})
	if err != nil {
		return domain.Boat{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Boat{}, fmt.Errorf("app: outbox insert: %w", err)
	}
	return b, nil
}

func boatsFromRows(rows []postgres.Boat) []domain.Boat {
	boats := make([]domain.Boat, len(rows))
	for i, row := range rows {
		boats[i] = boatFromRow(row)
	}
	return boats
}

func boatFromRow(row postgres.Boat) domain.Boat {
	return domain.Boat{
		ID:              fromPgUUID(row.ID),
		OperatorID:      fromPgUUID(row.OperatorID),
		HomePierID:      fromPgUUID(row.HomePierID),
		Name:            row.Name,
		DefaultCapacity: row.DefaultCapacity,
		Status:          domain.Status(row.Status),
		Archived:        row.ArchivedAt.Valid,
	}
}

// statusToProto converts an already-validated domain.Status to its proto
// enum value for the outgoing BoatUpserted event.
func statusToProto(s domain.Status) catalogv1.BoatStatus {
	switch s {
	case domain.StatusActive:
		return catalogv1.BoatStatus_BOAT_STATUS_ACTIVE
	case domain.StatusMaintenance:
		return catalogv1.BoatStatus_BOAT_STATUS_MAINTENANCE
	default:
		return catalogv1.BoatStatus_BOAT_STATUS_UNSPECIFIED
	}
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func fromPgUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}
