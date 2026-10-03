package domain

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

// CancellationTier is one row of a route's refund schedule (D-13):
// cancelling >= MinHoursBefore departure refunds RefundPercent.
type CancellationTier struct {
	MinHoursBefore int32 `json:"min_hours_before"`
	RefundPercent  int32 `json:"refund_percent"`
}

// DefaultCancellationPolicy is applied when a route is created with an
// empty cancellation policy (D-13).
func DefaultCancellationPolicy() []CancellationTier {
	return []CancellationTier{
		{MinHoursBefore: 24, RefundPercent: 100},
		{MinHoursBefore: 2, RefundPercent: 50},
		{MinHoursBefore: 0, RefundPercent: 0},
	}
}

// ValidateCancellationPolicy enforces D-13: 1-10 tiers, strictly descending
// MinHoursBefore >= 0, a 0-hour tier present, RefundPercent 0-100.
func ValidateCancellationPolicy(tiers []CancellationTier) error {
	if len(tiers) == 0 || len(tiers) > 10 {
		return fmt.Errorf("%w: cancellation policy must have 1-10 tiers, got %d", ErrInvalidArgument, len(tiers))
	}
	sawZero := false
	prev := int32(math.MaxInt32)
	for _, t := range tiers {
		if t.MinHoursBefore < 0 {
			return fmt.Errorf("%w: min_hours_before must be >= 0, got %d", ErrInvalidArgument, t.MinHoursBefore)
		}
		if t.MinHoursBefore >= prev {
			return fmt.Errorf("%w: min_hours_before must strictly descend", ErrInvalidArgument)
		}
		if t.RefundPercent < 0 || t.RefundPercent > 100 {
			return fmt.Errorf("%w: refund_percent must be 0-100, got %d", ErrInvalidArgument, t.RefundPercent)
		}
		if t.MinHoursBefore == 0 {
			sawZero = true
		}
		prev = t.MinHoursBefore
	}
	if !sawZero {
		return fmt.Errorf("%w: cancellation policy must include a 0-hour tier", ErrInvalidArgument)
	}
	return nil
}

// Route is a one-way departure/arrival pair (D-11). Route display name is
// derived from PierFromID/PierToID's own names, never stored (D-17).
// CurrentPrices holds each ticket type's price in effect for today's
// Asia/Bangkok date (D-14); populated only for display (ListRoutes), never
// persisted on the route itself. A route with no prices set has an empty
// CurrentPrices, never a zero-amount entry.
type Route struct {
	ID                 uuid.UUID
	OperatorID         uuid.UUID
	PierFromID         uuid.UUID
	PierToID           uuid.UUID
	DurationMinutes    int32
	CancellationPolicy []CancellationTier
	Archived           bool
	CurrentPrices      []RoutePrice
}

// Validate enforces the route's field invariants (also enforced by the
// routes table's check constraints — belt-and-suspenders, D-14).
func (r Route) Validate() error {
	if r.DurationMinutes < 1 || r.DurationMinutes > 1440 {
		return fmt.Errorf("%w: duration_minutes must be 1-1440, got %d", ErrInvalidArgument, r.DurationMinutes)
	}
	if r.PierFromID == r.PierToID {
		return fmt.Errorf("%w: pier_from_id and pier_to_id must differ", ErrInvalidArgument)
	}
	return ValidateCancellationPolicy(r.CancellationPolicy)
}
