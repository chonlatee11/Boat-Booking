package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// Trusted claim headers set only by the gateway (BFF) on the internal
// network — no other caller may set these directly (D-30, Anti-Pattern 2).
const (
	HeaderUserID        = "X-User-Id"
	HeaderOperatorID    = "X-Operator-Id"
	HeaderRole          = "X-Role"
	HeaderPierIDs       = "X-Pier-Ids"
	HeaderInternalToken = "X-Internal-Token"
)

// Claims is the verified identity forwarded by the gateway. PierIDs is the
// comma-joined uuid list carried in HeaderPierIDs (research Assumption A4) —
// the same single-value-header convention as the other claim headers.
type Claims struct {
	UserID     string
	OperatorID string
	Role       string
	PierIDs    []string
}

type claimsContextKey struct{}

// WithClaims returns a copy of ctx carrying c.
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, c)
}

// FromContext returns the Claims stored by RequireInternal/WithClaims, if any.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsContextKey{}).(Claims)
	return c, ok
}

// RequireInternal rejects any request whose X-Internal-Token header does not
// constant-time-match token. When the request also carries X-User-Id, the
// claim headers are stored in the request context for downstream handlers
// (D-30). Requests without X-User-Id are allowed through unauthenticated
// (health checks, public routes mounted behind the same middleware chain).
func RequireInternal(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get(HeaderInternalToken)
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				WriteError(w, connect.NewError(connect.CodeUnauthenticated, errors.New("missing or invalid internal token")))
				return
			}
			if uid := r.Header.Get(HeaderUserID); uid != "" {
				pierIDs, err := parsePierIDs(r.Header.Get(HeaderPierIDs))
				if err != nil {
					WriteError(w, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid pier ids header")))
					return
				}
				c := Claims{
					UserID:     uid,
					OperatorID: r.Header.Get(HeaderOperatorID),
					Role:       r.Header.Get(HeaderRole),
					PierIDs:    pierIDs,
				}
				r = r.WithContext(WithClaims(r.Context(), c))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireClaims rejects any request that reached it without verified claims
// already in context (set upstream by RequireInternal).
func RequireClaims(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			WriteError(w, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims")))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parsePierIDs splits the comma-joined HeaderPierIDs value, trims spaces,
// drops empty parts, and rejects (with an error) any part that isn't a
// valid uuid — never passes raw strings into a scoped SQL query.
func parsePierIDs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, err := uuid.Parse(p); err != nil {
			return nil, fmt.Errorf("httpx: invalid pier id %q: %w", p, err)
		}
		ids = append(ids, p)
	}
	return ids, nil
}

// ForwardClaims sets the trusted claim headers on an outbound request,
// deleting any inbound values first so a caller can never spoof them
// (Anti-Pattern 2, D-30). X-Pier-Ids is set to the comma-joined uuid list
// only when c.PierIDs is non-empty (research Assumption A4).
func ForwardClaims(h http.Header, c Claims, internalToken string) {
	h.Del(HeaderUserID)
	h.Del(HeaderOperatorID)
	h.Del(HeaderRole)
	h.Del(HeaderPierIDs)
	h.Del(HeaderInternalToken)

	h.Set(HeaderUserID, c.UserID)
	h.Set(HeaderOperatorID, c.OperatorID)
	h.Set(HeaderRole, c.Role)
	h.Set(HeaderInternalToken, internalToken)
	if len(c.PierIDs) > 0 {
		h.Set(HeaderPierIDs, strings.Join(c.PierIDs, ","))
	}
}
