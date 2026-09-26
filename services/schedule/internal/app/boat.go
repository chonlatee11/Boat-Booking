// Package app holds thin use-case functions taking pgx.Tx directly — no
// repository interfaces, no mocks (D-14).
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/services/schedule/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/schedule/internal/domain"
)

// EventCatalogBoatUpserted is the published contract string for the event
// this service consumes from catalog.events (D-01, PLAT-05). It is not this
// service's own event — schedule publishes nothing yet.
const EventCatalogBoatUpserted = "catalog.BoatUpserted"


// ApplyBoatUpserted unmarshals env's BoatUpserted payload and upserts
// schedule's own boats projection (D-13: the caller's tx already guarantees
// this runs at most once per event_id via processed_events). An
// UNSPECIFIED status is treated as an error so the event retries and, on
// exhaustion, lands in the DLQ rather than silently writing a bad
// projection.
func ApplyBoatUpserted(ctx context.Context, tx pgx.Tx, env *platformv1.Envelope) error {
	var payload catalogv1.BoatUpserted
	if err := env.Payload.UnmarshalTo(&payload); err != nil {
		return fmt.Errorf("app: unmarshal BoatUpserted payload: %w", err)
	}

	boatID, err := uuid.Parse(payload.BoatId)
	if err != nil {
		return fmt.Errorf("app: parse boat_id: %w", err)
	}
	operatorID, err := uuid.Parse(payload.OperatorId)
	if err != nil {
		return fmt.Errorf("app: parse operator_id: %w", err)
	}
	status, err := statusFromProto(payload.Status)
	if err != nil {
		return err
	}

	if err := postgres.New(tx).UpsertBoatProjection(ctx, postgres.UpsertBoatProjectionParams{
		BoatID:          toPgUUID(boatID),
		OperatorID:      toPgUUID(operatorID),
		DefaultCapacity: payload.DefaultCapacity,
		Status:          string(status),
	}); err != nil {
		return fmt.Errorf("app: upsert boat projection: %w", err)
	}
	return nil
}

func statusFromProto(s catalogv1.BoatStatus) (domain.Status, error) {
	switch s {
	case catalogv1.BoatStatus_BOAT_STATUS_ACTIVE:
		return domain.StatusActive, nil
	case catalogv1.BoatStatus_BOAT_STATUS_MAINTENANCE:
		return domain.StatusMaintenance, nil
	default:
		return "", errors.New("app: BoatUpserted status must be BOAT_STATUS_ACTIVE or BOAT_STATUS_MAINTENANCE")
	}
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
