package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// EventOperatorUpserted is the past-tense fact published when an operator is
// created, renamed, or archived (D-01).
const EventOperatorUpserted = "catalog.OperatorUpserted"

// UpsertOperator validates op, assigns a fresh uuid v7 id when op.ID is the
// zero value (create), writes the operators row and its OperatorUpserted
// outbox row in the same tx, and returns the stored operator. Only
// super_admin may call this (D-08); an update targeting an archived
// operator returns domain.ErrFailedPrecondition and leaves the row
// unchanged.
func UpsertOperator(ctx context.Context, tx pgx.Tx, scope Scope, op domain.Operator) (domain.Operator, error) {
	if !scope.All() {
		return domain.Operator{}, domain.ErrPermissionDenied
	}

	op.Name = strings.TrimSpace(op.Name)
	if err := op.Validate(); err != nil {
		return domain.Operator{}, err
	}

	id := op.ID
	if id == uuid.Nil {
		newID, err := uuid.NewV7()
		if err != nil {
			return domain.Operator{}, fmt.Errorf("app: new operator id: %w", err)
		}
		id = newID
	}

	row, err := postgres.New(tx).UpsertOperator(ctx, postgres.UpsertOperatorParams{
		ID:   toPgUUID(id),
		Name: op.Name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operator{}, domain.ErrFailedPrecondition
		}
		return domain.Operator{}, fmt.Errorf("app: upsert operator: %w", err)
	}

	stored := operatorFromRow(row)

	env, err := events.New(EventOperatorUpserted, stored.ID.String(), &catalogv1.OperatorUpserted{
		OperatorId: stored.ID.String(),
		Name:       stored.Name,
		Archived:   stored.Archived,
	})
	if err != nil {
		return domain.Operator{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Operator{}, fmt.Errorf("app: outbox insert: %w", err)
	}

	return stored, nil
}

// ListOperators returns operators in scope ordered by name then id:
// super_admin sees every operator, pier_admin/staff see only their own
// (AUTH-05).
func ListOperators(ctx context.Context, q *postgres.Queries, scope Scope) ([]domain.Operator, error) {
	rows, err := q.ListOperatorsScoped(ctx, postgres.ListOperatorsScopedParams{
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
	})
	if err != nil {
		return nil, fmt.Errorf("app: list operators: %w", err)
	}
	operators := make([]domain.Operator, len(rows))
	for i, row := range rows {
		operators[i] = operatorFromRow(row)
	}
	return operators, nil
}

// ArchiveOperator soft-deletes an operator (super_admin only). Rejected
// with domain.ErrFailedPrecondition while the operator still owns any
// non-archived pier — archiving is never a cascade (D-15). Archiving an
// already-archived operator succeeds as a no-op, keeping Archived=true.
func ArchiveOperator(ctx context.Context, tx pgx.Tx, scope Scope, id uuid.UUID) (domain.Operator, error) {
	if !scope.All() {
		return domain.Operator{}, domain.ErrPermissionDenied
	}

	q := postgres.New(tx)

	activePiers, err := q.CountActivePiersForOperator(ctx, toPgUUID(id))
	if err != nil {
		return domain.Operator{}, fmt.Errorf("app: count active piers: %w", err)
	}
	if activePiers > 0 {
		return domain.Operator{}, fmt.Errorf("%w: operator still has %d active piers", domain.ErrFailedPrecondition, activePiers)
	}

	row, err := q.ArchiveOperator(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operator{}, domain.ErrNotFound
		}
		return domain.Operator{}, fmt.Errorf("app: archive operator: %w", err)
	}
	stored := operatorFromRow(row)

	env, err := events.New(EventOperatorUpserted, stored.ID.String(), &catalogv1.OperatorUpserted{
		OperatorId: stored.ID.String(),
		Name:       stored.Name,
		Archived:   stored.Archived,
	})
	if err != nil {
		return domain.Operator{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Operator{}, fmt.Errorf("app: outbox insert: %w", err)
	}

	return stored, nil
}

func operatorFromRow(row postgres.Operator) domain.Operator {
	return domain.Operator{
		ID:       fromPgUUID(row.ID),
		Name:     row.Name,
		Archived: row.ArchivedAt.Valid,
	}
}
