package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
)

// fakeAuthServer implements identityv1connect.AuthServiceHandler, recording
// the headers the last call carried and returning whatever this test wired
// up — standing in for identity so auth.go's HTTP-shaping behavior can be
// tested without a real identity service.
type fakeAuthServer struct {
	identityv1connect.UnimplementedAuthServiceHandler
	lastHeaders   http.Header
	requestOtpErr error
	verifyOtpErr  error
	verifyOtpResp *identityv1.VerifyOtpResponse
}

func (f *fakeAuthServer) RequestOtp(_ context.Context, req *connect.Request[identityv1.RequestOtpRequest]) (*connect.Response[identityv1.RequestOtpResponse], error) {
	f.lastHeaders = req.Header().Clone()
	if f.requestOtpErr != nil {
		return nil, f.requestOtpErr
	}
	return connect.NewResponse(&identityv1.RequestOtpResponse{}), nil
}

func (f *fakeAuthServer) VerifyOtp(_ context.Context, req *connect.Request[identityv1.VerifyOtpRequest]) (*connect.Response[identityv1.VerifyOtpResponse], error) {
	f.lastHeaders = req.Header().Clone()
	if f.verifyOtpErr != nil {
		return nil, f.verifyOtpErr
	}
	return connect.NewResponse(f.verifyOtpResp), nil
}

// newAuthTestServer mounts AuthRoutes on a fresh chi router in front of fake,
// mirroring how cmd/main.go wires the real identity client.
func newAuthTestServer(t *testing.T, fake *fakeAuthServer) *httptest.Server {
	t.Helper()
	_, handler := identityv1connect.NewAuthServiceHandler(fake)
	identitySrv := httptest.NewServer(handler)
	t.Cleanup(identitySrv.Close)

	client := identityv1connect.NewAuthServiceClient(http.DefaultClient, identitySrv.URL)

	r := chi.NewRouter()
	AuthRoutes(r, client, testInternalToken)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestOtpVerifySetsCookiesAndBodyHasNoTokens(t *testing.T) {
	fake := &fakeAuthServer{verifyOtpResp: &identityv1.VerifyOtpResponse{
		AccessToken:  "the-access-token",
		RefreshToken: "the-refresh-token",
		User: &identityv1.SessionUser{
			UserId: "u1", Role: auth.RoleCustomer, OperatorId: "", PierIds: nil,
		},
	}}
	srv := newAuthTestServer(t, fake)

	resp, err := http.Post(srv.URL+"/api/v1/auth/otp/verify", "application/json",
		strings.NewReader(`{"destination":"x@example.com","code":"123456"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	cookiesByName := map[string]*http.Cookie{}
	for _, c := range resp.Cookies() {
		cookiesByName[c.Name] = c
	}
	access, ok := cookiesByName[auth.AccessCookie]
	if !ok {
		t.Fatal("no access_token cookie set")
	}
	if !access.HttpOnly || !access.Secure || access.SameSite != http.SameSiteLaxMode || access.Value != "the-access-token" {
		t.Errorf("access_token cookie = %+v, want HttpOnly+Secure+Lax with the issued token", access)
	}
	refresh, ok := cookiesByName[auth.RefreshCookie]
	if !ok {
		t.Fatal("no refresh_token cookie set")
	}
	if !refresh.HttpOnly || !refresh.Secure || refresh.SameSite != http.SameSiteLaxMode || refresh.Value != "the-refresh-token" {
		t.Errorf("refresh_token cookie = %+v, want HttpOnly+Secure+Lax with the issued token", refresh)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["accessToken"]; ok {
		t.Error("body contains accessToken — tokens must only travel in cookies")
	}
	if _, ok := body["refreshToken"]; ok {
		t.Error("body contains refreshToken — tokens must only travel in cookies")
	}
	if body["userId"] != "u1" || body["role"] != auth.RoleCustomer {
		t.Errorf("body = %+v, want userId=u1 role=customer", body)
	}
}

func TestOtpVerifyWrongCodeReturnsAttemptsLeft(t *testing.T) {
	connErr := connect.NewError(connect.CodeInvalidArgument, errString("code mismatch"))
	connErr.Meta().Set("Attempts-Left", "3")
	fake := &fakeAuthServer{verifyOtpErr: connErr}
	srv := newAuthTestServer(t, fake)

	resp, err := http.Post(srv.URL+"/api/v1/auth/otp/verify", "application/json",
		strings.NewReader(`{"destination":"x@example.com","code":"000000"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	var body struct {
		Code         string `json:"code"`
		AttemptsLeft int    `json:"attemptsLeft"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "invalid_argument" || body.AttemptsLeft != 3 {
		t.Errorf("body = %+v, want code=invalid_argument attemptsLeft=3", body)
	}
}

func TestOtpRequestResourceExhaustedReturns429(t *testing.T) {
	fake := &fakeAuthServer{requestOtpErr: connect.NewError(connect.CodeResourceExhausted, errString("resend requested too soon"))}
	srv := newAuthTestServer(t, fake)

	resp, err := http.Post(srv.URL+"/api/v1/auth/otp/request", "application/json",
		strings.NewReader(`{"destination":"x@example.com"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
}

func TestOtpRequestSpoofedClaimHeaderNeverReachesIdentity(t *testing.T) {
	fake := &fakeAuthServer{}
	srv := newAuthTestServer(t, fake)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/otp/request",
		strings.NewReader(`{"destination":"x@example.com"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "spoofed-admin")
	req.Header.Set("X-Role", "super_admin")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if fake.lastHeaders == nil {
		t.Fatal("identity was never called")
	}
	if got := fake.lastHeaders.Get("X-User-Id"); got != "" {
		t.Errorf("identity saw X-User-Id = %q, want empty (never forwarded)", got)
	}
	if got := fake.lastHeaders.Get("X-Role"); got != "" {
		t.Errorf("identity saw X-Role = %q, want empty (never forwarded)", got)
	}
}

func TestOtpRequestBodyTooLargeReturns400(t *testing.T) {
	fake := &fakeAuthServer{}
	srv := newAuthTestServer(t, fake)

	oversized := bytes.Repeat([]byte("a"), maxAuthBodyBytes+1)
	body := `{"destination":"` + string(oversized) + `"}`

	resp, err := http.Post(srv.URL+"/api/v1/auth/otp/request", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if fake.lastHeaders != nil {
		t.Error("identity was called with an oversized body, want rejected before any upstream contact")
	}
}

// errString is a tiny error type for tests that need a plain, comparable
// error message without pulling in errors.New indirection.
type errString string

func (e errString) Error() string { return string(e) }
