//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/pkg/testenv"
	"github.com/chonlatee11/boat-booking/services/identity/internal/app"
)

// TestEnsureSuperAdminIdempotent proves D-09: EnsureSuperAdmin makes
// SUPER_ADMIN_EMAIL a non-disabled super_admin, promotes (not duplicates) a
// pre-existing row for that email, re-enables a disabled super_admin, and
// never publishes a second identity.UserCreated for the same email.
func TestEnsureSuperAdminIdempotent(t *testing.T) {
	ctx := context.Background()
	dsn := testenv.NewDB(t, adminDSN, "../migrations")
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	const email = "bootstrap-admin@example.com"

	if err := app.EnsureSuperAdmin(ctx, pool, email); err != nil {
		t.Fatalf("first EnsureSuperAdmin: %v", err)
	}
	if err := app.EnsureSuperAdmin(ctx, pool, email); err != nil {
		t.Fatalf("second EnsureSuperAdmin (idempotent call): %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `select count(*) from users where email = $1`, email).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users count for %s = %d, want 1 (no duplicate row)", email, count)
	}

	var role string
	var disabled bool
	if err := pool.QueryRow(ctx, `select role, disabled_at is not null from users where email = $1`, email).Scan(&role, &disabled); err != nil {
		t.Fatalf("query super admin row: %v", err)
	}
	if role != "super_admin" || disabled {
		t.Fatalf("role=%q disabled=%v, want role=super_admin disabled=false", role, disabled)
	}

	var eventCount int
	if err := pool.QueryRow(ctx, `select count(*) from outbox where event_type = 'identity.UserCreated'`).Scan(&eventCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("identity.UserCreated count = %d, want 1 (two calls must not double-publish)", eventCount)
	}

	// A pre-existing non-super-admin row for a different email is promoted,
	// not duplicated, and promoting must not publish a second event.
	const promoteEmail = "bootstrap-promote@example.com"
	promoteID := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `insert into users (id, email, role) values ($1, $2, 'customer')`, promoteID, promoteEmail); err != nil {
		t.Fatalf("seed customer row: %v", err)
	}
	if err := app.EnsureSuperAdmin(ctx, pool, promoteEmail); err != nil {
		t.Fatalf("EnsureSuperAdmin(promote): %v", err)
	}
	var promotedRole string
	var promotedOperatorNull bool
	var promotedPierCount int
	if err := pool.QueryRow(ctx, `select role, operator_id is null, cardinality(pier_ids) from users where id = $1`, promoteID).
		Scan(&promotedRole, &promotedOperatorNull, &promotedPierCount); err != nil {
		t.Fatalf("query promoted row: %v", err)
	}
	if promotedRole != "super_admin" || !promotedOperatorNull || promotedPierCount != 0 {
		t.Fatalf("promoted row = role=%q operatorNull=%v pierCount=%d, want super_admin/true/0",
			promotedRole, promotedOperatorNull, promotedPierCount)
	}
	if err := pool.QueryRow(ctx, `select count(*) from outbox where event_type = 'identity.UserCreated'`).Scan(&eventCount); err != nil {
		t.Fatalf("count outbox after promote: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("identity.UserCreated count after promoting an existing row = %d, want 1 (unchanged)", eventCount)
	}

	// A disabled super_admin is re-enabled by the next EnsureSuperAdmin call.
	if _, err := pool.Exec(ctx, `update users set disabled_at = now() where email = $1`, email); err != nil {
		t.Fatalf("disable super admin: %v", err)
	}
	if err := app.EnsureSuperAdmin(ctx, pool, email); err != nil {
		t.Fatalf("EnsureSuperAdmin(re-enable): %v", err)
	}
	if err := pool.QueryRow(ctx, `select disabled_at is not null from users where email = $1`, email).Scan(&disabled); err != nil {
		t.Fatalf("query re-enabled row: %v", err)
	}
	if disabled {
		t.Error("super admin is still disabled after EnsureSuperAdmin, want re-enabled")
	}
}

// TestSuperAdminLoginViaOtp proves AUTH-02: signing in as SUPER_ADMIN_EMAIL
// through the normal OTP flow yields a session with role super_admin and
// empty pier_ids — the bootstrap runs automatically at service startup, no
// separate setup step needed.
func TestSuperAdminLoginViaOtp(t *testing.T) {
	addr, verifier, _ := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	const email = "super-admin-default@example.com" // matches setIdentityEnv's SUPER_ADMIN_EMAIL
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	session := otpLogin(t, client, email)

	claims, err := verifier.Verify(session.AccessToken, auth.KindAccess)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if claims.Role != "super_admin" {
		t.Errorf("claims.Role = %q, want super_admin", claims.Role)
	}
	if len(claims.PierIDs) != 0 {
		t.Errorf("claims.PierIDs = %v, want empty", claims.PierIDs)
	}
}
