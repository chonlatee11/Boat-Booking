package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/auth"
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

// staffNameMaxLen, staffPierIDsMin/Max bound StaffUserInput's fields (AUTH-04).
const (
	staffNameMaxLen = 100
	staffPierIDsMin = 1
	staffPierIDsMax = 50
)

// StaffUserInput is the validated input driving UserService.UpsertUser
// (AUTH-04, D-08). super_admin and customer can never be assigned through
// this API — only "staff" or "pier_admin" (super_admin only via
// SUPER_ADMIN_EMAIL, D-09).
type StaffUserInput struct {
	Email      string
	Name       string
	Role       string
	OperatorID uuid.UUID
	PierIDs    []uuid.UUID
}

// Validate normalises Email in place (via NormalizeDestination, rejecting
// anything that isn't an email) and enforces role/name/pier_ids invariants:
// role must be "staff" or "pier_admin"; name is 1-100 Unicode code points
// after trimming; pier_ids has 1-50 unique entries.
func (in *StaffUserInput) Validate() error {
	dest, err := NormalizeDestination(in.Email)
	if err != nil || dest.Kind != KindEmail {
		return fmt.Errorf("%w: email must be a valid email address", ErrInvalidArgument)
	}
	in.Email = dest.Value

	if in.Role != auth.RoleStaff && in.Role != auth.RolePierAdmin {
		return fmt.Errorf("%w: role must be %q or %q, got %q", ErrInvalidArgument, auth.RoleStaff, auth.RolePierAdmin, in.Role)
	}

	in.Name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > staffNameMaxLen {
		return fmt.Errorf("%w: name must be 1-%d characters, got %d", ErrInvalidArgument, staffNameMaxLen, n)
	}

	if n := len(in.PierIDs); n < staffPierIDsMin || n > staffPierIDsMax {
		return fmt.Errorf("%w: pier_ids must have %d-%d entries, got %d", ErrInvalidArgument, staffPierIDsMin, staffPierIDsMax, n)
	}
	seen := make(map[uuid.UUID]struct{}, len(in.PierIDs))
	for _, id := range in.PierIDs {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: duplicate pier id %s", ErrInvalidArgument, id)
		}
		seen[id] = struct{}{}
	}

	return nil
}
