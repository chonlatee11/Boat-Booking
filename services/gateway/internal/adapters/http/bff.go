// Package http holds the gateway's HTTP adapter: routes and the claim
// forwarding that establishes the internal trust boundary (D-29, D-30).
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

// maxUpsertBoatBodyBytes bounds POST /api/v1/boats request bodies (T-11-03).
const maxUpsertBoatBodyBytes = 16 * 1024

// Routes registers the gateway's BFF routes on r, verifying the access
// cookie (or bearer token) with v and calling catalog over connect-go with
// the internal token + verified claims it forwards (D-29, D-30). Mounted
// directly on the router — NOT behind httpx.RequireInternal — because the
// gateway is where that trust boundary originates, not a consumer of it.
func Routes(r chi.Router, v *auth.Verifier, catalog catalogv1connect.CatalogServiceClient, internalToken string) {
	r.Get("/api/v1/whoami", whoamiHandler(v))
	r.Get("/api/v1/public/boats", publicBoatsHandler(catalog, internalToken))
	r.Post("/api/v1/boats", upsertBoatHandler(v, catalog, internalToken))
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

// upsertBoatHandler verifies the access cookie, decodes the request body
// into UpsertBoatRequest (rejecting unknown fields, T-11-03), forwards
// verified claims + the internal token — deleting any client-supplied
// trust-boundary headers first (ForwardClaims, T-11-01) — and calls
// catalog.UpsertBoat over connect-go.
func upsertBoatHandler(v *auth.Verifier, catalog catalogv1connect.CatalogServiceClient, internalToken string) http.HandlerFunc {
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
		// TODO(WR-01): no claims.Role check here — Phase 1 has exactly one
		// role (pier_admin, see 01-RESEARCH.md "V4 Access Control": "no
		// business-level roles exist yet"). Phase 2 introduces customer/staff
		// roles (SKELETON.md); add a role check on this write path (and on
		// catalog's UpsertBoat, services/catalog/internal/adapters/http/routes.go)
		// before a second role can reach this handler.

		r.Body = http.MaxBytesReader(w, r.Body, maxUpsertBoatBodyBytes)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("request body too large or unreadable")))
			return
		}

		var msg catalogv1.UpsertBoatRequest
		if err := protojson.Unmarshal(data, &msg); err != nil {
			httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request body")))
			return
		}

		req := connect.NewRequest(&msg)
		ForwardClaims(req.Header(), claims, internalToken)

		resp, err := catalog.UpsertBoat(r.Context(), req)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		writeProtoJSON(w, http.StatusCreated, resp.Msg)
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
