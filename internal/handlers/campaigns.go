package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/auth"
	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// ListCampaigns returns all campaigns owned by the authenticated user.
func (h *Handlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	campaigns, err := h.Queries.ListCampaignsByUser(r.Context(), dbutil.ToPgtype(userID))
	if err != nil {
		h.Logger.Errorf("list campaigns: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list campaigns")
		return
	}

	resp := api.CampaignListResponse{
		Campaigns: make([]api.CampaignResponse, 0, len(campaigns)),
	}
	for _, c := range campaigns {
		resp.Campaigns = append(resp.Campaigns, campaignToResponse(c))
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateCampaign creates a new campaign for the authenticated user.
func (h *Handlers) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req api.CampaignCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	themes := req.Themes
	if themes == nil {
		themes = []string{}
	}

	campaign, err := h.Queries.CreateCampaign(r.Context(), statedb.CreateCampaignParams{
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Genre:       pgtype.Text{String: req.Genre, Valid: req.Genre != ""},
		Tone:        pgtype.Text{String: req.Tone, Valid: req.Tone != ""},
		Themes:      themes,
		Status:      "active",
		CreatedBy:   dbutil.ToPgtype(userID),
	})
	if err != nil {
		h.Logger.Errorf("create campaign: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create campaign")
		return
	}

	// Set rules_mode if provided (not in sqlc-generated INSERT params, use raw SQL).
	rulesMode := req.RulesMode
	if rulesMode != "" && h.Pool != nil {
		_, _ = h.Pool.Exec(r.Context(),
			"UPDATE campaigns SET rules_mode = $1 WHERE id = $2",
			rulesMode, campaign.ID,
		)
		campaign.RulesMode = pgtype.Text{String: rulesMode, Valid: true}
	}

	writeJSON(w, http.StatusCreated, campaignToResponse(campaign))
}

// GetCampaign returns a single campaign by ID.
func (h *Handlers) GetCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	campaign, err := h.Queries.GetCampaignByID(r.Context(), dbutil.ToPgtype(id))
	if err != nil {
		// pgx returns no rows as an error; treat as 404.
		h.Logger.Debugf("get campaign %s: %v", id, err)
		writeError(w, http.StatusNotFound, "campaign not found")
		return
	}

	writeJSON(w, http.StatusOK, campaignToResponse(campaign))
}

// UpdateCampaign updates an existing campaign.
func (h *Handlers) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	var req api.CampaignCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	themes := req.Themes
	if themes == nil {
		themes = []string{}
	}

	campaign, err := h.Queries.UpdateCampaign(r.Context(), statedb.UpdateCampaignParams{
		ID:          dbutil.ToPgtype(id),
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Genre:       pgtype.Text{String: req.Genre, Valid: req.Genre != ""},
		Tone:        pgtype.Text{String: req.Tone, Valid: req.Tone != ""},
		Themes:      themes,
	})
	if err != nil {
		h.Logger.Errorf("update campaign %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to update campaign")
		return
	}

	writeJSON(w, http.StatusOK, campaignToResponse(campaign))
}

// GetSessionHistory returns the turn history for a campaign.
func (h *Handlers) GetSessionHistory(w http.ResponseWriter, r *http.Request) {
	id, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	logs, err := h.Queries.ListSessionLogsByCampaign(r.Context(), dbutil.ToPgtype(id))
	if err != nil {
		h.Logger.Errorf("list session logs for campaign %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to list session history")
		return
	}

	entries := make([]api.SessionLogEntry, 0, len(logs))
	for _, sl := range logs {
		entries = append(entries, sessionLogToEntry(sl))
	}
	writeJSON(w, http.StatusOK, api.SessionHistoryResponse{Entries: entries})
}

// DeleteCampaign deletes a campaign by ID.
func (h *Handlers) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	if err := h.Queries.DeleteCampaign(r.Context(), dbutil.ToPgtype(id)); err != nil {
		h.Logger.Errorf("delete campaign %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to delete campaign")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
