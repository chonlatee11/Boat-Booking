// Package kafkaadapter registers this service's Kafka consumer handlers.
package kafkaadapter

import (
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	"github.com/chonlatee11/boat-booking/services/schedule/internal/app"
)

// Register wires every event handler this service consumes onto c.
// schedule consumes catalog.BoatUpserted from catalog.events and applies it
// to its own boats projection (D-01, PLAT-05).
func Register(c *kafka.Consumer) {
	c.Handle(app.EventCatalogBoatUpserted, app.ApplyBoatUpserted)
}
