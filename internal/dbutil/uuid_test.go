package dbutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFromPgtype(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		id := uuid.New()
		pg := pgtype.UUID{Bytes: id, Valid: true}
		got := FromPgtype(pg)
		if got != id {
			t.Errorf("FromPgtype() = %v, want %v", got, id)
		}
	})

	t.Run("invalid UUID returns Nil", func(t *testing.T) {
		pg := pgtype.UUID{Valid: false}
		got := FromPgtype(pg)
		if got != uuid.Nil {
			t.Errorf("FromPgtype() = %v, want uuid.Nil", got)
		}
	})
}

func TestToPgtype(t *testing.T) {
	t.Run("non-nil UUID", func(t *testing.T) {
		id := uuid.New()
		pg := ToPgtype(id)
		if !pg.Valid {
			t.Error("ToPgtype() Valid = false, want true")
		}
		if uuid.UUID(pg.Bytes) != id {
			t.Errorf("ToPgtype() Bytes = %v, want %v", pg.Bytes, id)
		}
	})

	t.Run("nil UUID", func(t *testing.T) {
		pg := ToPgtype(uuid.Nil)
		if pg.Valid {
			t.Error("ToPgtype(uuid.Nil) Valid = true, want false")
		}
	})
}

func TestPgUUIDsToStrings(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		got := PgUUIDsToStrings(nil)
		if len(got) != 0 {
			t.Errorf("PgUUIDsToStrings(nil) len = %d, want 0", len(got))
		}
	})

	t.Run("skips invalid entries", func(t *testing.T) {
		id := uuid.New()
		ids := []pgtype.UUID{
			{Bytes: id, Valid: true},
			{Valid: false},
		}
		got := PgUUIDsToStrings(ids)
		if len(got) != 1 {
			t.Fatalf("PgUUIDsToStrings() len = %d, want 1", len(got))
		}
		if got[0] != id.String() {
			t.Errorf("PgUUIDsToStrings()[0] = %s, want %s", got[0], id.String())
		}
	})
}

func TestUUIDsToPgtype(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		got := UUIDsToPgtype(nil)
		if got != nil {
			t.Errorf("UUIDsToPgtype(nil) = %v, want nil", got)
		}
	})

	t.Run("converts correctly", func(t *testing.T) {
		ids := []uuid.UUID{uuid.New(), uuid.New()}
		got := UUIDsToPgtype(ids)
		if len(got) != 2 {
			t.Fatalf("UUIDsToPgtype() len = %d, want 2", len(got))
		}
		for i, pg := range got {
			if !pg.Valid {
				t.Errorf("UUIDsToPgtype()[%d] Valid = false", i)
			}
			if uuid.UUID(pg.Bytes) != ids[i] {
				t.Errorf("UUIDsToPgtype()[%d] = %v, want %v", i, pg.Bytes, ids[i])
			}
		}
	})
}
