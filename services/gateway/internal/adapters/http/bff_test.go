package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

const testInternalToken = "test-internal-token"

// fakeCatalog is a minimal catalogv1connect.CatalogServiceHandler recording
// the last request's headers so tests can assert on the trust-boundary
// headers the gateway forwards (D-29, D-30).
type fakeCatalog struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	lastHeaders http.Header
}

func (f *fakeCatalog) ListBoats(_ context.Context, req *connect.Request[catalogv1.ListBoatsRequest]) (*connect.Response[catalogv1.ListBoatsResponse], error) {
	f.lastHeaders = req.Header()
	return connect.NewResponse(&catalogv1.ListBoatsResponse{Boats: nil}), nil
}

// newFakeCatalogServer hosts fc behind httpx.RequireInternal(testInternalToken)
// — exactly the trust boundary the real catalog service enforces — and
// returns a connect client pointed at it.
func newFakeCatalogServer(t *testing.T, fc *fakeCatalog) catalogv1connect.CatalogServiceClient {
	t.Helper()
	path, handler := catalogv1connect.NewCatalogServiceHandler(fc)
	mux := http.NewServeMux()
	mux.Handle(path, httpx.RequireInternal(testInternalToken)(handler))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return catalogv1connect.NewCatalogServiceClient(srv.Client(), srv.URL)
}

func newTestAuth(t *testing.T) (*auth.Issuer, *auth.Verifier) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return auth.NewIssuer(priv, "test-issuer"), auth.NewVerifier(&priv.PublicKey, "test-issuer")
}

// unusedUpstreamURL satisfies Routes' admin-proxy upstream parameters for
// tests in this file that never call the admin proxy (see proxy_test.go for
// those) — any well-formed *url.URL works, nothing ever dials it.
var unusedUpstreamURL = &url.URL{Scheme: "http", Host: "unused.invalid"}

// newGatewayServer wires Routes onto a fresh chi router behind an httptest
// server, mirroring how cmd/main.go mounts them (no RequireInternal — the
// gateway is the origin of trust, not a consumer of it, D-29).
func newGatewayServer(t *testing.T, v *auth.Verifier, catalog catalogv1connect.CatalogServiceClient) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	Routes(r, v, catalog, unusedUpstreamURL, unusedUpstreamURL, http.DefaultTransport, testInternalToken)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestPublicBoatsListNoAuthRequired(t *testing.T) {
	_, verifier := newTestAuth(t)
	fc := &fakeCatalog{}
	catalog := newFakeCatalogServer(t, fc)
	srv := newGatewayServer(t, verifier, catalog)

	resp, err := http.Get(srv.URL + "/api/v1/public/boats")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	boats, ok := body["boats"].([]any)
	if !ok {
		t.Fatalf(`body["boats"] = %#v (type %T), want a JSON array (empty list rendered as [], not null)`, body["boats"], body["boats"])
	}
	if len(boats) != 0 {
		t.Errorf("boats = %v, want empty", boats)
	}

	if fc.lastHeaders == nil {
		t.Fatal("fake catalog was never called")
	}
	if got := fc.lastHeaders.Get(httpx.HeaderInternalToken); got != testInternalToken {
		t.Errorf("catalog saw X-Internal-Token = %q, want %q", got, testInternalToken)
	}
	if got := fc.lastHeaders.Get(httpx.HeaderUserID); got != "" {
		t.Errorf("catalog saw X-User-Id = %q, want empty (no claims for a public route)", got)
	}
}

func TestWhoamiUnchanged(t *testing.T) {
	issuer, verifier := newTestAuth(t)
	fc := &fakeCatalog{}
	catalog := newFakeCatalogServer(t, fc)
	srv := newGatewayServer(t, verifier, catalog)

	tok, err := issuer.Issue(auth.Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin", Kind: auth.KindAccess}, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/whoami", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(auth.Cookie(auth.KindAccess, tok))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out struct {
		Sub        string   `json:"sub"`
		OperatorID string   `json:"operator_id"`
		Role       string   `json:"role"`
		PierIDs    []string `json:"pier_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Sub != "u1" || out.OperatorID != "op1" || out.Role != "pier_admin" {
		t.Errorf("whoami body = %+v, want sub=u1 operator_id=op1 role=pier_admin", out)
	}
}

func TestWhoamiReturnsPierIDs(t *testing.T) {
	issuer, verifier := newTestAuth(t)
	fc := &fakeCatalog{}
	catalog := newFakeCatalogServer(t, fc)
	srv := newGatewayServer(t, verifier, catalog)

	tok, err := issuer.Issue(auth.Claims{
		UserID: "u1", OperatorID: "op1", Role: "pier_admin",
		PierIDs: []string{"00000000-0000-0000-0000-0000000000b1", "00000000-0000-0000-0000-0000000000b2"},
		Kind:    auth.KindAccess,
	}, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/whoami", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(auth.Cookie(auth.KindAccess, tok))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out struct {
		PierIDs []string `json:"pier_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"00000000-0000-0000-0000-0000000000b1", "00000000-0000-0000-0000-0000000000b2"}
	if len(out.PierIDs) != len(want) || out.PierIDs[0] != want[0] || out.PierIDs[1] != want[1] {
		t.Fatalf("pier_ids = %v, want %v", out.PierIDs, want)
	}
}

func TestWhoamiReturnsEmptyPierIDsArray(t *testing.T) {
	issuer, verifier := newTestAuth(t)
	fc := &fakeCatalog{}
	catalog := newFakeCatalogServer(t, fc)
	srv := newGatewayServer(t, verifier, catalog)

	tok, err := issuer.Issue(auth.Claims{UserID: "u1", OperatorID: "op1", Role: "customer", Kind: auth.KindAccess}, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/whoami", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(auth.Cookie(auth.KindAccess, tok))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pierIDs, ok := out["pier_ids"].([]any)
	if !ok {
		t.Fatalf(`out["pier_ids"] = %#v (type %T), want a JSON array`, out["pier_ids"], out["pier_ids"])
	}
	if len(pierIDs) != 0 {
		t.Errorf("pier_ids = %v, want empty", pierIDs)
	}
}

// ForwardClaims itself now lives in pkg/httpx (moved, D-06) — its unit tests
// (TestForwardClaimsSetsVerifiedValues, TestForwardClaimsOverwritesSpoofedHeaders,
// TestForwardClaimsEmptyPierIDsSetsNoHeader) live in pkg/httpx/claims_test.go.
// The legacy POST /api/v1/boats route and its tests are gone (D-18): admin
// writes now go through the generic adminProxy tested in proxy_test.go.
