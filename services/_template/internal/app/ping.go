// Package app holds thin use-case functions taking pgx.Tx directly — no
// repository interfaces, no mocks (D-14). Real services replace this
// package's contents with their own use cases; RecordPing/AckPing exist only
// to prove the platform's HTTP -> tx -> outbox -> Kafka -> consumer loop.
package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/emptypb"

	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/__NAME__/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/__NAME__/internal/domain"
)

// PingRecorded is the sample event type this slice publishes and consumes.
// The payload is empty (ids only, D-45) — aggregate_id already identifies
// the ping, and the note is never PII but is still kept out of the event on
// principle, matching what every real domain event must do.
const PingRecorded = "__NAME__.PingRecorded"

// RecordPing validates note, inserts a pings row and its PingRecorded
// outbox row in the same tx (D-05), and returns the created ping.
func RecordPing(ctx context.Context, tx pgx.Tx, operatorID uuid.UUID, note string) (domain.Ping, error) {
	if err := domain.ValidateNote(note); err != nil {
		return domain.Ping{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Ping{}, fmt.Errorf("app: new ping id: %w", err)
	}

	row, err := postgres.New(tx).InsertPing(ctx, postgres.InsertPingParams{
		ID:         toPgUUID(id),
		OperatorID: toPgUUID(operatorID),
		Note:       note,
	})
	if err != nil {
		return domain.Ping{}, fmt.Errorf("app: insert ping: %w", err)
	}

	env, err := events.New(PingRecorded, fromPgUUID(row.ID).String(), &emptypb.Empty{})
	if err != nil {
		return domain.Ping{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Ping{}, fmt.Errorf("app: outbox insert: %w", err)
	}

	return domain.Ping{ID: fromPgUUID(row.ID), OperatorID: fromPgUUID(row.OperatorID), Note: row.Note}, nil
}

// AckPing marks env's ping as acknowledged — the template's own consumer
// handler for PingRecorded, proving the round trip back through Kafka.
func AckPing(ctx context.Context, tx pgx.Tx, env *platformv1.Envelope) error {
	id, err := uuid.Parse(env.AggregateId)
	if err != nil {
		return fmt.Errorf("app: parse aggregate id: %w", err)
	}
	if _, err := postgres.New(tx).AckPing(ctx, toPgUUID(id)); err != nil {
		return fmt.Errorf("app: ack ping: %w", err)
	}
	return nil
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func fromPgUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}
