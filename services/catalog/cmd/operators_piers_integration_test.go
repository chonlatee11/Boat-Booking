//go:build integration

package main

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
)

// countOutboxEvents queries dsn directly for the number of outbox rows
// matching eventType/aggregateID — proves an upsert wrote exactly the
// expected number of outbox facts (D-05), independent of whether the relay
// has published them yet.
func countOutboxEvents(t *testing.T, dsn, eventType, aggregateID string) int {
	t.Helper()
	ctx := context.Background()
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("countOutboxEvents: new pool: %v", err)
	}
	defer pool.Close()

	var count int
	err = pool.QueryRow(ctx,
		`select count(*) from outbox where event_type = $1 and aggregate_id = $2`,
		eventType, aggregateID).Scan(&count)
	if err != nil {
		t.Fatalf("countOutboxEvents: query: %v", err)
	}
	return count
}

// TestOperatorsSuperAdminOnly proves D-08/AUTH-05 for operators: only
// super_admin may create or rename an operator; every other role (including
// no claims at all) is denied; ListOperators applies the same Scope rule
// (super_admin sees everything, pier_admin sees only their own operator),
// ordered by name then id.
func TestOperatorsSuperAdminOnly(t *testing.T) {
	addr, dsn := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))

	resp, err := superAdmin.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{Name: "Original"}))
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}
	opID := resp.Msg.Operator.OperatorId
	if _, err := uuid.Parse(opID); err != nil {
		t.Fatalf("operator_id %q is not a valid uuid: %v", opID, err)
	}
	if resp.Msg.Operator.Name != "Original" || resp.Msg.Operator.Archived {
		t.Errorf("created operator = %+v, want Name=Original Archived=false", resp.Msg.Operator)
	}
	if n := countOutboxEvents(t, dsn, app.EventOperatorUpserted, opID); n != 1 {
		t.Fatalf("outbox rows after create = %d, want 1", n)
	}

	renameResp, err := superAdmin.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{
		OperatorId: opID, Name: "Renamed",
	}))
	if err != nil {
		t.Fatalf("rename operator: %v", err)
	}
	if renameResp.Msg.Operator.Name != "Renamed" {
		t.Errorf("renamed operator.Name = %q, want Renamed", renameResp.Msg.Operator.Name)
	}
	if n := countOutboxEvents(t, dsn, app.EventOperatorUpserted, opID); n != 2 {
		t.Fatalf("outbox rows after rename = %d, want 2 (still one row, two upserts)", n)
	}

	for _, role := range []string{auth.RolePierAdmin, auth.RoleStaff, auth.RoleCustomer} {
		client := newAuthedCatalogClient(baseURL, claimHeaders(role, opID))
		_, err := client.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{Name: "Hijack"}))
		assertConnectCode(t, err, connect.CodePermissionDenied, "UpsertOperator role="+role)
	}

	anon := newCatalogClient(baseURL)
	_, err = anon.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{Name: "Anon"}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "UpsertOperator no claims")

	// A lexically-earlier operator, and a second operator sharing opID's
	// ("Renamed") name to prove the name-then-id tiebreak.
	respAlpha, err := superAdmin.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{Name: "Alpha Co"}))
	if err != nil {
		t.Fatalf("create operator Alpha Co: %v", err)
	}
	respTwin, err := superAdmin.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{Name: "Renamed"}))
	if err != nil {
		t.Fatalf("create second Renamed operator: %v", err)
	}

	pierAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opID))
	listScoped, err := pierAdmin.ListOperators(ctx, connect.NewRequest(&catalogv1.ListOperatorsRequest{}))
	if err != nil {
		t.Fatalf("pier_admin ListOperators: %v", err)
	}
	if len(listScoped.Msg.Operators) != 1 || listScoped.Msg.Operators[0].OperatorId != opID {
		t.Fatalf("pier_admin ListOperators = %+v, want only operator %s", listScoped.Msg.Operators, opID)
	}

	listAll, err := superAdmin.ListOperators(ctx, connect.NewRequest(&catalogv1.ListOperatorsRequest{}))
	if err != nil {
		t.Fatalf("super_admin ListOperators: %v", err)
	}
	if len(listAll.Msg.Operators) != 3 {
		t.Fatalf("super_admin ListOperators len = %d, want 3", len(listAll.Msg.Operators))
	}
	if got := listAll.Msg.Operators[0].OperatorId; got != respAlpha.Msg.Operator.OperatorId {
		t.Errorf("operators[0] = %s, want Alpha Co (%s)", got, respAlpha.Msg.Operator.OperatorId)
	}
	// opID ("Renamed") was created before respTwin ("Renamed") -> id order
	// keeps opID first among the two equally-named rows.
	if got0, got1 := listAll.Msg.Operators[1].OperatorId, listAll.Msg.Operators[2].OperatorId; got0 != opID || got1 != respTwin.Msg.Operator.OperatorId {
		t.Errorf("name-tie order = [%s, %s], want [%s, %s]", got0, got1, opID, respTwin.Msg.Operator.OperatorId)
	}
}

// fetchOutboxEnvelope queries dsn directly for the most recent outbox row
// matching eventType/aggregateID and unmarshals its payload — proves what
// an upsert actually wrote to the outbox (D-05), independent of whether the
// relay has published it yet.
func fetchOutboxEnvelope(t *testing.T, dsn, eventType, aggregateID string) *catalogv1.PierUpserted {
	t.Helper()
	ctx := context.Background()
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("fetchOutboxEnvelope: new pool: %v", err)
	}
	defer pool.Close()

	var raw []byte
	err = pool.QueryRow(ctx,
		`select payload from outbox where event_type = $1 and aggregate_id = $2 order by id desc limit 1`,
		eventType, aggregateID).Scan(&raw)
	if err != nil {
		t.Fatalf("fetchOutboxEnvelope: query: %v", err)
	}
	env, err := events.Unmarshal(raw)
	if err != nil {
		t.Fatalf("fetchOutboxEnvelope: unmarshal envelope: %v", err)
	}
	var payload catalogv1.PierUpserted
	if err := env.Payload.UnmarshalTo(&payload); err != nil {
		t.Fatalf("fetchOutboxEnvelope: unmarshal payload: %v", err)
	}
	return &payload
}

func mustCreateOperator(t *testing.T, ctx context.Context, client catalogv1connect.CatalogServiceClient, name string) string {
	t.Helper()
	resp, err := client.UpsertOperator(ctx, connect.NewRequest(&catalogv1.UpsertOperatorRequest{Name: name}))
	if err != nil {
		t.Fatalf("create operator %q: %v", name, err)
	}
	return resp.Msg.Operator.OperatorId
}

func mustCreatePier(t *testing.T, ctx context.Context, client catalogv1connect.CatalogServiceClient, operatorID, nameTH, nameEN string, lat, lng float64) string {
	t.Helper()
	resp, err := client.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		OperatorId: operatorID, NameTh: nameTH, NameEn: nameEN, Lat: lat, Lng: lng,
	}))
	if err != nil {
		t.Fatalf("create pier %q: %v", nameTH, err)
	}
	return resp.Msg.Pier.PierId
}

func mustListPiers(t *testing.T, ctx context.Context, client catalogv1connect.CatalogServiceClient, filterOperatorID string) *catalogv1.ListPiersResponse {
	t.Helper()
	resp, err := client.ListPiers(ctx, connect.NewRequest(&catalogv1.ListPiersRequest{OperatorId: filterOperatorID}))
	if err != nil {
		t.Fatalf("ListPiers: %v", err)
	}
	return resp.Msg
}

// assertPierIDs fails unless resp contains exactly the given pier ids
// (order-independent — only ListOperators/ListPiers's own name/id ordering
// is asserted elsewhere; this just proves membership).
func assertPierIDs(t *testing.T, resp *catalogv1.ListPiersResponse, want ...string) {
	t.Helper()
	got := make([]string, len(resp.Piers))
	for i, p := range resp.Piers {
		got[i] = p.PierId
	}
	if len(got) != len(want) {
		t.Fatalf("pier ids = %v, want %v", got, want)
	}
	wantSet := make(map[string]bool, len(want))
	for _, id := range want {
		wantSet[id] = true
	}
	for _, id := range got {
		if !wantSet[id] {
			t.Fatalf("pier ids = %v, want %v", got, want)
		}
	}
}

func mustGetPier(t *testing.T, ctx context.Context, client catalogv1connect.CatalogServiceClient, pierID string) *catalogv1.Pier {
	t.Helper()
	resp := mustListPiers(t, ctx, client, "")
	for _, p := range resp.Piers {
		if p.PierId == pierID {
			return p
		}
	}
	t.Fatalf("pier %s not found in list", pierID)
	return nil
}

// TestPiersScoping proves D-07/D-08/AUTH-05/CAT-06 for piers: super_admin
// creates piers under any operator; pier_admin/staff are scoped to
// (operator_id, pier_ids); an update can never move a pier to a different
// operator, out-of-scope/cross-operator ids answer NotFound (never
// PermissionDenied, never the row); the public (no-claims) call returns
// every non-archived pier; and PierUpserted carries no address (Pitfall 5).
func TestPiersScoping(t *testing.T) {
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

	// PierUpserted carries no address field at all (Pitfall 5) — proven at
	// the proto level (no field to set) and confirmed against the actual
	// outbox row.
	payload := fetchOutboxEnvelope(t, dsn, app.EventPierUpserted, a1)
	if payload.PierId != a1 || payload.OperatorId != opA {
		t.Fatalf("PierUpserted payload = %+v, want pier_id=%s operator_id=%s", payload, a1, opA)
	}
	for i := 0; i < payload.ProtoReflect().Descriptor().Fields().Len(); i++ {
		if name := string(payload.ProtoReflect().Descriptor().Fields().Get(i).Name()); name == "address" {
			t.Fatalf("PierUpserted proto has an address field — must never be published (Pitfall 5)")
		}
	}

	// pier_admin(A, [A1]) sees only A1.
	pierAdminA1 := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA, a1))
	assertPierIDs(t, mustListPiers(t, ctx, pierAdminA1, ""), a1)

	// pier_admin(A, []) sees nothing — an empty pier_ids scope is empty,
	// never "everything" (AUTH-05 empty edge, Pitfall 6).
	pierAdminNoPiers := newAuthedCatalogClient(baseURL, claimHeaders(auth.RolePierAdmin, opA))
	assertPierIDs(t, mustListPiers(t, ctx, pierAdminNoPiers, ""))

	// staff(A, [A1, A2]) sees both — staff is read-only but shares the same
	// scoping rule as pier_admin.
	staffA := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleStaff, opA, a1, a2))
	assertPierIDs(t, mustListPiers(t, ctx, staffA, ""), a1, a2)

	// super_admin sees every pier across every operator.
	assertPierIDs(t, mustListPiers(t, ctx, superAdmin, ""), a1, a2, b1)

	// super_admin filtered by operator_id=B sees only B1 (filter honoured
	// for super_admin only).
	assertPierIDs(t, mustListPiers(t, ctx, superAdmin, opB), b1)

	// pier_admin(A,[A1]) can update A1.
	updResp, err := pierAdminA1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: a1, NameTh: "ท่า A1 ใหม่", NameEn: "Pier A1 New", Lat: 7.9, Lng: 98.3,
	}))
	if err != nil {
		t.Fatalf("pier_admin update A1: %v", err)
	}
	if updResp.Msg.Pier.NameEn != "Pier A1 New" {
		t.Errorf("updated pier name = %q, want Pier A1 New", updResp.Msg.Pier.NameEn)
	}

	// pier_admin(A,[A1]) cannot touch A2 — same operator, but out of
	// pier_ids scope — NotFound, never PermissionDenied, never the row.
	_, err = pierAdminA1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: a2, NameTh: "Hijack", NameEn: "Hijack", Lat: 7.91, Lng: 98.31,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "pier_admin update out-of-scope pier A2")

	// pier_admin(A,[A1]) cannot touch B1 — different operator entirely —
	// NotFound, and B1 stays unchanged.
	_, err = pierAdminA1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: b1, NameTh: "Hijack", NameEn: "Hijack", Lat: 8.0, Lng: 98.4,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "pier_admin update cross-operator pier B1")
	if b1Check := mustGetPier(t, ctx, superAdmin, b1); b1Check.NameEn != "Pier B1" {
		t.Errorf("B1 changed after rejected cross-operator update: %+v", b1Check)
	}

	// pier_admin cannot create (empty pier_id) — creation is super_admin
	// only (D-08).
	_, err = pierAdminA1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		OperatorId: opA, NameTh: "New", NameEn: "New", Lat: 7.92, Lng: 98.32,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "pier_admin create pier")

	// Sending operator_id=B on an update of A1 never moves it — the stored
	// operator_id always wins over the request body (D-30).
	_, err = pierAdminA1.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: a1, OperatorId: opB, NameTh: "ท่า A1 ใหม่", NameEn: "Pier A1 New", Lat: 7.9, Lng: 98.3,
	}))
	if err != nil {
		t.Fatalf("pier_admin update A1 with spoofed operator_id: %v", err)
	}
	if a1Check := mustGetPier(t, ctx, superAdmin, a1); a1Check.OperatorId != opA {
		t.Errorf("A1 operator_id changed to %q, want unchanged %q", a1Check.OperatorId, opA)
	}

	// staff is read-only — every write is denied.
	_, err = staffA.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		PierId: a1, NameTh: "x", NameEn: "x", Lat: 7.9, Lng: 98.3,
	}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "staff UpsertPier")

	// customer cannot even list (not an admin role at all).
	customer := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleCustomer, opA))
	_, err = customer.ListPiers(ctx, connect.NewRequest(&catalogv1.ListPiersRequest{}))
	assertConnectCode(t, err, connect.CodePermissionDenied, "customer ListPiers")

	// Internal token but no user claims -> the public projection: every
	// non-archived pier, regardless of operator (CAT-06). This is the shape
	// of the gateway's public BFF route: internal token present, no claim
	// headers.
	public := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	publicList, err := public.ListPiers(ctx, connect.NewRequest(&catalogv1.ListPiersRequest{}))
	if err != nil {
		t.Fatalf("public ListPiers: %v", err)
	}
	assertPierIDs(t, publicList.Msg, a1, a2, b1)
}

// TestArchiveOperatorBlockedByPiers proves D-15: archiving an operator with
// non-archived piers is rejected with FailedPrecondition (no cascade);
// archiving an operator with none succeeds and is idempotent; creating a
// pier for an archived operator fails with FailedPrecondition.
func TestArchiveOperatorBlockedByPiers(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))

	opWithPier := mustCreateOperator(t, ctx, superAdmin, "Has A Pier")
	mustCreatePier(t, ctx, superAdmin, opWithPier, "ท่า", "Pier", 7.9, 98.3)

	_, err := superAdmin.ArchiveOperator(ctx, connect.NewRequest(&catalogv1.ArchiveOperatorRequest{OperatorId: opWithPier}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "archive operator with an active pier")

	opEmpty := mustCreateOperator(t, ctx, superAdmin, "No Piers")
	resp, err := superAdmin.ArchiveOperator(ctx, connect.NewRequest(&catalogv1.ArchiveOperatorRequest{OperatorId: opEmpty}))
	if err != nil {
		t.Fatalf("archive operator with no piers: %v", err)
	}
	if !resp.Msg.Operator.Archived {
		t.Fatalf("archived operator.Archived = false, want true")
	}

	resp2, err := superAdmin.ArchiveOperator(ctx, connect.NewRequest(&catalogv1.ArchiveOperatorRequest{OperatorId: opEmpty}))
	if err != nil {
		t.Fatalf("re-archive already-archived operator: %v", err)
	}
	if !resp2.Msg.Operator.Archived {
		t.Fatalf("re-archived operator.Archived = false, want true (idempotent)")
	}

	_, err = superAdmin.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
		OperatorId: opEmpty, NameTh: "x", NameEn: "x", Lat: 7.9, Lng: 98.3,
	}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "create pier for archived operator")
}

// TestArchiveOperatorRaceWithPierCreate proves the WR-06 fix: a concurrent
// ArchiveOperator and UpsertPier (create) on the same operator must never
// both succeed. createPier's FOR SHARE and ArchiveOperator's FOR UPDATE on
// the same operator row make the two serialize instead of racing past each
// other under READ COMMITTED.
func TestArchiveOperatorRaceWithPierCreate(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	superAdmin := newAuthedCatalogClient(baseURL, claimHeaders(auth.RoleSuperAdmin, ""))
	op := mustCreateOperator(t, ctx, superAdmin, "Race Operator")

	var wg sync.WaitGroup
	var archiveErr, createErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, archiveErr = superAdmin.ArchiveOperator(ctx, connect.NewRequest(&catalogv1.ArchiveOperatorRequest{OperatorId: op}))
	}()
	go func() {
		defer wg.Done()
		_, createErr = superAdmin.UpsertPier(ctx, connect.NewRequest(&catalogv1.UpsertPierRequest{
			OperatorId: op, NameTh: "ท่าแข่ง", NameEn: "Race Pier", Lat: 7.9, Lng: 98.3,
		}))
	}()
	wg.Wait()

	if archiveErr == nil && createErr == nil {
		t.Fatalf("both ArchiveOperator and a concurrent UpsertPier (create) succeeded — an archived operator must never end up owning a non-archived pier (WR-06)")
	}
	if archiveErr != nil {
		assertConnectCode(t, archiveErr, connect.CodeFailedPrecondition, "ArchiveOperator lost the race")
	}
	if createErr != nil {
		assertConnectCode(t, createErr, connect.CodeFailedPrecondition, "UpsertPier lost the race")
	}
}
