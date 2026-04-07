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

// ListLocations returns all locations for a campaign.
func (h *WorldHandlers) ListLocations(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	locations, err := h.Queries.ListLocationsByCampaign(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		h.Logger.Errorf("list locations for campaign %s: %v", campaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to list locations")
		return
	}

	resp := make([]api.LocationResponse, 0, len(locations))
	for _, l := range locations {
		resp = append(resp, locationToResponse(l, nil))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetLocation returns a single location with its connections.
func (h *WorldHandlers) GetLocation(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	lid, err := uuid.Parse(chi.URLParam(r, "lid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid location id: %v", err))
		return
	}

	location, err := h.Queries.GetLocationByID(r.Context(), statedb.GetLocationByIDParams{
		ID:         dbutil.ToPgtype(lid),
		CampaignID: dbutil.ToPgtype(campaignID),
	})
	if err != nil {
		h.Logger.Debugf("get location %s: %v", lid, err)
		writeError(w, http.StatusNotFound, "location not found")
		return
	}

	conns, err := h.Queries.GetConnectionsFromLocation(r.Context(), statedb.GetConnectionsFromLocationParams{
		CampaignID: dbutil.ToPgtype(campaignID),
		LocationID: dbutil.ToPgtype(lid),
	})
	if err != nil {
		h.Logger.Errorf("get connections for location %s: %v", lid, err)
		// Non-fatal: return location without connections.
		conns = nil
	}

	writeJSON(w, http.StatusOK, locationToResponse(location, conns))
}
