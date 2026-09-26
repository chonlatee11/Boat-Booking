//go:build integration

package outbox_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
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
