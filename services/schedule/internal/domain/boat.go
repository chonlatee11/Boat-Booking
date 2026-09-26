package domain

import "github.com/google/uuid"

// Status is a boat's operational status, mirrored from catalog's own
// BoatUpserted event (D-01).
type Status string

const (
	StatusActive      Status = "active"
	StatusMaintenance Status = "maintenance"
)

// Boat is schedule's own projection of a catalog boat — a cache of just the
// fields Phase 3 departures need (default capacity, status), kept current
// by the catalog.BoatUpserted consumer. schedule never reads catalog's
// database (database-per-service).
type Boat struct {
	ID              uuid.UUID
	OperatorID      uuid.UUID
	DefaultCapacity int32
	Status          Status
}
