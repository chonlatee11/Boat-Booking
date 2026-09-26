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
	"testing"
	"time"

	"github.com/google/uuid"

	catalogv1 "github.com/chonlatee11/boat-booking/gen/go/catalog/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/pkg/testenv"
	"github.com/chonlatee11/boat-booking/services/schedule/internal/app"
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

	// "catalog" is provisioned too: this suite publishes onto catalog.events
	// (the topic schedule's own consumer subscribes to, derived from
	// app.EventCatalogBoatUpserted) exactly as the real catalog service would.
	bs, stopRP, err := testenv.StartRedpanda(ctx, "catalog", serviceName)
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

// setScheduleEnv sets the env vars run() reads, isolated per-test via a
// fresh DB and a fresh HTTP port. It returns both the listen address and the
// database's own DSN (the latter needed by tests that build their own
// verification Postgres pool).
func setScheduleEnv(t *testing.T) (addr, dsn string) {
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
	addr, _ := setScheduleEnv(t)
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

// TestBoatUpsertedAppliedOnce proves the consumer half of the proof event
// (D-01, PLAT-05): the same catalog.BoatUpserted envelope delivered twice is
// applied once (processed_events idempotency, D-13), and a later
// BoatUpserted for the same boat overwrites the projection so it always
// holds the latest capacity.
func TestBoatUpsertedAppliedOnce(t *testing.T) {
	addr, dsn := setScheduleEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	boatID := uuid.NewString()
	operatorID := uuid.NewString()

	topic, err := events.TopicFor(app.EventCatalogBoatUpserted)
	if err != nil {
		t.Fatalf("topic for %s: %v", app.EventCatalogBoatUpserted, err)
	}

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("new test producer: %v", err)
	}
	defer producer.Close()

	env1, err := events.New(app.EventCatalogBoatUpserted, boatID, &catalogv1.BoatUpserted{
		BoatId:          boatID,
		OperatorId:      operatorID,
		Name:            "Proof Boat",
		DefaultCapacity: 42,
		Status:          catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
	})
	if err != nil {
		t.Fatalf("build E1 envelope: %v", err)
	}

	// Publish E1 twice — a literal duplicate delivery of the same event_id.
	if err := producer.Publish(ctx, topic, boatID, env1); err != nil {
		t.Fatalf("publish E1 (1st): %v", err)
	}
	if err := producer.Publish(ctx, topic, boatID, env1); err != nil {
		t.Fatalf("publish E1 (2nd, duplicate): %v", err)
	}

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	waitFor(t, 15*time.Second, func() bool {
		var capacity int32
		err := pool.QueryRow(ctx,
			`select default_capacity from boats where boat_id = $1::uuid`, boatID,
		).Scan(&capacity)
		return err == nil && capacity == 42
	})

	// E2: a later BoatUpserted for the same boat with a different capacity —
	// the projection must hold the latest value.
	env2, err := events.New(app.EventCatalogBoatUpserted, boatID, &catalogv1.BoatUpserted{
		BoatId:          boatID,
		OperatorId:      operatorID,
		Name:            "Proof Boat",
		DefaultCapacity: 50,
		Status:          catalogv1.BoatStatus_BOAT_STATUS_ACTIVE,
	})
	if err != nil {
		t.Fatalf("build E2 envelope: %v", err)
	}
	if err := producer.Publish(ctx, topic, boatID, env2); err != nil {
		t.Fatalf("publish E2: %v", err)
	}

	waitFor(t, 15*time.Second, func() bool {
		var capacity int32
		err := pool.QueryRow(ctx,
			`select default_capacity from boats where boat_id = $1::uuid`, boatID,
		).Scan(&capacity)
		return err == nil && capacity == 50
	})

	var boatIDCheck, operatorIDCheck, status string
	if err := pool.QueryRow(ctx,
		`select boat_id::text, operator_id::text, status from boats where boat_id = $1::uuid`, boatID,
	).Scan(&boatIDCheck, &operatorIDCheck, &status); err != nil {
		t.Fatalf("read boats row: %v", err)
	}
	if operatorIDCheck != operatorID {
		t.Errorf("operator_id = %q, want %q", operatorIDCheck, operatorID)
	}
	if status != "active" {
		t.Errorf("status = %q, want %q", status, "active")
	}

	// Exactly-once effect (D-13): E1's duplicate delivery must not have
	// produced a second processed_events row — total rows == 2 (E1, E2).
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from processed_events`).Scan(&count); err != nil {
		t.Fatalf("count processed_events: %v", err)
	}
	if count != 2 {
		t.Fatalf("processed_events count = %d, want 2 (E1 applied once + E2)", count)
	}
}
