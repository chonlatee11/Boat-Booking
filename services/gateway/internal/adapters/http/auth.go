package http

import (
	"errors"
	"io"
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
func decodeAuthBody(w http.ResponseWriter, r *http.Request, msg proto.Message) bool {
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
