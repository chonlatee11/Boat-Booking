//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
)

const fakeCatalogToken = "test-token" // matches setIdentityEnv's INTERNAL_TOKEN — one shared secret, D-30.

// fakeCatalogPier is one pier served by the fake catalog upstream that
// UserService.UpsertUser validates piers against (research Pattern 3) — no
// real catalog service is involved in these tests.
type fakeCatalogPier struct {
	pierID, operatorID string
	archived           bool
}

// fakeCatalog implements just enough of catalogv1connect.CatalogServiceHandler
// (ListPiers, filtered by operator_id like the real one) to drive
// UpsertUser's pier-validation call.
type fakeCatalog struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	piers []fakeCatalogPier

	mu          sync.Mutex
	lastHeaders http.Header
}

func (f *fakeCatalog) ListPiers(_ context.Context, req *connect.Request[catalogv1.ListPiersRequest]) (*connect.Response[catalogv1.ListPiersResponse], error) {
	f.mu.Lock()
	f.lastHeaders = req.Header().Clone()
	f.mu.Unlock()

	resp := &catalogv1.ListPiersResponse{}
	for _, p := range f.piers {
		if p.operatorID != req.Msg.OperatorId {
			continue
		}
		resp.Piers = append(resp.Piers, &catalogv1.Pier{
			PierId:     p.pierID,
			OperatorId: p.operatorID,
			Archived:   p.archived,
		})
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeCatalog) headersSeen() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastHeaders
}

// startFakeCatalog serves fc's CatalogService handler behind
// httpx.RequireInternal(token) — the same trust boundary every real service
// mounts its sync API behind.
func startFakeCatalog(t *testing.T, fc *fakeCatalog) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Use(httpx.RequireInternal(fakeCatalogToken))
	path, handler := catalogv1connect.NewCatalogServiceHandler(fc)
	r.Mount(path, handler)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func newUserClient(baseURL string, headers map[string]string) identityv1connect.UserServiceClient {
	return identityv1connect.NewUserServiceClient(http.DefaultClient, baseURL,
		connect.WithInterceptors(headerInterceptor{headers: headers}))
}

func internalHeaders() map[string]string {
	return map[string]string{httpx.HeaderInternalToken: fakeCatalogToken}
}

func superAdminHeaders() map[string]string {
	h := internalHeaders()
	h[httpx.HeaderUserID] = uuid.NewString()
	h[httpx.HeaderRole] = auth.RoleSuperAdmin
	return h
}

// clearOtpCooldown deletes dest's D-03 60s resend-cooldown key directly in
// Valkey, so a test can send a second OTP to the same destination
// immediately instead of sleeping (same pattern as
// TestOtpCooldownAndHourlyLimit in main_integration_test.go).
func clearOtpCooldown(t *testing.T, dest string) {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: valkeyAddr})
	defer rdb.Close() //nolint:errcheck // test helper, nothing actionable
	if err := rdb.Del(context.Background(), "otp:cooldown:"+testDestHash(dest)).Err(); err != nil {
		t.Fatalf("clear otp cooldown: %v", err)
	}
}

func pierAdminHeaders(operatorID string) map[string]string {
	h := internalHeaders()
	h[httpx.HeaderUserID] = uuid.NewString()
	h[httpx.HeaderRole] = auth.RolePierAdmin
	h[httpx.HeaderOperatorID] = operatorID
	return h
}

// TestCreateStaffUserValidatesPiers proves AUTH-04's create path: only
// super_admin may call UpsertUser, and every requested pier must belong to
// the target operator and be non-archived (T-02-07-01, T-02-07-03) — a fresh
// insert publishes identity.UserCreated with no email/name in the payload
// (T-02-07-05).
func TestCreateStaffUserValidatesPiers(t *testing.T) {
	addr, _, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr

	opA, opB := uuid.New(), uuid.New()
	p1, p2Archived, p3OtherOperator := uuid.New(), uuid.New(), uuid.New()

	fc := &fakeCatalog{piers: []fakeCatalogPier{
		{pierID: p1.String(), operatorID: opA.String(), archived: false},
		{pierID: p2Archived.String(), operatorID: opA.String(), archived: true},
		{pierID: p3OtherOperator.String(), operatorID: opB.String(), archived: false},
	}}
	catalogSrv := startFakeCatalog(t, fc)
	t.Setenv("CATALOG_URL", catalogSrv.URL)

	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newUserClient(baseURL, superAdminHeaders())

	t.Run("valid pier creates staff and publishes UserCreated with no PII", func(t *testing.T) {
		resp, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email:      "staff1@example.com",
			Name:       "Staff One",
			Role:       auth.RoleStaff,
			OperatorId: opA.String(),
			PierIds:    []string{p1.String()},
		}))
		if err != nil {
			t.Fatalf("UpsertUser: %v", err)
		}
		if resp.Msg.User.Role != auth.RoleStaff || resp.Msg.User.OperatorId != opA.String() {
			t.Fatalf("UpsertUser response = %+v, want role=staff operator_id=%s", resp.Msg.User, opA)
		}

		if got := fc.headersSeen().Get(httpx.HeaderRole); got != auth.RoleSuperAdmin {
			t.Errorf("fake catalog saw X-Role = %q, want %q", got, auth.RoleSuperAdmin)
		}
		if got := fc.headersSeen().Get(httpx.HeaderInternalToken); got != fakeCatalogToken {
			t.Errorf("fake catalog saw internal token = %q, want %q", got, fakeCatalogToken)
		}

		pool, err := bbpgx.NewPool(ctx, dsn)
		if err != nil {
			t.Fatalf("new verification pool: %v", err)
		}
		defer pool.Close()
		waitFor(t, 15*time.Second, func() bool {
			var count int
			qErr := pool.QueryRow(ctx, `select count(*) from outbox where event_type = $1 and aggregate_id = $2`,
				"identity.UserCreated", resp.Msg.User.UserId).Scan(&count)
			return qErr == nil && count == 1
		})
	})

	t.Run("pier of another operator rejected", func(t *testing.T) {
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: "staff2@example.com", Name: "Staff Two", Role: auth.RoleStaff,
			OperatorId: opA.String(), PierIds: []string{p3OtherOperator.String()},
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument, "pier of another operator")
		if err != nil && !strings.Contains(err.Error(), p3OtherOperator.String()) {
			t.Errorf("error %v does not mention offending pier id %s", err, p3OtherOperator)
		}
	})

	t.Run("archived pier rejected", func(t *testing.T) {
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: "staff3@example.com", Name: "Staff Three", Role: auth.RoleStaff,
			OperatorId: opA.String(), PierIds: []string{p2Archived.String()},
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument, "archived pier")
	})

	t.Run("empty pier_ids rejected", func(t *testing.T) {
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: "staff4@example.com", Name: "Staff Four", Role: auth.RoleStaff,
			OperatorId: opA.String(), PierIds: nil,
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument, "empty pier_ids")
	})

	t.Run("role super_admin rejected", func(t *testing.T) {
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: "staff5@example.com", Name: "Staff Five", Role: auth.RoleSuperAdmin,
			OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument, "role super_admin")
	})

	t.Run("pier_admin caller gets PermissionDenied", func(t *testing.T) {
		pierAdminClient := newUserClient(baseURL, pierAdminHeaders(opA.String()))
		_, err := pierAdminClient.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: "staff6@example.com", Name: "Staff Six", Role: auth.RoleStaff,
			OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		assertConnectCode(t, err, connect.CodePermissionDenied, "pier_admin caller")
	})

	t.Run("no claims gets Unauthenticated", func(t *testing.T) {
		noClaimsClient := newUserClient(baseURL, internalHeaders())
		_, err := noClaimsClient.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: "staff7@example.com", Name: "Staff Seven", Role: auth.RoleStaff,
			OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		assertConnectCode(t, err, connect.CodeUnauthenticated, "no claims")
	})
}

// TestStaffLoginCarriesAssignedClaims proves the full AUTH-02 + AUTH-04
// loop: a staff member created by super_admin via UpsertUser signs in with
// OTP and receives a JWT carrying exactly that role/operator/pier.
func TestStaffLoginCarriesAssignedClaims(t *testing.T) {
	addr, verifier, _ := setIdentityEnv(t)
	baseURL := "http://" + addr

	opA := uuid.New()
	p1 := uuid.New()
	fc := &fakeCatalog{piers: []fakeCatalogPier{{pierID: p1.String(), operatorID: opA.String(), archived: false}}}
	catalogSrv := startFakeCatalog(t, fc)
	t.Setenv("CATALOG_URL", catalogSrv.URL)

	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	userClient := newUserClient(baseURL, superAdminHeaders())
	const email = "staff-login@example.com"

	if _, err := userClient.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
		Email: email, Name: "Staff Login", Role: auth.RoleStaff,
		OperatorId: opA.String(), PierIds: []string{p1.String()},
	})); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	authClient := newAuthClient(baseURL, map[string]string{httpx.HeaderInternalToken: fakeCatalogToken})
	if _, err := authClient.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: email})); err != nil {
		t.Fatalf("RequestOtp: %v", err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, email)

	resp, err := authClient.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: email, Code: code}))
	if err != nil {
		t.Fatalf("VerifyOtp: %v", err)
	}

	claims, err := verifier.Verify(resp.Msg.AccessToken, auth.KindAccess)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if claims.Role != auth.RoleStaff {
		t.Errorf("claims.Role = %q, want %q", claims.Role, auth.RoleStaff)
	}
	if claims.OperatorID != opA.String() {
		t.Errorf("claims.OperatorID = %q, want %q", claims.OperatorID, opA.String())
	}
	if len(claims.PierIDs) != 1 || claims.PierIDs[0] != p1.String() {
		t.Errorf("claims.PierIDs = %v, want [%s]", claims.PierIDs, p1)
	}
}

// TestUpsertUserIdempotencyAndConcurrency proves the AUTH-04 idempotency and
// concurrency edges (D-04-style race handling reused for staff creation):
// re-creating the same email is rejected, a customer email is promoted in
// place, and concurrent creates for one new email yield exactly one winner.
func TestUpsertUserIdempotencyAndConcurrency(t *testing.T) {
	addr, _, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr

	opA := uuid.New()
	p1 := uuid.New()
	fc := &fakeCatalog{piers: []fakeCatalogPier{{pierID: p1.String(), operatorID: opA.String(), archived: false}}}
	catalogSrv := startFakeCatalog(t, fc)
	t.Setenv("CATALOG_URL", catalogSrv.URL)

	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newUserClient(baseURL, superAdminHeaders())

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close() //nolint:errcheck // test helper, nothing actionable

	countUsers := func(email string) int {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from users where email = $1`, email).Scan(&n); err != nil {
			t.Fatalf("count users: %v", err)
		}
		return n
	}

	t.Run("re-creating the same email is rejected", func(t *testing.T) {
		const email = "dup@example.com"
		req := func() *connect.Request[identityv1.UpsertUserRequest] {
			return connect.NewRequest(&identityv1.UpsertUserRequest{
				Email: email, Name: "Dup", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
			})
		}
		if _, err := client.UpsertUser(ctx, req()); err != nil {
			t.Fatalf("first UpsertUser: %v", err)
		}
		_, err := client.UpsertUser(ctx, req())
		assertConnectCode(t, err, connect.CodeAlreadyExists, "re-created email")
		if n := countUsers(email); n != 1 {
			t.Errorf("users count for %s = %d, want 1", email, n)
		}
	})

	t.Run("existing customer email is promoted in place", func(t *testing.T) {
		const email = "promote@example.com"
		customerID := uuid.Must(uuid.NewV7())
		if _, err := pool.Exec(ctx, `insert into users (id, email, role) values ($1, $2, 'customer')`, customerID, email); err != nil {
			t.Fatalf("seed customer: %v", err)
		}

		resp, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			Email: email, Name: "Promoted", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		if err != nil {
			t.Fatalf("UpsertUser (promote): %v", err)
		}
		if resp.Msg.User.UserId != customerID.String() {
			t.Errorf("UpsertUser promote kept id = %q, want %q (same row)", resp.Msg.User.UserId, customerID)
		}
		if resp.Msg.User.Role != auth.RoleStaff {
			t.Errorf("UpsertUser promote role = %q, want %q", resp.Msg.User.Role, auth.RoleStaff)
		}
		if n := countUsers(email); n != 1 {
			t.Errorf("users count for %s = %d, want 1", email, n)
		}

		var eventCount int
		if err := pool.QueryRow(ctx, `select count(*) from outbox where event_type = $1 and aggregate_id = $2`,
			"identity.UserCreated", customerID.String()).Scan(&eventCount); err != nil {
			t.Fatalf("count outbox: %v", err)
		}
		if eventCount != 0 {
			t.Errorf("outbox UserCreated count for promoted customer = %d, want 0", eventCount)
		}
	})

	t.Run("concurrent creates for one new email yield exactly one winner", func(t *testing.T) {
		const email = "concurrent@example.com"
		const n = 5
		var wg sync.WaitGroup
		var successCount, alreadyExistsCount int64
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
					Email: email, Name: "Concurrent", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
				}))
				switch {
				case err == nil:
					atomic.AddInt64(&successCount, 1)
				case connect.CodeOf(err) == connect.CodeAlreadyExists:
					atomic.AddInt64(&alreadyExistsCount, 1)
				default:
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()

		if successCount != 1 {
			t.Errorf("successCount = %d, want 1", successCount)
		}
		if alreadyExistsCount != n-1 {
			t.Errorf("alreadyExistsCount = %d, want %d", alreadyExistsCount, n-1)
		}
		if got := countUsers(email); got != 1 {
			t.Errorf("users count for %s = %d, want 1", email, got)
		}
	})
}

// TestUpdateAndDisableUser proves the D-10 update/disable guards: update
// re-validates piers and keeps email immutable; a customer/super_admin row
// can never be touched; disabling revokes refresh immediately and blocks
// login, and self/super_admin disable is rejected (T-02-07-01, T-02-07-04,
// T-02-07-06).
func TestUpdateAndDisableUser(t *testing.T) {
	addr, _, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr

	opA, opB := uuid.New(), uuid.New()
	p1, p2, p3 := uuid.New(), uuid.New(), uuid.New()
	fc := &fakeCatalog{piers: []fakeCatalogPier{
		{pierID: p1.String(), operatorID: opA.String(), archived: false},
		{pierID: p2.String(), operatorID: opA.String(), archived: false},
		{pierID: p3.String(), operatorID: opB.String(), archived: false},
	}}
	catalogSrv := startFakeCatalog(t, fc)
	t.Setenv("CATALOG_URL", catalogSrv.URL)

	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	caller := superAdminHeaders()
	client := newUserClient(baseURL, caller)

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close() //nolint:errcheck // test helper, nothing actionable

	const email = "update-disable@example.com"
	created, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
		Email: email, Name: "Original Name", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	userID := created.Msg.User.UserId

	t.Run("update changes name/role/operator/piers", func(t *testing.T) {
		resp, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			UserId: userID, Email: email, Name: "New Name", Role: auth.RolePierAdmin, OperatorId: opB.String(), PierIds: []string{p3.String()},
		}))
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if resp.Msg.User.Name != "New Name" || resp.Msg.User.Role != auth.RolePierAdmin || resp.Msg.User.OperatorId != opB.String() {
			t.Errorf("update result = %+v, want name=New Name role=pier_admin operator_id=%s", resp.Msg.User, opB)
		}
		if len(resp.Msg.User.PierIds) != 1 || resp.Msg.User.PierIds[0] != p3.String() {
			t.Errorf("update pier_ids = %v, want [%s]", resp.Msg.User.PierIds, p3)
		}

		// Revert to opA/p1/staff for the remaining sub-tests.
		if _, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			UserId: userID, Email: email, Name: "Original Name", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
		})); err != nil {
			t.Fatalf("revert update: %v", err)
		}
	})

	t.Run("changing email on update is rejected", func(t *testing.T) {
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			UserId: userID, Email: "different@example.com", Name: "Original Name", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument, "email change on update")
	})

	t.Run("updating a customer row is rejected", func(t *testing.T) {
		customerID := uuid.Must(uuid.NewV7())
		const customerEmail = "customer-cannot-update@example.com"
		if _, err := pool.Exec(ctx, `insert into users (id, email, role) values ($1, $2, 'customer')`, customerID, customerEmail); err != nil {
			t.Fatalf("seed customer: %v", err)
		}
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			UserId: customerID.String(), Email: customerEmail, Name: "Nope", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition, "update customer row")
	})

	t.Run("updating a super_admin row is rejected", func(t *testing.T) {
		var superAdminID, superAdminEmail string
		if err := pool.QueryRow(ctx, `select id, email from users where role = 'super_admin' limit 1`).Scan(&superAdminID, &superAdminEmail); err != nil {
			t.Fatalf("find bootstrap super_admin: %v", err)
		}
		_, err := client.UpsertUser(ctx, connect.NewRequest(&identityv1.UpsertUserRequest{
			UserId: superAdminID, Email: superAdminEmail, Name: "Nope", Role: auth.RoleStaff, OperatorId: opA.String(), PierIds: []string{p1.String()},
		}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition, "update super_admin row")
	})

	t.Run("disabling self is rejected", func(t *testing.T) {
		selfID := caller[httpx.HeaderUserID]
		_, err := client.SetUserDisabled(ctx, connect.NewRequest(&identityv1.SetUserDisabledRequest{UserId: selfID, Disabled: true}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition, "disabling self")
	})

	t.Run("disabling a super_admin is rejected", func(t *testing.T) {
		var superAdminID string
		if err := pool.QueryRow(ctx, `select id from users where role = 'super_admin' limit 1`).Scan(&superAdminID); err != nil {
			t.Fatalf("find bootstrap super_admin: %v", err)
		}
		_, err := client.SetUserDisabled(ctx, connect.NewRequest(&identityv1.SetUserDisabledRequest{UserId: superAdminID, Disabled: true}))
		assertConnectCode(t, err, connect.CodeFailedPrecondition, "disabling super_admin")
	})

	t.Run("disable revokes refresh and blocks login; re-enable restores it", func(t *testing.T) {
		authClient := newAuthClient(baseURL, map[string]string{httpx.HeaderInternalToken: fakeCatalogToken})

		if _, err := authClient.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: email})); err != nil {
			t.Fatalf("RequestOtp: %v", err)
		}
		code := pollMailpitCode(t, mailpitAPIURL, email)
		loginResp, err := authClient.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: email, Code: code}))
		if err != nil {
			t.Fatalf("VerifyOtp: %v", err)
		}
		refreshToken := loginResp.Msg.RefreshToken

		if _, err := client.SetUserDisabled(ctx, connect.NewRequest(&identityv1.SetUserDisabledRequest{UserId: userID, Disabled: true})); err != nil {
			t.Fatalf("SetUserDisabled(true): %v", err)
		}

		_, err = authClient.Refresh(ctx, connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: refreshToken}))
		assertConnectCode(t, err, connect.CodeUnauthenticated, "refresh after disable")

		clearOtpCooldown(t, email)
		if _, err := authClient.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: email})); err != nil {
			t.Fatalf("RequestOtp (disabled): %v", err)
		}
		code2 := pollMailpitCode(t, mailpitAPIURL, email)
		_, err = authClient.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: email, Code: code2}))
		assertConnectCode(t, err, connect.CodePermissionDenied, "VerifyOtp while disabled")

		if _, err := client.SetUserDisabled(ctx, connect.NewRequest(&identityv1.SetUserDisabledRequest{UserId: userID, Disabled: false})); err != nil {
			t.Fatalf("SetUserDisabled(false): %v", err)
		}

		clearOtpCooldown(t, email)
		if _, err := authClient.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: email})); err != nil {
			t.Fatalf("RequestOtp (re-enabled): %v", err)
		}
		code3 := pollMailpitCode(t, mailpitAPIURL, email)
		if _, err := authClient.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: email, Code: code3})); err != nil {
			t.Fatalf("VerifyOtp (re-enabled): %v", err)
		}
	})

	t.Run("ListUsers excludes customers, orders by email then id, and filters by operator", func(t *testing.T) {
		resp, err := client.ListUsers(ctx, connect.NewRequest(&identityv1.ListUsersRequest{}))
		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		for _, u := range resp.Msg.Users {
			if u.Role == auth.RoleCustomer {
				t.Fatalf("ListUsers returned a customer: %+v", u)
			}
		}
		for i := 1; i < len(resp.Msg.Users); i++ {
			prev, cur := resp.Msg.Users[i-1], resp.Msg.Users[i]
			if prev.Email > cur.Email || (prev.Email == cur.Email && prev.UserId > cur.UserId) {
				t.Fatalf("ListUsers not ordered by email,id: %q(%s) before %q(%s)", prev.Email, prev.UserId, cur.Email, cur.UserId)
			}
		}

		filtered, err := client.ListUsers(ctx, connect.NewRequest(&identityv1.ListUsersRequest{OperatorId: opA.String()}))
		if err != nil {
			t.Fatalf("ListUsers (filtered): %v", err)
		}
		for _, u := range filtered.Msg.Users {
			if u.OperatorId != opA.String() {
				t.Errorf("ListUsers(operator_id=%s) returned user with operator_id=%s", opA, u.OperatorId)
			}
		}
	})
}
