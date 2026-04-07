package handlers

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/pkg/api"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// GetMapData returns player-known/visited locations with connections.
func (h *WorldHandlers) GetMapData(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pgCampaignID := dbutil.ToPgtype(campaignID)

	// Get all player-known locations (includes visited)
	locations, err := h.Queries.ListPlayerKnownLocations(r.Context(), pgCampaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list map locations")
		return
	}

	// Also include visited locations not yet marked as known
	visited, err := h.Queries.ListPlayerVisitedLocations(r.Context(), pgCampaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list visited locations")
		return
	}

	// Merge: deduplicate by ID
	seen := make(map[uuid.UUID]struct{}, len(locations))
	mapLocs := make([]api.MapLocationResponse, 0, len(locations)+len(visited))
	for _, loc := range locations {
		id := dbutil.FromPgtype(loc.ID)
		seen[id] = struct{}{}
		mapLocs = append(mapLocs, mapLocationToResponse(loc))
	}
	for _, loc := range visited {
		id := dbutil.FromPgtype(loc.ID)
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			mapLocs = append(mapLocs, mapLocationToResponse(loc))
		}
	}

	// Get connections between known locations
	var conns []api.LocationConnectionResponse
	for _, loc := range mapLocs {
		locID := uuid.MustParse(loc.ID)
		locConns, connErr := h.Queries.GetConnectionsFromLocation(r.Context(), statedb.GetConnectionsFromLocationParams{
			LocationID: dbutil.ToPgtype(locID),
			CampaignID: pgCampaignID,
		})
		if connErr != nil {
			continue
		}
		for _, c := range locConns {
			targetID := dbutil.FromPgtype(c.ConnectedLocationID)
			if _, ok := seen[targetID]; ok {
				conns = append(conns, api.LocationConnectionResponse{
					FromLocationID: loc.ID,
					ToLocationID:   targetID.String(),
					Description:    c.Description.String,
					Bidirectional:  c.Bidirectional,
					TravelTime:     c.TravelTime.String,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, api.MapDataResponse{
		Locations:   mapLocs,
		Connections: conns,
	})
}
