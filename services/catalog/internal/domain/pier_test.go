package domain

import (
	"errors"
	"strings"
	"testing"
)

func validPier() Pier {
	return Pier{NameTH: "ท่าเรือ", NameEN: "Pier", Lat: 7.9, Lng: 98.3}
}

func TestPierValidate(t *testing.T) {
	// thai100 is exactly 100 Unicode code points including a combining tone
	// mark per rune pair (base consonant + tone mark), proving length is
	// counted in code points, not grapheme clusters.
	thai100 := strings.Repeat("ก่", 50)
	thai101 := thai100 + "ก"

	cases := []struct {
		name    string
		pier    Pier
		wantErr bool
	}{
		{"valid minimal pier, no address, no hours", validPier(), false},
		{"name_th exactly 100 code points with combining marks", setNameTH(thai100), false},
		{"name_th 101 code points rejected", setNameTH(thai101), true},
		{"lat over range rejected", setLatLng(90.0001, 98.3), true},
		{"lng at -180 boundary ok", setLatLng(7.9, -180), false},
		{"lat 0 and lng 0 rejected", setLatLng(0, 0), true},
		{"opens without closes rejected", setHours("08:00", ""), true},
		{"opens before closes ok", setHours("08:00", "18:00"), false},
		{"opens after closes rejected", setHours("18:00", "08:00"), true},
		{"invalid hour rejected", setHours("25:00", "26:00"), true},
		{"address over 500 code points rejected", setAddress(strings.Repeat("a", 501)), true},
		{"address exactly 500 code points ok", setAddress(strings.Repeat("a", 500)), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.pier.Validate()
			if c.wantErr {
				if err == nil {
					t.Fatalf("Validate() = nil, want error")
				}
				if !errors.Is(err, ErrInvalidArgument) {
					t.Fatalf("Validate() = %v, want wrapping ErrInvalidArgument", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func setNameTH(name string) Pier {
	p := validPier()
	p.NameTH = name
	return p
}

func setLatLng(lat, lng float64) Pier {
	p := validPier()
	p.Lat = lat
	p.Lng = lng
	return p
}

func setAddress(address string) Pier {
	p := validPier()
	p.Address = address
	return p
}

func setHours(opens, closes string) Pier {
	p := validPier()
	p.OpensAt = opens
	p.ClosesAt = closes
	return p
}
