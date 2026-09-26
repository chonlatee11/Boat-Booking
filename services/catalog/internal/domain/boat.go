package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Status is a boat's operational status.
type Status string

const (
	StatusActive      Status = "active"
	StatusMaintenance Status = "maintenance"
)

// Boat is a boat owned by an operator's fleet — catalog's own aggregate
// (Phase 1). HomePierID scopes the boat under pier-level access (D-07):
// OperatorID is always its home pier's operator, never taken from the
// request.
type Boat struct {
	ID              uuid.UUID
	OperatorID      uuid.UUID
	HomePierID      uuid.UUID
	Name            string
	DefaultCapacity int32
	Status          Status
	Archived        bool
}

// Validate enforces the boat's field invariants (also enforced by the boats
// table's check constraints — belt-and-suspenders, D-14). Every write
// requires a home pier (D-07) — a nil HomePierID is invalid even though the
// column is nullable at the DB level (Phase-1 dev rows survive the
// migration, but the app requires it on every write).
func (b Boat) Validate() error {
	name := strings.TrimSpace(b.Name)
	if n := len([]rune(name)); n < 1 || n > 100 {
		return fmt.Errorf("%w: name must be 1-100 characters, got %d", ErrInvalidArgument, n)
	}
	if b.DefaultCapacity < 1 || b.DefaultCapacity > 1000 {
		return fmt.Errorf("%w: default_capacity must be 1-1000, got %d", ErrInvalidArgument, b.DefaultCapacity)
	}
	switch b.Status {
	case StatusActive, StatusMaintenance:
	default:
		return fmt.Errorf("%w: unknown status %q", ErrInvalidArgument, b.Status)
	}
	if b.HomePierID == uuid.Nil {
		return fmt.Errorf("%w: home_pier_id is required", ErrInvalidArgument)
	}
	return nil
}
