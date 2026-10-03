package app

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func fromPgUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

// pgText builds a pgtype.Text that is NULL when s is empty — matching the
// users table's nullable email/phone columns.
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func userFromRow(row postgres.User) domain.User {
	u := domain.User{
		ID:   fromPgUUID(row.ID),
		Name: row.Name,
		Role: row.Role,
	}
	if row.Email.Valid {
		u.Email = row.Email.String
	}
	if row.Phone.Valid {
		u.Phone = row.Phone.String
	}
	if row.OperatorID.Valid {
		u.OperatorID = fromPgUUID(row.OperatorID).String()
	}
	if row.DisabledAt.Valid {
		t := row.DisabledAt.Time
		u.DisabledAt = &t
	}
	u.PierIDs = make([]string, len(row.PierIds))
	for i, id := range row.PierIds {
		u.PierIDs[i] = fromPgUUID(id).String()
	}
	return u
}
