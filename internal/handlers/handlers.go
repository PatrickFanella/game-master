package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/db"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// CampaignHandlers handles campaign CRUD and session history.
type CampaignHandlers struct {
	Engine  engine.GameEngine
	Queries statedb.Querier
	Logger  *log.Logger
	Pool    db.DBTX
}

// CharacterHandlers handles character data, feats, and skills.
type CharacterHandlers struct {
	Queries statedb.Querier
	Logger  *log.Logger
	Pool    db.DBTX
}

// WorldHandlers handles NPCs, locations, quests, facts, codex, relationships, and map.
type WorldHandlers struct {
	Queries statedb.Querier
	Logger  *log.Logger
}

// ActionHandlers handles turn processing and WebSocket connections.
type ActionHandlers struct {
	Engine   engine.GameEngine
	Queries  statedb.Querier
	Provider llm.Provider
	Logger   *log.Logger
}

// StartupHandlers handles campaign creation wizard (interviews, proposals, world build).
type StartupHandlers struct {
	Provider llm.Provider
	Queries  statedb.Querier
	Logger   *log.Logger
	Pool     db.DBTX
	sessions *startupSessionStore
}

// NewStartupHandlers creates a StartupHandlers with an initialized session store.
func NewStartupHandlers(provider llm.Provider, queries statedb.Querier, logger *log.Logger, pool db.DBTX) *StartupHandlers {
	if logger == nil {
		logger = log.Default()
	}
	return &StartupHandlers{
		Provider: provider,
		Queries:  queries,
		Logger:   logger,
		Pool:     pool,
		sessions: newStartupSessionStore(),
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Errorf("writeJSON encode: %v", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// campaignIDFromURL extracts and parses the campaign ID from the {id} URL parameter.
func campaignIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}
