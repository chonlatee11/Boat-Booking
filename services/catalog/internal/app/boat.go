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

// EventBoatUpserted is the past-tense fact published when a boat is created
// or updated (D-01). The payload holds ids, name, capacity and status only —
// no personal data (D-45).
const EventBoatUpserted = "catalog.BoatUpserted"

// UpsertBoat validates b, assigns a fresh uuid v7 id when b.ID is the zero
// value (create), writes the boats row and its BoatUpserted outbox row in
// the same tx (D-05), and returns the stored boat. operatorID always comes
// from the caller's trusted claims, never the request body (D-30); the
// upsert only takes effect when the target row's operator_id matches it, so
// an id belonging to a different operator returns domain.ErrNotFound with
// the stored row left unchanged.
func UpsertBoat(ctx context.Context, tx pgx.Tx, operatorID uuid.UUID, b domain.Boat) (domain.Boat, error) {
	b.OperatorID = operatorID
	b.Name = strings.TrimSpace(b.Name)
	if err := b.Validate(); err != nil {
		return domain.Boat{}, err
	}

	id := b.ID
	if id == uuid.Nil {
		newID, err := uuid.NewV7()
		if err != nil {
			return domain.Boat{}, fmt.Errorf("app: new boat id: %w", err)
		}
		id = newID
	}

	row, err := postgres.New(tx).UpsertBoat(ctx, postgres.UpsertBoatParams{
		ID:              toPgUUID(id),
		OperatorID:      toPgUUID(operatorID),
		Name:            b.Name,
		DefaultCapacity: b.DefaultCapacity,
		Status:          string(b.Status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Boat{}, domain.ErrNotFound
		}
		return domain.Boat{}, fmt.Errorf("app: upsert boat: %w", err)
	}

	stored := boatFromRow(row)

	env, err := events.New(EventBoatUpserted, stored.ID.String(), &catalogv1.BoatUpserted{
		BoatId:          stored.ID.String(),
		OperatorId:      stored.OperatorID.String(),
		Name:            stored.Name,
		DefaultCapacity: stored.DefaultCapacity,
		Status:          statusToProto(stored.Status),
	})
	if err != nil {
		return domain.Boat{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Boat{}, fmt.Errorf("app: outbox insert: %w", err)
	}

	return stored, nil
}

// ListBoats returns every boat ordered by name then id (stable order) — a
// catalog-wide read needing only the internal token, no claims.
func ListBoats(ctx context.Context, q *postgres.Queries) ([]domain.Boat, error) {
	rows, err := q.ListBoats(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list boats: %w", err)
	}
	boats := make([]domain.Boat, len(rows))
	for i, row := range rows {
		boats[i] = boatFromRow(row)
	}
	return boats, nil
}

func boatFromRow(row postgres.Boat) domain.Boat {
	return domain.Boat{
		ID:              fromPgUUID(row.ID),
		OperatorID:      fromPgUUID(row.OperatorID),
		Name:            row.Name,
		DefaultCapacity: row.DefaultCapacity,
		Status:          domain.Status(row.Status),
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
