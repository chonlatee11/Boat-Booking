package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/money"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

func TestRoutePriceValidate(t *testing.T) {
	today := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	base := func() domain.RoutePrice {
		return domain.RoutePrice{
			RouteID:       uuid.New(),
			TicketType:    domain.TicketTypeAdult,
			AmountSatang:  15000,
			EffectiveFrom: today,
		}
	}

	cases := []struct {
		name    string
		mutate  func(p domain.RoutePrice) domain.RoutePrice
		wantErr bool
	}{
		{name: "valid", mutate: func(p domain.RoutePrice) domain.RoutePrice { return p }, wantErr: false},
		{name: "amount negative", mutate: func(p domain.RoutePrice) domain.RoutePrice { p.AmountSatang = -1; return p }, wantErr: true},
		{name: "amount over max", mutate: func(p domain.RoutePrice) domain.RoutePrice { p.AmountSatang = 10_000_001; return p }, wantErr: true},
		{name: "amount zero ok (free child fare)", mutate: func(p domain.RoutePrice) domain.RoutePrice { p.AmountSatang = 0; return p }, wantErr: false},
		{name: "empty ticket type", mutate: func(p domain.RoutePrice) domain.RoutePrice { p.TicketType = ""; return p }, wantErr: true},
		{
			name: "effective_from yesterday",
			mutate: func(p domain.RoutePrice) domain.RoutePrice {
				p.EffectiveFrom = today.AddDate(0, 0, -1)
				return p
			},
			wantErr: true,
		},
		{name: "effective_from today ok", mutate: func(p domain.RoutePrice) domain.RoutePrice { p.EffectiveFrom = today; return p }, wantErr: false},
		{
			name: "effective_from today+30 ok",
			mutate: func(p domain.RoutePrice) domain.RoutePrice {
				p.EffectiveFrom = today.AddDate(0, 0, 30)
				return p
			},
			wantErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.mutate(base())
			err := p.Validate(today)
			if c.wantErr && !errors.Is(err, domain.ErrInvalidArgument) {
				t.Errorf("Validate() = %v, want ErrInvalidArgument", err)
			}
			if !c.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

// TestRoutePriceAmountIsSatang guards against a float ever creeping into
// AmountSatang's type (Pitfall 7) — this is a compile-time assertion, not a
// runtime check, but keeping it as a table-test entry documents the
// invariant alongside the other price rules.
func TestRoutePriceAmountIsSatang(t *testing.T) {
	var amount money.Satang = 15000
	p := domain.RoutePrice{AmountSatang: amount}
	if p.AmountSatang != 15000 {
		t.Errorf("AmountSatang = %d, want 15000", p.AmountSatang)
	}
}
