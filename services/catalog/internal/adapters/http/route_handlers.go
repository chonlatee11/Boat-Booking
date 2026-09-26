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

// UpsertRoute requires verified claims (else Unauthenticated); app.UpsertRoute
// enforces the pier_from scope rule and the pier_to any-operator rule
// (D-11, D-12).
func (s *server) UpsertRoute(ctx context.Context, req *connect.Request[catalogv1.UpsertRouteRequest]) (*connect.Response[catalogv1.UpsertRouteResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	if req.Msg.RouteId != "" {
		id, err = uuid.Parse(req.Msg.RouteId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid route_id"))
		}
	}
	pierFromID, err := uuid.Parse(req.Msg.PierFromId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pier_from_id"))
	}
	pierToID, err := uuid.Parse(req.Msg.PierToId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pier_to_id"))
	}

	var stored domain.Route
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.UpsertRoute(ctx, tx, scope, domain.Route{
			ID:                 id,
			PierFromID:         pierFromID,
			PierToID:           pierToID,
			DurationMinutes:    req.Msg.DurationMinutes,
			CancellationPolicy: cancellationPolicyFromProto(req.Msg.CancellationPolicy),
		})
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.UpsertRouteResponse{Route: toProtoRoute(stored)}), nil
}

// ListRoutes with no claims returns the public projection (CAT-06); with
// claims it applies the Scope rule (AUTH-05) — the request's operator_id
// filter is honoured only for super_admin.
func (s *server) ListRoutes(ctx context.Context, req *connect.Request[catalogv1.ListRoutesRequest]) (*connect.Response[catalogv1.ListRoutesResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if err != nil {
		return nil, err
	}

	var filterOperatorID *uuid.UUID
	if req.Msg.OperatorId != "" {
		parsed, parseErr := uuid.Parse(req.Msg.OperatorId)
		if parseErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator_id"))
		}
		filterOperatorID = &parsed
	}

	routes, err := app.ListRoutes(ctx, postgres.New(s.pool), scope, !hasClaims, filterOperatorID)
	if err != nil {
		return nil, toConnectErr(err)
	}
	resp := &catalogv1.ListRoutesResponse{Routes: make([]*catalogv1.Route, len(routes))}
	for i, r := range routes {
		resp.Routes[i] = toProtoRoute(r)
	}
	return connect.NewResponse(resp), nil
}

func toProtoRoute(r domain.Route) *catalogv1.Route {
	prices := make([]*catalogv1.RoutePrice, len(r.CurrentPrices))
	for i, p := range r.CurrentPrices {
		prices[i] = toProtoRoutePrice(p)
	}
	return &catalogv1.Route{
		RouteId:            r.ID.String(),
		OperatorId:         r.OperatorID.String(),
		PierFromId:         r.PierFromID.String(),
		PierToId:           r.PierToID.String(),
		DurationMinutes:    r.DurationMinutes,
		CancellationPolicy: cancellationPolicyToProto(r.CancellationPolicy),
		Archived:           r.Archived,
		CurrentPrices:      prices,
	}
}

func cancellationPolicyFromProto(tiers []*catalogv1.CancellationTier) []domain.CancellationTier {
	out := make([]domain.CancellationTier, len(tiers))
	for i, t := range tiers {
		out[i] = domain.CancellationTier{MinHoursBefore: t.MinHoursBefore, RefundPercent: t.RefundPercent}
	}
	return out
}

func cancellationPolicyToProto(tiers []domain.CancellationTier) []*catalogv1.CancellationTier {
	out := make([]*catalogv1.CancellationTier, len(tiers))
	for i, t := range tiers {
		out[i] = &catalogv1.CancellationTier{MinHoursBefore: t.MinHoursBefore, RefundPercent: t.RefundPercent}
	}
	return out
}
