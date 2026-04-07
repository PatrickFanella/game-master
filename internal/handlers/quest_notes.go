package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// ListQuestNotes returns all notes for a quest.
func (h *WorldHandlers) ListQuestNotes(w http.ResponseWriter, r *http.Request) {
	questID, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid quest id: %v", err))
		return
	}
	notes, err := h.Queries.ListQuestNotes(r.Context(), dbutil.ToPgtype(questID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list quest notes")
		return
	}
	resp := make([]api.QuestNoteResponse, 0, len(notes))
	for _, n := range notes {
		resp = append(resp, api.QuestNoteResponse{
			ID:        dbutil.FromPgtype(n.ID).String(),
			QuestID:   dbutil.FromPgtype(n.QuestID).String(),
			Content:   n.Content,
			CreatedAt: n.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt: n.UpdatedAt.Time.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateQuestNote adds a new note to a quest.
func (h *WorldHandlers) CreateQuestNote(w http.ResponseWriter, r *http.Request) {
	questID, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid quest id: %v", err))
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	note, err := h.Queries.CreateQuestNote(r.Context(), statedb.CreateQuestNoteParams{
		QuestID: dbutil.ToPgtype(questID),
		Content: body.Content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create quest note")
		return
	}

	writeJSON(w, http.StatusCreated, api.QuestNoteResponse{
		ID:        dbutil.FromPgtype(note.ID).String(),
		QuestID:   dbutil.FromPgtype(note.QuestID).String(),
		Content:   note.Content,
		CreatedAt: note.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt: note.UpdatedAt.Time.Format(time.RFC3339),
	})
}

// DeleteQuestNote removes a note from a quest.
func (h *WorldHandlers) DeleteQuestNote(w http.ResponseWriter, r *http.Request) {
	questID, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid quest id: %v", err))
		return
	}
	noteID, err := uuid.Parse(chi.URLParam(r, "noteID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid note id: %v", err))
		return
	}

	if err := h.Queries.DeleteQuestNote(r.Context(), statedb.DeleteQuestNoteParams{
		ID:      dbutil.ToPgtype(noteID),
		QuestID: dbutil.ToPgtype(questID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete quest note")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListQuestHistory returns state snapshots for a quest.
func (h *WorldHandlers) ListQuestHistory(w http.ResponseWriter, r *http.Request) {
	questID, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid quest id: %v", err))
		return
	}
	entries, err := h.Queries.ListQuestHistory(r.Context(), dbutil.ToPgtype(questID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list quest history")
		return
	}
	resp := make([]api.QuestHistoryEntry, 0, len(entries))
	for _, e := range entries {
		resp = append(resp, api.QuestHistoryEntry{
			ID:        dbutil.FromPgtype(e.ID).String(),
			QuestID:   dbutil.FromPgtype(e.QuestID).String(),
			Snapshot:  e.Snapshot,
			CreatedAt: e.CreatedAt.Time.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
