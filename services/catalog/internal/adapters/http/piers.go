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

// UpsertPier requires verified claims (a public call gets Unauthenticated);
// app.UpsertPier enforces the create-vs-update scope rule (D-07, D-08).
func (s *server) UpsertPier(ctx context.Context, req *connect.Request[catalogv1.UpsertPierRequest]) (*connect.Response[catalogv1.UpsertPierResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	if req.Msg.PierId != "" {
		id, err = uuid.Parse(req.Msg.PierId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pier_id"))
		}
	}
	var operatorID uuid.UUID
	if req.Msg.OperatorId != "" {
		operatorID, err = uuid.Parse(req.Msg.OperatorId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator_id"))
		}
	}

	var stored domain.Pier
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.UpsertPier(ctx, tx, scope, domain.Pier{
			ID:         id,
			OperatorID: operatorID,
			NameTH:     req.Msg.NameTh,
			NameEN:     req.Msg.NameEn,
			Lat:        req.Msg.Lat,
			Lng:        req.Msg.Lng,
			Address:    req.Msg.Address,
			OpensAt:    req.Msg.OpensAt,
			ClosesAt:   req.Msg.ClosesAt,
			PhotoKey:   req.Msg.PhotoKey,
		})
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.UpsertPierResponse{Pier: s.toProtoPier(stored)}), nil
}

// ListPiers with no claims returns the public projection (CAT-06); with
// claims it applies the Scope rule (AUTH-05) — the request's operator_id
// filter is honoured only for super_admin (scope.All()).
func (s *server) ListPiers(ctx context.Context, req *connect.Request[catalogv1.ListPiersRequest]) (*connect.Response[catalogv1.ListPiersResponse], error) {
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

	piers, err := app.ListPiers(ctx, postgres.New(s.pool), scope, !hasClaims, filterOperatorID)
	if err != nil {
		return nil, toConnectErr(err)
	}
	resp := &catalogv1.ListPiersResponse{Piers: make([]*catalogv1.Pier, len(piers))}
	for i, p := range piers {
		resp.Piers[i] = s.toProtoPier(p)
	}
	return connect.NewResponse(resp), nil
}

// ArchivePier requires verified claims; app.ArchivePier enforces the pier
// scope rule from UpsertPier and the active-routes block (D-15).
func (s *server) ArchivePier(ctx context.Context, req *connect.Request[catalogv1.ArchivePierRequest]) (*connect.Response[catalogv1.ArchivePierResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Msg.PierId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pier_id"))
	}

	var stored domain.Pier
	err = bbpgx.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var txErr error
		stored, txErr = app.ArchivePier(ctx, tx, scope, id)
		return txErr
	})
	if err != nil {
		return nil, toConnectErr(err)
	}
	s.nudge()

	return connect.NewResponse(&catalogv1.ArchivePierResponse{Pier: s.toProtoPier(stored)}), nil
}

// toProtoPier fills photo_url from s.photos.URL(p.PhotoKey) — the public
// browser-usable URL, empty when no photo is set or storage isn't
// configured (D-19).
func (s *server) toProtoPier(p domain.Pier) *catalogv1.Pier {
	return &catalogv1.Pier{
		PierId:     p.ID.String(),
		OperatorId: p.OperatorID.String(),
		NameTh:     p.NameTH,
		NameEn:     p.NameEN,
		Lat:        p.Lat,
		Lng:        p.Lng,
		Address:    p.Address,
		OpensAt:    p.OpensAt,
		ClosesAt:   p.ClosesAt,
		Archived:   p.Archived,
		PhotoKey:   p.PhotoKey,
		PhotoUrl:   s.photos.URL(p.PhotoKey),
	}
}

// PresignPierPhoto requires verified claims; app.Photos.PresignPierPhoto
// enforces scope.CanWrite() and the content-type/size allow-list (D-19).
func (s *server) PresignPierPhoto(ctx context.Context, req *connect.Request[catalogv1.PresignPierPhotoRequest]) (*connect.Response[catalogv1.PresignPierPhotoResponse], error) {
	scope, hasClaims, err := scopeFrom(ctx)
	if !hasClaims {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing claims"))
	}
	if err != nil {
		return nil, err
	}

	uploadURL, key, err := s.photos.PresignPierPhoto(ctx, scope, req.Msg.ContentType, req.Msg.SizeBytes)
	if err != nil {
		return nil, toConnectErr(err)
	}

	return connect.NewResponse(&catalogv1.PresignPierPhotoResponse{
		UploadUrl:   uploadURL,
		PhotoKey:    key,
		ContentType: req.Msg.ContentType,
	}), nil
}
