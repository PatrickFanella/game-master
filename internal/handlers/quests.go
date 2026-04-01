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

// ListQuests returns quests for a campaign, optionally filtered by type or status.
func (h *Handlers) ListQuests(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	pgCampaignID := dbutil.ToPgtype(campaignID)
	questType := r.URL.Query().Get("type")
	statusFilter := r.URL.Query().Get("status")

	var quests []statedb.Quest

	switch {
	case questType != "":
		quests, err = h.Queries.ListQuestsByType(r.Context(), statedb.ListQuestsByTypeParams{
			CampaignID: pgCampaignID,
			QuestType:  questType,
		})
	case statusFilter == "active":
		quests, err = h.Queries.ListActiveQuests(r.Context(), pgCampaignID)
	default:
		quests, err = h.Queries.ListQuestsByCampaign(r.Context(), pgCampaignID)
	}
	if err != nil {
		h.Logger.Errorf("list quests for campaign %s: %v", campaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to list quests")
		return
	}

	resp := make([]api.QuestResponse, 0, len(quests))
	for _, q := range quests {
		resp = append(resp, questToResponse(q, nil))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetQuest returns a single quest with its objectives.
func (h *Handlers) GetQuest(w http.ResponseWriter, r *http.Request) {
	_, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	qid, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid quest id: %v", err))
		return
	}

	pgQID := dbutil.ToPgtype(qid)

	quest, err := h.Queries.GetQuestByID(r.Context(), pgQID)
	if err != nil {
		h.Logger.Debugf("get quest %s: %v", qid, err)
		writeError(w, http.StatusNotFound, "quest not found")
		return
	}

	objs, err := h.Queries.ListObjectivesByQuest(r.Context(), pgQID)
	if err != nil {
		h.Logger.Errorf("list objectives for quest %s: %v", qid, err)
		objs = nil
	}

	writeJSON(w, http.StatusOK, questToResponse(quest, objs))
}
