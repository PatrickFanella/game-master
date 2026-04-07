package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// ListNPCs returns all NPCs for a campaign.
func (h *WorldHandlers) ListNPCs(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	npcs, err := h.Queries.ListNPCsByCampaign(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		h.Logger.Errorf("list npcs for campaign %s: %v", campaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to list npcs")
		return
	}

	resp := make([]api.NPCResponse, 0, len(npcs))
	for _, n := range npcs {
		resp = append(resp, npcToResponse(n))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetNPC returns a single NPC by ID.
func (h *WorldHandlers) GetNPC(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	nid, err := uuid.Parse(chi.URLParam(r, "nid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid npc id: %v", err))
		return
	}

	npc, err := h.Queries.GetNPCByID(r.Context(), statedb.GetNPCByIDParams{
		ID:         dbutil.ToPgtype(nid),
		CampaignID: dbutil.ToPgtype(campaignID),
	})
	if err != nil {
		h.Logger.Debugf("get npc %s: %v", nid, err)
		writeError(w, http.StatusNotFound, "npc not found")
		return
	}

	writeJSON(w, http.StatusOK, npcToResponse(npc))
}
