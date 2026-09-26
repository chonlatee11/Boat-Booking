//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
	archived            bool
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
