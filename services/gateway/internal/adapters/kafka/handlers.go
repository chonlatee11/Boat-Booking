// Package kafkaadapter registers this service's Kafka consumer handlers.
package kafkaadapter

import "github.com/chonlatee11/boat-booking/pkg/kafka"

// Register wires every event handler this service consumes onto c. gateway
// consumes nothing — no DATABASE_URL/KAFKA_BROKERS in its deployment means
// the consumer is disabled by env (D-04); this stays empty for template
// parity with every other service.
func Register(c *kafka.Consumer) {}
