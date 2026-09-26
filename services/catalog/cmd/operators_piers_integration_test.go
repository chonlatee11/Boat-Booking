//go:build integration

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/auth"
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
