package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/pkg/api"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// ListEncounteredNPCs returns NPCs that appear in session logs (npcs_involved).
func (h *WorldHandlers) ListEncounteredNPCs(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pgCampaignID := dbutil.ToPgtype(campaignID)

	// Get all session logs and collect unique NPC IDs
	logs, err := h.Queries.ListSessionLogsByCampaign(r.Context(), pgCampaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list session logs")
		return
	}

	seen := make(map[uuid.UUID]struct{})
	for _, log := range logs {
		for _, npcID := range log.NpcsInvolved {
			if npcID.Valid {
				seen[dbutil.FromPgtype(npcID)] = struct{}{}
			}
		}
	}

	// Fetch NPC details for each encountered NPC
	resp := make([]api.EncounteredNPCResponse, 0, len(seen))
	for npcID := range seen {
		npc, npcErr := h.Queries.GetNPCByID(r.Context(), statedb.GetNPCByIDParams{
			ID:         dbutil.ToPgtype(npcID),
			CampaignID: pgCampaignID,
		})
		if npcErr != nil {
			continue
		}
		var factionID *string
		if npc.FactionID.Valid {
			s := dbutil.FromPgtype(npc.FactionID).String()
			factionID = &s
		}
		disposition := int(npc.Disposition)
		resp = append(resp, api.EncounteredNPCResponse{
			ID:          dbutil.FromPgtype(npc.ID).String(),
			CampaignID:  dbutil.FromPgtype(npc.CampaignID).String(),
			Name:        npc.Name,
			Description: npc.Description.String,
			Disposition: &disposition,
			Alive:       npc.Alive,
			FactionID:   factionID,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetNPCDialogue returns session log entries that involved a specific NPC.
func (h *WorldHandlers) GetNPCDialogue(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	npcIDStr := chi.URLParam(r, "nid")
	npcID, err := uuid.Parse(npcIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid npc id: %v", err))
		return
	}

	logs, err := h.Queries.ListSessionLogsByCampaign(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list session logs")
		return
	}

	pgNPCID := dbutil.ToPgtype(npcID)
	resp := make([]api.DialogueEntry, 0)
	for _, log := range logs {
		for _, involved := range log.NpcsInvolved {
			if involved == pgNPCID {
				resp = append(resp, api.DialogueEntry{
					TurnNumber:  int(log.TurnNumber),
					PlayerInput: log.PlayerInput,
					LLMResponse: log.LlmResponse,
					CreatedAt:   log.CreatedAt.Time.Format(time.RFC3339),
				})
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
