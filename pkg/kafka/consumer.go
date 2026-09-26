package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/clock"
	"github.com/chonlatee11/boat-booking/pkg/events"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
)

// HandlerFunc processes one decoded event inside the same Postgres
// transaction that recorded it as processed (D-13). Any error rolls back
// that transaction (including the processed_events insert) and triggers the
// retry/DLQ path (D-12).
type HandlerFunc func(ctx context.Context, tx pgx.Tx, env *platformv1.Envelope) error

// Consumer owns idempotency, manual offset commits, retries, and DLQ for
// every event type registered via Handle (D-12, D-13). No service or app
// code may construct a Kafka consumer client directly (PLAT-02).
type Consumer struct {
	brokers []string
	group   string
	pool    *pgxpool.Pool
	dlq     *Producer
	log     *slog.Logger

	// Backoff is the delay before each retry attempt, in order (D-12).
	// Defaults to 1s/5s/25s; tests shorten it.
	Backoff []time.Duration

	handlers map[string]HandlerFunc
	topics   map[string]struct{}

	tracer    *kotel.Tracer
	processed metric.Int64Counter
	dlqCount  metric.Int64Counter
	lag       metric.Int64Gauge
}

// NewConsumer builds a Consumer. dlq is the Producer used to publish
// envelopes that exhaust their retries to <topic>.dlq (D-12).
func NewConsumer(brokers []string, group string, pool *pgxpool.Pool, dlq *Producer, log *slog.Logger) *Consumer {
	meter := otel.Meter("github.com/chonlatee11/boat-booking/pkg/kafka")

	processed, err := meter.Int64Counter("kafka.consumer.processed")
	if err != nil {
		panic(fmt.Errorf("kafka: register kafka.consumer.processed counter: %w", err))
	}
	dlqCounter, err := meter.Int64Counter("dlq")
	if err != nil {
		panic(fmt.Errorf("kafka: register dlq counter: %w", err))
	}
	lag, err := meter.Int64Gauge("kafka.consumer.lag")
	if err != nil {
		panic(fmt.Errorf("kafka: register kafka.consumer.lag gauge: %w", err))
	}

	return &Consumer{
		brokers:   brokers,
		group:     group,
		pool:      pool,
		dlq:       dlq,
		log:       log,
		Backoff:   []time.Duration{1 * time.Second, 5 * time.Second, 25 * time.Second},
		handlers:  make(map[string]HandlerFunc),
		topics:    make(map[string]struct{}),
		tracer:    kotel.NewTracer(kotel.TracerProvider(otel.GetTracerProvider()), kotel.ConsumerGroup(group)),
		processed: processed,
		dlqCount:  dlqCounter,
		lag:       lag,
	}
}

// Handle registers fn as the handler for eventType (D-13). The Kafka topic
// consumed for it is derived via events.TopicFor.
func (c *Consumer) Handle(eventType string, fn HandlerFunc) {
	topic, err := events.TopicFor(eventType)
	if err != nil {
		panic(fmt.Errorf("kafka: handle %q: %w", eventType, err))
	}
	c.handlers[eventType] = fn
	c.topics[topic] = struct{}{}
}

// Topics returns the deduplicated Kafka topics derived from every event type
// registered via Handle.
func (c *Consumer) Topics() []string {
	topics := make([]string, 0, len(c.topics))
	for t := range c.topics {
		topics = append(topics, t)
	}
	return topics
}

// Run consumes every topic derived from registered handlers until ctx is
// cancelled. A Consumer with no handlers starts no Kafka client and returns
// cleanly on shutdown.
func (c *Consumer) Run(ctx context.Context) error {
	topics := c.Topics()
	if len(topics) == 0 {
		<-ctx.Done()
		return nil
	}

	kt := kotel.NewKotel(kotel.WithTracer(c.tracer))
	cl, err := kgo.NewClient( //nolint:forbidigo // pkg/kafka is the sanctioned constructor (D-08, PLAT-02)
		kgo.SeedBrokers(c.brokers...),
		kgo.ConsumerGroup(c.group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.WithHooks(kt.Hooks()...),
	)
	if err != nil {
		return fmt.Errorf("kafka: new consumer client: %w", err)
	}
	defer cl.Close()

	for {
		if ctx.Err() != nil {
			return nil
		}

		fetches := cl.PollFetches(ctx)
		// kgo's BlockRebalanceOnPoll adds a poller on every PollFetches call —
		// including one that returns immediately with a ctx-cancellation fake
		// fetch and no real records — so AllowRebalance must run on every
		// iteration regardless of outcome. Skipping it here (e.g. an early
		// return on ctx.Err()) leaves the poller count permanently non-zero
		// and deadlocks cl.Close()'s graceful group-leave forever.
		ctxDone := ctx.Err() != nil

		// When ctxDone, the only "error" kgo reports is the synthetic
		// ctx-cancellation fake fetch itself (topic "", partition -1) — not a
		// real fetch failure, so it's not logged as one.
		if !ctxDone {
			for _, fe := range fetches.Errors() {
				c.log.Error("kafka: fetch error", "topic", fe.Topic, "partition", fe.Partition, "error", fe.Err)
			}
		}

		var toCommit []*kgo.Record
		aborted := false
		if !ctxDone {
			fetches.EachRecord(func(rec *kgo.Record) {
				if aborted {
					return
				}
				if ctx.Err() != nil {
					aborted = true
					return
				}
				if !c.processRecord(ctx, rec) {
					aborted = true
					return
				}
				toCommit = append(toCommit, rec)
			})
		}

		if len(toCommit) > 0 {
			commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if err := cl.CommitRecords(commitCtx, toCommit...); err != nil {
				c.log.Error("kafka: commit records failed", "error", err)
			}
			cancel()
		}
		c.observeLag(context.WithoutCancel(ctx), fetches)
		cl.AllowRebalance()

		if ctxDone || aborted {
			return nil
		}
	}
}

// processRecord applies rec's envelope exactly once (D-13), retrying a
// failing handler per Backoff and routing to the DLQ once retries are
// exhausted (D-12). It returns whether rec is safe to include in this
// batch's offset commit — false means the caller is shutting down mid
// backoff and rec must be abandoned uncommitted for redelivery.
func (c *Consumer) processRecord(runCtx context.Context, rec *kgo.Record) bool {
	spanCtx, span := c.tracer.WithProcessSpan(rec)
	defer span.End()

	env, err := events.Unmarshal(rec.Value)
	if err != nil {
		// ponytail: our own producer never emits malformed envelopes; a
		// corrupt record here can't be fixed by retrying. Log and commit
		// past it rather than wedge the partition forever.
		c.log.Error("kafka: unmarshal envelope failed, skipping",
			"topic", rec.Topic, "partition", rec.Partition, "offset", rec.Offset, "error", err)
		return true
	}

	span.SetAttributes(
		attribute.String("event_type", env.EventType),
		attribute.String("event_id", env.EventId),
		attribute.String("aggregate_id", env.AggregateId),
	)

	detachedCtx := context.WithoutCancel(spanCtx)

	fn, ok := c.handlers[env.EventType]
	if !ok {
		c.log.Info("kafka: no handler registered, skipping",
			"event_id", env.EventId, "event_type", env.EventType, "aggregate_id", env.AggregateId, "result", "skipped")
		c.processed.Add(detachedCtx, 1, metric.WithAttributes(
			attribute.String("event_type", env.EventType), attribute.String("result", "skipped")))
		return true
	}

	maxAttempts := 1 + len(c.Backoff)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, applyErr := c.apply(detachedCtx, fn, env)
		if applyErr == nil {
			c.log.Info("kafka: event processed",
				"event_id", env.EventId, "event_type", env.EventType, "aggregate_id", env.AggregateId, "result", result)
			c.processed.Add(detachedCtx, 1, metric.WithAttributes(
				attribute.String("event_type", env.EventType), attribute.String("result", result)))
			return true
		}
		lastErr = applyErr
		if attempt == maxAttempts {
			break
		}

		c.log.Warn("kafka: handler failed, retrying",
			"event_id", env.EventId, "event_type", env.EventType, "aggregate_id", env.AggregateId,
			"attempt", attempt, "error", applyErr)
		select {
		case <-time.After(c.Backoff[attempt-1]):
		case <-runCtx.Done():
			c.log.Warn("kafka: abandoning record uncommitted, shutting down mid-backoff",
				"event_id", env.EventId, "event_type", env.EventType, "aggregate_id", env.AggregateId, "attempt", attempt)
			return false
		}
	}

	return c.sendToDLQ(runCtx, spanCtx, rec, env, lastErr, maxAttempts)
}

// apply runs fn inside one tx that first inserts into processed_events
// (on conflict do nothing) — 0 rows affected means this event_id was already
// applied, so fn is skipped and the tx (and therefore the offset commit)
// still succeeds as a no-op (D-13).
func (c *Consumer) apply(ctx context.Context, fn HandlerFunc, env *platformv1.Envelope) (string, error) {
	result := "applied"
	err := bbpgx.WithTx(ctx, c.pool, func(tx pgx.Tx) error {
		tag, execErr := tx.Exec(ctx,
			`insert into processed_events (event_id, event_type) values ($1, $2) on conflict do nothing`,
			env.EventId, env.EventType)
		if execErr != nil {
			return fmt.Errorf("kafka: insert processed_events: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			result = "duplicate"
			return nil
		}
		return fn(ctx, tx, env)
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// sendToDLQ produces rec verbatim (original key/value/headers) to its
// dead-letter topic with full failure metadata (D-12), retrying the DLQ
// write itself with the same backoff until runCtx is done. It returns
// whether rec is safe to commit — false only when runCtx is cancelled before
// the DLQ write succeeds, per "never commit without a successful DLQ write".
func (c *Consumer) sendToDLQ(runCtx, spanCtx context.Context, rec *kgo.Record, env *platformv1.Envelope, cause error, attempts int) bool {
	dlqTopic := events.DLQTopic(rec.Topic)

	headers := make([]kgo.RecordHeader, 0, len(rec.Headers)+7)
	headers = append(headers, rec.Headers...)
	headers = append(headers,
		kgo.RecordHeader{Key: "error", Value: []byte(cause.Error())},
		kgo.RecordHeader{Key: "consumer_group", Value: []byte(c.group)},
		kgo.RecordHeader{Key: "attempts", Value: []byte(strconv.Itoa(attempts))},
		kgo.RecordHeader{Key: "failed_at", Value: []byte(clock.Now().UTC().Format(time.RFC3339Nano))},
		kgo.RecordHeader{Key: "source_topic", Value: []byte(rec.Topic)},
		kgo.RecordHeader{Key: "source_partition", Value: []byte(strconv.Itoa(int(rec.Partition)))},
		kgo.RecordHeader{Key: "source_offset", Value: []byte(strconv.FormatInt(rec.Offset, 10))},
	)

	dlqRec := &kgo.Record{
		Topic:   dlqTopic,
		Key:     rec.Key,
		Value:   rec.Value,
		Headers: headers,
		Context: spanCtx,
	}

	for i := 0; ; i++ {
		if produceErr := c.dlq.produceRecord(context.WithoutCancel(runCtx), dlqRec); produceErr != nil {
			c.log.Warn("kafka: dlq produce failed, retrying",
				"event_id", env.EventId, "event_type", env.EventType, "dlq_topic", dlqTopic, "error", produceErr)
			select {
			case <-time.After(c.dlqRetryDelay(i)):
				continue
			case <-runCtx.Done():
				return false
			}
		}
		break
	}

	c.log.Error("kafka: event sent to dlq",
		"event_id", env.EventId, "event_type", env.EventType, "aggregate_id", env.AggregateId,
		"attempts", attempts, "error", cause, "dlq_topic", dlqTopic)
	c.dlqCount.Add(context.WithoutCancel(runCtx), 1, metric.WithAttributes(
		attribute.String("source_topic", rec.Topic), attribute.String("consumer_group", c.group)))
	c.processed.Add(context.WithoutCancel(runCtx), 1, metric.WithAttributes(
		attribute.String("event_type", env.EventType), attribute.String("result", "dlq")))
	return true
}

// dlqRetryDelay returns the backoff delay for the i'th DLQ produce retry,
// holding at the longest configured Backoff once exhausted.
func (c *Consumer) dlqRetryDelay(i int) time.Duration {
	if i < len(c.Backoff) {
		return c.Backoff[i]
	}
	return c.Backoff[len(c.Backoff)-1]
}

// observeLag records kafka.consumer.lag (D-50) for every partition present
// in fetches, computed as HighWatermark - (last fetched offset + 1).
func (c *Consumer) observeLag(ctx context.Context, fetches kgo.Fetches) {
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, part := range topic.Partitions {
				if len(part.Records) == 0 {
					continue
				}
				last := part.Records[len(part.Records)-1]
				lagValue := part.HighWatermark - (last.Offset + 1)
				c.lag.Record(ctx, lagValue, metric.WithAttributes(
					attribute.String("topic", topic.Topic), attribute.Int("partition", int(part.Partition))))
			}
		}
	}
}
