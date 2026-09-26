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

	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/pkg/testenv"
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

// setTemplateEnv sets the env vars run() reads, isolated per-test via a
// fresh DB and a fresh HTTP port.
func setTemplateEnv(t *testing.T) (addr string) {
	t.Helper()
	dsn := testenv.NewDB(t, adminDSN, "../migrations")
	port := freePort(t)
	addr = fmt.Sprintf("127.0.0.1:%d", port)

	t.Setenv("HTTP_ADDR", addr)
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("KAFKA_BROKERS", strings.Join(brokers, ","))
	t.Setenv("INTERNAL_TOKEN", "test-token")
	t.Setenv("LOG_FORMAT", "text")

	return addr
}

func TestTemplateReadyAndGracefulShutdown(t *testing.T) {
	addr := setTemplateEnv(t)
	baseURL := "http://" + addr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx) }()

	waitForFullyReady(t, baseURL)

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}

	cancel()

	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Fatalf("run returned error after shutdown: %v", runErr)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return within 15s of ctx cancellation")
	}
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

// TestPingRoundTrip proves the full HTTP -> tx -> outbox -> Kafka ->
// consumer loop: the trust-boundary 401s, note validation, the created ping
// getting acked by the service's own consumer, and idempotent replay via
// processed_events (D-03, D-13, D-14, D-15, D-30, D-44).
func TestPingRoundTrip(t *testing.T) {
	addr := setTemplateEnv(t)
	baseURL := "http://" + addr
	dsn := os.Getenv("DATABASE_URL")

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx) }()
	// t.Cleanup runs LIFO, so this single cleanup — cancel, then wait —
	// keeps the shutdown-then-await ordering explicit in one place instead
	// of relying on registration order across two separate Cleanup calls.
	t.Cleanup(func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(15 * time.Second):
			t.Error("run did not return within 15s of test cleanup cancellation")
		}
	})

	waitForFullyReady(t, baseURL)

	// No X-Internal-Token at all -> 401.
	assertPingStatus(t, baseURL, nil, `{"note":"hi"}`, http.StatusUnauthorized)

	// Token present, but no claim headers -> 401.
	assertPingStatus(t, baseURL, map[string]string{
		httpx.HeaderInternalToken: "test-token",
	}, `{"note":"hi"}`, http.StatusUnauthorized)

	claimHeaders := map[string]string{
		httpx.HeaderInternalToken: "test-token",
		httpx.HeaderUserID:        uuid.NewString(),
		httpx.HeaderOperatorID:    uuid.NewString(),
		httpx.HeaderRole:          "operator",
	}

	// Empty note -> 400 invalid_argument.
	assertPingStatus(t, baseURL, claimHeaders, `{"note":""}`, http.StatusBadRequest)
	// Note over 280 chars -> 400 invalid_argument.
	assertPingStatus(t, baseURL, claimHeaders, `{"note":"`+strings.Repeat("a", 281)+`"}`, http.StatusBadRequest)

	// Valid request -> 201 {"id": <uuid v7>}.
	id := createPing(t, baseURL, claimHeaders, "hi")

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	waitFor(t, 15*time.Second, func() bool {
		var acked bool
		err := pool.QueryRow(ctx,
			`select acked_at is not null from pings where id = $1::uuid`, id,
		).Scan(&acked)
		return err == nil && acked
	})

	// Replay: read the outbox row for this ping, unmarshal it, and publish
	// it again with a fresh test producer — processed_events must still
	// show exactly one row for this event_id (idempotent apply, D-13).
	var eventID, topic string
	var payload []byte
	if err := pool.QueryRow(ctx,
		`select event_id::text, topic, payload from outbox where aggregate_id = $1`, id,
	).Scan(&eventID, &topic, &payload); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}

	env, err := events.Unmarshal(payload)
	if err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("new test producer: %v", err)
	}
	defer producer.Close()
	if err := producer.Publish(ctx, topic, id, env); err != nil {
		t.Fatalf("republish envelope: %v", err)
	}

	time.Sleep(3 * time.Second)

	var count int
	if err := pool.QueryRow(ctx,
		`select count(*) from processed_events where event_id = $1::uuid`, eventID,
	).Scan(&count); err != nil {
		t.Fatalf("count processed_events: %v", err)
	}
	if count != 1 {
		t.Fatalf("processed_events count for event_id %s = %d, want 1", eventID, count)
	}
}

func doPing(t *testing.T, baseURL string, headers map[string]string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/pings", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func assertPingStatus(t *testing.T, baseURL string, headers map[string]string, body string, want int) {
	t.Helper()
	resp := doPing(t, baseURL, headers, body)
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != want {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, want, data)
	}
}

func createPing(t *testing.T, baseURL string, headers map[string]string, note string) string {
	t.Helper()
	resp := doPing(t, baseURL, headers, `{"note":"`+note+`"}`)
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("create ping status = %d, want 201, body=%s", resp.StatusCode, data)
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return body.ID
}
