// Package http holds the gateway's HTTP adapter: routes and the claim
// forwarding that establishes the internal trust boundary (D-29, D-30).
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

// identityServiceName is the connect full name of identity's future
// UserService (implemented in 02-07) — allow-listed here so the admin proxy
// starts routing to it the moment it exists, with no gateway change.
const identityServiceName = "boatbooking.identity.v1.UserService"

// Routes registers the gateway's BFF routes on r, verifying the access
// cookie (or bearer token) with v (D-29, D-30). Mounted directly on the
// router — NOT behind httpx.RequireInternal — because the gateway is where
// that trust boundary originates, not a consumer of it.
func Routes(r chi.Router, v *auth.Verifier, client *http.Client, catalogURL, identityURL *url.URL, internalToken string) {
	r.Get("/api/v1/whoami", whoamiHandler(v))
	r.Get("/api/v1/public/{resource}", publicHandler(client, catalogURL, internalToken))

	upstreams := map[string]*url.URL{
		catalogv1connect.CatalogServiceName: catalogURL,
		identityServiceName:                 identityURL,
	}
	r.Post("/api/v1/admin/{service}/{method}", adminProxy(v, upstreams, client.Transport, internalToken))
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
		pierIDs := claims.PierIDs
		if pierIDs == nil {
			pierIDs = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sub":         claims.UserID,
			"operator_id": claims.OperatorID,
			"role":        claims.Role,
			"pier_ids":    pierIDs,
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
