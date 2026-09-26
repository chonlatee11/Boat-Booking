package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// EnsureSuperAdmin idempotently makes rawEmail a non-disabled super_admin
// (D-09): insert a fresh row, or promote/re-enable an existing one. Safe to
// call on every startup — a repeat call neither duplicates the row nor
// re-publishes identity.UserCreated (only a brand-new insert does). There is
// no other path (API, CLI, or otherwise) that can mint a super_admin.
func EnsureSuperAdmin(ctx context.Context, pool *pgxpool.Pool, rawEmail string) error {
	dest, err := domain.NormalizeDestination(rawEmail)
	if err != nil {
		return fmt.Errorf("app: normalize SUPER_ADMIN_EMAIL: %w", err)
	}
	if dest.Kind != domain.KindEmail {
		return fmt.Errorf("app: SUPER_ADMIN_EMAIL must be an email address, got %q", rawEmail)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("app: new super admin id: %w", err)
	}

	return bbpgx.WithTx(ctx, pool, func(tx pgx.Tx) error {
		q := postgres.New(tx)
		row, err := q.UpsertSuperAdmin(ctx, postgres.UpsertSuperAdminParams{ID: toPgUUID(id), Email: pgText(dest.Value)})
		if err != nil {
			return fmt.Errorf("app: upsert super admin: %w", err)
		}
		if !row.Inserted {
			// Promoted or re-enabled an existing row — its next refresh
			// re-reads the new role (D-10); no fresh identity to announce.
			return nil
		}

		userID := fromPgUUID(row.ID)
		env, evErr := events.New(EventUserCreated, userID.String(), &identityv1.UserCreated{
			UserId: userID.String(),
			Role:   auth.RoleSuperAdmin,
		})
		if evErr != nil {
			return fmt.Errorf("app: build UserCreated event: %w", evErr)
		}
		if evErr := outbox.Insert(ctx, tx, env); evErr != nil {
			return fmt.Errorf("app: outbox insert: %w", evErr)
		}
		return nil
	})
}
