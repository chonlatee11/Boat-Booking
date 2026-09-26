package money

import "testing"

func TestPercentOf(t *testing.T) {
	cases := []struct {
		name   string
		amount Satang
		pct    int
		want   Satang
	}{
		{"typical refund percentage", 12345, 50, 6172},
		{"truncates below one satang", 1, 50, 0},
		{"100 percent returns the exact amount", 9999, 100, 9999},
		{"0 percent returns zero", 9999, 0, 0},
		{"odd amount truncates the fraction", 333, 33, 109},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PercentOf(tc.amount, tc.pct)
			if err != nil {
				t.Fatalf("PercentOf(%d, %d) returned error: %v", tc.amount, tc.pct, err)
			}
			if got != tc.want {
				t.Fatalf("PercentOf(%d, %d) = %d, want %d", tc.amount, tc.pct, got, tc.want)
			}
		})
	}
}

func TestPercentOfRejectsOutOfRangeInput(t *testing.T) {
	cases := []struct {
		name   string
		amount Satang
		pct    int
	}{
		{"negative percentage", 100, -1},
		{"percentage above 100", 100, 101},
		{"negative amount", -1, 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PercentOf(tc.amount, tc.pct); err == nil {
				t.Fatalf("PercentOf(%d, %d) expected an error, got nil", tc.amount, tc.pct)
			}
		})
	}
}

func TestFormatBaht(t *testing.T) {
	cases := []struct {
		name string
		in   Satang
		want string
	}{
		{"zero", 0, "฿0.00"},
		{"single digit satang", 5, "฿0.05"},
		{"thousands separator", 123456, "฿1,234.56"},
		{"millions separator", 100000000, "฿1,000,000.00"},
		{"negative amount", -250, "-฿2.50"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatBaht(tc.in); got != tc.want {
				t.Fatalf("FormatBaht(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
