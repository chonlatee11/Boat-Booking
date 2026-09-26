package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is identity's own aggregate — a login identity plus the role/scope
// claims later issued in its JWTs (D-01, D-04, D-06). Email and phone are
// two nullable, independently-unique login identifiers on the same row (at
// least one is required, enforced by a DB check constraint); linking them is
// out of scope for v1 (assumption_delta_decision, 02-02-PLAN.md).
type User struct {
	ID         uuid.UUID
	Email      string // "" when the user signed up by phone
	Phone      string // "" when the user signed up by email
	Name       string
	Role       string
	OperatorID string // "" for customer
	PierIDs    []string
	DisabledAt *time.Time
}
