package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/config"
)

func TestParseConfigPath(t *testing.T) {
	t.Parallel()

	defaultPath := "/tmp/default.yaml"
	got, err := parseConfigPath(nil, defaultPath)
	if err != nil {
		t.Fatalf("parseConfigPath() error = %v", err)
	}
	if got != defaultPath {
		t.Fatalf("parseConfigPath() = %q, want %q", got, defaultPath)
	}

	overridePath := "/tmp/override.yaml"
	got, err = parseConfigPath([]string{"--config", overridePath}, defaultPath)
	if err != nil {
		t.Fatalf("parseConfigPath() error = %v", err)
	}
	if got != overridePath {
		t.Fatalf("parseConfigPath() = %q, want %q", got, overridePath)
	}
}

func TestParseConfigPathUnknownFlag(t *testing.T) {
	t.Parallel()

	if _, err := parseConfigPath([]string{"--unknown"}, ""); err == nil {
		t.Fatal("parseConfigPath() expected error for unknown flag, got nil")
	}
}

func TestNewRouterHealthAndAPIGroups(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard)
	router := newRouterWithProvider(logger, nil, nil, nil, nil, uuid.Nil, config.Config{})

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRes := httptest.NewRecorder()
	router.ServeHTTP(healthRes, healthReq)
	if healthRes.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", healthRes.Code, http.StatusOK)
	}
	if strings.TrimSpace(healthRes.Body.String()) != "ok" {
		t.Fatalf("GET /healthz body = %q, want %q", healthRes.Body.String(), "ok")
	}

	apiHealthReq := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	apiHealthRes := httptest.NewRecorder()
	router.ServeHTTP(apiHealthRes, apiHealthReq)
	if apiHealthRes.Code != http.StatusOK {
		t.Fatalf("GET /api/healthz status = %d, want %d", apiHealthRes.Code, http.StatusOK)
	}
	if !strings.Contains(apiHealthRes.Body.String(), `"status":"ok"`) {
		t.Fatalf("GET /api/healthz body = %q, want JSON status", apiHealthRes.Body.String())
	}

	// With real handlers and nil dependencies, campaigns returns an auth error or server error.
	// Just verify the route exists and doesn't panic.
	campaignReq := httptest.NewRequest(http.MethodGet, "/api/v1/campaigns", nil)
	campaignRes := httptest.NewRecorder()
	router.ServeHTTP(campaignRes, campaignReq)
	// Accept any non-404 status (route is registered).
	if campaignRes.Code == http.StatusNotFound {
		t.Fatalf("GET /api/v1/campaigns status = %d, want route to exist", campaignRes.Code)
	}
}

func TestNewRouterRecovererAndCORS(t *testing.T) {
	t.Parallel()

	logger := log.New(io.Discard)
	router := newRouterWithProvider(logger, nil, nil, nil, nil, uuid.Nil, config.Config{})

	mux, ok := router.(*chi.Mux)
	if !ok {
		t.Fatalf("newRouter() type = %T, want *chi.Mux", router)
	}
	mux.Get("/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})

	panicReq := httptest.NewRequest(http.MethodGet, "/panic", nil)
	panicRes := httptest.NewRecorder()
	router.ServeHTTP(panicRes, panicReq)
	if panicRes.Code != http.StatusInternalServerError {
		t.Fatalf("GET /panic status = %d, want %d", panicRes.Code, http.StatusInternalServerError)
	}

	preflightReq := httptest.NewRequest(http.MethodOptions, "/api/v1/campaigns/", nil)
	preflightReq.Header.Set("Origin", "http://localhost:3000")
	preflightReq.Header.Set("Access-Control-Request-Method", http.MethodGet)
	preflightRes := httptest.NewRecorder()
	router.ServeHTTP(preflightRes, preflightReq)
	if got := preflightRes.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("OPTIONS allow-origin = %q, want %q", got, "http://localhost:3000")
	}

	reqIDReq := httptest.NewRequest(http.MethodGet, "/request-id", nil)
	reqIDRes := httptest.NewRecorder()
	mux.Get("/request-id", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(middleware.GetReqID(r.Context())))
	})
	router.ServeHTTP(reqIDRes, reqIDReq)
	if strings.TrimSpace(reqIDRes.Body.String()) == "" {
		t.Fatal("request ID middleware did not populate request context")
	}
}
