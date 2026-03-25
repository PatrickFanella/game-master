// Package dbutil provides shared helpers for converting between domain types
// and database-specific types (pgtype, pgvector).
package dbutil

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// FromPgtype converts a pgtype.UUID to a uuid.UUID.
// Invalid UUIDs are returned as uuid.Nil.
func FromPgtype(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

// ToPgtype converts a uuid.UUID to a pgtype.UUID.
// uuid.Nil is stored with Valid set to false.
func ToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: u != uuid.Nil}
}

// PgUUIDsToStrings converts a slice of pgtype.UUID to string representations,
// skipping any invalid entries.
func PgUUIDsToStrings(ids []pgtype.UUID) []string {
	if len(ids) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, FromPgtype(id).String())
	}
	return out
}

// UUIDsToPgtype converts a slice of uuid.UUID to pgtype.UUID.
func UUIDsToPgtype(ids []uuid.UUID) []pgtype.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = ToPgtype(id)
	}
	return out
}
