// Package domain holds the template's sample aggregate: types and rules
// only, no persistence or transport concerns (D-14). Real services replace
// this package with their own domain types.
package domain

import "errors"

// ErrInvalidArgument and ErrNotFound are the sentinel errors app/adapters
// code wraps and callers match with errors.Is (D-44) — no custom error
// hierarchy.
var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
)
