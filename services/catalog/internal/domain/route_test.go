package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

func TestValidateCancellationPolicy(t *testing.T) {
	cases := []struct {
		name    string
		tiers   []domain.CancellationTier
		wantErr bool
	}{
		{name: "default policy", tiers: domain.DefaultCancellationPolicy(), wantErr: false},
		{name: "empty", tiers: []domain.CancellationTier{}, wantErr: true},
		{
			name: "not descending",
			tiers: []domain.CancellationTier{
				{MinHoursBefore: 2, RefundPercent: 50},
				{MinHoursBefore: 24, RefundPercent: 100},
				{MinHoursBefore: 0, RefundPercent: 0},
			},
			wantErr: true,
		},
		{
			name: "not strictly descending (duplicate)",
			tiers: []domain.CancellationTier{
				{MinHoursBefore: 24, RefundPercent: 100},
				{MinHoursBefore: 24, RefundPercent: 50},
				{MinHoursBefore: 0, RefundPercent: 0},
			},
			wantErr: true,
		},
		{
			name: "missing 0-hour tier",
			tiers: []domain.CancellationTier{
				{MinHoursBefore: 24, RefundPercent: 100},
				{MinHoursBefore: 2, RefundPercent: 50},
			},
			wantErr: true,
		},
		{
			name:    "refund percent over 100",
			tiers:   []domain.CancellationTier{{MinHoursBefore: 0, RefundPercent: 101}},
			wantErr: true,
		},
		{
			name:    "negative min hours",
			tiers:   []domain.CancellationTier{{MinHoursBefore: -1, RefundPercent: 0}},
			wantErr: true,
		},
		{
			name: "11 tiers exceeds max",
			tiers: func() []domain.CancellationTier {
				tiers := make([]domain.CancellationTier, 11)
				for i := range tiers {
					tiers[i] = domain.CancellationTier{MinHoursBefore: int32(10 - i), RefundPercent: 0}
				}
				return tiers
			}(),
			wantErr: true,
		},
		{name: "single zero tier ok", tiers: []domain.CancellationTier{{MinHoursBefore: 0, RefundPercent: 0}}, wantErr: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.ValidateCancellationPolicy(c.tiers)
			if c.wantErr && !errors.Is(err, domain.ErrInvalidArgument) {
				t.Errorf("ValidateCancellationPolicy(%+v) = %v, want ErrInvalidArgument", c.tiers, err)
			}
			if !c.wantErr && err != nil {
				t.Errorf("ValidateCancellationPolicy(%+v) = %v, want nil", c.tiers, err)
			}
		})
	}
}

func TestRouteValidate(t *testing.T) {
	pierA := uuid.New()
	pierB := uuid.New()

	base := func() domain.Route {
		return domain.Route{
			ID:                 uuid.New(),
			OperatorID:         uuid.New(),
			PierFromID:         pierA,
			PierToID:           pierB,
			DurationMinutes:    30,
			CancellationPolicy: domain.DefaultCancellationPolicy(),
		}
	}

	t.Run("valid route ok", func(t *testing.T) {
		if err := base().Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("duration zero", func(t *testing.T) {
		r := base()
		r.DurationMinutes = 0
		if err := r.Validate(); !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("Validate() = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("duration too long", func(t *testing.T) {
		r := base()
		r.DurationMinutes = 1441
		if err := r.Validate(); !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("Validate() = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("pier_from equals pier_to", func(t *testing.T) {
		r := base()
		r.PierToID = r.PierFromID
		if err := r.Validate(); !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("Validate() = %v, want ErrInvalidArgument", err)
		}
	})
}
