// Package http holds the gateway's HTTP adapter: routes and the claim
// forwarding that establishes the internal trust boundary (D-29, D-30).
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

// Routes registers the gateway's BFF routes on r, verifying the access
// cookie (or bearer token) with v.
func Routes(r chi.Router, v *auth.Verifier) {
	r.Get("/api/v1/whoami", whoamiHandler(v))
}

func whoamiHandler(v *auth.Verifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := tokenFromRequest(r)
		if tok == "" {
			httpx.WriteError(w, unauthenticated("missing access token"))
			return
		}
		claims, err := v.Verify(tok, auth.KindAccess)
		if err != nil {
			httpx.WriteError(w, unauthenticated("invalid access token"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"sub":         claims.UserID,
			"operator_id": claims.OperatorID,
			"role":        claims.Role,
		})
	}
}

func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(auth.AccessCookie); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func unauthenticated(msg string) error {
	return connect.NewError(connect.CodeUnauthenticated, errors.New(msg))
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// ForwardClaims sets the trusted claim headers on an outbound request,
// deleting any inbound values first so a caller can never spoof them
// (Anti-Pattern 2, D-30).
func ForwardClaims(h http.Header, c auth.Claims, internalToken string) {
	h.Del(httpx.HeaderUserID)
	h.Del(httpx.HeaderOperatorID)
	h.Del(httpx.HeaderRole)
	h.Del(httpx.HeaderInternalToken)

	h.Set(httpx.HeaderUserID, c.UserID)
	h.Set(httpx.HeaderOperatorID, c.OperatorID)
	h.Set(httpx.HeaderRole, c.Role)
	h.Set(httpx.HeaderInternalToken, internalToken)
}
