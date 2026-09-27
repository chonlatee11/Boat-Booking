package http

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
)

// maxAuthBodyBytes bounds the public /api/v1/auth/* request bodies — small,
// fixed-shape messages, rejected before ever reaching identity.
const maxAuthBodyBytes = 4 * 1024

// AuthRoutes registers the unauthenticated OTP login routes (AUTH-01). Every
// outbound call to identity carries only X-Internal-Token — never claims,
// since the caller isn't authenticated yet (D-29); tokens issued on success
// travel only in httpOnly cookies, never in a JSON response body.
func AuthRoutes(r chi.Router, identity identityv1connect.AuthServiceClient, internalToken string) {
	r.Post("/api/v1/auth/otp/request", otpRequestHandler(identity, internalToken))
	r.Post("/api/v1/auth/otp/verify", otpVerifyHandler(identity, internalToken))
	r.Post("/api/v1/auth/refresh", refreshHandler(identity, internalToken))
	r.Post("/api/v1/auth/logout", logoutHandler(identity, internalToken))
}

func otpRequestHandler(identity identityv1connect.AuthServiceClient, internalToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var msg identityv1.RequestOtpRequest
		if !decodeAuthBody(w, r, &msg) {
			return
		}
		req := connect.NewRequest(&msg)
		req.Header().Set(httpx.HeaderInternalToken, internalToken)
		if _, err := identity.RequestOtp(r.Context(), req); err != nil {
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})
	}
}

func otpVerifyHandler(identity identityv1connect.AuthServiceClient, internalToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var msg identityv1.VerifyOtpRequest
		if !decodeAuthBody(w, r, &msg) {
			return
		}
		req := connect.NewRequest(&msg)
		req.Header().Set(httpx.HeaderInternalToken, internalToken)
		resp, err := identity.VerifyOtp(r.Context(), req)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		http.SetCookie(w, auth.Cookie(auth.KindAccess, resp.Msg.AccessToken))
		http.SetCookie(w, auth.Cookie(auth.KindRefresh, resp.Msg.RefreshToken))
		writeJSON(w, http.StatusOK, sessionUserBody(resp.Msg.User))
	}
}

// refreshHandler rotates the session (D-10): missing cookie or any identity
// error clears both cookies and fails; success re-sets both from the fresh
// pair. Never trusts a client-supplied user id — only the cookie's opaque
// refresh token identifies the session.
func refreshHandler(identity identityv1connect.AuthServiceClient, internalToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.RefreshCookie)
		if err != nil || c.Value == "" {
			clearAuthCookies(w)
			httpx.WriteError(w, connect.NewError(connect.CodeUnauthenticated, errors.New("missing refresh token")))
			return
		}
		req := connect.NewRequest(&identityv1.RefreshRequest{RefreshToken: c.Value})
		req.Header().Set(httpx.HeaderInternalToken, internalToken)
		resp, err := identity.Refresh(r.Context(), req)
		if err != nil {
			clearAuthCookies(w)
			httpx.WriteError(w, err)
			return
		}
		http.SetCookie(w, auth.Cookie(auth.KindAccess, resp.Msg.AccessToken))
		http.SetCookie(w, auth.Cookie(auth.KindRefresh, resp.Msg.RefreshToken))
		writeJSON(w, http.StatusOK, sessionUserBody(resp.Msg.User))
	}
}

// logoutHandler always clears both cookies and returns 204, even when
// identity errors or no refresh cookie was presented — from the browser's
// point of view the session is gone either way.
func logoutHandler(identity identityv1connect.AuthServiceClient, internalToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.RefreshCookie); err == nil && c.Value != "" {
			req := connect.NewRequest(&identityv1.LogoutRequest{RefreshToken: c.Value})
			req.Header().Set(httpx.HeaderInternalToken, internalToken)
			_, _ = identity.Logout(r.Context(), req)
		}
		clearAuthCookies(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

// clearAuthCookies expires both session cookies — same name/path/flags as
// auth.Cookie, empty value, MaxAge -1, so the browser deletes them.
func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, clearedCookie(auth.AccessCookie))
	http.SetCookie(w, clearedCookie(auth.RefreshCookie))
}

func clearedCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}

// sessionUserBody is the JSON shape returned on a successful otp/verify or
// refresh — user identity fields only, never a token (AUTH-01).
func sessionUserBody(u *identityv1.SessionUser) map[string]any {
	pierIDs := u.PierIds
	if pierIDs == nil {
		pierIDs = []string{}
	}
	return map[string]any{
		"userId":     u.UserId,
		"role":       u.Role,
		"operatorId": u.OperatorId,
		"pierIds":    pierIDs,
	}
}

// decodeAuthBody reads r's body (bounded to maxAuthBodyBytes) and unmarshals
// it as JSON into msg, rejecting unknown fields (protojson's default). On
// any failure it writes a 400 and returns false.
//
// WR-01: Content-Type must be exactly application/json, the same rule
// adminProxy already enforces. A form-like Content-Type (e.g. text/plain)
// can be produced by a plain cross-site auto-submitting <form>, letting an
// attacker drive a victim's browser into POSTing a JSON-shaped body without
// a CORS preflight (login CSRF). Rejecting non-JSON here forces a preflight,
// which Kong's cors origin allow-list then blocks.
func decodeAuthBody(w http.ResponseWriter, r *http.Request, msg proto.Message) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("content-type must be application/json")))
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("request body too large")))
		return false
	}
	if err := protojson.Unmarshal(body, msg); err != nil {
		httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request body")))
		return false
	}
	return true
}

// writeAuthError maps an identity error to the response. A wrong-code
// InvalidArgument carries how many attempts remain in the connect error's
// Attempts-Left metadata (set by identity's mapAuthError); every other error
// goes through the standard httpx.WriteError mapping.
func writeAuthError(w http.ResponseWriter, err error) {
	if connect.CodeOf(err) == connect.CodeInvalidArgument {
		var connErr *connect.Error
		if errors.As(err, &connErr) {
			if left := connErr.Meta().Get("Attempts-Left"); left != "" {
				if n, convErr := strconv.Atoi(left); convErr == nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"code":         "invalid_argument",
						"message":      connErr.Message(),
						"attemptsLeft": n,
					})
					return
				}
			}
		}
	}
	httpx.WriteError(w, err)
}
