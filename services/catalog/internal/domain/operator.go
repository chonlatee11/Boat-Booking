package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Operator is a boat-operator tenant owning piers, routes, boats and prices
// (D-08). Only super_admin may create or edit operators.
type Operator struct {
	ID       uuid.UUID
	Name     string
	Archived bool
}

// Validate enforces the operator's field invariants (also enforced by the
// operators table's check constraint — belt-and-suspenders, D-14). Name
// length is counted in Unicode code points after trimming, matching
// Postgres's char_length (CAT-02 encoding edge).
func (o Operator) Validate() error {
	name := strings.TrimSpace(o.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return fmt.Errorf("%w: name must be 1-100 characters, got %d", ErrInvalidArgument, n)
	}
	return nil
}
