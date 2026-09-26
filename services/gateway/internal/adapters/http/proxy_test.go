package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

// recordedRequest captures what a fakeUpstream saw, for tests to assert on
// the trust-boundary headers and body the admin proxy actually forwarded.
type recordedRequest struct {
	called  bool
	method  string
	path    string
	headers http.Header
	body    []byte
}

// newFakeUpstream is a raw httptest server (not a connect service) that
// records the request it received and replies with status/body — used to
// test adminProxy's rewrite/passthrough behavior independent of any real
// connect handler.
func newFakeUpstream(t *testing.T, rec *recordedRequest, status int, respBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.called = true
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.headers = r.Header.Clone()
		data, _ := io.ReadAll(r.Body)
		rec.body = data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}

// newAdminProxyServer mounts adminProxy alone on a fresh chi router.
func newAdminProxyServer(t *testing.T, v *auth.Verifier, upstreams map[string]*url.URL) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/api/v1/admin/{service}/{method}", adminProxy(v, upstreams, http.DefaultTransport, testInternalToken))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestAdminProxyForwardsVerifiedClaimsAndStripsSpoofed(t *testing.T) {
	issuer, verifier := newTestAuth(t)
	rec := &recordedRequest{}
	upstream := newFakeUpstream(t, rec, http.StatusBadRequest, `{"code":"failed_precondition","message":"x"}`)
	upstreams := map[string]*url.URL{
		catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL),
	}
	srv := newAdminProxyServer(t, verifier, upstreams)

	tok, err := issuer.Issue(auth.Claims{UserID: "user-1", OperatorID: "op-real", Role: auth.RolePierAdmin, PierIDs: []string{"pier-1"}, Kind: auth.KindAccess}, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	body := `{"name":"x"}`
	reqURL := srv.URL + "/api/v1/admin/" + catalogv1connect.CatalogServiceName + "/ListPiers"
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpx.HeaderUserID, "attacker")
	req.Header.Set(httpx.HeaderOperatorID, "attacker-op")
	req.Header.Set(httpx.HeaderRole, auth.RoleSuperAdmin)
	req.Header.Set(httpx.HeaderPierIDs, "attacker-pier")
	req.Header.Set(httpx.HeaderInternalToken, "attacker-token")
	req.Header.Set("Authorization", "Bearer attacker-bearer")
	req.AddCookie(&http.Cookie{Name: "somecookie", Value: "attacker"})
	req.AddCookie(auth.Cookie(auth.KindAccess, tok))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	data, _ := io.ReadAll(resp.Body)

	if !rec.called {
		t.Fatal("upstream was never called")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (passed through verbatim), body=%s", resp.StatusCode, data)
	}
	var errBody struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &errBody); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, data)
	}
	if errBody.Code != "failed_precondition" {
		t.Errorf("code = %q, want failed_precondition", errBody.Code)
	}

	if rec.path != "/"+catalogv1connect.CatalogServiceName+"/ListPiers" {
		t.Errorf("upstream path = %q, want the rewritten connect path", rec.path)
	}
	if string(rec.body) != body {
		t.Errorf("upstream body = %q, want %q (passed through)", rec.body, body)
	}
	if got := rec.headers.Get(httpx.HeaderUserID); got != "user-1" {
		t.Errorf("upstream saw X-User-Id = %q, want verified %q", got, "user-1")
	}
	if got := rec.headers.Get(httpx.HeaderOperatorID); got != "op-real" {
		t.Errorf("upstream saw X-Operator-Id = %q, want verified %q (not spoofed)", got, "op-real")
	}
	if got := rec.headers.Get(httpx.HeaderRole); got != auth.RolePierAdmin {
		t.Errorf("upstream saw X-Role = %q, want verified %q (not spoofed)", got, auth.RolePierAdmin)
	}
	if got := rec.headers.Get(httpx.HeaderPierIDs); got != "pier-1" {
		t.Errorf("upstream saw X-Pier-Ids = %q, want verified %q (not spoofed)", got, "pier-1")
	}
	if got := rec.headers.Get(httpx.HeaderInternalToken); got != testInternalToken {
		t.Errorf("upstream saw X-Internal-Token = %q, want gateway's own %q", got, testInternalToken)
	}
	if got := rec.headers.Get("Authorization"); got != "" {
		t.Errorf("upstream saw Authorization = %q, want stripped", got)
	}
	if got := rec.headers.Get("Cookie"); got != "" {
		t.Errorf("upstream saw Cookie = %q, want stripped", got)
	}
}

func TestAdminProxyRejects(t *testing.T) {
	issuer, verifier := newTestAuth(t)

	adminTok, err := issuer.Issue(auth.Claims{UserID: "u1", OperatorID: "op1", Role: auth.RoleStaff, Kind: auth.KindAccess}, time.Now())
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	custTok, err := issuer.Issue(auth.Claims{UserID: "u2", OperatorID: "op1", Role: auth.RoleCustomer, Kind: auth.KindAccess}, time.Now())
	if err != nil {
		t.Fatalf("issue customer token: %v", err)
	}

	newReq := func(t *testing.T, srvURL, path, contentType string, body string, cookie *http.Cookie) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, srvURL+path, strings.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		return resp
	}

	adminPath := "/api/v1/admin/" + catalogv1connect.CatalogServiceName + "/ListPiers"

	t.Run("no token 401", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		resp := newReq(t, srv.URL, adminPath, "application/json", "{}", nil)
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call")
		}
	})

	t.Run("customer 403", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		resp := newReq(t, srv.URL, adminPath, "application/json", "{}", auth.Cookie(auth.KindAccess, custTok))
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call")
		}
	})

	t.Run("unknown service 404", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		resp := newReq(t, srv.URL, "/api/v1/admin/some.other.v1.Service/ListPiers", "application/json", "{}", auth.Cookie(auth.KindAccess, adminTok))
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call")
		}
	})

	t.Run("method list 404", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		resp := newReq(t, srv.URL, "/api/v1/admin/"+catalogv1connect.CatalogServiceName+"/list", "application/json", "{}", auth.Cookie(auth.KindAccess, adminTok))
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call")
		}
	})

	t.Run("GET 405", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		resp, err := http.Get(srv.URL + adminPath)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call")
		}
	})

	t.Run("text/plain 400", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		resp := newReq(t, srv.URL, adminPath, "text/plain", "not json", auth.Cookie(auth.KindAccess, adminTok))
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call")
		}
	})

	t.Run("70 KiB body 400, upstream never called", func(t *testing.T) {
		rec := &recordedRequest{}
		upstream := newFakeUpstream(t, rec, http.StatusOK, "{}")
		upstreams := map[string]*url.URL{catalogv1connect.CatalogServiceName: mustParseURL(t, upstream.URL)}
		srv := newAdminProxyServer(t, verifier, upstreams)

		bigBody := strings.Repeat("a", 70*1024)
		resp := newReq(t, srv.URL, adminPath, "application/json", bigBody, auth.Cookie(auth.KindAccess, adminTok))
		defer resp.Body.Close() //nolint:errcheck // test helper
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
		if rec.called {
			t.Error("upstream was called, want no call for an oversized body")
		}
	})
}

func TestAdminProxyRoutesUserServiceToIdentity(t *testing.T) {
	issuer, verifier := newTestAuth(t)

	catalogRec := &recordedRequest{}
	catalogUpstream := newFakeUpstream(t, catalogRec, http.StatusOK, "{}")
	identityRec := &recordedRequest{}
	identityUpstream := newFakeUpstream(t, identityRec, http.StatusOK, "{}")

	upstreams := map[string]*url.URL{
		catalogv1connect.CatalogServiceName: mustParseURL(t, catalogUpstream.URL),
		identityServiceName:                 mustParseURL(t, identityUpstream.URL),
	}
	srv := newAdminProxyServer(t, verifier, upstreams)

	tok, err := issuer.Issue(auth.Claims{UserID: "u1", OperatorID: "op1", Role: auth.RoleSuperAdmin, Kind: auth.KindAccess}, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/admin/"+identityServiceName+"/ListUsers", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(auth.Cookie(auth.KindAccess, tok))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test helper

	if !identityRec.called {
		t.Error("identity upstream was never called")
	}
	if catalogRec.called {
		t.Error("catalog upstream was called, want only identity called")
	}
	if identityRec.path != "/"+identityServiceName+"/ListUsers" {
		t.Errorf("identity upstream path = %q, want the rewritten connect path", identityRec.path)
	}
}
