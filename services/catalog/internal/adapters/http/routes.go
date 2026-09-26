// Package httpadapter wires services/catalog's HTTP routes: the
// CatalogService connect-go handler, mounted behind the internal-token trust
// boundary set up in cmd/main.go.
package httpadapter

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// Routes mounts CatalogService's connect handler onto r. pool is the
// service's Postgres pool; nudge wakes the outbox relay immediately after a
// write instead of waiting for its next poll tick (a no-op when the relay is
// disabled).
func Routes(r chi.Router, pool *pgxpool.Pool, nudge func()) {
	otelOpt, err := httpx.ConnectOtel()
	if err != nil {
		panic(fmt.Errorf("catalog: %w", err))
	}
	path, handler := catalogv1connect.NewCatalogServiceHandler(&server{pool: pool, nudge: nudge}, otelOpt)
	r.Mount(path, handler)
}

// server implements catalogv1connect.CatalogServiceHandler.
type server struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	pool  *pgxpool.Pool
	nudge func()
}

// UpsertBoat requires verified claims (else Unauthenticated) — operator_id
// always comes from the claims, never the request body (D-30).
func (s *server) UpsertBoat(ctx context.Context, req *connect.Request[catalogv1.UpsertBoatRequest]) (*connect.Response[catalogv1.UpsertBoatResponse], error) {
	claims, ok := httpx.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	operatorID, err := uuid.Parse(claims.OperatorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid operator id in claims"))
	}

	status, err := statusFromProto(req.Msg.Status)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var id uuid.UUID
	if req.Msg.BoatId != "" {
		id, err = uuid.Parse(req.Msg.BoatId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid boat_id"))
		}
	}

	var stored domain.Boat
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.UpsertBoat(ctx, tx, operatorID, domain.Boat{
			ID:              id,
			Name:            req.Msg.Name,
			DefaultCapacity: req.Msg.DefaultCapacity,
			Status:          status,
		})
		return txErr
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidArgument):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, domain.ErrNotFound):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.UpsertBoatResponse{Boat: toProtoBoat(stored)}), nil
}

// ListBoats needs only the internal token, no claims.
func (s *server) ListBoats(ctx context.Context, _ *connect.Request[catalogv1.ListBoatsRequest]) (*connect.Response[catalogv1.ListBoatsResponse], error) {
	boats, err := app.ListBoats(ctx, postgres.New(s.pool))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &catalogv1.ListBoatsResponse{Boats: make([]*catalogv1.Boat, len(boats))}
	for i, b := range boats {
		resp.Boats[i] = toProtoBoat(b)
	}
	return connect.NewResponse(resp), nil
}

func toProtoBoat(b domain.Boat) *catalogv1.Boat {
	return &catalogv1.Boat{
		BoatId:          b.ID.String(),
		OperatorId:      b.OperatorID.String(),
		Name:            b.Name,
		DefaultCapacity: b.DefaultCapacity,
		Status:          statusToProto(b.Status),
	}
}

func statusFromProto(s catalogv1.BoatStatus) (domain.Status, error) {
	switch s {
	case catalogv1.BoatStatus_BOAT_STATUS_ACTIVE:
		return domain.StatusActive, nil
	case catalogv1.BoatStatus_BOAT_STATUS_MAINTENANCE:
		return domain.StatusMaintenance, nil
	default:
		return "", errors.New("status must be BOAT_STATUS_ACTIVE or BOAT_STATUS_MAINTENANCE")
	}
}

func statusToProto(s domain.Status) catalogv1.BoatStatus {
	switch s {
	case domain.StatusActive:
		return catalogv1.BoatStatus_BOAT_STATUS_ACTIVE
	case domain.StatusMaintenance:
		return catalogv1.BoatStatus_BOAT_STATUS_MAINTENANCE
	default:
		return catalogv1.BoatStatus_BOAT_STATUS_UNSPECIFIED
	}
}
