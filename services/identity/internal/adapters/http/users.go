package httpadapter

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/app"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// userServer implements identityv1connect.UserServiceHandler.
type userServer struct {
	identityv1connect.UnimplementedUserServiceHandler
	users *app.Users
}

func (s *userServer) UpsertUser(ctx context.Context, req *connect.Request[identityv1.UpsertUserRequest]) (*connect.Response[identityv1.UpsertUserResponse], error) {
	claims, ok := httpx.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}

	var userID uuid.UUID
	if req.Msg.UserId != "" {
		id, err := uuid.Parse(req.Msg.UserId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
		}
		userID = id
	}

	operatorID, err := uuid.Parse(req.Msg.OperatorId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator_id"))
	}

	pierIDs := make([]uuid.UUID, len(req.Msg.PierIds))
	for i, raw := range req.Msg.PierIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pier id %q", raw))
		}
		pierIDs[i] = id
	}

	in := domain.StaffUserInput{
		Email:      req.Msg.Email,
		Name:       req.Msg.Name,
		Role:       req.Msg.Role,
		OperatorID: operatorID,
		PierIDs:    pierIDs,
	}

	user, err := s.users.UpsertUser(ctx, claims, in, userID)
	if err != nil {
		return nil, mapUserError(err)
	}
	return connect.NewResponse(&identityv1.UpsertUserResponse{User: toProtoStaffUser(user)}), nil
}

func (s *userServer) ListUsers(ctx context.Context, req *connect.Request[identityv1.ListUsersRequest]) (*connect.Response[identityv1.ListUsersResponse], error) {
	claims, ok := httpx.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}

	var operatorFilter uuid.UUID
	if req.Msg.OperatorId != "" {
		id, err := uuid.Parse(req.Msg.OperatorId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator_id"))
		}
		operatorFilter = id
	}

	users, err := s.users.ListUsers(ctx, claims, operatorFilter)
	if err != nil {
		return nil, mapUserError(err)
	}
	resp := &identityv1.ListUsersResponse{Users: make([]*identityv1.StaffUser, len(users))}
	for i, user := range users {
		resp.Users[i] = toProtoStaffUser(user)
	}
	return connect.NewResponse(resp), nil
}

func (s *userServer) SetUserDisabled(ctx context.Context, req *connect.Request[identityv1.SetUserDisabledRequest]) (*connect.Response[identityv1.SetUserDisabledResponse], error) {
	claims, ok := httpx.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}

	id, err := uuid.Parse(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	user, err := s.users.SetUserDisabled(ctx, claims, id, req.Msg.Disabled)
	if err != nil {
		return nil, mapUserError(err)
	}
	return connect.NewResponse(&identityv1.SetUserDisabledResponse{User: toProtoStaffUser(user)}), nil
}

func toProtoStaffUser(u domain.User) *identityv1.StaffUser {
	return &identityv1.StaffUser{
		UserId:     u.ID.String(),
		Email:      u.Email,
		Name:       u.Name,
		Role:       u.Role,
		OperatorId: u.OperatorID,
		PierIds:    u.PierIDs,
		Disabled:   u.DisabledAt != nil,
	}
}

// mapUserError converts app/domain errors to connect codes.
func mapUserError(err error) error {
	switch {
	case errors.Is(err, domain.ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, domain.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, domain.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, domain.ErrUnavailable):
		return connect.NewError(connect.CodeUnavailable, err)
	case errors.Is(err, domain.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, domain.ErrFailedPrecondition):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
