package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// EventPierUpserted is the past-tense fact published when a pier is
// created, edited, or archived (D-01). The payload carries no address
// (Pitfall 5, D-45).
const EventPierUpserted = "catalog.PierUpserted"

// UpsertPier validates p and writes the piers row plus its PierUpserted
// outbox row in the same tx, returning the stored pier.
//
// Create (p.ID zero) requires scope.All() (D-08) and a non-archived target
// operator (p.OperatorID, taken from the request). Update requires
// scope.CanWrite() and a pier already in scope (Scope.All() or
// (OperatorID, PierIDs)) — out-of-scope or missing ids return
// domain.ErrNotFound, never revealing whether the row exists (D-07);
// operator_id is always kept from the stored row on update, never taken
// from the request (D-30). Either path on an archived operator/pier returns
// domain.ErrFailedPrecondition.
func UpsertPier(ctx context.Context, tx pgx.Tx, scope Scope, p domain.Pier) (domain.Pier, error) {
	q := postgres.New(tx)

	if p.ID == uuid.Nil {
		return createPier(ctx, tx, q, scope, p)
	}
	return updatePier(ctx, tx, q, scope, p)
}

func createPier(ctx context.Context, tx pgx.Tx, q *postgres.Queries, scope Scope, p domain.Pier) (domain.Pier, error) {
	if !scope.All() {
		return domain.Pier{}, domain.ErrPermissionDenied
	}

	// WR-06: FOR SHARE against the operator row, mirroring how pier/route
	// writes already take GetPierForShareScoped. ArchiveOperator takes
	// FOR UPDATE before counting active piers, so the two block each other
	// instead of racing: either this create commits first and the archive
	// then sees the new pier and is rejected, or the archive commits first
	// and this create sees archived_at set.
	op, err := q.GetOperatorForShare(ctx, toPgUUID(p.OperatorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pier{}, domain.ErrNotFound
		}
		return domain.Pier{}, fmt.Errorf("app: get operator: %w", err)
	}
	if op.ArchivedAt.Valid {
		return domain.Pier{}, domain.ErrFailedPrecondition
	}

	p = trimPier(p)
	if err := p.Validate(); err != nil {
		return domain.Pier{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Pier{}, fmt.Errorf("app: new pier id: %w", err)
	}

	row, err := q.InsertPier(ctx, postgres.InsertPierParams{
		ID:         toPgUUID(id),
		OperatorID: toPgUUID(p.OperatorID),
		NameTh:     p.NameTH,
		NameEn:     p.NameEN,
		Lat:        p.Lat,
		Lng:        p.Lng,
		Address:    p.Address,
		OpensAt:    toPgTime(p.OpensAt),
		ClosesAt:   toPgTime(p.ClosesAt),
		PhotoKey:   p.PhotoKey,
	})
	if err != nil {
		return domain.Pier{}, fmt.Errorf("app: insert pier: %w", err)
	}

	return publishPierUpserted(ctx, tx, pierFromRow(row))
}

func updatePier(ctx context.Context, tx pgx.Tx, q *postgres.Queries, scope Scope, p domain.Pier) (domain.Pier, error) {
	if !scope.CanWrite() {
		return domain.Pier{}, domain.ErrPermissionDenied
	}

	stored, err := q.GetPierForUpdateScoped(ctx, postgres.GetPierForUpdateScopedParams{
		ID:         toPgUUID(p.ID),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pier{}, domain.ErrNotFound
		}
		return domain.Pier{}, fmt.Errorf("app: get pier for update: %w", err)
	}
	if stored.ArchivedAt.Valid {
		return domain.Pier{}, domain.ErrFailedPrecondition
	}

	// operator_id always comes from the stored row — the request's
	// operator_id is ignored on update (D-30).
	p.OperatorID = fromPgUUID(stored.OperatorID)
	p = trimPier(p)
	if err := p.Validate(); err != nil {
		return domain.Pier{}, err
	}

	row, err := q.UpdatePier(ctx, postgres.UpdatePierParams{
		ID:       toPgUUID(p.ID),
		NameTh:   p.NameTH,
		NameEn:   p.NameEN,
		Lat:      p.Lat,
		Lng:      p.Lng,
		Address:  p.Address,
		OpensAt:  toPgTime(p.OpensAt),
		ClosesAt: toPgTime(p.ClosesAt),
		PhotoKey: p.PhotoKey,
	})
	if err != nil {
		return domain.Pier{}, fmt.Errorf("app: update pier: %w", err)
	}

	return publishPierUpserted(ctx, tx, pierFromRow(row))
}

// ListPiers returns piers ordered by name_th then id. public=true (CAT-06,
// no claims) always returns the non-archived-only public projection,
// regardless of scope. Otherwise super_admin sees every pier (optionally
// filtered by filterOperatorID), pier_admin/staff see only piers in
// (OperatorID, PierIDs) (AUTH-05).
func ListPiers(ctx context.Context, q *postgres.Queries, scope Scope, public bool, filterOperatorID *uuid.UUID) ([]domain.Pier, error) {
	if public {
		rows, err := q.ListPiersPublic(ctx)
		if err != nil {
			return nil, fmt.Errorf("app: list public piers: %w", err)
		}
		return piersFromRows(rows), nil
	}

	var filter pgtype.UUID
	if scope.All() && filterOperatorID != nil {
		filter = pgtype.UUID{Bytes: *filterOperatorID, Valid: true}
	}

	rows, err := q.ListPiersAdmin(ctx, postgres.ListPiersAdminParams{
		AllScope:         scope.All(),
		FilterOperatorID: filter,
		OperatorID:       toPgUUID(scope.OperatorID),
		PierIds:          toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		return nil, fmt.Errorf("app: list piers: %w", err)
	}
	return piersFromRows(rows), nil
}

// ArchivePier soft-deletes id (super_admin or pier_admin in scope, D-08).
// Rejected with domain.ErrFailedPrecondition while any non-archived route
// uses id as pier_from or pier_to; the error message lists only routes
// visible in the caller's scope and counts the rest as
// "+N routes of other operators" — never their names or ids (D-15,
// CAT-02/CAT-03, T-02-06-03). The row lock (FOR UPDATE via
// GetPierForUpdateScoped) serializes against UpsertRoute's FOR SHARE lock
// on pier_from/pier_to, so a route can never be created on a pier archived
// concurrently. Archiving an already-archived pier is a successful no-op.
func ArchivePier(ctx context.Context, tx pgx.Tx, scope Scope, id uuid.UUID) (domain.Pier, error) {
	if !scope.CanWrite() {
		return domain.Pier{}, domain.ErrPermissionDenied
	}
	q := postgres.New(tx)

	stored, err := q.GetPierForUpdateScoped(ctx, postgres.GetPierForUpdateScopedParams{
		ID:         toPgUUID(id),
		AllScope:   scope.All(),
		OperatorID: toPgUUID(scope.OperatorID),
		PierIds:    toPgUUIDs(scope.PierIDArray()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Pier{}, domain.ErrNotFound
		}
		return domain.Pier{}, fmt.Errorf("app: get pier for archive: %w", err)
	}
	if stored.ArchivedAt.Valid {
		return pierFromRow(stored), nil
	}

	activeRoutes, err := q.ListActiveRoutesForPier(ctx, toPgUUID(id))
	if err != nil {
		return domain.Pier{}, fmt.Errorf("app: list active routes for pier: %w", err)
	}
	if len(activeRoutes) > 0 {
		return domain.Pier{}, blockedArchiveError(scope, activeRoutes)
	}

	row, err := q.ArchivePier(ctx, toPgUUID(id))
	if err != nil {
		return domain.Pier{}, fmt.Errorf("app: archive pier: %w", err)
	}
	return publishPierUpserted(ctx, tx, pierFromRow(row))
}

// blockedArchiveError builds the FailedPrecondition error for ArchivePier:
// routes visible in scope are named, routes belonging to other operators
// are only counted (T-02-06-03).
func blockedArchiveError(scope Scope, routes []postgres.ListActiveRoutesForPierRow) error {
	var visible []string
	otherCount := 0
	for _, r := range routes {
		if scope.All() || fromPgUUID(r.OperatorID) == scope.OperatorID {
			visible = append(visible, fmt.Sprintf("%s → %s", r.PierFromNameTh, r.PierToNameTh))
		} else {
			otherCount++
		}
	}
	msg := "active routes: " + strings.Join(visible, ", ")
	if otherCount > 0 {
		msg = fmt.Sprintf("%s (+%d routes of other operators)", msg, otherCount)
	}
	return fmt.Errorf("%w: %s", domain.ErrFailedPrecondition, msg)
}

func publishPierUpserted(ctx context.Context, tx pgx.Tx, p domain.Pier) (domain.Pier, error) {
	env, err := events.New(EventPierUpserted, p.ID.String(), &catalogv1.PierUpserted{
		PierId:     p.ID.String(),
		OperatorId: p.OperatorID.String(),
		NameTh:     p.NameTH,
		NameEn:     p.NameEN,
		Lat:        p.Lat,
		Lng:        p.Lng,
		Archived:   p.Archived,
	})
	if err != nil {
		return domain.Pier{}, fmt.Errorf("app: build event: %w", err)
	}
	if err := outbox.Insert(ctx, tx, env); err != nil {
		return domain.Pier{}, fmt.Errorf("app: outbox insert: %w", err)
	}
	return p, nil
}

func trimPier(p domain.Pier) domain.Pier {
	p.NameTH = strings.TrimSpace(p.NameTH)
	p.NameEN = strings.TrimSpace(p.NameEN)
	p.Address = strings.TrimSpace(p.Address)
	return p
}

func pierFromRow(row postgres.Pier) domain.Pier {
	return domain.Pier{
		ID:         fromPgUUID(row.ID),
		OperatorID: fromPgUUID(row.OperatorID),
		NameTH:     row.NameTh,
		NameEN:     row.NameEn,
		Lat:        row.Lat,
		Lng:        row.Lng,
		Address:    row.Address,
		OpensAt:    fromPgTime(row.OpensAt),
		ClosesAt:   fromPgTime(row.ClosesAt),
		Archived:   row.ArchivedAt.Valid,
		PhotoKey:   row.PhotoKey,
	}
}

func piersFromRows(rows []postgres.Pier) []domain.Pier {
	piers := make([]domain.Pier, len(rows))
	for i, row := range rows {
		piers[i] = pierFromRow(row)
	}
	return piers
}

// toPgUUIDs never returns nil — pgx must send an empty array literal to
// Postgres, not NULL, for `= any($1)` to match nothing (Scope.PierIDArray's
// same rule, applied at the pgtype boundary).
func toPgUUIDs(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = toPgUUID(id)
	}
	return out
}

// toPgTime converts an "HH:MM" string (or "" for NULL) to pgtype.Time.
// Callers pass values already validated by domain.Pier.Validate.
func toPgTime(hhmm string) pgtype.Time {
	if hhmm == "" {
		return pgtype.Time{}
	}
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return pgtype.Time{}
	}
	micros := (int64(t.Hour())*3600 + int64(t.Minute())*60) * 1_000_000
	return pgtype.Time{Microseconds: micros, Valid: true}
}

// fromPgTime is toPgTime's inverse, returning "" for NULL.
func fromPgTime(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	totalSeconds := t.Microseconds / 1_000_000
	return fmt.Sprintf("%02d:%02d", totalSeconds/3600, (totalSeconds%3600)/60)
}
