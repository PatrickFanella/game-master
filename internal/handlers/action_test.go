package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/auth"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// stubEngine implements engine.GameEngine for testing.
type stubEngine struct {
	turnResult *engine.TurnResult
	turnErr    error
}

func (s *stubEngine) ProcessTurn(_ context.Context, _ uuid.UUID, _ string) (*engine.TurnResult, error) {
	if s.turnErr != nil {
		return nil, s.turnErr
	}
	return s.turnResult, nil
}

func (s *stubEngine) GetGameState(_ context.Context, _ uuid.UUID) (*engine.GameState, error) {
	return nil, nil
}

func (s *stubEngine) NewCampaign(_ context.Context, _ uuid.UUID) (*domain.Campaign, error) {
	return nil, nil
}

func (s *stubEngine) LoadCampaign(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *stubEngine) ProcessTurnStream(_ context.Context, _ uuid.UUID, _ string) (<-chan engine.StreamEvent, error) {
	ch := make(chan engine.StreamEvent)
	close(ch)
	return ch, nil
}

func newActionRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	authMW := auth.NewNoOpMiddleware(uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	r.Use(authMW.Authenticate)
	r.Route("/campaigns/{id}", func(r chi.Router) {
		r.Post("/action", h.ProcessAction)
	})
	return r
}

func TestProcessAction_Success(t *testing.T) {
	eng := &stubEngine{
		turnResult: &engine.TurnResult{
			Narrative: "You enter the dark forest.",
		},
	}
	h := New(eng, nil, log.Default())
	router := newActionRouter(h)

	campaignID := uuid.New().String()
	body, _ := json.Marshal(api.ActionRequest{Input: "go north"})
	req := httptest.NewRequest(http.MethodPost, "/campaigns/"+campaignID+"/action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp api.TurnResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Narrative != "You enter the dark forest." {
		t.Errorf("expected narrative %q, got %q", "You enter the dark forest.", resp.Narrative)
	}
}

func TestProcessAction_EmptyInput(t *testing.T) {
	h := New(&stubEngine{}, nil, log.Default())
	router := newActionRouter(h)

	campaignID := uuid.New().String()
	body, _ := json.Marshal(api.ActionRequest{Input: ""})
	req := httptest.NewRequest(http.MethodPost, "/campaigns/"+campaignID+"/action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "input is required" {
		t.Errorf("expected error %q, got %q", "input is required", resp["error"])
	}
}

func TestProcessAction_InvalidCampaignID(t *testing.T) {
	h := New(&stubEngine{}, nil, log.Default())
	router := newActionRouter(h)

	body, _ := json.Marshal(api.ActionRequest{Input: "look around"})
	req := httptest.NewRequest(http.MethodPost, "/campaigns/not-a-uuid/action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessAction_EngineError(t *testing.T) {
	eng := &stubEngine{turnErr: errors.New("llm timeout")}
	h := New(eng, nil, log.Default())
	router := newActionRouter(h)

	campaignID := uuid.New().String()
	body, _ := json.Marshal(api.ActionRequest{Input: "attack dragon"})
	req := httptest.NewRequest(http.MethodPost, "/campaigns/"+campaignID+"/action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}
