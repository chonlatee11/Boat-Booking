package http

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

// maxAdminBodyBytes bounds POST /api/v1/admin/{service}/{method} request
// bodies (T-02-04-04) — rejected before any upstream contact.
const maxAdminBodyBytes = 64 * 1024

// adminMethodPattern is the allow-listed shape of a connect RPC method name
// (T-02-04-03) — never contains "/" or "." so it can't escape the rewritten
// upstream path.
var adminMethodPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{0,63}$`)

// adminProxy reverse-proxies POST /api/v1/admin/{service}/{method} to the
// allow-listed upstream for {service} (D-18). Every client-supplied
// trust-boundary header, Cookie and Authorization is deleted and replaced
// with the caller's verified claims before the request ever leaves the
// gateway (Anti-Pattern 2, T-02-04-01); only staff/pier_admin/super_admin
// tokens are forwarded (T-02-04-02).
func adminProxy(v *auth.Verifier, upstreams map[string]*url.URL, transport http.RoundTripper, internalToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := chi.URLParam(r, "service")
		target, ok := upstreams[service]
		if !ok {
			httpx.WriteError(w, connect.NewError(connect.CodeNotFound, errors.New("unknown service")))
			return
		}
		method := chi.URLParam(r, "method")
		if !adminMethodPattern.MatchString(method) {
			httpx.WriteError(w, connect.NewError(connect.CodeNotFound, errors.New("unknown method")))
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("content-type must be application/json")))
			return
		}

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
		switch claims.Role {
		case auth.RoleStaff, auth.RolePierAdmin, auth.RoleSuperAdmin:
		default:
			httpx.WriteError(w, connect.NewError(connect.CodePermissionDenied, errors.New("role not permitted")))
			return
		}

		// Read the whole (bounded) body upfront rather than streaming it
		// through the reverse proxy: an oversized body is rejected here,
		// before any connection to the upstream is attempted.
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("request body too large")))
			return
		}

		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.Out.URL.Path = "/" + service + "/" + method
				pr.Out.URL.RawPath = ""
				pr.Out.Header.Del("Cookie")
				pr.Out.Header.Del("Authorization")
				httpx.ForwardClaims(pr.Out.Header, httpx.Claims{
					UserID:     claims.UserID,
					OperatorID: claims.OperatorID,
					Role:       claims.Role,
					PierIDs:    claims.PierIDs,
				}, internalToken)
				pr.Out.Body = io.NopCloser(bytes.NewReader(body))
				pr.Out.ContentLength = int64(len(body))
			},
			Transport: transport,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				var maxErr *http.MaxBytesError
				if errors.As(err, &maxErr) {
					httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("request body too large")))
					return
				}
				httpx.WriteError(w, connect.NewError(connect.CodeUnavailable, err))
			},
		}
		proxy.ServeHTTP(w, r)
	}
}
