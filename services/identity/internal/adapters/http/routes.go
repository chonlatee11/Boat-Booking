// Package httpadapter wires identity's HTTP routes: the AuthService
// connect-go handler, mounted behind the internal-token trust boundary set
// up in cmd/main.go. RequestOtp/VerifyOtp need only the internal token, not
// verified claims — the caller isn't authenticated yet (like catalog's
// ListBoats).
package httpadapter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/app"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// Routes mounts AuthService's and UserService's connect handlers onto r. a
// already carries the Postgres pool, Valkey client, and outbox nudge it
// needs; u carries the catalog client UserService needs for pier validation
// (research Pattern 3).
func Routes(r chi.Router, a *app.Auth, u *app.Users) {
	otelOpt, err := httpx.ConnectOtel()
	if err != nil {
		panic(fmt.Errorf("identity: %w", err))
	}
	authPath, authHandler := identityv1connect.NewAuthServiceHandler(&server{auth: a}, otelOpt)
	r.Mount(authPath, authHandler)

	userPath, userHandler := identityv1connect.NewUserServiceHandler(&userServer{users: u}, otelOpt)
	r.Mount(userPath, userHandler)
}

// server implements identityv1connect.AuthServiceHandler.
type server struct {
	identityv1connect.UnimplementedAuthServiceHandler
	auth *app.Auth
}

func (s *server) RequestOtp(ctx context.Context, req *connect.Request[identityv1.RequestOtpRequest]) (*connect.Response[identityv1.RequestOtpResponse], error) {
	if err := s.auth.RequestOtp(ctx, req.Msg.Destination); err != nil {
		return nil, mapAuthError(err)
	}
	return connect.NewResponse(&identityv1.RequestOtpResponse{}), nil
}

func (s *server) VerifyOtp(ctx context.Context, req *connect.Request[identityv1.VerifyOtpRequest]) (*connect.Response[identityv1.VerifyOtpResponse], error) {
	sess, err := s.auth.VerifyOtp(ctx, req.Msg.Destination, req.Msg.Code)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return connect.NewResponse(&identityv1.VerifyOtpResponse{
		AccessToken:  sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		User:         sessionUserProto(sess.User),
	}), nil
}

func (s *server) Refresh(ctx context.Context, req *connect.Request[identityv1.RefreshRequest]) (*connect.Response[identityv1.RefreshResponse], error) {
	sess, err := s.auth.Refresh(ctx, req.Msg.RefreshToken)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return connect.NewResponse(&identityv1.RefreshResponse{
		AccessToken:  sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		User:         sessionUserProto(sess.User),
	}), nil
}

func (s *server) Logout(ctx context.Context, req *connect.Request[identityv1.LogoutRequest]) (*connect.Response[identityv1.LogoutResponse], error) {
	if err := s.auth.Logout(ctx, req.Msg.RefreshToken); err != nil {
		return nil, mapAuthError(err)
	}
	return connect.NewResponse(&identityv1.LogoutResponse{}), nil
}

func sessionUserProto(u domain.User) *identityv1.SessionUser {
	return &identityv1.SessionUser{
		UserId:     u.ID.String(),
		Role:       u.Role,
		OperatorId: u.OperatorID,
		PierIds:    u.PierIDs,
	}
}

// mapAuthError converts app/domain errors to connect codes. CodeMismatchError
// carries how many attempts remain via the Attempts-Left response header.
func mapAuthError(err error) error {
	var mismatch *domain.CodeMismatchError
	switch {
	case errors.As(err, &mismatch):
		ce := connect.NewError(connect.CodeInvalidArgument, err)
		ce.Meta().Set("Attempts-Left", strconv.Itoa(mismatch.AttemptsLeft))
		return ce
	case errors.Is(err, domain.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrCodeExpired), errors.Is(err, domain.ErrDeliveryUnavailable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, domain.ErrResendTooSoon), errors.Is(err, domain.ErrRateLimited):
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, domain.ErrDisabled):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, domain.ErrSessionInvalid):
		return connect.NewError(connect.CodeUnauthenticated, err)
	default:
		// WR-09: connect-go puts err.Error() on the wire for every code,
		// including CodeInternal, and the gateway's auth routes/proxies
		// forward that body — never let an unmapped error (schema/
		// constraint/host detail) reach a client. Log the real error,
		// return a generic one.
		slog.Error("identity: unmapped auth error", "error", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
