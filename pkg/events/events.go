// Package events builds and parses the platform Envelope — the single Kafka
// value wire contract every domain event is published as (D-06). Publishing
// itself always goes through pkg/outbox → pkg/kafka; this package only
// constructs/serializes the envelope.
package events

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	platformv1 "github.com/chonlatee11/boat-booking/gen/go/platform/v1"
	"github.com/chonlatee11/boat-booking/pkg/clock"
)

// SchemaVersion is the current Envelope schema version (D-06). Bump this,
// never reuse or remove a proto field number, when the envelope shape
// changes (proto/README.md).
const SchemaVersion = 1

// New builds an Envelope wrapping payload with a fresh UUIDv7 event id and
// the current time (via pkg/clock, never time.Now directly).
func New(eventType, aggregateID string, payload proto.Message) (*platformv1.Envelope, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("events: new event id: %w", err)
	}

	any, err := anypb.New(payload)
	if err != nil {
		return nil, fmt.Errorf("events: wrap payload: %w", err)
	}

	return &platformv1.Envelope{
		EventId:     id.String(),
		EventType:   eventType,
		AggregateId: aggregateID,
		OccurredAt:  timestamppb.New(clock.Now()),
		Version:     SchemaVersion,
		Payload:     any,
	}, nil
}

// Marshal serializes env as the Kafka record value.
func Marshal(env *platformv1.Envelope) ([]byte, error) {
	data, err := proto.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("events: marshal envelope: %w", err)
	}
	return data, nil
}

// Unmarshal parses a Kafka record value back into an Envelope.
func Unmarshal(data []byte) (*platformv1.Envelope, error) {
	env := &platformv1.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("events: unmarshal envelope: %w", err)
	}
	return env, nil
}

// TopicFor derives the Kafka topic from an event type, e.g.
// "catalog.BoatUpserted" -> "catalog.events". eventType must carry a
// "<service>." prefix.
func TopicFor(eventType string) (string, error) {
	i := strings.IndexByte(eventType, '.')
	if i <= 0 {
		return "", fmt.Errorf("events: event type %q has no service prefix", eventType)
	}
	return eventType[:i] + ".events", nil
}

// DLQTopic returns the dead-letter topic for topic (D-12).
func DLQTopic(topic string) string {
	return topic + ".dlq"
}
