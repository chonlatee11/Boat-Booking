package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/clock"
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
