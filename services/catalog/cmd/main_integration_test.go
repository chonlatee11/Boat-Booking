//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/pkg/testenv"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/app"
)

var (
	adminDSN string
	brokers  []string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	dsn, stopPG, err := testenv.StartPostgres(ctx)
	if err != nil {
		panic(err)
	}
	adminDSN = dsn

	bs, stopRP, err := testenv.StartRedpanda(ctx, serviceName)
	if err != nil {
		stopPG()
		panic(err)
	}
	brokers = bs

	code := m.Run()
	stopRP()
	stopPG()
	os.Exit(code)
}

// freePort picks a currently-unused 127.0.0.1 port for the service under
// test to bind. Closing the probe listener before the service starts leaves
// a small window for reuse by something else, acceptable for tests.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: listen: %v", err)
	}
	defer l.Close() //nolint:errcheck // test helper, nothing actionable
	return l.Addr().(*net.TCPAddr).Port
}

// setCatalogEnv sets the env vars run() reads, isolated per-test via a fresh
// DB and a fresh HTTP port. It returns both the listen address and the
// database's own DSN (the latter needed by tests that build their own
// verification Postgres pool).
func setCatalogEnv(t *testing.T) (addr, dsn string) {
	t.Helper()
	dsn = testenv.NewDB(t, adminDSN, "../migrations")
	port := freePort(t)
	addr = fmt.Sprintf("127.0.0.1:%d", port)

	t.Setenv("HTTP_ADDR", addr)
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("KAFKA_BROKERS", strings.Join(brokers, ","))
	t.Setenv("INTERNAL_TOKEN", "test-token")
	t.Setenv("LOG_FORMAT", "text")

	return addr, dsn
}

// runService starts run(ctx) in the background and registers cleanup that
// cancels it and waits for it to return, failing the test if it doesn't
// within budget. t.Cleanup runs LIFO, so merging cancel+wait into one
// registration keeps the shutdown-then-await ordering explicit regardless
// of what else is registered after it.
func runService(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(15 * time.Second):
			t.Error("run did not return within 15s of test cleanup cancellation")
		}
	})
}

// waitForFullyReady polls /readyz until it reports 200 with both db and
// kafka ok, or fails the test after a generous startup budget.
func waitForFullyReady(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if readyBody, ok := getReadyz(baseURL); ok {
			if readyBody["db"] == "ok" && readyBody["kafka"] == "ok" {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("service did not report db+kafka ready in time")
}

func getReadyz(baseURL string) (map[string]string, bool) {
	resp, err := http.Get(baseURL + "/readyz")
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	var body map[string]string
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, false
	}
	return body, true
}

// waitFor polls cond until it returns true or timeout elapses, failing the
// test if it never does.
func waitFor(t testing.TB, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", timeout)
	}
}

func TestTemplateReadyAndGracefulShutdown(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)

	waitForFullyReady(t, baseURL)

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
}

// newCatalogClient builds a plain (unauthenticated) connect client — used to
// prove the missing-token 401.
func newCatalogClient(baseURL string) catalogv1connect.CatalogServiceClient {
	return catalogv1connect.NewCatalogServiceClient(http.DefaultClient, baseURL)
}

// claimsInterceptor adds the trust-boundary headers RequireInternal expects
// (X-Internal-Token, plus claim headers when present) to every outgoing
// request — the real headers a gateway would set (D-30).
type claimsInterceptor struct {
	headers map[string]string
}

func (i claimsInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		for k, v := range i.headers {
			req.Header().Set(k, v)
		}
		return next(ctx, req)
	}
}

func (i claimsInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i claimsInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func newAuthedCatalogClient(baseURL string, headers map[string]string) catalogv1connect.CatalogServiceClient {
	return catalogv1connect.NewCatalogServiceClient(http.DefaultClient, baseURL,
		connect.WithInterceptors(claimsInterceptor{headers: headers}))
}

func newClaimHeaders(operatorID string) map[string]string {
	return map[string]string{
		httpx.HeaderInternalToken: "test-token",
		httpx.HeaderUserID:        uuid.NewString(),
		httpx.HeaderOperatorID:    operatorID,
		httpx.HeaderRole:          "pier_admin",
	}
}

// claimHeaders builds the trust-boundary headers RequireInternal expects for
// a request carrying a specific role/operator/pier scope — the general
// helper used by the operators/piers scoping tests (plan 02-03). X-Pier-Ids
// is set only when pierIDs is non-empty, matching httpx.ForwardClaims's
// convention; operatorID is omitted from the headers when empty (the
// super_admin case, which never reads it).
func claimHeaders(role, operatorID string, pierIDs ...string) map[string]string {
	h := map[string]string{
		httpx.HeaderInternalToken: "test-token",
		httpx.HeaderUserID:        uuid.NewString(),
		httpx.HeaderRole:          role,
	}
	if operatorID != "" {
		h[httpx.HeaderOperatorID] = operatorID
	}
	if len(pierIDs) > 0 {
		h[httpx.HeaderPierIDs] = strings.Join(pierIDs, ",")
	}
	return h
}

// waitForBoatUpserted spins up a throwaway pkg/kafka.Consumer (a fresh
// consumer group reads catalog.events from the beginning by default) and
// waits for the first catalog.BoatUpserted envelope it delivers — proving
// the relay actually published what UpsertBoat wrote to the outbox (D-01,
// PLAT-05). The producer's record key is always the outbox row's
// aggregate_id (pkg/kafka.Producer.Publish), which app.UpsertBoat sets to
// the boat id, so env.AggregateId == boat id is equivalent to "keyed by
// boat_id".
func waitForBoatUpserted(t *testing.T, dsn string) *platformv1.Envelope {
	t.Helper()
	ctx := context.Background()

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("waitForBoatUpserted: new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	log := httpx.NewLogger("catalog-test-verifier")
	consumer := kafka.NewConsumer(brokers, "verify-"+uuid.NewString(), pool, nil, log)

	var (
		mu  sync.Mutex
		got *platformv1.Envelope
	)
	consumer.Handle(app.EventBoatUpserted, func(_ context.Context, _ pgx.Tx, env *platformv1.Envelope) error {
		mu.Lock()
		got = env
		mu.Unlock()
		return nil
	})

	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go consumer.Run(runCtx) //nolint:errcheck // best-effort background consumer, test asserts via got

	waitFor(t, 15*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got != nil
	})

	mu.Lock()
	defer mu.Unlock()
	return got
}

// TestUpsertBoatPublishesBoatUpserted proves the write side of the proof
// event end to end: UpsertBoat (connect) writes a boats row and its
// catalog.BoatUpserted outbox row in one tx, taking operator_id from trusted
// claims; the relay publishes it to catalog.events keyed by boat_id (D-01,
// PLAT-05).
func TestUpsertBoatPublishesBoatUpserted(t *testing.T) {
	addr, dsn := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	operatorID := uuid.NewString()
	client := newAuthedCatalogClient(baseURL, newClaimHeaders(operatorID))

	resp, err := client.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name:            "Proof Boat",
		DefaultCapacity: 42,
		Status:          catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
	}))
	if err != nil {
		t.Fatalf("UpsertBoat: %v", err)
	}
	boatID := resp.Msg.Boat.BoatId
	if _, err := uuid.Parse(boatID); err != nil {
		t.Fatalf("boat_id %q is not a valid uuid: %v", boatID, err)
	}

	env := waitForBoatUpserted(t, dsn)
	if env.EventType != app.EventBoatUpserted {
		t.Errorf("EventType = %q, want %q", env.EventType, app.EventBoatUpserted)
	}
	if env.AggregateId != boatID {
		t.Errorf("AggregateId (record key) = %q, want %q", env.AggregateId, boatID)
	}

	var payload catalogv1.BoatUpserted
	if err := env.Payload.UnmarshalTo(&payload); err != nil {
		t.Fatalf("unmarshal BoatUpserted payload: %v", err)
	}
	if payload.BoatId != boatID {
		t.Errorf("payload.BoatId = %q, want %q", payload.BoatId, boatID)
	}
	if payload.OperatorId != operatorID {
		t.Errorf("payload.OperatorId = %q, want %q (claims operator id)", payload.OperatorId, operatorID)
	}
	if payload.DefaultCapacity != 42 {
		t.Errorf("payload.DefaultCapacity = %d, want 42", payload.DefaultCapacity)
	}
	if payload.Status != catalogv1.BoatStatus_BOAT_STATUS_ACTIVE {
		t.Errorf("payload.Status = %v, want BOAT_STATUS_ACTIVE", payload.Status)
	}
}

func assertConnectCode(t *testing.T, err error, want connect.Code, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: err = nil, want %s", context, want)
	}
	if got := connect.CodeOf(err); got != want {
		t.Fatalf("%s: code = %s, want %s (err: %v)", context, got, want, err)
	}
}

// TestUpsertBoatValidationAndTenancy covers UpsertBoat's trust-boundary,
// validation, and cross-operator tenancy behavior (plan 10 task 2).
func TestUpsertBoatValidationAndTenancy(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	// No claims at all -> Unauthenticated.
	anonClient := newCatalogClient(baseURL)
	_, err := anonClient.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "x", DefaultCapacity: 1, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
	}))
	assertConnectCode(t, err, connect.CodeUnauthenticated, "no claim headers")

	operatorA := uuid.NewString()
	clientA := newAuthedCatalogClient(baseURL, newClaimHeaders(operatorA))

	cases := []struct {
		name            string
		defaultCapacity int32
		status          catalogv1.BoatStatus
	}{
		{name: "", defaultCapacity: 10, status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE},
		{name: "x", defaultCapacity: 0, status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE},
		{name: "x", defaultCapacity: 1001, status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE},
		{name: "x", defaultCapacity: 10, status: catalogv1.BoatStatus_BOAT_STATUS_UNSPECIFIED},
	}
	for _, c := range cases {
		_, err := clientA.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
			Name: c.name, DefaultCapacity: c.defaultCapacity, Status: c.status,
		}))
		assertConnectCode(t, err, connect.CodeInvalidArgument,
			fmt.Sprintf("name=%q capacity=%d status=%v", c.name, c.defaultCapacity, c.status))
	}

	// Create a real boat under operator A.
	resp, err := clientA.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		Name: "Original", DefaultCapacity: 20, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
	}))
	if err != nil {
		t.Fatalf("create boat under operator A: %v", err)
	}
	boatID := resp.Msg.Boat.BoatId

	// Operator B reuses the same boat_id -> NotFound, stored row unchanged.
	clientB := newAuthedCatalogClient(baseURL, newClaimHeaders(uuid.NewString()))
	_, err = clientB.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
		BoatId: boatID, Name: "Hijacked", DefaultCapacity: 99, Status: catalogv1.BoatStatus_BOAT_STATUS_MAINTENANCE,
	}))
	assertConnectCode(t, err, connect.CodeNotFound, "cross-operator boat_id reuse")

	listResp, err := clientA.ListBoats(ctx, connect.NewRequest(&catalogv1.ListBoatsRequest{}))
	if err != nil {
		t.Fatalf("ListBoats: %v", err)
	}
	var found *catalogv1.Boat
	for _, b := range listResp.Msg.Boats {
		if b.BoatId == boatID {
			found = b
		}
	}
	if found == nil {
		t.Fatalf("boat %s not found in ListBoats", boatID)
	}
	if found.Name != "Original" || found.DefaultCapacity != 20 || found.Status != catalogv1.BoatStatus_BOAT_STATUS_ACTIVE {
		t.Errorf("stored boat changed after rejected cross-operator upsert: %+v", found)
	}
}

// TestListBoatsOrdered proves ListBoats returns boats ordered by name then
// id and needs only the internal token (no claim headers).
func TestListBoatsOrdered(t *testing.T) {
	addr, _ := setCatalogEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)
	ctx := context.Background()

	writer := newAuthedCatalogClient(baseURL, newClaimHeaders(uuid.NewString()))
	for _, name := range []string{"Zeta", "Alpha", "Mid"} {
		if _, err := writer.UpsertBoat(ctx, connect.NewRequest(&catalogv1.UpsertBoatRequest{
			Name: name, DefaultCapacity: 5, Status: catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
		})); err != nil {
			t.Fatalf("create boat %q: %v", name, err)
		}
	}

	// ListBoats needs only the internal token — no claim headers.
	reader := newAuthedCatalogClient(baseURL, map[string]string{httpx.HeaderInternalToken: "test-token"})
	resp, err := reader.ListBoats(ctx, connect.NewRequest(&catalogv1.ListBoatsRequest{}))
	if err != nil {
		t.Fatalf("ListBoats: %v", err)
	}
	if len(resp.Msg.Boats) != 3 {
		t.Fatalf("len(boats) = %d, want 3", len(resp.Msg.Boats))
	}
	names := make([]string, len(resp.Msg.Boats))
	for i, b := range resp.Msg.Boats {
		names[i] = b.Name
	}
	want := []string{"Alpha", "Mid", "Zeta"}
	for i, name := range want {
		if names[i] != name {
			t.Errorf("boats[%d].Name = %q, want %q (order: %v)", i, names[i], name, names)
		}
	}
}
