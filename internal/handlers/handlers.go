package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/engine"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	Engine  engine.GameEngine
	Queries statedb.Querier
	Logger  *log.Logger
}

// New creates a Handlers with the given dependencies.
func New(eng engine.GameEngine, queries statedb.Querier, logger *log.Logger) *Handlers {
	if logger == nil {
		logger = log.Default()
	}
	return &Handlers{Engine: eng, Queries: queries, Logger: logger}
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
