package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newNextHandler(t *testing.T, called *bool, gotClaims *Claims, gotOK *bool) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		c, ok := FromContext(r.Context())
		*gotClaims = c
		*gotOK = ok
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireInternalMissingToken(t *testing.T) {
	var called bool
	var c Claims
	var ok bool
	h := RequireInternal("secret")(newNextHandler(t, &called, &c, &ok))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("next handler ran despite missing internal token")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["code"] != "unauthenticated" {
		t.Fatalf("code = %q, want %q", body["code"], "unauthenticated")
	}
}

func TestRequireInternalWrongToken(t *testing.T) {
	var called bool
	var c Claims
	var ok bool
	h := RequireInternal("secret")(newNextHandler(t, &called, &c, &ok))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderInternalToken, "wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("next handler ran despite wrong internal token")
	}
}

func TestRequireInternalCorrectTokenNoUserID(t *testing.T) {
	var called bool
	var c Claims
	var ok bool
	h := RequireInternal("secret")(newNextHandler(t, &called, &c, &ok))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderInternalToken, "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !called {
		t.Fatal("next handler did not run")
	}
	if ok {
		t.Fatal("FromContext ok = true, want false (no X-User-Id header)")
	}
}

func TestRequireInternalCorrectTokenWithClaims(t *testing.T) {
	var called bool
	var c Claims
	var ok bool
	h := RequireInternal("secret")(newNextHandler(t, &called, &c, &ok))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderInternalToken, "secret")
	req.Header.Set(HeaderUserID, "u1")
	req.Header.Set(HeaderOperatorID, "op1")
	req.Header.Set(HeaderRole, "pier_admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !ok {
		t.Fatal("FromContext ok = false, want true")
	}
	want := Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin"}
	if c != want {
		t.Fatalf("claims = %+v, want %+v", c, want)
	}
}

func TestRequireClaimsRejectsMissing(t *testing.T) {
	var called bool
	h := RequireClaims(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("next handler ran despite missing claims")
	}
}

func TestRequireClaimsPassesWithClaims(t *testing.T) {
	var called bool
	h := RequireClaims(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithClaims(req.Context(), Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !called {
		t.Fatal("next handler did not run despite claims present")
	}
}
