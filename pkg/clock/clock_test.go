package clock

import (
	"testing"
	"time"
)

func TestLocalDate(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "23:59:59 Bangkok stays on the same local date",
			in:   time.Date(2026, 9, 25, 16, 59, 59, 0, time.UTC),
			want: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "00:00:00 Bangkok rolls over to the next local date",
			in:   time.Date(2026, 9, 25, 17, 0, 0, 0, time.UTC),
			want: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "00:30 Bangkok stays inside the 00:00-01:00 window",
			in:   time.Date(2026, 9, 25, 17, 30, 0, 0, time.UTC),
			want: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "23:00 Bangkok stays inside the 23:00-00:00 window",
			in:   time.Date(2026, 9, 26, 16, 0, 0, 0, time.UTC),
			want: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LocalDate(tc.in)
			if !got.Equal(tc.want) {
				t.Fatalf("LocalDate(%v) = %v, want %v", tc.in, got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("LocalDate(%v).Location() = %v, want time.UTC", tc.in, got.Location())
			}
		})
	}
}

func TestNow(t *testing.T) {
	original := Now
	t.Cleanup(func() { Now = original })

	fixed := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	Now = func() time.Time { return fixed }
	if !Now().Equal(fixed) {
		t.Fatalf("Now() = %v, want overridden %v", Now(), fixed)
	}

	Now = original
	if Now().Equal(fixed) {
		t.Fatalf("Now() still returns the overridden value after restore")
	}
}
