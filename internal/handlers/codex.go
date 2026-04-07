package handlers

import (
	"net/http"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// ListKnownLanguages returns player-known languages.
func (h *WorldHandlers) ListKnownLanguages(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	langs, err := h.Queries.ListPlayerKnownLanguages(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list languages")
		return
	}
	resp := make([]api.LanguageResponse, 0, len(langs))
	for _, l := range langs {
		resp = append(resp, languageToResponse(l))
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListKnownCultures returns player-known cultures.
func (h *WorldHandlers) ListKnownCultures(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cultures, err := h.Queries.ListPlayerKnownCultures(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cultures")
		return
	}
	resp := make([]api.CultureResponse, 0, len(cultures))
	for _, c := range cultures {
		resp = append(resp, cultureToResponse(c))
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListKnownBeliefSystems returns player-known belief systems.
func (h *WorldHandlers) ListKnownBeliefSystems(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	beliefs, err := h.Queries.ListPlayerKnownBeliefSystems(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list belief systems")
		return
	}
	resp := make([]api.BeliefSystemResponse, 0, len(beliefs))
	for _, b := range beliefs {
		resp = append(resp, beliefSystemToResponse(b))
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListKnownEconomicSystems returns player-known economic systems.
func (h *WorldHandlers) ListKnownEconomicSystems(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	systems, err := h.Queries.ListPlayerKnownEconomicSystems(r.Context(), dbutil.ToPgtype(campaignID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list economic systems")
		return
	}
	resp := make([]api.EconomicSystemResponse, 0, len(systems))
	for _, s := range systems {
		resp = append(resp, economicSystemToResponse(s))
	}
	writeJSON(w, http.StatusOK, resp)
}
