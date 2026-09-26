//go:build integration

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/clock"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
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

// archiveRouteDirectly sets archived_at on routeID via a direct DB write —
// used to exercise AddRoutePrice's archived-route rejection ahead of
// ArchiveRoute existing as an RPC (Task 3).
func archiveRouteDirectly(t *testing.T, dsn, routeID string) {
	t.Helper()
	ctx := context.Background()
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("archiveRouteDirectly: new pool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `update routes set archived_at = now() where id = $1::uuid`, routeID); err != nil {
		t.Fatalf("archiveRouteDirectly: update: %v", err)
	}
}

// TestRoutePricesEffectiveDating proves D-14: the price in effect on a date
// is the row with the latest effective_from <= that date (inclusive
// boundary); re-adding the same (route, ticket_type, effective_from)
// replaces the amount in one row; ListRoutePrices orders effective_from
// desc then ticket_type; a route with no prices has no current_prices and
// an empty ListRoutePrices; and the route-scope/role rules from AddRoutePrice
// mirror UpsertRoute's.
func TestRoutePricesEffectiveDating(t *testing.T) {
	addr, dsn := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))

	opA := mustCreateOperator(t, ctx, superAdmin, "Operator Prices A")
	opB := mustCreateOperator(t, ctx, superAdmin, "Operator Prices B")
	a1 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า PA1", "Pier PA1", 7.5, 98.1)
	b1 := mustCreatePier(t, ctx, superAdmin, opB, "ท่า PB1", "Pier PB1", 7.6, 98.2)
	b2 := mustCreatePier(t, ctx, superAdmin, opB, "ท่า PB2", "Pier PB2", 7.7, 98.3)

	pierAdminA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA, a1))
	route := mustCreateRoute(t, ctx, pierAdminA1, a1, b1, 30)

	today := clock.LocalDate(clock.Now())
	todayStr := today.Format("2006-01-02")
	plus10 := today.AddDate(0, 0, 10)
	plus10Str := plus10.Format("2006-01-02")

	mustAddPrice := func(client catalogv1connect.CatalogServiceClient, routeID string, ticketType catalogv1.TicketType, amount int64, effectiveFrom string) *catalogv1.RoutePrice {
		t.Helper()
		resp, err := client.AddRoutePrice(ctx, connect.NewRequest(&catalogv1.AddRoutePriceRequest{
			RouteId: routeID, TicketType: ticketType, AmountSatang: amount, EffectiveFrom: effectiveFrom,
		}))
		if err != nil {
			t.Fatalf("AddRoutePrice route=%s ticket=%v amount=%d effective=%s: %v", routeID, ticketType, amount, effectiveFrom, err)
		}
		return resp.Msg.Price
	}

	mustAddPrice(pierAdminA1, route.RouteId, catalogv1.TicketType_TICKET_TYPE_ADULT, 15000, todayStr)
	mustAddPrice(pierAdminA1, route.RouteId, catalogv1.TicketType_TICKET_TYPE_ADULT, 20000, plus10Str)
	mustAddPrice(pierAdminA1, route.RouteId, catalogv1.TicketType_TICKET_TYPE_CHILD, 8000, todayStr)

	public := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	assertCurrentPrices := func(wantAdult, wantChild int64) {
		t.Helper()
		resp, err := public.ListRoutes(ctx, connect.NewRequest(&catalogv1.ListRoutesRequest{}))
		if err != nil {
			t.Fatalf("public ListRoutes: %v", err)
		}
		var found *catalogv1.Route
		for _, r := range resp.Msg.Routes {
			if r.RouteId == route.RouteId {
				found = r
			}
		}
		if found == nil {
			t.Fatalf("route %s not found in public ListRoutes", route.RouteId)
		}
		got := map[catalogv1.TicketType]int64{}
		for _, p := range found.CurrentPrices {
			got[p.TicketType] = p.AmountSatang
		}
		if got[catalogv1.TicketType_TICKET_TYPE_ADULT] != wantAdult {
			t.Errorf("current adult price = %d, want %d", got[catalogv1.TicketType_TICKET_TYPE_ADULT], wantAdult)
		}
		if got[catalogv1.TicketType_TICKET_TYPE_CHILD] != wantChild {
			t.Errorf("current child price = %d, want %d", got[catalogv1.TicketType_TICKET_TYPE_CHILD], wantChild)
		}
	}
	assertCurrentPrices(15000, 8000)

	// Override clock.Now to a moment on the plus10 local date — the
	// service runs in-process, so this affects its own clock.LocalDate
	// calls directly (inclusive boundary: a row effective exactly on D
	// applies on D).
	origNow := clock.Now
	clock.Now = func() time.Time { return plus10.Add(12 * time.Hour) }
	assertCurrentPrices(20000, 8000)
	clock.Now = origNow

	// Re-adding the same (route, adult, today) replaces the amount in
	// place — still one row, now 16000.
	mustAddPrice(pierAdminA1, route.RouteId, catalogv1.TicketType_TICKET_TYPE_ADULT, 16000, todayStr)
	assertCurrentPrices(16000, 8000)

	if n := countOutboxEvents(t, dsn, app.EventPriceChanged, route.RouteId); n != 4 {
		t.Fatalf("PriceChanged outbox rows = %d, want 4 (3 adds + 1 replace)", n)
	}

	listResp, err := pierAdminA1.ListRoutePrices(ctx, connect.NewRequest(&catalogv1.ListRoutePricesRequest{RouteId: route.RouteId}))
	if err != nil {
		t.Fatalf("ListRoutePrices: %v", err)
	}
	if len(listResp.Msg.Prices) != 3 {
		t.Fatalf("ListRoutePrices len = %d, want 3", len(listResp.Msg.Prices))
	}
	if p := listResp.Msg.Prices[0]; p.EffectiveFrom != plus10Str || p.TicketType != catalogv1.TicketType_TICKET_TYPE_ADULT {
		t.Errorf("Prices[0] = %+v, want effective_from=%s ticket=ADULT", p, plus10Str)
	}
	if p := listResp.Msg.Prices[1]; p.EffectiveFrom != todayStr || p.TicketType != catalogv1.TicketType_TICKET_TYPE_ADULT {
		t.Errorf("Prices[1] = %+v, want effective_from=%s ticket=ADULT", p, todayStr)
	}
	if p := listResp.Msg.Prices[2]; p.EffectiveFrom != todayStr || p.TicketType != catalogv1.TicketType_TICKET_TYPE_CHILD {
		t.Errorf("Prices[2] = %+v, want effective_from=%s ticket=CHILD", p, todayStr)
	}

	// A route with no prices has no current_prices entries and an empty
	// ListRoutePrices.
	routeNoPrices := mustCreateRoute(t, ctx, pierAdminA1, a1, b2, 20)
	emptyResp, err := pierAdminA1.ListRoutePrices(ctx, connect.NewRequest(&catalogv1.ListRoutePricesRequest{RouteId: routeNoPrices.RouteId}))
	if err != nil {
		t.Fatalf("ListRoutePrices (no prices): %v", err)
	}
	if len(emptyResp.Msg.Prices) != 0 {
		t.Fatalf("ListRoutePrices (no prices) = %+v, want empty", emptyResp.Msg.Prices)
	}
	publicResp2, err := public.ListRoutes(ctx, connect.NewRequest(&catalogv1.ListRoutesRequest{}))
	if err != nil {
		t.Fatalf("public ListRoutes (no-prices route): %v", err)
	}
	for _, r := range publicResp2.Msg.Routes {
		if r.RouteId == routeNoPrices.RouteId && len(r.CurrentPrices) != 0 {
			t.Errorf("route without prices has current_prices = %+v, want empty (absent, never zero)", r.CurrentPrices)
		}
	}

	// pier_admin of another operator -> NotFound (route not in their scope).
	pierAdminB1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opB, b1))
	_, err = pierAdminB1.AddRoutePrice(ctx, connect.NewRequest(&catalogv1.AddRoutePriceRequest{
		RouteId: route.RouteId, TicketType: catalogv1.TicketType_TICKET_TYPE_ADULT, AmountSatang: 1, EffectiveFrom: todayStr,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "pier_admin(B) AddRoutePrice on A's route")

	// staff -> PermissionDenied (read-only).
	staffA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleStaff, opA, a1))
	_, err = staffA1.AddRoutePrice(ctx, connect.NewRequest(&catalogv1.AddRoutePriceRequest{
		RouteId: route.RouteId, TicketType: catalogv1.TicketType_TICKET_TYPE_ADULT, AmountSatang: 1, EffectiveFrom: todayStr,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "staff AddRoutePrice")

	// archived route -> FailedPrecondition (archived directly since
	// ArchiveRoute is Task 3's RPC).
	archiveRouteDirectly(t, dsn, routeNoPrices.RouteId)
	_, err = pierAdminA1.AddRoutePrice(ctx, connect.NewRequest(&catalogv1.AddRoutePriceRequest{
		RouteId: routeNoPrices.RouteId, TicketType: catalogv1.TicketType_TICKET_TYPE_ADULT, AmountSatang: 1, EffectiveFrom: todayStr,
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "AddRoutePrice on archived route")
}

// TestArchivePierBlockedByRoutes proves D-15/CAT-02/CAT-03/T-02-06-03:
// ArchivePier is rejected with FailedPrecondition while any non-archived
// route uses the pier, naming only routes in the caller's scope and
// counting the rest as "+N routes of other operators" — never leaking
// another operator's route/pier names or ids; archiving is idempotent; and
// once no active routes remain, ArchivePier succeeds and the public
// listings and dependent writes reflect the archive.
func TestArchivePierBlockedByRoutes(t *testing.T) {
	addr, dsn := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))

	opA := mustCreateOperator(t, ctx, superAdmin, "Operator Archive A")
	opB := mustCreateOperator(t, ctx, superAdmin, "Operator Archive B")
	a1 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า AR1", "Pier AR1", 7.2, 98.0)
	a2 := mustCreatePier(t, ctx, superAdmin, opA, "ท่า AR2", "Pier AR2", 7.21, 98.01)
	b1 := mustCreatePier(t, ctx, superAdmin, opB, "ท่า BR1", "Pier BR1", 7.3, 98.1)

	pierAdminA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA, a1))
	route := mustCreateRoute(t, ctx, pierAdminA1, a1, b1, 25)

	// pier_admin(A,[A1]) ArchivePier(A1) -> FailedPrecondition naming both
	// piers of A's own route.
	_, err := pierAdminA1.ArchivePier(ctx, connect.NewRequest(&catalogv1.ArchivePierRequest{PierId: a1}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ArchivePier(A1) code = %v, want FailedPrecondition (err: %v)", connect.CodeOf(err), err)
	}
	if msg := err.Error(); !strings.Contains(msg, "ท่า AR1") || !strings.Contains(msg, "ท่า BR1") {
		t.Errorf("ArchivePier(A1) message = %q, want to contain both pier names", msg)
	}

	// pier_admin(B,[B1]) ArchivePier(B1) -> FailedPrecondition counting the
	// route as "other operators" (the route's operator_id is A's, not B's)
	// and naming neither A1 nor the route id.
	pierAdminB1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opB, b1))
	_, err = pierAdminB1.ArchivePier(ctx, connect.NewRequest(&catalogv1.ArchivePierRequest{PierId: b1}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ArchivePier(B1) code = %v, want FailedPrecondition (err: %v)", connect.CodeOf(err), err)
	}
	msgB := err.Error()
	if !strings.Contains(msgB, "+1 routes of other operators") {
		t.Errorf("ArchivePier(B1) message = %q, want to contain '+1 routes of other operators'", msgB)
	}
	if strings.Contains(msgB, "ท่า AR1") || strings.Contains(msgB, route.RouteId) {
		t.Errorf("ArchivePier(B1) message = %q, must not name A1 or the route id", msgB)
	}

	// ArchiveRoute -> ok; repeat -> ok (idempotent, single archive event).
	archResp, err := pierAdminA1.ArchiveRoute(ctx, connect.NewRequest(&catalogv1.ArchiveRouteRequest{RouteId: route.RouteId}))
	if err != nil {
		t.Fatalf("ArchiveRoute: %v", err)
	}
	if !archResp.Msg.Route.Archived {
		t.Fatalf("archived route.Archived = false, want true")
	}
	archResp2, err := pierAdminA1.ArchiveRoute(ctx, connect.NewRequest(&catalogv1.ArchiveRouteRequest{RouteId: route.RouteId}))
	if err != nil {
		t.Fatalf("re-archive route: %v", err)
	}
	if !archResp2.Msg.Route.Archived {
		t.Fatalf("re-archived route.Archived = false, want true (idempotent)")
	}
	if n := countOutboxEvents(t, dsn, app.EventRouteUpserted, route.RouteId); n != 2 {
		t.Fatalf("RouteUpserted outbox rows = %d, want 2 (1 create + 1 archive; the no-op re-archive publishes nothing)", n)
	}

	// Now ArchivePier(A1) succeeds — no active routes remain.
	pierArchResp, err := pierAdminA1.ArchivePier(ctx, connect.NewRequest(&catalogv1.ArchivePierRequest{PierId: a1}))
	if err != nil {
		t.Fatalf("ArchivePier(A1) after route archived: %v", err)
	}
	if !pierArchResp.Msg.Pier.Archived {
		t.Fatalf("archived pier.Archived = false, want true")
	}

	// Public ListPiers/ListRoutes exclude A1 and the (also archived) route.
	public := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	publicPiers, err := public.ListPiers(ctx, connect.NewRequest(&catalogv1.ListPiersRequest{}))
	if err != nil {
		t.Fatalf("public ListPiers: %v", err)
	}
	for _, p := range publicPiers.Msg.Piers {
		if p.PierId == a1 {
			t.Errorf("public ListPiers includes archived pier A1")
		}
	}
	publicRoutes, err := public.ListRoutes(ctx, connect.NewRequest(&catalogv1.ListRoutesRequest{}))
	if err != nil {
		t.Fatalf("public ListRoutes: %v", err)
	}
	for _, r := range publicRoutes.Msg.Routes {
		if r.RouteId == route.RouteId {
			t.Errorf("public ListRoutes includes archived route")
		}
	}

	// UpsertPier on archived A1 -> FailedPrecondition.
	_, err = pierAdminA1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: a1, NameTh: "x", NameEn: "x", Lat: 7.2, Lng: 98.0,
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "UpsertPier on archived A1")

	// UpsertRoute from A2 to archived A1 -> FailedPrecondition.
	pierAdminA2 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA, a2))
	_, err = pierAdminA2.UpsertRoute(ctx, connect.NewRequest(&catalogv1.UpsertRouteRequest{
		PierFromId: a2, PierToId: a1, DurationMinutes: 15,
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "UpsertRoute to archived pier A1")
}
