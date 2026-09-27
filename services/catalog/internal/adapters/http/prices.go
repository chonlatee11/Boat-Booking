package httpadapter

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/money"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// AddRoutePrice requires verified claims; app.AddRoutePrice enforces the
// route-scope rule and the not-before-today rule (D-14).
func (s *server) AddRoutePrice(ctx context.Context, req *connect.Request[catalogv1.AddRoutePriceRequest]) (*connect.Response[catalogv1.AddRoutePriceResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(req.Msg.RouteId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid route_id"))
	}
	ticketType, err := ticketTypeFromProto(req.Msg.TicketType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	effectiveFrom, err := time.Parse("2006-01-02", req.Msg.EffectiveFrom)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("effective_from must be YYYY-MM-DD"))
	}

	var stored domain.RoutePrice
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.AddRoutePrice(ctx, tx, scope, domain.RoutePrice{
			RouteID:       routeID,
			TicketType:    ticketType,
			AmountSatang:  money.Satang(req.Msg.AmountSatang),
			EffectiveFrom: effectiveFrom,
		})
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(ctx, err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.AddRoutePriceResponse{Price: toProtoRoutePrice(stored)}), nil
}

// ListRoutePrices requires verified claims; app.ListRoutePrices enforces the
// route-scope rule (staff may read, unlike AddRoutePrice).
func (s *server) ListRoutePrices(ctx context.Context, req *connect.Request[catalogv1.ListRoutePricesRequest]) (*connect.Response[catalogv1.ListRoutePricesResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(req.Msg.RouteId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid route_id"))
	}

	prices, err := app.ListRoutePrices(ctx, postgres.New(s.pool), scope, routeID)
	if err != nil {
		return nil, toConnectErr(ctx, err)
	}
	resp := &catalogv1.ListRoutePricesResponse{Prices: make([]*catalogv1.RoutePrice, len(prices))}
	for i, p := range prices {
		resp.Prices[i] = toProtoRoutePrice(p)
	}
	return connect.NewResponse(resp), nil
}

func toProtoRoutePrice(p domain.RoutePrice) *catalogv1.RoutePrice {
	return &catalogv1.RoutePrice{
		TicketType:    ticketTypeToProto(p.TicketType),
		AmountSatang:  int64(p.AmountSatang),
		EffectiveFrom: p.EffectiveFrom.Format("2006-01-02"),
	}
}

func ticketTypeToProto(t domain.TicketType) catalogv1.TicketType {
	switch t {
	case domain.TicketTypeAdult:
		return catalogv1.TicketType_TICKET_TYPE_ADULT
	case domain.TicketTypeChild:
		return catalogv1.TicketType_TICKET_TYPE_CHILD
	default:
		return catalogv1.TicketType_TICKET_TYPE_UNSPECIFIED
	}
}

func ticketTypeFromProto(t catalogv1.TicketType) (domain.TicketType, error) {
	switch t {
	case catalogv1.TicketType_TICKET_TYPE_ADULT:
		return domain.TicketTypeAdult, nil
	case catalogv1.TicketType_TICKET_TYPE_CHILD:
		return domain.TicketTypeChild, nil
	default:
		return "", errors.New("ticket_type must be TICKET_TYPE_ADULT or TICKET_TYPE_CHILD")
	}
}
