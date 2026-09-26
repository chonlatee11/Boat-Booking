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

// updateUser changes name/role/operator/piers on an existing staff/pier_admin
// row (D-08). email is immutable — a value differing from the stored row is
// rejected (ErrInvalidArgument); updating a customer or super_admin row is
// rejected with ErrFailedPrecondition (D-09 — those roles are never touched
// through this API).
func (u *Users) updateUser(ctx context.Context, id uuid.UUID, in domain.StaffUserInput) (domain.User, error) {
	var user domain.User
	err := bbpgx.WithTx(ctx, u.Pool, func(tx pgx.Tx) error {
		q := postgres.New(tx)

		existing, err := q.GetUser(ctx, toPgUUID(id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("app: get user: %w", err)
		}
		stored := userFromRow(existing)
		if stored.Role != auth.RoleStaff && stored.Role != auth.RolePierAdmin {
			return domain.ErrFailedPrecondition
		}
		if stored.Email != in.Email {
			return fmt.Errorf("%w: email is immutable", domain.ErrInvalidArgument)
		}

		row, err := q.UpdateStaffUser(ctx, postgres.UpdateStaffUserParams{
			ID:         toPgUUID(id),
			Name:       in.Name,
			Role:       in.Role,
			OperatorID: toPgUUID(in.OperatorID),
			PierIds:    toPgUUIDs(in.PierIDs),
		})
		if err != nil {
			return fmt.Errorf("app: update staff user: %w", err)
		}
		user = userFromRow(row)
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// SetUserDisabled toggles a staff/pier_admin user's disabled_at (D-10). Only
// super_admin may call this; disabling the caller's own account or a
// super_admin row is rejected with ErrFailedPrecondition (T-02-07-04). When
// disabling, every refresh token for the user is revoked in the same tx, so
// a refresh fails immediately and access ends within one access-token TTL
// (<=15 min).
func (u *Users) SetUserDisabled(ctx context.Context, caller httpx.Claims, id uuid.UUID, disabled bool) (domain.User, error) {
	if caller.Role != auth.RoleSuperAdmin {
		return domain.User{}, domain.ErrPermissionDenied
	}
	if caller.UserID == id.String() {
		return domain.User{}, domain.ErrFailedPrecondition
	}

	var user domain.User
	err := bbpgx.WithTx(ctx, u.Pool, func(tx pgx.Tx) error {
		q := postgres.New(tx)

		existing, err := q.GetUser(ctx, toPgUUID(id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("app: get user: %w", err)
		}
		stored := userFromRow(existing)
		if stored.Role != auth.RoleStaff && stored.Role != auth.RolePierAdmin {
			return domain.ErrFailedPrecondition
		}

		row, err := q.SetUserDisabledAt(ctx, postgres.SetUserDisabledAtParams{ID: toPgUUID(id), Disabled: disabled})
		if err != nil {
			return fmt.Errorf("app: set user disabled: %w", err)
		}
		user = userFromRow(row)

		if disabled {
			if err := q.RevokeAllRefreshTokens(ctx, toPgUUID(id)); err != nil {
				return fmt.Errorf("app: revoke all refresh tokens: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// ListUsers returns every staff/pier_admin/super_admin user (never
// customers, PDPA minimal exposure), ordered by email then id. Only
// super_admin may call this; an empty operatorFilter returns every operator.
func (u *Users) ListUsers(ctx context.Context, caller httpx.Claims, operatorFilter uuid.UUID) ([]domain.User, error) {
	if caller.Role != auth.RoleSuperAdmin {
		return nil, domain.ErrPermissionDenied
	}

	var filter pgtype.UUID
	if operatorFilter != uuid.Nil {
		filter = toPgUUID(operatorFilter)
	}
	rows, err := postgres.New(u.Pool).ListStaffUsers(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("app: list staff users: %w", err)
	}
	users := make([]domain.User, len(rows))
	for i, row := range rows {
		users[i] = userFromRow(row)
	}
	return users, nil
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
