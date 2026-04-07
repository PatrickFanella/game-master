package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// AuthQuerier defines the database methods required by auth handlers.
// This is satisfied by the generated statedb.Queries after running sqlc generate.
type AuthQuerier interface {
	CreateUserWithAuth(ctx context.Context, arg CreateUserWithAuthParams) (UserWithAuth, error)
	GetUserByEmail(ctx context.Context, email string) (UserWithAuth, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (UserWithAuth, error)
}

// CreateUserWithAuthParams holds the parameters for creating an authenticated user.
type CreateUserWithAuthParams struct {
	Name         string
	Email        string
	PasswordHash string
}

// UserWithAuth represents a user row including auth fields.
type UserWithAuth struct {
	ID           pgtype.UUID
	Name         string
	Email        pgtype.Text
	PasswordHash pgtype.Text
	CreatedAt    pgtype.Timestamptz
	UpdatedAt    pgtype.Timestamptz
}

// AuthHandlers provides HTTP handlers for authentication.
type AuthHandlers struct {
	queries   AuthQuerier
	jwtSecret string
}

// NewAuthHandlers creates a new AuthHandlers.
func NewAuthHandlers(queries AuthQuerier, jwtSecret string) *AuthHandlers {
	return &AuthHandlers{queries: queries, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string   `json:"token"`
	User  userJSON `json:"user"`
}

type userJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.queries.CreateUserWithAuth(r.Context(), CreateUserWithAuthParams{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
	})
	if err != nil {
		// Check for unique constraint violation on email.
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	uid := uuidFromPgtype(user.ID)
	token, err := GenerateToken(uid, h.jwtSecret, DefaultTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		Token: token,
		User: userJSON{
			ID:    uid.String(),
			Name:  user.Name,
			Email: user.Email.String,
		},
	})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to query user")
		return
	}

	if !user.PasswordHash.Valid || !CheckPassword(req.Password, user.PasswordHash.String) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	uid := uuidFromPgtype(user.ID)
	token, err := GenerateToken(uid, h.jwtSecret, DefaultTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token: token,
		User: userJSON{
			ID:    uid.String(),
			Name:  user.Name,
			Email: user.Email.String,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Me handles GET /api/v1/auth/me — returns the current authenticated user.
func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var pgID pgtype.UUID
	pgID.Bytes = userID
	pgID.Valid = true

	user, err := h.queries.GetUserByID(r.Context(), pgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		User: userJSON{
			ID:    uuidFromPgtype(user.ID).String(),
			Name:  user.Name,
			Email: user.Email.String,
		},
	})
}

func uuidFromPgtype(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}
