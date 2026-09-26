package httpadapter

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// scopeFrom turns the forwarded claims (set only by the gateway/internal
// callers, D-30) into an app.Scope — the one place role checking happens
// before any catalog handler runs. hasClaims is false only when the request
// carried no claims at all (a public call); it is true whenever claims were
// present, even if the role itself is denied.
func scopeFrom(ctx context.Context) (scope app.Scope, hasClaims bool, err error) {
	claims, ok := httpx.FromContext(ctx)
	if !ok {
		return app.Scope{}, false, nil
	}

	switch claims.Role {
	case auth.RoleSuperAdmin:
		return app.Scope{Role: claims.Role}, true, nil
	case auth.RolePierAdmin, auth.RoleStaff:
		operatorID, err := uuid.Parse(claims.OperatorID)
		if err != nil {
			return app.Scope{}, true, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid operator id in claims"))
		}
		pierIDs := make([]uuid.UUID, 0, len(claims.PierIDs))
		for _, s := range claims.PierIDs {
			id, err := uuid.Parse(s)
			if err != nil {
				return app.Scope{}, true, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid pier id in claims"))
			}
			pierIDs = append(pierIDs, id)
		}
		return app.Scope{Role: claims.Role, OperatorID: operatorID, PierIDs: pierIDs}, true, nil
	default:
		return app.Scope{}, true, connect.NewError(connect.CodePermissionDenied, errors.New("role not permitted"))
	}
}

// toConnectErr maps a domain sentinel error to its Connect code — the one
// error switch every catalog handler uses.
func toConnectErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, domain.ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, domain.ErrFailedPrecondition):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, domain.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
