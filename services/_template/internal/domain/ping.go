package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// Ping is the template's sample aggregate — it exists only to exercise
// every platform layer (HTTP -> tx -> outbox -> Kafka -> consumer). Real
// services delete it.
type Ping struct {
	ID         uuid.UUID
	OperatorID uuid.UUID
	Note       string
}

// ValidateNote enforces the note length invariant (also enforced by the
// pings table's check constraint — belt-and-suspenders, D-14).
func ValidateNote(note string) error {
	n := len([]rune(note))
	if n < 1 || n > 280 {
		return fmt.Errorf("%w: note must be 1-280 characters, got %d", ErrInvalidArgument, n)
	}
	return nil
}
