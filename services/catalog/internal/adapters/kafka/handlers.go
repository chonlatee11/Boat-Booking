// Package kafkaadapter registers this service's Kafka consumer handlers.
package kafkaadapter

import "github.com/chonlatee11/boat-booking/pkg/kafka"

// Register wires every event handler this service consumes onto c. catalog
// consumes no events in Phase 1 — it only publishes catalog.BoatUpserted.
func Register(c *kafka.Consumer) {}
