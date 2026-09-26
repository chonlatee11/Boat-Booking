// Package kafka is the only sanctioned Kafka client constructor (D-08,
// T-04-01). No service or app code may call kgo.NewClient directly — every
// publish goes through pkg/outbox's Relay calling Producer.Publish here.
package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/otel"

	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
)

// Producer wraps a kotel-traced, acks=all Kafka client (D-08).
type Producer struct {
	cl *kgo.Client
}

// NewProducer creates a Producer. opts are appended after the required
// broker/acks/tracing options, so callers can override defaults (e.g. a
// short RecordDeliveryTimeout for failure-path tests).
func NewProducer(brokers []string, opts ...kgo.Opt) (*Producer, error) {
	tracer := kotel.NewTracer(kotel.TracerProvider(otel.GetTracerProvider()))
	kt := kotel.NewKotel(kotel.WithTracer(tracer))

	all := append([]kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.WithHooks(kt.Hooks()...),
	}, opts...)

	cl, err := kgo.NewClient(all...) //nolint:forbidigo // pkg/kafka is the sanctioned constructor (D-08)
	if err != nil {
		return nil, fmt.Errorf("kafka: new client: %w", err)
	}
	return &Producer{cl: cl}, nil
}

// Publish sends env to topic keyed by aggregateID (D-08): the record key
// becomes aggregateID (per-aggregate partition ordering), and headers
// event_type/event_id are always set. ctx must carry the trace context to
// attach to the publish span — kotel's produce hook injects the resulting
// traceparent header from record.Context; callers must never set that
// header by hand (Pitfall: kotel owns it).
func (p *Producer) Publish(ctx context.Context, topic, aggregateID string, env *platformv1.Envelope) error {
	value, err := events.Marshal(env)
	if err != nil {
		return fmt.Errorf("kafka: marshal envelope: %w", err)
	}

	rec := &kgo.Record{
		Topic:   topic,
		Key:     []byte(aggregateID),
		Value:   value,
		Context: ctx,
		Headers: []kgo.RecordHeader{
			{Key: "event_type", Value: []byte(env.EventType)},
			{Key: "event_id", Value: []byte(env.EventId)},
		},
	}

	return p.produceRecord(ctx, rec)
}

// produceRecord synchronously produces rec exactly as given. Used internally
// for records that are not domain envelopes built via Publish — e.g. the
// Consumer's DLQ path, which passes a failed record through verbatim
// (original key/value/headers plus failure metadata) rather than building a
// new envelope (D-12).
func (p *Producer) produceRecord(ctx context.Context, rec *kgo.Record) error {
	if err := p.cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return fmt.Errorf("kafka: produce: %w", err)
	}
	return nil
}

// Ping verifies broker connectivity (used by /readyz, D-39).
func (p *Producer) Ping(ctx context.Context) error {
	if err := p.cl.Ping(ctx); err != nil {
		return fmt.Errorf("kafka: ping: %w", err)
	}
	return nil
}

// Close releases the underlying client.
func (p *Producer) Close() {
	p.cl.Close()
}
