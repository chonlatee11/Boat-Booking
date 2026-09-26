//go:build integration

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
)

func mustCreateRoute(t *testing.T, ctx context.Context, client catalogv1connect.CatalogServiceClient, pierFrom, pierTo string, duration int32) *catalogv1.Route {
	t.Helper()
	resp, err := client.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		PierFromId: pierFrom, PierToId: pierTo, DurationMinutes: duration,
	}))
	if err != nil {
		t.Fatalf("create route %s->%s: %v", pierFrom, pierTo, err)
	}
	return resp.Msg.Route
}

// TestRoutesScopingAndSharedPierTo proves D-11/D-12/CAT-03: pier_from must
// be in the caller's (operator_id, pier_ids) scope, pier_to may be any
// non-archived pier of any operator; route.operator_id is always derived
// from pier_from, never the request; a second active route for the same
// pair is AlreadyExists; update-by-route_id updates the same row; and the
// public (no-claims) listing includes the route.
func TestRoutesScopingAndSharedPierTo(t *testing.T) {
	addr, dsn := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))

	opA := mustCreateOperator(t, ctx, superAdmin, "Operator A")
	opB := mustCreateOperator(t, ctx, superAdmin, "Operator B")

	a1 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า A1", "Pier A1", 7.9, 98.3)
	a2 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า A2", "Pier A2", 7.91, 98.31)
	b1 := mustCreatePier(t, ctx, superAdmin, opB, "ท่า B1", "Pier B1", 8.0, 98.4)

	pierAdminA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA, a1))

	route := mustCreateRoute(t, ctx, pierAdminA1, a1, b1, 45)
	if route.OperatorId != opA {
		t.Errorf("route.OperatorId = %q, want %q", route.OperatorId, opA)
	}
	wantDefault := []struct{ min, pct int32 }{{24, 100}, {2, 50}, {0, 0}}
	if len(route.CancellationPolicy) != len(wantDefault) {
		t.Fatalf("cancellation_policy len = %d, want %d", len(route.CancellationPolicy), len(wantDefault))
	}
	for i, w := range wantDefault {
		got := route.CancellationPolicy[i]
		if got.MinHoursBefore != w.min || got.RefundPercent != w.pct {
			t.Errorf("cancellation_policy[%d] = {%d,%d}, want {%d,%d}", i, got.MinHoursBefore, got.RefundPercent, w.min, w.pct)
		}
	}
	if n := countOutboxEvents(t, dsn, app.EventRouteUpserted, route.RouteId); n != 1 {
		t.Fatalf("outbox rows after create = %d, want 1", n)
	}

	// pier_admin(A,[A1]) cannot create from A2 (out-of-scope pier_from).
	_, err := pierAdminA1.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		PierFromId: a2, PierToId: a1, DurationMinutes: 10,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "pier_admin(A,[A1]) create route from out-of-scope pier A2")

	staffA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleStaff, opA, a1))
	_, err = staffA1.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		PierFromId: a1, PierToId: b1, DurationMinutes: 10,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "staff UpsertRoute")

	customer := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleCustomer, opA))
	_, err = customer.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		PierFromId: a1, PierToId: b1, DurationMinutes: 10,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "customer UpsertRoute")

	// A second create for the same active (pier_from, pier_to) pair.
	_, err = pierAdminA1.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		PierFromId: a1, PierToId: b1, DurationMinutes: 20,
	}))
	assertConnectCode(t, err, connect.CodeAlreadyExists, "repeat create same active pair")

	// Update by route_id changes the same row in place.
	updResp, err := pierAdminA1.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		RouteId: route.RouteId, PierFromId: a1, PierToId: b1, DurationMinutes: 60,
	}))
	if err != nil {
		t.Fatalf("update route duration: %v", err)
	}
	if updResp.Msg.Route.RouteId != route.RouteId {
		t.Errorf("updated route id = %q, want %q (same row)", updResp.Msg.Route.RouteId, route.RouteId)
	}
	if updResp.Msg.Route.DurationMinutes != 60 {
		t.Errorf("updated duration = %d, want 60", updResp.Msg.Route.DurationMinutes)
	}

	// pier_admin(B,[B1]) ListRoutes excludes A's route (pier_from A1 is not
	// in B's scope); ListPiers for pier_admin(B,[B1]) sees only B1.
	pierAdminB1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opB, b1))
	listB, err := pierAdminB1.ListRoutes(ctx, connect.NewRequest(&catalogv1.ListRoutesRequest{}))
	if err != nil {
		t.Fatalf("pier_admin(B) ListRoutes: %v", err)
	}
	if len(listB.Msg.Routes) != 0 {
		t.Fatalf("pier_admin(B,[B1]) ListRoutes = %+v, want empty", listB.Msg.Routes)
	}
	assertPierIDs(t, mustListPiers(t, ctx, pierAdminB1, ""), b1)

	// pier_admin(B,[B1]) cannot touch A1 (different operator entirely).
	_, err = pierAdminB1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: a1, NameTh: "Hijack", NameEn: "Hijack", Lat: 7.9, Lng: 98.3,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "pier_admin(B) UpsertPier on A1")

	// No-claims ListRoutes (the public projection) includes the route with
	// its pier ids and duration.
	public := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	publicList, err := public.ListRoutes(ctx, connect.NewRequest(&catalogv1.ListRoutesRequest{}))
	if err != nil {
		t.Fatalf("public ListRoutes: %v", err)
	}
	var found *catalogv1.Route
	for _, r := range publicList.Msg.Routes {
		if r.RouteId == route.RouteId {
			found = r
		}
	}
	if found == nil {
		t.Fatalf("public ListRoutes did not include route %s", route.RouteId)
	}
	if found.PierFromId != a1 || found.PierToId != b1 || found.DurationMinutes != 60 {
		t.Errorf("public route = %+v, want pier_from=%s pier_to=%s duration=60", found, a1, b1)
	}
}
