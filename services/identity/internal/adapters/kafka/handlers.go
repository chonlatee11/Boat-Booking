// Package kafkaadapter registers this service's Kafka consumer handlers.
package kafkaadapter

import (
	"github.com/chonlatee11/boat-booking/pkg/kafka"
)

// Register wires every event handler this service consumes onto c — nothing
// yet (identity only publishes identity.UserCreated).
func Register(_ *kafka.Consumer) {}
