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
// disabled). photos issues presigned pier photo upload URLs and builds
// their public URL (nil Photos.Client when object storage isn't
// configured, D-19).
func Routes(r chi.Router, pool *pgxpool.Pool, nudge func(), photos app.Photos) {
	otelOpt, err := httpx.ConnectOtel()
	if err != nil {
		panic(fmt.Errorf("catalog: %w", err))
	}
	path, handler := catalogv1connect.NewCatalogServiceHandler(&server{pool: pool, nudge: nudge, photos: photos}, otelOpt)
	r.Mount(path, handler)
}

// server implements catalogv1connect.CatalogServiceHandler.
type server struct {
	catalogv1connect.UnimplementedCatalogServiceHandler
	pool   *pgxpool.Pool
	nudge  func()
	photos app.Photos
}

// UpsertBoat requires verified claims; app.UpsertBoat enforces the
// home-pier scope rule (D-07) — operator_id is always derived from the
// home pier, never taken from the request body (D-30).
func (s *server) UpsertBoat(ctx context.Context, req *connect.Request[catalogv1.UpsertBoatRequest]) (*connect.Response[catalogv1.UpsertBoatResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
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
	var homePierID uuid.UUID
	if req.Msg.HomePierId != "" {
		homePierID, err = uuid.Parse(req.Msg.HomePierId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid home_pier_id"))
		}
	}

	var stored domain.Boat
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.UpsertBoat(ctx, tx, scope, domain.Boat{
			ID:              id,
			HomePierID:      homePierID,
			Name:            req.Msg.Name,
			DefaultCapacity: req.Msg.DefaultCapacity,
			Status:          status,
		})
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(ctx, err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.UpsertBoatResponse{Boat: toProtoBoat(stored)}), nil
}

// ListBoats with no claims returns the public (non-archived only)
// projection; with claims it applies the Scope rule (AUTH-05, CAT-04).
func (s *server) ListBoats(ctx context.Context, _ *connect.Request[catalogv1.ListBoatsRequest]) (*connect.Response[catalogv1.ListBoatsResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if err != nil {
		return nil, err
	}

	boats, err := app.ListBoats(ctx, postgres.New(s.pool), scope, !hasClaims)
	if err != nil {
		return nil, toConnectErr(ctx, err)
	}
	resp := &catalogv1.ListBoatsResponse{Boats: make([]*catalogv1.Boat, len(boats))}
	for i, b := range boats {
		resp.Boats[i] = toProtoBoat(b)
	}
	return connect.NewResponse(resp), nil
}

// ArchiveBoat requires verified claims; app.ArchiveBoat enforces the same
// home-pier scope rule as UpsertBoat (D-07).
func (s *server) ArchiveBoat(ctx context.Context, req *connect.Request[catalogv1.ArchiveBoatRequest]) (*connect.Response[catalogv1.ArchiveBoatResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Msg.BoatId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid boat_id"))
	}

	var stored domain.Boat
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.ArchiveBoat(ctx, tx, scope, id)
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(ctx, err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.ArchiveBoatResponse{Boat: toProtoBoat(stored)}), nil
}

func toProtoBoat(b domain.Boat) *catalogv1.Boat {
	return &catalogv1.Boat{
		BoatId:          b.ID.String(),
		OperatorId:      b.OperatorID.String(),
		Name:            b.Name,
		DefaultCapacity: b.DefaultCapacity,
		Status:          statusToProto(b.Status),
		HomePierId:      b.HomePierID.String(),
		Archived:        b.Archived,
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
