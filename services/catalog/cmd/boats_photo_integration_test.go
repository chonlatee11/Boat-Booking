//go:build integration

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
)

// TestBoatsScopedByHomePier proves D-07/CAT-04 end to end: a boat's home
// pier gates every write and read, its operator is always derived from the
// home pier (never the request), and archiving is idempotent and freezes
// further edits.
func TestBoatsScopedByHomePier(t *testing.T) {
	addr, dsn := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))
	opA := mustCreateOperator(t, ctx, superAdmin, "Home Pier Operator A")
	opB := mustCreateOperator(t, ctx, superAdmin, "Home Pier Operator B")
	a1 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า HP A1", "Pier HP A1", 7.55, 98.11)
	a2 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า HP A2", "Pier HP A2", 7.56, 98.12)
	_ = mustCreatePier(t, ctx, superAdmin, opB, "ท่า HP B1", "Pier HP B1", 7.57, 98.13)

	pierAdminA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA, a1))

	// pier_admin(A,[A1]) creates a boat at A1 — operator is derived from A1,
	// never taken from the request.
	resp, err := pierAdminA1.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "Home Pier Boat", DefaultCapacity: 10, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE, HomePierId: a1,
	}))
	if err != nil {
		t.Fatalf("create boat at A1: %v", err)
	}
	boat := resp.Msg.Boat
	if boat.OperatorId != opA {
		t.Errorf("boat.OperatorId = %q, want %q (derived from home pier A1)", boat.OperatorId, opA)
	}
	if boat.HomePierId != a1 {
		t.Errorf("boat.HomePierId = %q, want %q", boat.HomePierId, a1)
	}
	if n := countOutboxEvents(t, dsn, app.EventBoatUpserted, boat.BoatId); n != 1 {
		t.Fatalf("outbox rows after create = %d, want 1", n)
	}

	// pier_admin(A,[A1]) cannot create/edit at A2 — out-of-scope home pier ->
	// NotFound.
	_, err = pierAdminA1.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "Should Fail", DefaultCapacity: 5, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE, HomePierId: a2,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "pier_admin(A,[A1]) create boat at out-of-scope pier A2")

	// pier_admin(A,[]) — empty pier scope — ListBoats returns empty, never
	// "everything" (AUTH-05 empty edge, CAT-04 empty edge).
	pierAdminNoPiers := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA))
	listEmpty, err := pierAdminNoPiers.ListBoats(ctx, connect.NewRequest(&catalogv1.ListBoatsRequest{}))
	if err != nil {
		t.Fatalf("pier_admin(A,[]) ListBoats: %v", err)
	}
	if len(listEmpty.Msg.Boats) != 0 {
		t.Fatalf("pier_admin(A,[]) ListBoats = %+v, want empty", listEmpty.Msg.Boats)
	}

	// staff cannot write (read-only role in the catalog for v1).
	staffA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleStaff, opA, a1))
	_, err = staffA1.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "Staff Boat", DefaultCapacity: 5, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE, HomePierId: a1,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "staff UpsertBoat")

	// customer cannot write either.
	customer := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleCustomer, opA))
	_, err = customer.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "Customer Boat", DefaultCapacity: 5, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE, HomePierId: a1,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "customer UpsertBoat")

	// Missing home pier -> InvalidArgument (not NotFound — the request
	// carries no home_pier_id at all).
	_, err = pierAdminA1.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "No Home Pier", DefaultCapacity: 5, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
	}))
	assertConnectCode(t, err, connect.CodeInvalidArgument, "UpsertBoat with no home_pier_id")

	// ArchiveBoat: first call archives, second is an idempotent no-op —
	// exactly one BoatUpserted outbox row for the archive.
	archResp, err := pierAdminA1.ArchiveBoat(ctx, connect.NewRequest(&catalogv1.ArchiveBoatRequest{BoatId: boat.BoatId}))
	if err != nil {
		t.Fatalf("ArchiveBoat: %v", err)
	}
	if !archResp.Msg.Boat.Archived {
		t.Fatalf("archived boat.Archived = false, want true")
	}
	archResp2, err := pierAdminA1.ArchiveBoat(ctx, connect.NewRequest(&catalogv1.ArchiveBoatRequest{BoatId: boat.BoatId}))
	if err != nil {
		t.Fatalf("re-archive boat: %v", err)
	}
	if !archResp2.Msg.Boat.Archived {
		t.Fatalf("re-archived boat.Archived = false, want true (idempotent)")
	}
	if n := countOutboxEvents(t, dsn, app.EventBoatUpserted, boat.BoatId); n != 2 {
		t.Fatalf("BoatUpserted outbox rows = %d, want 2 (1 create + 1 archive; the no-op re-archive publishes nothing)", n)
	}

	// Editing an archived boat -> FailedPrecondition.
	_, err = pierAdminA1.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		BoatId: boat.BoatId, Name: "Edit After Archive", DefaultCapacity: 5, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE, HomePierId: a1,
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "UpsertBoat on archived boat")

	// Public (no-claims) ListBoats excludes the archived boat.
	public := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	publicList, err := public.ListBoats(ctx, connect.NewRequest(&catalogv1.ListBoatsRequest{}))
	if err != nil {
		t.Fatalf("public ListBoats: %v", err)
	}
	for _, b := range publicList.Msg.Boats {
		if b.BoatId == boat.BoatId {
			t.Errorf("public ListBoats includes archived boat %s", boat.BoatId)
		}
	}
}

// TestPierPhotoKeyAndUrl proves D-19 end to end: PresignPierPhoto mints a
// key only super_admin/pier_admin may request; UpsertPier accepts that key
// and returns photo_url built from PHOTO_PUBLIC_BASE_URL; the public (no
// claims) ListPiers projects the same photo_url.
func TestPierPhotoKeyAndUrl(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	t.Setenv("S3_PUBLIC_ENDPOINT", "localhost:8333")
	t.Setenv("S3_ACCESS_KEY", "test-access-key")
	t.Setenv("S3_SECRET_KEY", "test-secret-key")
	t.Setenv("PHOTO_PUBLIC_BASE_URL", "http://localhost:8333/pier-photos")
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))
	opID := mustCreateOperator(t, ctx, superAdmin, "Photo Pier Operator")

	presignResp, err := superAdmin.PresignPierPhoto(ctx, connect.NewRequest(&catalogv1.PresignPierPhotoRequest{
		ContentType: "image/png", SizeBytes: 1024,
	}))
	if err != nil {
		t.Fatalf("PresignPierPhoto: %v", err)
	}
	key := presignResp.Msg.PhotoKey
	if presignResp.Msg.UploadUrl == "" {
		t.Fatalf("PresignPierPhoto returned empty upload_url")
	}

	pierResp, err := superAdmin.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		OperatorId: opID, NameTh: "ท่ารูป", NameEn: "Photo Pier", Lat: 7.45, Lng: 98.15, PhotoKey: key,
	}))
	if err != nil {
		t.Fatalf("UpsertPier with photo_key: %v", err)
	}
	if pierResp.Msg.Pier.PhotoKey != key {
		t.Errorf("stored photo_key = %q, want %q", pierResp.Msg.Pier.PhotoKey, key)
	}
	wantURL := "http://localhost:8333/pier-photos/" + key
	if pierResp.Msg.Pier.PhotoUrl != wantURL {
		t.Errorf("UpsertPier response photo_url = %q, want %q", pierResp.Msg.Pier.PhotoUrl, wantURL)
	}

	public := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	listResp, err := public.ListPiers(ctx, connect.NewRequest(&catalogv1.ListPiersRequest{}))
	if err != nil {
		t.Fatalf("public ListPiers: %v", err)
	}
	var found *catalogv1.Pier
	for _, p := range listResp.Msg.Piers {
		if p.PierId == pierResp.Msg.Pier.PierId {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("pier %s not found in public ListPiers", pierResp.Msg.Pier.PierId)
	}
	if found.PhotoUrl != wantURL {
		t.Errorf("public ListPiers photo_url = %q, want %q", found.PhotoUrl, wantURL)
	}

	// staff cannot presign (read-only role).
	staffClient := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleStaff, opID))
	_, err = staffClient.PresignPierPhoto(ctx, connect.NewRequest(&catalogv1.PresignPierPhotoRequest{
		ContentType: "image/png", SizeBytes: 1024,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "staff PresignPierPhoto")
}

// TestPierPhotoUrlEmptyWithoutStorageConfig proves the D-19 fallbacks: with
// no S3_PUBLIC_ENDPOINT configured, PresignPierPhoto returns
// FailedPrecondition and catalog still starts and serves every other RPC;
// and with a valid stored photo_key but no PHOTO_PUBLIC_BASE_URL, photo_url
// stays empty rather than a malformed URL.
func TestPierPhotoUrlEmptyWithoutStorageConfig(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))

	_, err := superAdmin.PresignPierPhoto(ctx, connect.NewRequest(&catalogv1.PresignPierPhotoRequest{
		ContentType: "image/png", SizeBytes: 1024,
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "PresignPierPhoto with no storage configured")

	// A directly-assigned, validly-shaped key (bypassing the presign flow,
	// which needs storage configured) still round-trips through UpsertPier;
	// with PHOTO_PUBLIC_BASE_URL unset, photo_url is empty rather than a
	// malformed URL missing its base.
	opID := mustCreateOperator(t, ctx, superAdmin, "No Storage Pier Operator")
	manualKey := "piers/" + uuid.NewString() + ".png"
	pierResp, err := superAdmin.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		OperatorId: opID, NameTh: "ท่าไม่มีรูป", NameEn: "No Photo Pier", Lat: 7.46, Lng: 98.16, PhotoKey: manualKey,
	}))
	if err != nil {
		t.Fatalf("UpsertPier with manual photo_key: %v", err)
	}
	if pierResp.Msg.Pier.PhotoKey != manualKey {
		t.Errorf("stored photo_key = %q, want %q", pierResp.Msg.Pier.PhotoKey, manualKey)
	}
	if pierResp.Msg.Pier.PhotoUrl != "" {
		t.Errorf("photo_url = %q, want empty (no PHOTO_PUBLIC_BASE_URL configured)", pierResp.Msg.Pier.PhotoUrl)
	}
}
