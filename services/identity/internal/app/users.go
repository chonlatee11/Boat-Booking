package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// Users implements super_admin's staff-management use case (AUTH-04, D-08):
// create/update, list, and disable/re-enable staff and pier_admin users.
// Pier ownership is confirmed by a synchronous call to catalog.ListPiers —
// database-per-service forbids a foreign key across services.
type Users struct {
	Pool          *pgxpool.Pool
	Catalog       catalogv1connect.CatalogServiceClient
	InternalToken string
	Nudge         func()
}

// UpsertUser creates (userID zero) or updates (userID set) a staff/pier_admin
// user. Only super_admin may call this (T-02-07-01). in's operator+piers are
// confirmed against catalog before anything is persisted (T-02-07-03): every
// requested pier must belong to in.OperatorID and be non-archived, else
// ErrInvalidArgument names the offending ids.
func (u *Users) UpsertUser(ctx context.Context, caller httpx.Claims, in domain.StaffUserInput, userID uuid.UUID) (domain.User, error) {
	if caller.Role != auth.RoleSuperAdmin {
		return domain.User{}, domain.ErrPermissionDenied
	}
	if err := in.Validate(); err != nil {
		return domain.User{}, err
	}
	if err := u.validatePiers(ctx, caller, in.OperatorID, in.PierIDs); err != nil {
		return domain.User{}, err
	}

	if userID == uuid.Nil {
		return u.createUser(ctx, in)
	}
	return u.updateUser(ctx, userID, in)
}

// validatePiers confirms every id in pierIDs belongs to operatorID and is
// non-archived, by forwarding caller's claims (plus the internal token) to
// catalog.ListPiers filtered by operatorID (research Pattern 3). A transport
// error maps to ErrUnavailable; any id catalog didn't return (or returned
// archived, or under a different operator) is named in ErrInvalidArgument.
func (u *Users) validatePiers(ctx context.Context, caller httpx.Claims, operatorID uuid.UUID, pierIDs []uuid.UUID) error {
	req := connect.NewRequest(&catalogv1.ListPiersRequest{OperatorId: operatorID.String()})
	httpx.ForwardClaims(req.Header(), caller, u.InternalToken)

	resp, err := u.Catalog.ListPiers(ctx, req)
	if err != nil {
		return fmt.Errorf("%w: catalog list piers: %v", domain.ErrUnavailable, err) //nolint:errorlint // wraps the sentinel, %v is intentional for the transport error text
	}

	valid := make(map[string]bool, len(resp.Msg.Piers))
	for _, p := range resp.Msg.Piers {
		if p.OperatorId == operatorID.String() && !p.Archived {
			valid[p.PierId] = true
		}
	}

	var bad []string
	for _, id := range pierIDs {
		if !valid[id.String()] {
			bad = append(bad, id.String())
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: pier ids not owned by operator %s or archived: %s", domain.ErrInvalidArgument, operatorID, strings.Join(bad, ", "))
	}
	return nil
}

// createUser inserts a fresh staff/pier_admin row for in.Email. If the email
// already belongs to a customer, that row is promoted in place (same user
// id, D-04 idempotency edge) instead — no UserCreated is published for a
// promotion. If the email already belongs to a staff/pier_admin/super_admin,
// both the insert and the promote attempt find no row and this returns
// ErrAlreadyExists (also the outcome when several concurrent creates race
// for the same new email: exactly one InsertStaffUser wins, every other
// caller's promote attempt also finds nothing and reports ErrAlreadyExists).
func (u *Users) createUser(ctx context.Context, in domain.StaffUserInput) (domain.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return domain.User{}, fmt.Errorf("app: new user id: %w", err)
	}

	var (
		user    domain.User
		created bool
	)
	err = bbpgx.WithTx(ctx, u.Pool, func(tx pgx.Tx) error {
		q := postgres.New(tx)

		row, insErr := q.InsertStaffUser(ctx, postgres.InsertStaffUserParams{
			ID:         toPgUUID(id),
			Email:      pgText(in.Email),
			Name:       in.Name,
			Role:       in.Role,
			OperatorID: toPgUUID(in.OperatorID),
			PierIds:    toPgUUIDs(in.PierIDs),
		})
		if errors.Is(insErr, pgx.ErrNoRows) {
			promoted, promErr := q.PromoteCustomerToStaff(ctx, postgres.PromoteCustomerToStaffParams{
				Email:      pgText(in.Email),
				Role:       in.Role,
				OperatorID: toPgUUID(in.OperatorID),
				PierIds:    toPgUUIDs(in.PierIDs),
				Name:       in.Name,
			})
			if errors.Is(promErr, pgx.ErrNoRows) {
				return domain.ErrAlreadyExists
			}
			if promErr != nil {
				return fmt.Errorf("app: promote customer to staff: %w", promErr)
			}
			user = userFromRow(promoted)
			return nil
		}
		if insErr != nil {
			return fmt.Errorf("app: insert staff user: %w", insErr)
		}

		created = true
		user = userFromRow(row)

		env, evErr := events.New(EventUserCreated, user.ID.String(), &identityv1.UserCreated{
			UserId:     user.ID.String(),
			Role:       user.Role,
			OperatorId: user.OperatorID,
			PierIds:    user.PierIDs,
		})
		if evErr != nil {
			return fmt.Errorf("app: build UserCreated event: %w", evErr)
		}
		return outbox.Insert(ctx, tx, env)
	})
	if err != nil {
		return domain.User{}, err
	}
	if created && u.Nudge != nil {
		u.Nudge()
	}
	return user, nil
}

// updateUser is implemented in Task 2 (list/update/disable).
func (u *Users) updateUser(ctx context.Context, id uuid.UUID, in domain.StaffUserInput) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

// toPgUUIDs never returns nil — pgx must send an empty array literal to
// Postgres, not NULL, matching the same rule catalog's app/pier.go applies.
func toPgUUIDs(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = toPgUUID(id)
	}
	return out
}
