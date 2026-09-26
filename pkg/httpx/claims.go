package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"connectrpc.com/connect"
)

// Trusted claim headers set only by the gateway (BFF) on the internal
// network — no other caller may set these directly (D-30, Anti-Pattern 2).
const (
	HeaderUserID        = "X-User-Id"
	HeaderOperatorID    = "X-Operator-Id"
	HeaderRole          = "X-Role"
	HeaderInternalToken = "X-Internal-Token"
)

// Claims is the verified identity forwarded by the gateway.
type Claims struct {
	UserID     string
	OperatorID string
	Role       string
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
				c := Claims{
					UserID:     uid,
					OperatorID: r.Header.Get(HeaderOperatorID),
					Role:       r.Header.Get(HeaderRole),
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
