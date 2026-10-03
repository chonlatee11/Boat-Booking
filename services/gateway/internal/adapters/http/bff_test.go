package http

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chonlatee11/boat-booking/pkg/auth"
)

const testInternalToken = "test-internal-token"

func newTestAuth(t *testing.T) (*auth.Issuer, *auth.Verifier) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return auth.NewIssuer(priv, "test-issuer"), auth.NewVerifier(&priv.PublicKey, "test-issuer")
}

// unusedUpstreamURL satisfies Routes' admin/public-proxy upstream parameters
// for tests in this file that never reach either proxy (see proxy_test.go
// for those) — any well-formed *url.URL works, nothing ever dials it.
var unusedUpstreamURL = &url.URL{Scheme: "http", Host: "unused.invalid"}

// newGatewayServer wires Routes onto a fresh chi router behind an httptest
// server, mirroring how cmd/main.go mounts them (no RequireInternal — the
// gateway is the origin of trust, not a consumer of it, D-29).
func newGatewayServer(t *testing.T, v *auth.Verifier) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	Routes(r, v, http.DefaultClient, unusedUpstreamURL, unusedUpstreamURL, testInternalToken)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestWhoamiUnchanged(t *testing.T) {
	issuer, verifier := newTestAuth(t)
	srv := newGatewayServer(t, verifier)

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
	srv := newGatewayServer(t, verifier)

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
	srv := newGatewayServer(t, verifier)

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
// The legacy POST /api/v1/boats route and the typed public-boats handler are
// both gone (D-18, D-21): every catalog RPC now goes through the generic
// adminProxy/publicHandler tested in proxy_test.go.
