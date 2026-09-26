package events_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/chonlatee11/boat-booking/pkg/clock"
	"github.com/chonlatee11/boat-booking/pkg/events"
)

func TestTopicFor(t *testing.T) {
	cases := []struct {
		eventType string
		want      string
		wantErr   bool
	}{
		{eventType: "catalog.BoatUpserted", want: "catalog.events"},
		{eventType: "schedule.DepartureCreated", want: "schedule.events"},
		{eventType: "NoDotAtAll", wantErr: true},
		{eventType: "", wantErr: true},
	}

	for _, c := range cases {
		got, err := events.TopicFor(c.eventType)
		if c.wantErr {
			if err == nil {
				t.Errorf("TopicFor(%q) = %q, want error", c.eventType, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("TopicFor(%q): unexpected error: %v", c.eventType, err)
			continue
		}
		if got != c.want {
			t.Errorf("TopicFor(%q) = %q, want %q", c.eventType, got, c.want)
		}
	}
}

func TestDLQTopic(t *testing.T) {
	if got := events.DLQTopic("catalog.events"); got != "catalog.events.dlq" {
		t.Errorf("DLQTopic(%q) = %q, want %q", "catalog.events", got, "catalog.events.dlq")
	}
}

func TestNewEventIDIsUUIDv7(t *testing.T) {
	env, err := events.New("catalog.BoatUpserted", "boat-1", wrapperspb.String("x"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	id, err := uuid.Parse(env.EventId)
	if err != nil {
		t.Fatalf("event id %q is not a valid uuid: %v", env.EventId, err)
	}
	if id.Version() != 7 {
		t.Errorf("event id version = %d, want 7", id.Version())
	}
}

func TestNewOccurredAtUsesClockNow(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	orig := clock.Now
	clock.Now = func() time.Time { return fixed }
	t.Cleanup(func() { clock.Now = orig })

	env, err := events.New("catalog.BoatUpserted", "boat-1", wrapperspb.String("x"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !env.OccurredAt.AsTime().Equal(fixed) {
		t.Errorf("OccurredAt = %v, want %v", env.OccurredAt.AsTime(), fixed)
	}
}

func TestNewSetsSchemaVersionAndFields(t *testing.T) {
	env, err := events.New("catalog.BoatUpserted", "boat-1", wrapperspb.String("x"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if env.Version != events.SchemaVersion {
		t.Errorf("Version = %d, want %d", env.Version, events.SchemaVersion)
	}
	if env.EventType != "catalog.BoatUpserted" {
		t.Errorf("EventType = %q, want %q", env.EventType, "catalog.BoatUpserted")
	}
	if env.AggregateId != "boat-1" {
		t.Errorf("AggregateId = %q, want %q", env.AggregateId, "boat-1")
	}
	if env.Payload == nil {
		t.Error("Payload is nil, want a wrapped anypb.Any")
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	env, err := events.New("catalog.BoatUpserted", "boat-1", wrapperspb.String("x"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data, err := events.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := events.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.EventId != env.EventId {
		t.Errorf("round-tripped EventId = %q, want %q", got.EventId, env.EventId)
	}
}
