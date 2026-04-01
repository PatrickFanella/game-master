package handlers

import (
	"fmt"
	"net/http"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// GetCharacter returns the player character for a campaign.
func (h *Handlers) GetCharacter(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	pcs, err := h.Queries.GetPlayerCharacterByCampaign(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		h.Logger.Errorf("get character for campaign %s: %v", campaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to get character")
		return
	}
	if len(pcs) == 0 {
		writeError(w, http.StatusNotFound, "no character found for campaign")
		return
	}

	writeJSON(w, http.StatusOK, playerCharacterToResponse(pcs[0]))
}

// GetCharacterInventory returns the inventory items for the player character.
func (h *Handlers) GetCharacterInventory(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	pcs, err := h.Queries.GetPlayerCharacterByCampaign(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil || len(pcs) == 0 {
		writeError(w, http.StatusNotFound, "no character found for campaign")
		return
	}

	items, err := h.Queries.ListItemsByPlayer(r.Context(), statedb.ListItemsByPlayerParams{
		CampaignID:        dbutil.ToPgtype(campaignID),
		PlayerCharacterID: pcs[0].ID,
	})
	if err != nil {
		h.Logger.Errorf("list inventory for campaign %s: %v", campaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to list inventory")
		return
	}

	resp := make([]api.ItemResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, itemToResponse(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetCharacterAbilities returns the abilities for the player character.
func (h *Handlers) GetCharacterAbilities(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	pcs, err := h.Queries.GetPlayerCharacterByCampaign(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil || len(pcs) == 0 {
		writeError(w, http.StatusNotFound, "no character found for campaign")
		return
	}

	writeJSON(w, http.StatusOK, parseAbilities(pcs[0].Abilities))
}
