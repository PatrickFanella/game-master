package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/PatrickFanella/game-master/pkg/api"
)

// ProcessAction handles a player turn action.
func (h *ActionHandlers) ProcessAction(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	var req api.ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Input == "" {
		writeError(w, http.StatusBadRequest, "input is required")
		return
	}

	result, err := h.Engine.ProcessTurn(r.Context(), campaignID, req.Input)
	if err != nil {
		h.Logger.Errorf("process turn for campaign %s: %v", campaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to process turn")
		return
	}

	writeJSON(w, http.StatusOK, engineTurnResultToAPI(result))
}
