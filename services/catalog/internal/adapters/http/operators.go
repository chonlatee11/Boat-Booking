package httpadapter

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// UpsertOperator requires verified claims (a public call gets
// Unauthenticated); the scope check inside app.UpsertOperator rejects every
// role but super_admin (D-08).
func (s *server) UpsertOperator(ctx context.Context, req *connect.Request[catalogv1.UpsertOperatorRequest]) (*connect.Response[catalogv1.UpsertOperatorResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	if req.Msg.OperatorId != "" {
		id, err = uuid.Parse(req.Msg.OperatorId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator_id"))
		}
	}

	var stored domain.Operator
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.UpsertOperator(ctx, tx, scope, domain.Operator{ID: id, Name: req.Msg.Name})
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.UpsertOperatorResponse{Operator: toProtoOperator(stored)}), nil
}

// ListOperators requires verified claims; super_admin sees every operator,
// pier_admin/staff see only their own (AUTH-05).
func (s *server) ListOperators(ctx context.Context, _ *connect.Request[catalogv1.ListOperatorsRequest]) (*connect.Response[catalogv1.ListOperatorsResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	operators, err := app.ListOperators(ctx, postgres.New(s.pool), scope)
	if err != nil {
		return nil, toConnectErr(err)
	}
	resp := &catalogv1.ListOperatorsResponse{Operators: make([]*catalogv1.Operator, len(operators))}
	for i, o := range operators {
		resp.Operators[i] = toProtoOperator(o)
	}
	return connect.NewResponse(resp), nil
}

func toProtoOperator(o domain.Operator) *catalogv1.Operator {
	return &catalogv1.Operator{
		OperatorId: o.ID.String(),
		Name:       o.Name,
		Archived:   o.Archived,
	}
}
