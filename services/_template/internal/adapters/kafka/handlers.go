// Package kafkaadapter registers this service's Kafka consumer handlers.
package kafkaadapter

import (
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	"github.com/chonlatee11/boat-booking/services/__NAME__/internal/app"
)

// Register wires every event handler this service consumes onto c —
// PingRecorded is the template's own sample handler, proving the round trip
// back through Kafka (D-03).
func Register(c *kafka.Consumer) {
	c.Handle(app.PingRecorded, app.AckPing)
}
