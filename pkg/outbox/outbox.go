// Package outbox is the transactional outbox writer and relay (D-05, D-10,
// D-11). No service or app code publishes to Kafka directly — every event
// goes: business tx writes state + Insert in the same transaction, then the
// Relay polls and publishes via pkg/kafka.Producer (T-04-01).
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
)

// Insert writes env as one outbox row inside tx (D-05), in the same
// transaction as the business state change it describes. The stored
// traceparent is injected from ctx so the relay can later reconstruct the
// originating trace across the async Kafka hop (Pitfall 11).
func Insert(ctx context.Context, tx pgx.Tx, env *platformv1.Envelope) error {
	topic, err := events.TopicFor(env.EventType)
	if err != nil {
		return err
	}

	payload, err := events.Marshal(env)
	if err != nil {
		return fmt.Errorf("outbox: marshal envelope: %w", err)
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	_, err = tx.Exec(ctx, `
		insert into outbox (event_id, topic, aggregate_id, event_type, payload, traceparent)
		values ($1, $2, $3, $4, $5, $6)
	`, env.EventId, topic, env.AggregateId, env.EventType, payload, carrier.Get("traceparent"))
	if err != nil {
		return fmt.Errorf("outbox: insert row: %w", err)
	}
	return nil
}

// Relay polls the outbox table and publishes unpublished rows to Kafka in
// order (D-10, D-11).
type Relay struct {
	pool     *pgxpool.Pool
	producer *kafka.Producer
	log      *slog.Logger

	// PollInterval is how often Run wakes to check for unpublished rows.
	PollInterval time.Duration
	// BatchSize is the max rows read per poll.
	BatchSize int
}

// NewRelay builds a Relay with the default poll interval (500ms, D-10) and
// batch size (100, D-11).
func NewRelay(pool *pgxpool.Pool, producer *kafka.Producer, log *slog.Logger) *Relay {
	return &Relay{
		pool:         pool,
		producer:     producer,
		log:          log,
		PollInterval: 500 * time.Millisecond,
		BatchSize:    100,
	}
}

// Run polls for unpublished outbox rows until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.publishOnce(ctx); err != nil {
				r.log.Error("outbox: publish batch failed", "error", err)
			}
		}
	}
}

type outboxRow struct {
	id          int64
	topic       string
	aggregateID string
	eventType   string
	eventID     string
	payload     []byte
	traceparent string
}

// publishOnce reads at most BatchSize unpublished rows (FOR UPDATE SKIP
// LOCKED, ORDER BY id) and publishes them in order, stopping at the first
// publish failure so a later row of the same aggregate never overtakes an
// earlier one still queued (D-11).
func (r *Relay) publishOnce(ctx context.Context) error {
	return bbpgx.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id, topic, aggregate_id, event_type, event_id, payload, traceparent
			from outbox
			where published_at is null
			order by id
			limit $1
			for update skip locked
		`, r.BatchSize)
		if err != nil {
			return fmt.Errorf("outbox: select batch: %w", err)
		}

		var batch []outboxRow
		for rows.Next() {
			var row outboxRow
			if err := rows.Scan(&row.id, &row.topic, &row.aggregateID, &row.eventType, &row.eventID, &row.payload, &row.traceparent); err != nil {
				rows.Close()
				return fmt.Errorf("outbox: scan row: %w", err)
			}
			batch = append(batch, row)
		}
		scanErr := rows.Err()
		rows.Close()
		if scanErr != nil {
			return fmt.Errorf("outbox: read batch: %w", scanErr)
		}

		if len(batch) == 0 {
			return nil
		}

		published := make([]int64, 0, len(batch))
		for _, row := range batch {
			env, err := events.Unmarshal(row.payload)
			if err != nil {
				return fmt.Errorf("outbox: unmarshal row %d: %w", row.id, err)
			}

			carrier := propagation.MapCarrier{"traceparent": row.traceparent}
			rowCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

			if err := r.producer.Publish(rowCtx, row.topic, row.aggregateID, env); err != nil {
				r.log.Warn("outbox: publish failed, stopping batch",
					"event_id", row.eventID, "event_type", row.eventType, "aggregate_id", row.aggregateID, "error", err)
				break
			}
			published = append(published, row.id)
		}

		if len(published) == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, `update outbox set published_at = now() where id = any($1)`, published); err != nil {
			return fmt.Errorf("outbox: mark published: %w", err)
		}
		return nil
	})
}
