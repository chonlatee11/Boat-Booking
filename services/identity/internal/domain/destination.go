package domain

import (
	"net/mail"
	"strings"
)

// Destination kinds.
const (
	KindEmail = "email"
	KindPhone = "phone"
)

// Destination is a normalised login identifier — either an email address or
// an E.164 phone number (D-01).
type Destination struct {
	Kind  string
	Value string
}

// maxEmailLen bounds a normalised email address (RFC 5321 mailbox limit).
const maxEmailLen = 254

// NormalizeDestination trims and classifies raw as an email or phone
// destination (D-01). Email: trimmed, lower-cased, and net/mail.ParseAddress
// must yield exactly the bare address back (no display name, no extra
// angle-bracket noise) and at most maxEmailLen characters. Phone: a Thai
// local number (leading "0" + 8-9 digits) is normalised to "+66" + the
// digits after the leading zero; anything already starting with "+" is kept
// as-is once formatting characters (spaces, "-", "(", ")") are stripped and
// a "+660" trunk-zero prefix is collapsed to "+66" (WR-07: "+66 081..." and
// "081..." are the same subscriber), as long as 8-15 digits remain.
func NormalizeDestination(raw string) (Destination, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Destination{}, ErrInvalidArgument
	}

	if strings.Contains(trimmed, "@") {
		return normalizeEmail(trimmed)
	}

	return normalizePhone(trimmed)
}

func normalizeEmail(trimmed string) (Destination, error) {
	lower := strings.ToLower(trimmed)
	if len(lower) > maxEmailLen {
		return Destination{}, ErrInvalidArgument
	}
	addr, err := mail.ParseAddress(lower)
	if err != nil || addr.Address != lower {
		return Destination{}, ErrInvalidArgument
	}
	return Destination{Kind: KindEmail, Value: lower}, nil
}

// thaiLocalDigits and e164Digits bound the digit count accepted for the two
// phone forms normalizePhone recognises.
const (
	thaiLocalDigitsMin = 8
	thaiLocalDigitsMax = 9
	e164DigitsMin      = 8
	e164DigitsMax      = 15
)

func normalizePhone(trimmed string) (Destination, error) {
	stripped := stripPhoneFormatting(trimmed)

	switch {
	case strings.HasPrefix(stripped, "0"):
		digits := stripped[1:]
		if !isAllDigits(digits) || len(digits) < thaiLocalDigitsMin || len(digits) > thaiLocalDigitsMax {
			return Destination{}, ErrInvalidArgument
		}
		return Destination{Kind: KindPhone, Value: "+66" + digits}, nil
	case strings.HasPrefix(stripped, "+"):
		if strings.HasPrefix(stripped, "+660") {
			// WR-07: "+66 0XXXXXXXX" and "0XXXXXXXX" are the same Thai
			// subscriber -- E.164 never keeps the trunk 0 after a country
			// code, so strip it here too, or the two spellings would
			// normalise to two different destinations (two customer
			// accounts, two separate OTP rate-limit budgets).
			stripped = "+66" + stripped[4:]
		}
		digits := stripped[1:]
		if !isAllDigits(digits) || len(digits) < e164DigitsMin || len(digits) > e164DigitsMax {
			return Destination{}, ErrInvalidArgument
		}
		return Destination{Kind: KindPhone, Value: stripped}, nil
	default:
		return Destination{}, ErrInvalidArgument
	}
}

// stripPhoneFormatting removes the punctuation people commonly type in a
// phone number — spaces, dashes, parens — before digit classification.
func stripPhoneFormatting(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '-', '(', ')':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
