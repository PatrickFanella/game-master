package auth

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// DBTX is the database interface satisfied by *pgxpool.Pool, pgx.Conn, and pgx.Tx.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// dbAuthQuerier implements AuthQuerier using raw SQL against a DBTX.
// This avoids depending on sqlc-generated code for the two auth-specific queries,
// making the auth package self-contained. Once sqlc generate is run with the
// updated queries/users.sql, these queries will also be available via statedb.Queries.
type dbAuthQuerier struct {
	db DBTX
}

// NewDBAuthQuerier creates an AuthQuerier backed by the given database connection.
// Accepts *pgxpool.Pool, pgx.Conn, or pgx.Tx.
func NewDBAuthQuerier(db DBTX) AuthQuerier {
	return &dbAuthQuerier{db: db}
}

const createUserWithAuthSQL = `
INSERT INTO users (name, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, name, email, password_hash, created_at, updated_at
`

func (q *dbAuthQuerier) CreateUserWithAuth(ctx context.Context, arg CreateUserWithAuthParams) (UserWithAuth, error) {
	row := q.db.QueryRow(ctx, createUserWithAuthSQL, arg.Name, arg.Email, arg.PasswordHash)
	var u UserWithAuth
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

const getUserByEmailSQL = `
SELECT id, name, email, password_hash, created_at, updated_at
FROM users
WHERE email = $1
`

func (q *dbAuthQuerier) GetUserByEmail(ctx context.Context, email string) (UserWithAuth, error) {
	row := q.db.QueryRow(ctx, getUserByEmailSQL, email)
	var u UserWithAuth
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		// Preserve pgx.ErrNoRows for callers that check for it.
		return UserWithAuth{}, err
	}
	return u, nil
}

const getUserByIDSQL = `
SELECT id, name, email, password_hash, created_at, updated_at
FROM users
WHERE id = $1
`

func (q *dbAuthQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (UserWithAuth, error) {
	row := q.db.QueryRow(ctx, getUserByIDSQL, id)
	var u UserWithAuth
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return UserWithAuth{}, err
	}
	return u, nil
}

// Compile-time check.
var _ AuthQuerier = (*dbAuthQuerier)(nil)
