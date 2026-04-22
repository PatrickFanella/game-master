package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type userIDContextKey struct{}

const AuthCookieName = "gm_token"

var errInvalidAuthorizationHeader = errors.New("invalid authorization header format")

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
		token, err := tokenFromRequest(r)
		if err != nil {
			if errors.Is(err, errInvalidAuthorizationHeader) {
				http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
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

func tokenFromRequest(r *http.Request) (string, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		token, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found || token == "" {
			return "", errInvalidAuthorizationHeader
		}
		return token, nil
	}

	if !isWebSocketUpgradeRequest(r) {
		return "", http.ErrNoCookie
	}

	cookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return "", http.ErrNoCookie
	}

	return token, nil
}

func isWebSocketUpgradeRequest(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
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
