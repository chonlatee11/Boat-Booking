// Package money is the single source of integer money arithmetic and
// formatting. All money fields — proto, DB columns, and in-process values —
// are int64 satang (1 baht = 100 satang). No float/double appears anywhere in
// this package or in any money-carrying field elsewhere in the codebase
// (PITFALLS.md #7).
package money

import (
	"errors"
	"strconv"
	"strings"
)

// Satang is an integer amount of Thai satang (1 baht = 100 satang).
type Satang int64

// PercentOf computes pct percent of amount using integer arithmetic only.
//
// Rounding rule (THE rule, applied everywhere a percentage-based amount is
// computed — refunds, fees, etc.): fractional satang are truncated toward
// zero. A percentage refund therefore never exceeds the exact percentage,
// and any sub-satang remainder stays with the operator.
func PercentOf(amount Satang, pct int) (Satang, error) {
	if pct < 0 || pct > 100 {
		return 0, errors.New("money: pct must be between 0 and 100")
	}
	if amount < 0 {
		return 0, errors.New("money: amount must not be negative")
	}
	return amount * Satang(pct) / 100, nil
}

// FormatBaht renders s as a Thai-baht display string: a ฿ prefix, a
// thousands-separated baht integer part, and a two-digit satang fraction.
// Negative amounts are prefixed with "-" before the ฿ symbol.
func FormatBaht(s Satang) string {
	neg := s < 0
	abs := s
	if neg {
		abs = -abs
	}
	baht := int64(abs) / 100
	satang := int64(abs) % 100

	out := "฿" + groupThousands(baht) + "." + pad2(satang)
	if neg {
		out = "-" + out
	}
	return out
}

func pad2(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) < 2 {
		return "0" + s
	}
	return s
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}

	var groups []string
	for len(s) > 3 {
		groups = append([]string{s[len(s)-3:]}, groups...)
		s = s[:len(s)-3]
	}
	groups = append([]string{s}, groups...)
	return strings.Join(groups, ",")
}
