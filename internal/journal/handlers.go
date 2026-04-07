package journal

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handlers provides HTTP handlers for session summaries and journal entries.
type Handlers struct {
	store      *Store
	summarizer *Summarizer
}

// NewHandlers creates Handlers backed by the given store.
func NewHandlers(store *Store) *Handlers {
	return &Handlers{store: store}
}

// NewHandlersWithSummarizer creates Handlers with an optional summarizer for manual summarize endpoints.
func NewHandlersWithSummarizer(store *Store, summarizer *Summarizer) *Handlers {
	return &Handlers{store: store, summarizer: summarizer}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func campaignIDFromURL(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

type summaryJSON struct {
	ID         string `json:"id"`
	CampaignID string `json:"campaign_id"`
	FromTurn   int    `json:"from_turn"`
	ToTurn     int    `json:"to_turn"`
	Summary    string `json:"summary"`
	CreatedAt  string `json:"created_at"`
}

func summaryToJSON(s Summary) summaryJSON {
	return summaryJSON{
		ID:         s.ID.String(),
		CampaignID: s.CampaignID.String(),
		FromTurn:   s.FromTurn,
		ToTurn:     s.ToTurn,
		Summary:    s.Summary,
		CreatedAt:  s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type entryJSON struct {
	ID         string `json:"id"`
	CampaignID string `json:"campaign_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func entryToJSON(e Entry) entryJSON {
	return entryJSON{
		ID:         e.ID.String(),
		CampaignID: e.CampaignID.String(),
		Title:      e.Title,
		Content:    e.Content,
		CreatedAt:  e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ListSummaries handles GET /api/v1/campaigns/{id}/journal/summaries.
func (h *Handlers) ListSummaries(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

	summaries, err := h.store.ListSummaries(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list summaries")
		return
	}

	result := make([]summaryJSON, 0, len(summaries))
	for _, s := range summaries {
		result = append(result, summaryToJSON(s))
	}
	writeJSON(w, http.StatusOK, result)
}

// ListEntries handles GET /api/v1/campaigns/{id}/journal/entries.
func (h *Handlers) ListEntries(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

	entries, err := h.store.ListEntries(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list journal entries")
		return
	}

	result := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		result = append(result, entryToJSON(e))
	}
	writeJSON(w, http.StatusOK, result)
}

type createEntryRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// CreateEntry handles POST /api/v1/campaigns/{id}/journal/entries.
func (h *Handlers) CreateEntry(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

	var req createEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	entry, err := h.store.CreateEntry(r.Context(), campaignID, req.Title, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create journal entry")
		return
	}

	writeJSON(w, http.StatusCreated, entryToJSON(entry))
}

// DeleteEntry handles DELETE /api/v1/campaigns/{id}/journal/entries/{eid}.
func (h *Handlers) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := uuid.Parse(chi.URLParam(r, "eid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entry id")
		return
	}

	if err := h.store.DeleteEntry(r.Context(), entryID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete journal entry")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type summarizeRequest struct {
	FromTurn *int `json:"from_turn"`
	ToTurn   *int `json:"to_turn"`
}

// Summarize handles POST /api/v1/campaigns/{id}/journal/summarize.
func (h *Handlers) Summarize(w http.ResponseWriter, r *http.Request) {
	if h.summarizer == nil {
		writeError(w, http.StatusServiceUnavailable, "summarization is not available")
		return
	}

	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

	var req summarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body — defaults to summarizing all unsummarized turns.
		req = summarizeRequest{}
	}

	var summary *Summary
	if req.FromTurn != nil && req.ToTurn != nil {
		summary, err = h.summarizer.Summarize(r.Context(), campaignID, *req.FromTurn, *req.ToTurn)
	} else {
		summary, err = h.summarizer.SummarizeUnsummarized(r.Context(), campaignID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate summary")
		return
	}

	if summary == nil {
		writeJSON(w, http.StatusOK, map[string]string{"message": "no unsummarized turns found"})
		return
	}

	writeJSON(w, http.StatusCreated, summaryToJSON(*summary))
}
