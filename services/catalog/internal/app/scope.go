package app

import (
	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/auth"
)

// Scope is the single pier/operator scoping rule every catalog entity's
// list/get/write path applies (AUTH-05): super_admin bypasses operator/pier
// filtering entirely; pier_admin/staff are scoped to OperatorID and, for
// pier-bound entities, PierIDs. Written once, reused everywhere (Pitfall 6).
type Scope struct {
	Role       string
	OperatorID uuid.UUID
	PierIDs    []uuid.UUID
}

// All reports whether scope bypasses operator/pier filtering — super_admin
// only. Without this explicit bypass, a super_admin's naturally-empty
// PierIDs would make every scoped query return zero rows (Pitfall 6).
func (s Scope) All() bool {
	return s.Role == auth.RoleSuperAdmin
}

// CanWrite reports whether the role may write within its scope. staff is
// read-only in the catalog for v1.
func (s Scope) CanWrite() bool {
	return s.Role == auth.RoleSuperAdmin || s.Role == auth.RolePierAdmin
}

// PierIDArray never returns nil — pgx must send an empty array literal to
// Postgres for `= any($1)` to match nothing, not NULL.
func (s Scope) PierIDArray() []uuid.UUID {
	if s.PierIDs == nil {
		return []uuid.UUID{}
	}
	return s.PierIDs
}
