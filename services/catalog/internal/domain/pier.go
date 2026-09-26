package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Pier is a physical departure/arrival point owned by an operator (D-07,
// D-17). OpensAt/ClosesAt are "HH:MM" 24h local time, or both empty when the
// pier has no posted hours (display-only, D-16).
type Pier struct {
	ID         uuid.UUID
	OperatorID uuid.UUID
	NameTH     string
	NameEN     string
	Lat        float64
	Lng        float64
	Address    string
	OpensAt    string
	ClosesAt   string
	Archived   bool
}

// Validate enforces the pier's field invariants (also enforced by the piers
// table's check constraints — belt-and-suspenders, D-14). Name/address
// lengths are counted in Unicode code points after trimming, matching
// Postgres's char_length (CAT-02 encoding edge). (0,0) is rejected as
// "location not set" — no Thai pier lies there (D-16 empty edge).
func (p Pier) Validate() error {
	if n := utf8.RuneCountInString(strings.TrimSpace(p.NameTH)); n < 1 || n > 100 {
		return fmt.Errorf("%w: name_th must be 1-100 characters, got %d", ErrInvalidArgument, n)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.NameEN)); n < 1 || n > 100 {
		return fmt.Errorf("%w: name_en must be 1-100 characters, got %d", ErrInvalidArgument, n)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Address)); n > 500 {
		return fmt.Errorf("%w: address must be at most 500 characters, got %d", ErrInvalidArgument, n)
	}
	if math.IsNaN(p.Lat) || math.IsInf(p.Lat, 0) || p.Lat < -90 || p.Lat > 90 {
		return fmt.Errorf("%w: lat must be between -90 and 90, got %v", ErrInvalidArgument, p.Lat)
	}
	if math.IsNaN(p.Lng) || math.IsInf(p.Lng, 0) || p.Lng < -180 || p.Lng > 180 {
		return fmt.Errorf("%w: lng must be between -180 and 180, got %v", ErrInvalidArgument, p.Lng)
	}
	if p.Lat == 0 && p.Lng == 0 {
		return fmt.Errorf("%w: lat/lng (0,0) means location not set", ErrInvalidArgument)
	}

	if (p.OpensAt == "") != (p.ClosesAt == "") {
		return fmt.Errorf("%w: opens_at and closes_at must both be set or both empty", ErrInvalidArgument)
	}
	if p.OpensAt != "" {
		opens, err := time.Parse("15:04", p.OpensAt)
		if err != nil {
			return fmt.Errorf("%w: opens_at must be HH:MM, got %q", ErrInvalidArgument, p.OpensAt)
		}
		closes, err := time.Parse("15:04", p.ClosesAt)
		if err != nil {
			return fmt.Errorf("%w: closes_at must be HH:MM, got %q", ErrInvalidArgument, p.ClosesAt)
		}
		if !opens.Before(closes) {
			return fmt.Errorf("%w: opens_at must be before closes_at", ErrInvalidArgument)
		}
	}

	return nil
}
