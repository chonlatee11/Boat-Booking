//go:build integration

package kafka_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/protobuf/types/known/wrapperspb"

	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
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

func headerValue(rec *kgo.Record, key string) string {
	for _, h := range rec.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// consumeMatching polls topic from the beginning until it collects `want`
// records satisfying match, or timeout elapses. Every test in this package
// shares one Redpanda broker/topic (TestMain), so records from other tests
// are always present in the log — match must disambiguate by something
// unique to the calling test (e.g. its own consumer group header).
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

func TestHandleAppliesOnce(t *testing.T) {
	ctx := context.Background()

	dsn := testenv.NewDB(t, adminDSN, testenv.PlatformMigrations())
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `create table applied (event_id uuid primary key, v text)`); err != nil {
		t.Fatalf("create applied table: %v", err)
	}

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(producer.Close)

	group := "test-handle-once-" + uuid.NewString()
	aggID := "agg-" + uuid.NewString()

	consumer := kafka.NewConsumer(brokers, group, pool, producer, testLogger())
	var handledCount int32
	consumer.Handle("test.ThingHappened", func(ctx context.Context, tx pgx.Tx, env *platformv1.Envelope) error {
		if env.AggregateId != aggID {
			return nil // ignore other tests' events replayed from topic history
		}
		atomic.AddInt32(&handledCount, 1)
		var val wrapperspb.StringValue
		if err := env.Payload.UnmarshalTo(&val); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `insert into applied (event_id, v) values ($1, $2)`, env.EventId, val.Value)
		return err
	})

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(runCtx) }()

	tracer := tp.Tracer("test")
	spanCtx, span := tracer.Start(ctx, "http-request")
	wantTraceID := span.SpanContext().TraceID().String()

	env1, err := events.New("test.ThingHappened", aggID, wrapperspb.String("v1"))
	if err != nil {
		t.Fatalf("events.New E1: %v", err)
	}
	env2, err := events.New("test.ThingHappened", aggID, wrapperspb.String("v2"))
	if err != nil {
		t.Fatalf("events.New E2: %v", err)
	}

	if err := producer.Publish(spanCtx, "test.events", aggID, env1); err != nil {
		t.Fatalf("publish E1 (1st): %v", err)
	}
	if err := producer.Publish(spanCtx, "test.events", aggID, env1); err != nil {
		t.Fatalf("publish E1 (2nd, duplicate): %v", err)
	}
	if err := producer.Publish(spanCtx, "test.events", aggID, env2); err != nil {
		t.Fatalf("publish E2: %v", err)
	}
	span.End()

	waitFor(t, 20*time.Second, func() bool {
		var count int
		if err := pool.QueryRow(ctx, `select count(*) from applied`).Scan(&count); err != nil {
			t.Fatalf("count applied: %v", err)
		}
		return count >= 2
	})

	var appliedCount int
	if err := pool.QueryRow(ctx, `select count(*) from applied`).Scan(&appliedCount); err != nil {
		t.Fatalf("count applied: %v", err)
	}
	if appliedCount != 2 {
		t.Errorf("applied count = %d, want 2", appliedCount)
	}

	var processedCount int
	if err := pool.QueryRow(ctx, `select count(*) from processed_events where event_id in ($1, $2)`,
		env1.EventId, env2.EventId).Scan(&processedCount); err != nil {
		t.Fatalf("count processed_events: %v", err)
	}
	if processedCount != 2 {
		t.Errorf("processed_events count = %d, want 2", processedCount)
	}

	if got := atomic.LoadInt32(&handledCount); got != 2 {
		t.Errorf("handler ran %d times, want 2 (not 3 — E1's duplicate delivery must be a no-op)", got)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("consumer A Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer A did not stop within 10s")
	}

	found := false
	for _, s := range recorder.Ended() {
		if s.Name() != "test.events process" {
			continue
		}
		for _, attr := range s.Attributes() {
			if string(attr.Key) == "event_id" && attr.Value.AsString() == env1.EventId {
				found = true
				if got := s.SpanContext().TraceID().String(); got != wantTraceID {
					t.Errorf("process span trace id = %s, want %s (parent span's trace id)", got, wantTraceID)
				}
			}
		}
	}
	if !found {
		t.Error("no recorded process span found for E1's event_id")
	}

	// A second consumer in the same group resumes at the committed offset —
	// it must receive nothing for this test's aggregate id.
	var secondHandled int32
	consumer2 := kafka.NewConsumer(brokers, group, pool, producer, testLogger())
	consumer2.Handle("test.ThingHappened", func(_ context.Context, _ pgx.Tx, env *platformv1.Envelope) error {
		if env.AggregateId == aggID {
			atomic.AddInt32(&secondHandled, 1)
		}
		return nil
	})

	runCtx2, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel2()
	if err := consumer2.Run(runCtx2); err != nil {
		t.Fatalf("consumer B Run: %v", err)
	}
	if got := atomic.LoadInt32(&secondHandled); got != 0 {
		t.Errorf("second consumer (same group) handled %d records, want 0 (offsets should already be committed)", got)
	}
}

func TestFailedHandlerLandsInDLQAfter3Retries(t *testing.T) {
	ctx := context.Background()

	dsn := testenv.NewDB(t, adminDSN, testenv.PlatformMigrations())
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `create table applied (event_id uuid primary key, v text)`); err != nil {
		t.Fatalf("create applied table: %v", err)
	}

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(producer.Close)

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevMP := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(prevMP) })

	group := "test-dlq-" + uuid.NewString()
	aggID := "agg-" + uuid.NewString()

	consumer := kafka.NewConsumer(brokers, group, pool, producer, testLogger())
	consumer.Backoff = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}

	poisonEnv, err := events.New("test.ThingHappened", aggID, wrapperspb.String("poison"))
	if err != nil {
		t.Fatalf("events.New poison: %v", err)
	}
	poisonBytes, err := events.Marshal(poisonEnv)
	if err != nil {
		t.Fatalf("events.Marshal poison: %v", err)
	}

	consumer.Handle("test.ThingHappened", func(ctx context.Context, tx pgx.Tx, env *platformv1.Envelope) error {
		if env.AggregateId != aggID {
			return nil
		}
		if env.EventId == poisonEnv.EventId {
			return errors.New("boom: handler always fails for the poison event")
		}
		_, err := tx.Exec(ctx, `insert into applied (event_id, v) values ($1, $2)`, env.EventId, "ok")
		return err
	})

	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- consumer.Run(runCtx) }()

	if err := producer.Publish(ctx, "test.events", aggID, poisonEnv); err != nil {
		t.Fatalf("publish poison event: %v", err)
	}

	dlqRecs := consumeMatching(t, "test.events.dlq", 20*time.Second, 1, func(r *kgo.Record) bool {
		return headerValue(r, "consumer_group") == group
	})
	if len(dlqRecs) != 1 {
		t.Fatalf("got %d matching dlq records, want 1", len(dlqRecs))
	}
	dlqRec := dlqRecs[0]

	if string(dlqRec.Value) != string(poisonBytes) {
		t.Error("dlq record value does not match the original envelope bytes")
	}
	if headerValue(dlqRec, "error") == "" {
		t.Error("dlq record missing error header")
	}
	if got := headerValue(dlqRec, "attempts"); got != "4" {
		t.Errorf("dlq attempts header = %q, want %q", got, "4")
	}
	if headerValue(dlqRec, "failed_at") == "" {
		t.Error("dlq record missing failed_at header")
	}
	if got := headerValue(dlqRec, "source_topic"); got != "test.events" {
		t.Errorf("dlq source_topic header = %q, want %q", got, "test.events")
	}
	if headerValue(dlqRec, "source_partition") == "" {
		t.Error("dlq record missing source_partition header")
	}
	if headerValue(dlqRec, "source_offset") == "" {
		t.Error("dlq record missing source_offset header")
	}

	var processedCount int
	if err := pool.QueryRow(ctx, `select count(*) from processed_events where event_id = $1`,
		poisonEnv.EventId).Scan(&processedCount); err != nil {
		t.Fatalf("count processed_events: %v", err)
	}
	if processedCount != 0 {
		t.Errorf("processed_events has %d rows for the poisoned event, want 0", processedCount)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	if got := counterValue(rm, "dlq"); got != 1 {
		t.Errorf("dlq counter = %d, want 1", got)
	}

	goodEnv, err := events.New("test.ThingHappened", aggID, wrapperspb.String("good"))
	if err != nil {
		t.Fatalf("events.New good: %v", err)
	}
	if err := producer.Publish(ctx, "test.events", aggID, goodEnv); err != nil {
		t.Fatalf("publish good event: %v", err)
	}

	waitFor(t, 10*time.Second, func() bool {
		var found bool
		if err := pool.QueryRow(ctx, `select exists(select 1 from applied where event_id = $1)`,
			goodEnv.EventId).Scan(&found); err != nil {
			t.Fatalf("check applied: %v", err)
		}
		return found
	})

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop within 10s")
	}
}

func TestUncommittedRecordIsRedelivered(t *testing.T) {
	ctx := context.Background()

	dsn := testenv.NewDB(t, adminDSN, testenv.PlatformMigrations())
	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `create table applied (event_id uuid primary key, v text)`); err != nil {
		t.Fatalf("create applied table: %v", err)
	}

	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("NewProducer: %v", err)
	}
	t.Cleanup(producer.Close)

	group := "test-redelivery-" + uuid.NewString()
	aggID := "agg-" + uuid.NewString()

	env, err := events.New("test.ThingHappened", aggID, wrapperspb.String("redeliver-me"))
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}

	consumerA := kafka.NewConsumer(brokers, group, pool, producer, testLogger())
	consumerA.Backoff = []time.Duration{30 * time.Second, 30 * time.Second, 30 * time.Second}
	consumerA.Handle("test.ThingHappened", func(_ context.Context, _ pgx.Tx, e *platformv1.Envelope) error {
		if e.AggregateId != aggID {
			return nil
		}
		return errors.New("boom: consumer A always fails")
	})

	runCtxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)
	go func() { doneA <- consumerA.Run(runCtxA) }()

	if err := producer.Publish(ctx, "test.events", aggID, env); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Give consumer A time to fetch the record, fail its first attempt, and
	// enter its (long) backoff wait — well short of the 30s backoff itself.
	time.Sleep(1 * time.Second)
	cancelA()

	select {
	case err := <-doneA:
		if err != nil {
			t.Errorf("consumer A Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer A did not stop within 10s of cancellation")
	}

	var processedCount int
	if err := pool.QueryRow(ctx, `select count(*) from processed_events where event_id = $1`,
		env.EventId).Scan(&processedCount); err != nil {
		t.Fatalf("count processed_events: %v", err)
	}
	if processedCount != 0 {
		t.Errorf("processed_events has %d rows, want 0 (handler never succeeded, tx rolled back)", processedCount)
	}

	var appliedBeforeRedelivery int
	if err := pool.QueryRow(ctx, `select count(*) from applied`).Scan(&appliedBeforeRedelivery); err != nil {
		t.Fatalf("count applied: %v", err)
	}
	if appliedBeforeRedelivery != 0 {
		t.Errorf("applied has %d rows, want 0 before redelivery (offset must not have been committed)", appliedBeforeRedelivery)
	}

	consumerB := kafka.NewConsumer(brokers, group, pool, producer, testLogger())
	var handledByB int32
	consumerB.Handle("test.ThingHappened", func(ctx context.Context, tx pgx.Tx, e *platformv1.Envelope) error {
		if e.AggregateId != aggID {
			return nil
		}
		atomic.AddInt32(&handledByB, 1)
		_, err := tx.Exec(ctx, `insert into applied (event_id, v) values ($1, $2)`, e.EventId, "recovered")
		return err
	})

	runCtxB, cancelB := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelB()
	doneB := make(chan error, 1)
	go func() { doneB <- consumerB.Run(runCtxB) }()

	waitFor(t, 20*time.Second, func() bool {
		var found bool
		if err := pool.QueryRow(ctx, `select exists(select 1 from applied where event_id = $1)`,
			env.EventId).Scan(&found); err != nil {
			t.Fatalf("check applied: %v", err)
		}
		return found
	})

	cancelB()
	select {
	case <-doneB:
	case <-time.After(10 * time.Second):
		t.Fatal("consumer B did not stop within 10s")
	}

	if got := atomic.LoadInt32(&handledByB); got != 1 {
		t.Errorf("consumer B's handler ran %d times, want exactly 1 (applied exactly once)", got)
	}
}
