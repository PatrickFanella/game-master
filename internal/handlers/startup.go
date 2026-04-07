package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/auth"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/rules"
	"github.com/PatrickFanella/game-master/internal/world"
	"github.com/PatrickFanella/game-master/pkg/api"
)

type startupSessionStore struct {
	mu                 sync.RWMutex
	campaignInterviews map[string]*world.Interviewer
	characterSessions  map[string]*world.CharacterInterviewer
}

func newStartupSessionStore() *startupSessionStore {
	return &startupSessionStore{
		campaignInterviews: make(map[string]*world.Interviewer),
		characterSessions:  make(map[string]*world.CharacterInterviewer),
	}
}

func (s *startupSessionStore) createCampaignSession(iv *world.Interviewer) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID := uuid.NewString()
	s.campaignInterviews[sessionID] = iv
	return sessionID
}

func (s *startupSessionStore) campaignSession(id string) (*world.Interviewer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	iv, ok := s.campaignInterviews[id]
	return iv, ok
}

func (s *startupSessionStore) deleteCampaignSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.campaignInterviews, id)
}

func (s *startupSessionStore) createCharacterSession(iv *world.CharacterInterviewer) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID := uuid.NewString()
	s.characterSessions[sessionID] = iv
	return sessionID
}

func (s *startupSessionStore) characterSession(id string) (*world.CharacterInterviewer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	iv, ok := s.characterSessions[id]
	return iv, ok
}

func (s *startupSessionStore) deleteCharacterSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.characterSessions, id)
}

func (h *StartupHandlers) StartCampaignInterview(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.requireProvider(w)
	if !ok {
		return
	}

	iv := world.NewInterviewer(provider)
	sessionID := h.sessions.createCampaignSession(iv)
	result, err := iv.Start(r.Context())
	if err != nil {
		h.sessions.deleteCampaignSession(sessionID)
		h.Logger.Errorf("start campaign interview: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to start campaign interview")
		return
	}
	if result.Done {
		h.sessions.deleteCampaignSession(sessionID)
	}

	writeJSON(w, http.StatusOK, api.CampaignInterviewResponse{
		SessionID: sessionID,
		Message:   result.Message,
		Done:      result.Done,
		Profile:   result.Profile,
	})
}

func (h *StartupHandlers) StepCampaignInterview(w http.ResponseWriter, r *http.Request) {
	var req api.InterviewStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input := strings.TrimSpace(req.Input)
	if input == "" {
		writeError(w, http.StatusBadRequest, "input is required")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	iv, ok := h.sessions.campaignSession(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "campaign interview session not found")
		return
	}

	result, err := iv.Step(r.Context(), input)
	if err != nil {
		h.Logger.Errorf("step campaign interview %s: %v", sessionID, err)
		writeError(w, http.StatusInternalServerError, "failed to continue campaign interview")
		return
	}
	if result.Done {
		h.sessions.deleteCampaignSession(sessionID)
	}

	writeJSON(w, http.StatusOK, api.CampaignInterviewResponse{
		SessionID: sessionID,
		Message:   result.Message,
		Done:      result.Done,
		Profile:   result.Profile,
	})
}

func (h *StartupHandlers) GenerateCampaignProposals(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.requireProvider(w)
	if !ok {
		return
	}

	var req api.CampaignProposalsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	generator := world.NewProposalGenerator(provider)
	proposals, err := generator.Generate(r.Context(), req.Genre, req.SettingStyle, req.Tone)
	if err != nil {
		h.Logger.Errorf("generate campaign proposals: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate campaign proposals")
		return
	}

	resp := api.CampaignProposalsResponse{Proposals: make([]api.CampaignProposal, len(proposals))}
	for i, proposal := range proposals {
		resp.Proposals[i] = api.CampaignProposal{
			Name:    proposal.Name,
			Summary: proposal.Summary,
			Profile: proposal.Profile,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *StartupHandlers) GenerateCampaignName(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.requireProvider(w)
	if !ok {
		return
	}

	var req api.CampaignNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Profile == nil {
		writeError(w, http.StatusBadRequest, "profile is required")
		return
	}

	name, err := world.GenerateCampaignName(r.Context(), provider, req.Profile)
	if err != nil {
		h.Logger.Errorf("generate campaign name: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate campaign name")
		return
	}
	writeJSON(w, http.StatusOK, api.CampaignNameResponse{Name: name})
}

func (h *StartupHandlers) StartCharacterInterview(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.requireProvider(w)
	if !ok {
		return
	}

	var req api.CharacterInterviewStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CampaignProfile == nil {
		writeError(w, http.StatusBadRequest, "campaign_profile is required")
		return
	}

	iv := world.NewCharacterInterviewer(provider, req.CampaignProfile)
	sessionID := h.sessions.createCharacterSession(iv)
	result, err := iv.Start(r.Context())
	if err != nil {
		h.sessions.deleteCharacterSession(sessionID)
		h.Logger.Errorf("start character interview: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to start character interview")
		return
	}
	if result.Done {
		h.sessions.deleteCharacterSession(sessionID)
	}

	writeJSON(w, http.StatusOK, api.CharacterInterviewResponse{
		SessionID: sessionID,
		Message:   result.Message,
		Done:      result.Done,
		Profile:   result.Profile,
	})
}

func (h *StartupHandlers) StepCharacterInterview(w http.ResponseWriter, r *http.Request) {
	var req api.InterviewStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input := strings.TrimSpace(req.Input)
	if input == "" {
		writeError(w, http.StatusBadRequest, "input is required")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	iv, ok := h.sessions.characterSession(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "character interview session not found")
		return
	}

	result, err := iv.Step(r.Context(), input)
	if err != nil {
		h.Logger.Errorf("step character interview %s: %v", sessionID, err)
		writeError(w, http.StatusInternalServerError, "failed to continue character interview")
		return
	}
	if result.Done {
		h.sessions.deleteCharacterSession(sessionID)
	}

	writeJSON(w, http.StatusOK, api.CharacterInterviewResponse{
		SessionID: sessionID,
		Message:   result.Message,
		Done:      result.Done,
		Profile:   result.Profile,
	})
}

func (h *StartupHandlers) BuildWorld(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	provider, ok := h.requireProvider(w)
	if !ok {
		return
	}
	if h.Queries == nil {
		writeError(w, http.StatusInternalServerError, "startup world builder unavailable")
		return
	}

	var req api.WorldBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Profile == nil {
		writeError(w, http.StatusBadRequest, "profile is required")
		return
	}
	if req.CharacterProfile == nil {
		writeError(w, http.StatusBadRequest, "character_profile is required")
		return
	}

	result, err := world.NewOrchestrator(provider, h.Queries).Run(r.Context(), world.OrchestratorInput{
		Name:             strings.TrimSpace(req.Name),
		Summary:          strings.TrimSpace(req.Summary),
		Profile:          req.Profile,
		CharacterProfile: req.CharacterProfile,
		UserID:           userID,
	}, nil)
	if err != nil {
		h.Logger.Errorf("build startup world: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to build world")
		return
	}
	if result.Scene == nil {
		h.Logger.Errorf("build startup world: orchestrator returned nil scene")
		writeError(w, http.StatusInternalServerError, "failed to build world")
		return
	}

	campaignUUID := result.Campaign.ID

	// Initialize campaign time to Day 1, 08:00.
	if h.Pool != nil {
		_, _ = h.Pool.Exec(r.Context(),
			"INSERT INTO campaign_time (campaign_id, day, hour, minute) VALUES ($1, 1, 8, 0) ON CONFLICT (campaign_id) DO NOTHING",
			campaignUUID,
		)
	}

	// Apply rules_mode if provided in the request.
	rulesMode := strings.TrimSpace(req.RulesMode)
	if rulesMode != "" && h.Pool != nil {
		_, _ = h.Pool.Exec(r.Context(),
			"UPDATE campaigns SET rules_mode = $1 WHERE id = $2",
			rulesMode, campaignUUID,
		)

		// Seed default feats and skills for crunch mode.
		if rulesMode == "crunch" {
			cID := campaignUUID.Bytes
			if err := rules.SeedDefaultFeats(r.Context(), h.Pool, cID); err != nil {
				h.Logger.Errorf("seed default feats for %s: %v", cID, err)
			}
			if err := rules.SeedDefaultSkills(r.Context(), h.Pool, cID); err != nil {
				h.Logger.Errorf("seed default skills for %s: %v", cID, err)
			}
		}
	}

	writeJSON(w, http.StatusOK, api.WorldBuildResponse{
		Campaign: campaignToResponse(result.Campaign),
		OpeningScene: api.OpeningSceneResponse{
			Narrative: result.Scene.Narrative,
			Choices:   append([]string(nil), result.Scene.Choices...),
		},
	})
}

func (h *StartupHandlers) requireProvider(w http.ResponseWriter) (llm.Provider, bool) {
	if h.Provider == nil {
		writeError(w, http.StatusInternalServerError, "llm provider is not configured")
		return nil, false
	}
	return h.Provider, true
}
