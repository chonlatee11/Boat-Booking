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
	"strconv"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/app"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// Routes mounts AuthService's connect handler onto r. a already carries the
// Postgres pool, Valkey client, and outbox nudge it needs.
func Routes(r chi.Router, a *app.Auth) {
	otelOpt, err := httpx.ConnectOtel()
	if err != nil {
		panic(fmt.Errorf("identity: %w", err))
	}
	path, handler := identityv1connect.NewAuthServiceHandler(&server{auth: a}, otelOpt)
	r.Mount(path, handler)
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
		User: &identityv1.SessionUser{
			UserId:     sess.User.ID.String(),
			Role:       sess.User.Role,
			OperatorId: sess.User.OperatorID,
			PierIds:    sess.User.PierIDs,
		},
	}), nil
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
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
