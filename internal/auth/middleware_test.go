package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestNoOpMiddleware_AuthenticateSetsDefaultUser(t *testing.T) {
	defaultUserID := uuid.New()
	mw := NewNoOpMiddleware(defaultUserID)

	var gotUserID uuid.UUID
	var got bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, got = UserFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.Authenticate(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !got {
		t.Fatalf("expected user ID in context")
	}
	if gotUserID != defaultUserID {
		t.Fatalf("user ID = %s, want %s", gotUserID, defaultUserID)
	}
}

func TestUserFromContext_NoValue(t *testing.T) {
	userID, ok := UserFromContext(context.Background())
	if ok {
		t.Fatalf("expected no user ID")
	}
	if userID != uuid.Nil {
		t.Fatalf("user ID = %s, want nil UUID", userID)
	}

	userID, ok = UserFromContext(context.Background())
	if ok {
		t.Fatalf("expected no user ID from empty context")
	}
	if userID != uuid.Nil {
		t.Fatalf("user ID = %s, want nil UUID", userID)
	}
}
