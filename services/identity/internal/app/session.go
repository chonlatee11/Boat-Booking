package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/clock"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// Session is the result of a successful VerifyOtp: a signed access token
// plus an opaque refresh token, and the user it was issued for.
type Session struct {
	AccessToken  string
	RefreshToken string
	User         domain.User
}

// issueSession mints an access JWT carrying user's role/operator_id/pier_ids
// (D-06) and stores a fresh opaque refresh token's sha256 hash in
// refresh_tokens inside tx — the raw token is returned but never persisted
// (D-09).
func issueSession(ctx context.Context, tx pgx.Tx, issuer *auth.Issuer, user domain.User) (Session, error) {
	now := clock.Now()

	access, err := issuer.Issue(auth.Claims{
		UserID:     user.ID.String(),
		OperatorID: user.OperatorID,
		Role:       user.Role,
		PierIDs:    user.PierIDs,
		Kind:       auth.KindAccess,
	}, now)
	if err != nil {
		return Session{}, fmt.Errorf("app: issue access token: %w", err)
	}

	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return Session{}, fmt.Errorf("app: generate refresh token: %w", err)
	}
	refreshToken := base64.RawURLEncoding.EncodeToString(rawToken)
	hash := sha256.Sum256([]byte(refreshToken))

	tokenID, err := uuid.NewV7()
	if err != nil {
		return Session{}, fmt.Errorf("app: new refresh token id: %w", err)
	}

	if err := postgres.New(tx).InsertRefreshToken(ctx, postgres.InsertRefreshTokenParams{
		ID:        toPgUUID(tokenID),
		UserID:    toPgUUID(user.ID),
		TokenHash: hash[:],
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(auth.RefreshTTL), Valid: true},
	}); err != nil {
		return Session{}, fmt.Errorf("app: insert refresh token: %w", err)
	}

	return Session{AccessToken: access, RefreshToken: refreshToken, User: user}, nil
}

// Refresh rotates token (D-10, T-02-05-02): the presented refresh token is
// revoked and, if it was valid, a fresh access+refresh pair is issued from
// the user's CURRENT role/operator_id/pier_ids/disabled state — never the
// state captured when the old token was issued. Presenting a token that was
// already rotated (or belongs to a since-disabled user) revokes every
// refresh token for that user, so a stolen-then-replayed token kills the
// whole session family, not just itself.
func (a *Auth) Refresh(ctx context.Context, rawToken string) (Session, error) {
	hash := sha256.Sum256([]byte(rawToken))

	var (
		session Session
		invalid bool
	)
	err := bbpgx.WithTx(ctx, a.Pool, func(tx pgx.Tx) error {
		q := postgres.New(tx)

		row, err := q.GetRefreshTokenForUpdate(ctx, hash[:])
		if errors.Is(err, pgx.ErrNoRows) {
			invalid = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("app: get refresh token: %w", err)
		}

		if row.RevokedAt.Valid {
			// Reuse of an already-rotated token: assume compromise, kill the
			// whole family — but the revoke itself must still commit.
			if err := q.RevokeAllRefreshTokens(ctx, row.UserID); err != nil {
				return fmt.Errorf("app: revoke all refresh tokens (reuse): %w", err)
			}
			invalid = true
			return nil
		}

		if !row.ExpiresAt.Time.After(clock.Now()) {
			invalid = true
			return nil
		}

		if err := q.RevokeRefreshToken(ctx, row.ID); err != nil {
			return fmt.Errorf("app: revoke refresh token: %w", err)
		}

		userRow, err := q.GetUser(ctx, row.UserID)
		if err != nil {
			return fmt.Errorf("app: get user: %w", err)
		}
		user := userFromRow(userRow)
		if user.DisabledAt != nil {
			if err := q.RevokeAllRefreshTokens(ctx, row.UserID); err != nil {
				return fmt.Errorf("app: revoke all refresh tokens (disabled): %w", err)
			}
			invalid = true
			return nil
		}

		sess, err := issueSession(ctx, tx, a.Issuer, user)
		if err != nil {
			return err
		}
		session = sess
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	if invalid {
		return Session{}, domain.ErrSessionInvalid
	}
	return session, nil
}

// Logout revokes rawToken's refresh_tokens row. Idempotent: an unknown or
// already-revoked token is not an error — the caller's session is gone
// either way.
func (a *Auth) Logout(ctx context.Context, rawToken string) error {
	hash := sha256.Sum256([]byte(rawToken))
	return bbpgx.WithTx(ctx, a.Pool, func(tx pgx.Tx) error {
		return postgres.New(tx).RevokeRefreshTokenByHash(ctx, hash[:])
	})
}
