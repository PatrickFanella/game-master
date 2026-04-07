package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type userIDContextKey struct{}

// AuthMiddleware defines middleware that authenticates a request and enriches context.
type AuthMiddleware interface {
	Authenticate(next http.Handler) http.Handler
}

// ---------------------------------------------------------------------------
// NoOpMiddleware — injects a hardcoded default user (used by TUI)
// ---------------------------------------------------------------------------

// NoOpMiddleware injects a default user ID into every request context.
type NoOpMiddleware struct {
	defaultUserID uuid.UUID
}

// NewNoOpMiddleware creates an auth middleware that always authenticates as defaultUserID.
func NewNoOpMiddleware(defaultUserID uuid.UUID) AuthMiddleware {
	return &NoOpMiddleware{defaultUserID: defaultUserID}
}

// Authenticate sets the default user ID in request context and passes through.
func (m *NoOpMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userIDContextKey{}, m.defaultUserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ---------------------------------------------------------------------------
// JWTMiddleware — validates Bearer tokens from the Authorization header
// ---------------------------------------------------------------------------

// JWTMiddleware validates JWT tokens and injects the user ID into context.
type JWTMiddleware struct {
	secret string
}

// NewJWTMiddleware creates an auth middleware that validates JWT Bearer tokens.
func NewJWTMiddleware(secret string) AuthMiddleware {
	return &JWTMiddleware{secret: secret}
}

// Authenticate reads the Authorization header, validates the JWT, and sets
// the user ID in request context. Returns 401 if the token is missing or invalid.
func (m *JWTMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		token, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found || token == "" {
			http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		userID, err := ValidateToken(token, m.secret)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

// UserFromContext extracts an authenticated user ID from context.
// A uuid.Nil value is treated as unauthenticated and returns ok=false.
func UserFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	userID, ok := ctx.Value(userIDContextKey{}).(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, false
	}
	return userID, true
}
