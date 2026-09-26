// Package kafkaadapter registers this service's Kafka consumer handlers.
package kafkaadapter

import "github.com/chonlatee11/boat-booking/pkg/kafka"

// Register wires every event handler this service consumes onto c. The
// sample PingRecorded handler is added in Task 2.
func Register(c *kafka.Consumer) {
}
