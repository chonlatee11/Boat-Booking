package http

import (
	"net/http"
	"testing"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

func TestForwardClaimsSetsVerifiedValues(t *testing.T) {
	h := http.Header{}
	c := auth.Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin"}
	ForwardClaims(h, c, "internal-secret")

	if got := h.Get(httpx.HeaderUserID); got != "u1" {
		t.Errorf("HeaderUserID = %q, want %q", got, "u1")
	}
	if got := h.Get(httpx.HeaderOperatorID); got != "op1" {
		t.Errorf("HeaderOperatorID = %q, want %q", got, "op1")
	}
	if got := h.Get(httpx.HeaderRole); got != "pier_admin" {
		t.Errorf("HeaderRole = %q, want %q", got, "pier_admin")
	}
	if got := h.Get(httpx.HeaderInternalToken); got != "internal-secret" {
		t.Errorf("HeaderInternalToken = %q, want %q", got, "internal-secret")
	}
}

func TestForwardClaimsOverwritesSpoofedHeaders(t *testing.T) {
	h := http.Header{}
	h.Set(httpx.HeaderUserID, "attacker")
	h.Set(httpx.HeaderOperatorID, "attacker-op")
	h.Set(httpx.HeaderRole, "super_admin")
	h.Set(httpx.HeaderInternalToken, "guessed-token")

	c := auth.Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin"}
	ForwardClaims(h, c, "internal-secret")

	if got := h.Get(httpx.HeaderUserID); got != "u1" {
		t.Errorf("HeaderUserID = %q, want %q (spoofed value must not survive)", got, "u1")
	}
	if got := h.Get(httpx.HeaderOperatorID); got != "op1" {
		t.Errorf("HeaderOperatorID = %q, want %q (spoofed value must not survive)", got, "op1")
	}
	if got := h.Get(httpx.HeaderRole); got != "pier_admin" {
		t.Errorf("HeaderRole = %q, want %q (spoofed value must not survive)", got, "pier_admin")
	}
	if got := h.Get(httpx.HeaderInternalToken); got != "internal-secret" {
		t.Errorf("HeaderInternalToken = %q, want %q (spoofed value must not survive)", got, "internal-secret")
	}
	if len(h[http.CanonicalHeaderKey(httpx.HeaderUserID)]) != 1 {
		t.Errorf("HeaderUserID has %d values, want exactly 1 (Del then Set)", len(h[http.CanonicalHeaderKey(httpx.HeaderUserID)]))
	}
}
