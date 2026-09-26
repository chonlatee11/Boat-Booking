//go:build integration

package outbox_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
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

	bs, stopRP, err := testenv.StartRedpanda(ctx, "test")
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

// testLogger discards output (integration test noise) but keeps the same
// shape as the production slog logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestTraceparentSurvivesRelay(t *testing.T) {
	ctx := context.Background()

	dsn := testenv.NewDB(t, adminDSN, testenv.PlatformMigrations())
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	defer producer.Close()

	relay := outbox.NewRelay(pool, producer, testLogger())
	relay.PollInterval = 100 * time.Millisecond

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	spanCtx, span := tp.Tracer("test").Start(ctx, "http-request")
	wantTraceID := span.SpanContext().TraceID().String()

	aggID := "boat-test-1"
	env, err := events.New("test.ThingHappened", aggID, wrapperspb.String("x"))
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}

	err = bbpgx.WithTx(spanCtx, pool, func(tx pgx.Tx) error {
		return outbox.Insert(spanCtx, tx, env)
	})
	span.End()
	if err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}

	var storedTraceparent string
	if err := pool.QueryRow(ctx, `select traceparent from outbox where event_id = $1`, env.EventId).Scan(&storedTraceparent); err != nil {
		t.Fatalf("query stored traceparent: %v", err)
	}
	if !strings.Contains(storedTraceparent, wantTraceID) {
		t.Fatalf("stored traceparent %q does not contain trace id %q", storedTraceparent, wantTraceID)
	}

	relayCtx, cancelRelay := context.WithCancel(context.Background())
	relayDone := make(chan error, 1)
	go func() { relayDone <- relay.Run(relayCtx) }()
	t.Cleanup(func() {
		cancelRelay()
		<-relayDone
	})

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics("test.events"),
	)
	if err != nil {
		t.Fatalf("consumer NewClient: %v", err)
	}
	defer consumer.Close()

	pollCtx, cancelPoll := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelPoll()

	fetches := consumer.PollFetches(pollCtx)
	if err := fetches.Err0(); err != nil {
		t.Fatalf("poll fetches: %v", err)
	}
	records := fetches.Records()
	if len(records) == 0 {
		t.Fatal("no records consumed from test.events")
	}
	rec := records[0]

	if string(rec.Key) != aggID {
		t.Errorf("record key = %q, want %q", string(rec.Key), aggID)
	}

	headers := map[string]string{}
	for _, h := range rec.Headers {
		headers[h.Key] = string(h.Value)
	}
	if headers["event_type"] != env.EventType {
		t.Errorf("event_type header = %q, want %q", headers["event_type"], env.EventType)
	}
	if headers["event_id"] != env.EventId {
		t.Errorf("event_id header = %q, want %q", headers["event_id"], env.EventId)
	}
	if !strings.Contains(headers["traceparent"], wantTraceID) {
		t.Errorf("traceparent header %q does not contain trace id %q", headers["traceparent"], wantTraceID)
	}

	gotEnv, err := events.Unmarshal(rec.Value)
	if err != nil {
		t.Fatalf("unmarshal record value: %v", err)
	}
	if gotEnv.EventId != env.EventId {
		t.Errorf("consumed EventId = %q, want %q", gotEnv.EventId, env.EventId)
	}

	var publishedAt *time.Time
	if err := pool.QueryRow(ctx, `select published_at from outbox where event_id = $1`, env.EventId).Scan(&publishedAt); err != nil {
		t.Fatalf("query published_at: %v", err)
	}
	if publishedAt == nil {
		t.Error("published_at is still NULL after the relay ran")
	}
}

// newRelayForTest gives each test its own fresh database and a Relay backed
// by a real Producer against the shared package Redpanda, with the given
// poll interval.
func newRelayForTest(t testing.TB, ctx context.Context, pollInterval time.Duration) (*pgxpool.Pool, *outbox.Relay) {
	t.Helper()

	dsn := testenv.NewDB(t, adminDSN, testenv.PlatformMigrations())
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(producer.Close)

	relay := outbox.NewRelay(pool, producer, testLogger())
	relay.PollInterval = pollInterval
	return pool, relay
}

// runRelay starts relay.Run in the background and stops it (waiting for the
// shutdown flush) via t.Cleanup.
func runRelay(t testing.TB, relay *outbox.Relay) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

// consumeMatching polls topic from the beginning until it collects `want`
// records satisfying match, or timeout elapses. Every test shares one
// Redpanda broker/topic (TestMain), so records from other tests are always
// present in the log — match must disambiguate by the test's own aggregate id.
func consumeMatching(t testing.TB, topic string, timeout time.Duration, want int, match func(*kgo.Record) bool) []*kgo.Record {
	t.Helper()

	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic))
	if err != nil {
		t.Fatalf("consumeMatching: new client: %v", err)
	}
	defer cl.Close()

	deadline := time.Now().Add(timeout)
	var got []*kgo.Record
	for time.Now().Before(deadline) {
		pollCtx, cancel := context.WithDeadline(context.Background(), deadline)
		fetches := cl.PollFetches(pollCtx)
		cancel()
		fetches.EachRecord(func(r *kgo.Record) {
			if match(r) {
				got = append(got, r)
			}
		})
		if len(got) >= want {
			return got
		}
	}
	return got
}

func TestRelayPreservesPerAggregateOrder(t *testing.T) {
	ctx := context.Background()
	pool, relay := newRelayForTest(t, ctx, 100*time.Millisecond)

	aggID := "agg-" + uuid.NewString()
	err := bbpgx.WithTx(ctx, pool, func(tx pgx.Tx) error {
		for i := 1; i <= 5; i++ {
			env, err := events.New("test.ThingHappened", aggID, wrapperspb.String(fmt.Sprintf("%d", i)))
			if err != nil {
				return err
			}
			if err := outbox.Insert(ctx, tx, env); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}

	runRelay(t, relay)

	recs := consumeMatching(t, "test.events", 20*time.Second, 5, func(r *kgo.Record) bool {
		return string(r.Key) == aggID
	})
	if len(recs) != 5 {
		t.Fatalf("got %d matching records, want 5", len(recs))
	}

	part := recs[0].Partition
	for i, r := range recs {
		if r.Partition != part {
			t.Errorf("record %d partition = %d, want %d (same partition for one aggregate)", i, r.Partition, part)
		}
		if i > 0 && r.Offset <= recs[i-1].Offset {
			t.Errorf("record %d offset %d is not ascending after %d", i, r.Offset, recs[i-1].Offset)
		}
	}

	for i, r := range recs {
		env, err := events.Unmarshal(r.Value)
		if err != nil {
			t.Fatalf("unmarshal record %d: %v", i, err)
		}
		var val wrapperspb.StringValue
		if err := env.Payload.UnmarshalTo(&val); err != nil {
			t.Fatalf("unwrap payload %d: %v", i, err)
		}
		want := fmt.Sprintf("%d", i+1)
		if val.Value != want {
			t.Errorf("record %d payload = %q, want %q (insertion order not preserved)", i, val.Value, want)
		}
	}
}

func TestRelayNudgePublishesImmediately(t *testing.T) {
	ctx := context.Background()
	pool, relay := newRelayForTest(t, ctx, 10*time.Second)

	aggID := "agg-" + uuid.NewString()
	env, err := events.New("test.ThingHappened", aggID, wrapperspb.String("nudge"))
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	if err := bbpgx.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return outbox.Insert(ctx, tx, env)
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	runRelay(t, relay)
	relay.Nudge()

	recs := consumeMatching(t, "test.events", 2*time.Second, 1, func(r *kgo.Record) bool {
		return string(r.Key) == aggID
	})
	if len(recs) != 1 {
		t.Fatalf("Nudge did not publish within 2s (poll interval is 10s); got %d matching records", len(recs))
	}
}

func TestRelayKeepsRowsWhenPublishFails(t *testing.T) {
	ctx := context.Background()

	dsn := testenv.NewDB(t, adminDSN, testenv.PlatformMigrations())
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Unreachable broker + a short delivery timeout so the publish attempt
	// fails deterministically instead of hanging.
	producer, err := kafka.NewProducer([]string{"127.0.0.1:19999"}, kgo.RecordDeliveryTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(producer.Close)

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevMP := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(prevMP) })

	relay := outbox.NewRelay(pool, producer, testLogger())
	relay.PollInterval = 200 * time.Millisecond

	aggID := "agg-" + uuid.NewString()
	env, err := events.New("test.ThingHappened", aggID, wrapperspb.String("fail"))
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	if err := bbpgx.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return outbox.Insert(ctx, tx, env)
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	runRelay(t, relay)

	// Give the relay time to attempt (and time out) at least one publish.
	time.Sleep(5 * time.Second)

	var publishedAt *time.Time
	if err := pool.QueryRow(ctx, `select published_at from outbox where event_id = $1`, env.EventId).Scan(&publishedAt); err != nil {
		t.Fatalf("query published_at: %v", err)
	}
	if publishedAt != nil {
		t.Errorf("published_at = %v, want NULL (publish should have failed and stopped the batch)", *publishedAt)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	if got := counterValue(rm, "outbox.publish_errors"); got < 1 {
		t.Errorf("outbox.publish_errors = %d, want >= 1", got)
	}
}

func counterValue(rm metricdata.ResourceMetrics, name string) int64 {
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
			}
		}
	}
	return total
}

func TestSweepDeletesOnlyOldPublished(t *testing.T) {
	ctx := context.Background()
	pool, relay := newRelayForTest(t, ctx, time.Hour) // long poll: only Sweep matters here

	payload := []byte{0}
	insertRow := func(createdAgoDays int, publishedAgoDays *int) uuid.UUID {
		id := uuid.New()
		var err error
		if publishedAgoDays == nil {
			_, err = pool.Exec(ctx, `
				insert into outbox (event_id, topic, aggregate_id, event_type, payload, traceparent, created_at, published_at)
				values ($1, 'test.events', 'agg', 'test.ThingHappened', $2, '', now() - ($3::text || ' days')::interval, null)
			`, id, payload, createdAgoDays)
		} else {
			_, err = pool.Exec(ctx, `
				insert into outbox (event_id, topic, aggregate_id, event_type, payload, traceparent, created_at, published_at)
				values ($1, 'test.events', 'agg', 'test.ThingHappened', $2, '', now() - ($3::text || ' days')::interval, now() - ($4::text || ' days')::interval)
			`, id, payload, createdAgoDays, *publishedAgoDays)
		}
		if err != nil {
			t.Fatalf("insert row: %v", err)
		}
		return id
	}

	eightDaysAgo := 8
	oneDayAgo := 1
	oldPublished := insertRow(9, &eightDaysAgo)
	recentPublished := insertRow(2, &oneDayAgo)
	oldUnpublished := insertRow(9, nil)

	deleted, err := relay.Sweep(ctx)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Sweep deleted %d rows, want 1", deleted)
	}

	exists := func(id uuid.UUID) bool {
		var found bool
		if err := pool.QueryRow(ctx, `select exists(select 1 from outbox where event_id = $1)`, id).Scan(&found); err != nil {
			t.Fatalf("exists query: %v", err)
		}
		return found
	}
	if exists(oldPublished) {
		t.Error("row published 8 days ago should have been swept")
	}
	if !exists(recentPublished) {
		t.Error("row published 1 day ago should have been kept")
	}
	if !exists(oldUnpublished) {
		t.Error("unpublished row should never be swept, regardless of age")
	}
}

func TestRelayFlushesOnShutdown(t *testing.T) {
	ctx := context.Background()
	// Poll interval long enough that only the shutdown flush (not a regular
	// tick) can plausibly publish before we assert.
	pool, relay := newRelayForTest(t, ctx, 10*time.Second)

	aggID := "agg-" + uuid.NewString()
	env, err := events.New("test.ThingHappened", aggID, wrapperspb.String("flush"))
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}

	relayCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(relayCtx) }()

	if err := bbpgx.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return outbox.Insert(ctx, tx, env)
	}); err != nil {
		cancel()
		<-done
		t.Fatalf("insert: %v", err)
	}

	cancel() // trigger the shutdown flush path, not the 10s poll tick

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10s of ctx cancel")
	}

	var publishedAt *time.Time
	if err := pool.QueryRow(ctx, `select published_at from outbox where event_id = $1`, env.EventId).Scan(&publishedAt); err != nil {
		t.Fatalf("query published_at: %v", err)
	}
	if publishedAt == nil {
		t.Error("published_at is NULL — shutdown flush did not publish before Run returned")
	}
}
