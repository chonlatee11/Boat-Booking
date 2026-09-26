package domain

import "testing"

func TestNormalizeDestination(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    Destination
		wantErr bool
	}{
		{name: "thai local phone", raw: "0812345678", want: Destination{Kind: KindPhone, Value: "+66812345678"}},
		{name: "thai phone with spaces and dashes", raw: "+66 81-234-5678", want: Destination{Kind: KindPhone, Value: "+66812345678"}},
		{name: "already e164 non-thai phone", raw: "+14155550100", want: Destination{Kind: KindPhone, Value: "+14155550100"}},
		{name: "too short local phone", raw: "081234", wantErr: true},
		{name: "email trimmed and lower-cased", raw: " A@B.co ", want: Destination{Kind: KindEmail, Value: "a@b.co"}},
		{name: "not an email", raw: "not-an-email", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeDestination(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("NormalizeDestination(%q) = %+v, nil; want error", c.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeDestination(%q): unexpected error: %v", c.raw, err)
			}
			if got != c.want {
				t.Fatalf("NormalizeDestination(%q) = %+v, want %+v", c.raw, got, c.want)
			}
		})
	}
}
