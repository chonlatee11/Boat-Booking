// Package domain holds identity's own aggregate: types and validation rules
// only, no persistence or transport concerns (D-14).
package domain

import "errors"

// Sentinel errors app/adapters code wraps and callers match with errors.Is
// (D-44) — no custom error hierarchy, except CodeMismatchError below, which
// needs to carry AttemptsLeft.
var (
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrNotFound            = errors.New("not found")
	ErrCodeExpired         = errors.New("code expired, locked, or already used")
	ErrResendTooSoon       = errors.New("resend requested too soon")
	ErrRateLimited         = errors.New("rate limited")
	ErrDisabled            = errors.New("user disabled")
	ErrDeliveryUnavailable = errors.New("delivery channel unavailable")
	ErrSessionInvalid      = errors.New("session invalid, expired, or reused")
)

// CodeMismatchError is returned when a wrong OTP code is presented, and
// carries how many attempts remain before the code is locked (D-03).
type CodeMismatchError struct {
	AttemptsLeft int
}

func (e *CodeMismatchError) Error() string {
	return "code mismatch"
}
