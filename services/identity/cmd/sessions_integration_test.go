//go:build integration

package main

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
)

// refreshTokenHash mirrors app.issueSession's sha256(token) storage key, so
// tests can reach directly into refresh_tokens by hash.
func refreshTokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// otpLogin runs the full RequestOtp -> Mailpit -> VerifyOtp loop for dest and
// returns the issued session.
func otpLogin(t *testing.T, client identityv1connect.AuthServiceClient, dest string) *identityv1.VerifyOtpResponse {
	t.Helper()
	ctx := context.Background()
	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest})); err != nil {
		t.Fatalf("RequestOtp(%s): %v", dest, err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, dest)
	resp, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: code}))
	if err != nil {
		t.Fatalf("VerifyOtp(%s): %v", dest, err)
	}
	return resp.Msg
}

// TestRefreshRotationAndReuseDetection proves single-use rotation: Refresh(A)
// issues B and revokes A; replaying A afterwards revokes the whole family, so
// even the just-issued B is rejected next (D-10 threat T-02-05-02).
func TestRefreshRotationAndReuseDetection(t *testing.T) {
	addr, verifier, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	initial := otpLogin(t, client, "refresh-rotate@example.com")

	refreshed, err := client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: initial.RefreshToken}))
	if err != nil {
		t.Fatalf("Refresh(A): %v", err)
	}
	if refreshed.Msg.RefreshToken == initial.RefreshToken {
		t.Fatal("Refresh(A) returned the same refresh token, want a fresh one")
	}
	// Note: the access token can be byte-identical to the previous one when
	// nothing in the claims changed and both are issued within the same
	// wall-clock second — RS256 (PKCS1v15) signing is deterministic for an
	// identical header+payload. The refresh token is the property that
	// proves rotation happened; TestRefreshRereadsClaimsAndDisabled proves
	// the access token DOES change once the underlying claims do.
	if _, err := verifier.Verify(refreshed.Msg.AccessToken, auth.KindAccess); err != nil {
		t.Fatalf("verify refreshed access token: %v", err)
	}

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	var revokedAt *time.Time
	if err := pool.QueryRow(ctx, `select revoked_at from refresh_tokens where token_hash = $1`,
		refreshTokenHash(initial.RefreshToken)).Scan(&revokedAt); err != nil {
		t.Fatalf("query old token revoked_at: %v", err)
	}
	if revokedAt == nil {
		t.Error("old refresh token's revoked_at is still null after rotation")
	}

	// Replaying the already-rotated token A must revoke the whole family.
	_, err = client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: initial.RefreshToken}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "replay of rotated token A")

	// B is now dead too, even though it was never itself replayed.
	_, err = client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: refreshed.Msg.RefreshToken}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "refresh of B after A was replayed")
}

// TestRefreshRereadsClaimsAndDisabled proves D-10: a role/pier change reaches
// the next access token, and a disabled user is refused at refresh time.
func TestRefreshRereadsClaimsAndDisabled(t *testing.T) {
	addr, verifier, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	pool, err := bbpgx.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	userID := uuid.Must(uuid.NewV7())
	operatorID := uuid.New()
	pierA := uuid.New()
	const email = "refresh-claims@example.com"
	if _, err := pool.Exec(context.Background(), `
		insert into users (id, email, role, operator_id, pier_ids)
		values ($1, $2, 'pier_admin', $3, $4)
	`, userID, email, operatorID, []uuid.UUID{pierA}); err != nil {
		t.Fatalf("seed staff user: %v", err)
	}

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	initial := otpLogin(t, client, email)

	pierB := uuid.New()
	if _, err := pool.Exec(ctx, `update users set pier_ids = $1 where id = $2`, []uuid.UUID{pierB}, userID); err != nil {
		t.Fatalf("update pier_ids: %v", err)
	}

	refreshed, err := client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: initial.RefreshToken}))
	if err != nil {
		t.Fatalf("Refresh after pier_ids change: %v", err)
	}
	claims, err := verifier.Verify(refreshed.Msg.AccessToken, auth.KindAccess)
	if err != nil {
		t.Fatalf("verify refreshed access token: %v", err)
	}
	if len(claims.PierIDs) != 1 || claims.PierIDs[0] != pierB.String() {
		t.Errorf("claims.PierIDs = %v, want [%s]", claims.PierIDs, pierB.String())
	}

	if _, err := pool.Exec(ctx, `update users set disabled_at = now() where id = $1`, userID); err != nil {
		t.Fatalf("disable user: %v", err)
	}

	_, err = client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: refreshed.Msg.RefreshToken}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "refresh after disable")
}

// TestRefreshExpired proves an expired or unknown refresh token is refused.
func TestRefreshExpired(t *testing.T) {
	addr, _, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	session := otpLogin(t, client, "refresh-expired@example.com")

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `update refresh_tokens set expires_at = now() - interval '1 hour' where token_hash = $1`,
		refreshTokenHash(session.RefreshToken)); err != nil {
		t.Fatalf("expire refresh token: %v", err)
	}

	_, err = client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: session.RefreshToken}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "refresh with expired token")

	_, err = client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: "unknown-token-value"}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "refresh with unknown token")
}

// TestLogoutIdempotent proves Logout revokes the token, tolerates being
// called twice, and a logged-out token can no longer be refreshed.
func TestLogoutIdempotent(t *testing.T) {
	addr, _, _ := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	session := otpLogin(t, client, "logout-idempotent@example.com")

	if _, err := client.Logout(ctx, connect.NewRequest(&identityv1.LogoutRequest{RefreshToken: session.RefreshToken})); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := client.Logout(ctx, connect.NewRequest(&identityv1.LogoutRequest{RefreshToken: session.RefreshToken})); err != nil {
		t.Fatalf("Logout (second call): %v", err)
	}

	_, err := client.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: session.RefreshToken}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "refresh after logout")
}
