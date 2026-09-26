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
// destination. Email path only in this task: trimmed, lower-cased, and
// net/mail.ParseAddress must yield exactly the bare address back (no display
// name, no extra angle-bracket noise) and at most maxEmailLen characters.
// Phone parsing arrives alongside this function's phone branch in a later
// task.
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

// normalizePhone is filled in by a later task (phone channel); until then
// every non-email destination is rejected.
func normalizePhone(_ string) (Destination, error) {
	return Destination{}, ErrInvalidArgument
}
