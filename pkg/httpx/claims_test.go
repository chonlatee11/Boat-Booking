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
	if c.UserID != "u1" || c.OperatorID != "op1" || c.Role != "pier_admin" {
		t.Fatalf("claims = %+v, want UserID=u1 OperatorID=op1 Role=pier_admin", c)
	}
	if len(c.PierIDs) != 0 {
		t.Fatalf("PierIDs = %v, want empty (no X-Pier-Ids header)", c.PierIDs)
	}
}

func TestRequireInternalValidPierIDsHeader(t *testing.T) {
	var called bool
	var c Claims
	var ok bool
	h := RequireInternal("secret")(newNextHandler(t, &called, &c, &ok))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderInternalToken, "secret")
	req.Header.Set(HeaderUserID, "u1")
	req.Header.Set(HeaderPierIDs, "00000000-0000-0000-0000-0000000000b1,00000000-0000-0000-0000-0000000000b2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := []string{"00000000-0000-0000-0000-0000000000b1", "00000000-0000-0000-0000-0000000000b2"}
	if len(c.PierIDs) != len(want) || c.PierIDs[0] != want[0] || c.PierIDs[1] != want[1] {
		t.Fatalf("PierIDs = %v, want %v", c.PierIDs, want)
	}
	if !ok {
		t.Fatal("FromContext ok = false, want true")
	}
}

func TestRequireInternalInvalidPierIDsHeaderRejects(t *testing.T) {
	var called bool
	var c Claims
	var ok bool
	h := RequireInternal("secret")(newNextHandler(t, &called, &c, &ok))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderInternalToken, "secret")
	req.Header.Set(HeaderUserID, "u1")
	req.Header.Set(HeaderPierIDs, "00000000-0000-0000-0000-0000000000b1,not-a-uuid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("next handler ran despite invalid X-Pier-Ids part")
	}
}

func TestForwardClaimsSetsVerifiedValues(t *testing.T) {
	h := http.Header{}
	c := Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin", PierIDs: []string{"pier-a", "pier-b"}}
	ForwardClaims(h, c, "internal-secret")

	if got := h.Get(HeaderUserID); got != "u1" {
		t.Errorf("HeaderUserID = %q, want %q", got, "u1")
	}
	if got := h.Get(HeaderOperatorID); got != "op1" {
		t.Errorf("HeaderOperatorID = %q, want %q", got, "op1")
	}
	if got := h.Get(HeaderRole); got != "pier_admin" {
		t.Errorf("HeaderRole = %q, want %q", got, "pier_admin")
	}
	if got := h.Get(HeaderInternalToken); got != "internal-secret" {
		t.Errorf("HeaderInternalToken = %q, want %q", got, "internal-secret")
	}
	if got := h.Get(HeaderPierIDs); got != "pier-a,pier-b" {
		t.Errorf("HeaderPierIDs = %q, want %q", got, "pier-a,pier-b")
	}
}

func TestForwardClaimsEmptyPierIDsSetsNoHeader(t *testing.T) {
	h := http.Header{}
	c := Claims{UserID: "u1", OperatorID: "op1", Role: "customer"}
	ForwardClaims(h, c, "internal-secret")

	if got := h.Get(HeaderPierIDs); got != "" {
		t.Errorf("HeaderPierIDs = %q, want empty (no pier ids)", got)
	}
}

func TestForwardClaimsOverwritesSpoofedHeaders(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderUserID, "attacker")
	h.Set(HeaderOperatorID, "attacker-op")
	h.Set(HeaderRole, "super_admin")
	h.Set(HeaderInternalToken, "guessed-token")
	h.Set(HeaderPierIDs, "spoofed-pier")

	c := Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin", PierIDs: []string{"pier-a"}}
	ForwardClaims(h, c, "internal-secret")

	if got := h.Get(HeaderUserID); got != "u1" {
		t.Errorf("HeaderUserID = %q, want %q (spoofed value must not survive)", got, "u1")
	}
	if got := h.Get(HeaderOperatorID); got != "op1" {
		t.Errorf("HeaderOperatorID = %q, want %q (spoofed value must not survive)", got, "op1")
	}
	if got := h.Get(HeaderRole); got != "pier_admin" {
		t.Errorf("HeaderRole = %q, want %q (spoofed value must not survive)", got, "pier_admin")
	}
	if got := h.Get(HeaderInternalToken); got != "internal-secret" {
		t.Errorf("HeaderInternalToken = %q, want %q (spoofed value must not survive)", got, "internal-secret")
	}
	if got := h.Get(HeaderPierIDs); got != "pier-a" {
		t.Errorf("HeaderPierIDs = %q, want %q (spoofed value must not survive)", got, "pier-a")
	}
	if len(h[http.CanonicalHeaderKey(HeaderUserID)]) != 1 {
		t.Errorf("HeaderUserID has %d values, want exactly 1 (Del then Set)", len(h[http.CanonicalHeaderKey(HeaderUserID)]))
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
