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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
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
func Routes(r chi.Router, v *auth.Verifier, catalog catalogv1connect.CatalogServiceClient, catalogURL, identityURL *url.URL, transport http.RoundTripper, internalToken string) {
	r.Get("/api/v1/whoami", whoamiHandler(v))
	r.Get("/api/v1/public/boats", publicBoatsHandler(catalog, internalToken))

	upstreams := map[string]*url.URL{
		catalogv1connect.CatalogServiceName: catalogURL,
		identityServiceName:                 identityURL,
	}
	r.Post("/api/v1/admin/{service}/{method}", adminProxy(v, upstreams, transport, internalToken))
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

// publicBoatsHandler proxies ListBoats with only the internal token — a
// catalog-wide read needing no verified claims (D-29).
func publicBoatsHandler(catalog catalogv1connect.CatalogServiceClient, internalToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := connect.NewRequest(&catalogv1.ListBoatsRequest{})
		req.Header().Set(httpx.HeaderInternalToken, internalToken)

		resp, err := catalog.ListBoats(r.Context(), req)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		writeProtoJSON(w, http.StatusOK, resp.Msg)
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

// writeProtoJSON renders msg as protojson, emitting unpopulated fields so an
// empty repeated field serializes as [] rather than being omitted (and read
// back as null).
func writeProtoJSON(w http.ResponseWriter, status int, msg proto.Message) {
	data, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(msg)
	if err != nil {
		httpx.WriteError(w, connect.NewError(connect.CodeInternal, err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
