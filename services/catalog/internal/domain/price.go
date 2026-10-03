package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/money"
)

// TicketType is a fare category (D-14). v1 supports adult and child only.
type TicketType string

const (
	TicketTypeAdult TicketType = "adult"
	TicketTypeChild TicketType = "child"
)

// RoutePrice is one effective-dated fare row (D-14): the price in effect on
// date D is the row with the latest EffectiveFrom <= D. EffectiveFrom is a
// local (Asia/Bangkok) calendar date, encoded as pkg/clock.LocalDate does
// (midnight UTC of that date).
type RoutePrice struct {
	RouteID       uuid.UUID
	TicketType    TicketType
	AmountSatang  money.Satang
	EffectiveFrom time.Time
}

// Validate enforces the price's field invariants (also enforced by the
// route_prices table's check constraints — belt-and-suspenders, D-14).
// today must already be a local calendar date (pkg/clock.LocalDate) — a
// price may never be scheduled for a past date (no retroactive rewrites).
func (p RoutePrice) Validate(today time.Time) error {
	switch p.TicketType {
	case TicketTypeAdult, TicketTypeChild:
	default:
		return fmt.Errorf("%w: ticket_type must be adult or child, got %q", ErrInvalidArgument, p.TicketType)
	}
	if p.AmountSatang < 0 || p.AmountSatang > 10_000_000 {
		return fmt.Errorf("%w: amount_satang must be 0-10,000,000, got %d", ErrInvalidArgument, p.AmountSatang)
	}
	if p.EffectiveFrom.Before(today) {
		return fmt.Errorf("%w: effective_from must not be before today", ErrInvalidArgument)
	}
	return nil
}
