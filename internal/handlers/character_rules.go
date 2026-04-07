package handlers

import (
	"fmt"
	"net/http"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// GetCharacterFeats returns the feats granted to the player character in a campaign.
func (h *CharacterHandlers) GetCharacterFeats(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}
	if h.Pool == nil {
		writeError(w, http.StatusInternalServerError, "database pool unavailable")
		return
	}

	pgCampaignID := dbutil.ToPgtype(campaignID)

	// Get the player character for this campaign.
	pcs, err := h.Queries.GetPlayerCharacterByCampaign(r.Context(), pgCampaignID)
	if err != nil || len(pcs) == 0 {
		writeError(w, http.StatusNotFound, "player character not found")
		return
	}
	characterID := dbutil.FromPgtype(pcs[0].ID)

	rows, err := h.Pool.Query(r.Context(),
		`SELECT cf.id, cf.feat_id, fd.name, fd.description, fd.bonus_type, fd.bonus_value, fd.prerequisites
		 FROM character_feats cf
		 JOIN feat_definitions fd ON fd.id = cf.feat_id
		 WHERE cf.character_id = $1
		 ORDER BY fd.name`,
		characterID,
	)
	if err != nil {
		h.Logger.Errorf("get character feats: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list character feats")
		return
	}
	defer rows.Close()

	feats := make([]api.FeatResponse, 0)
	for rows.Next() {
		var f api.FeatResponse
		if err := rows.Scan(&f.ID, &f.FeatID, &f.Name, &f.Description, &f.BonusType, &f.BonusValue, &f.Prerequisites); err != nil {
			h.Logger.Errorf("scan character feat: %v", err)
			continue
		}
		feats = append(feats, f)
	}

	writeJSON(w, http.StatusOK, feats)
}

// GetCharacterSkills returns the skills allocated to the player character in a campaign.
func (h *CharacterHandlers) GetCharacterSkills(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}
	if h.Pool == nil {
		writeError(w, http.StatusInternalServerError, "database pool unavailable")
		return
	}

	pgCampaignID := dbutil.ToPgtype(campaignID)

	// Get the player character for this campaign.
	pcs2, err := h.Queries.GetPlayerCharacterByCampaign(r.Context(), pgCampaignID)
	if err != nil || len(pcs2) == 0 {
		writeError(w, http.StatusNotFound, "player character not found")
		return
	}
	characterID := dbutil.FromPgtype(pcs2[0].ID)

	rows, err := h.Pool.Query(r.Context(),
		`SELECT cs.id, cs.skill_id, sd.name, sd.description, sd.base_ability, cs.points
		 FROM character_skills cs
		 JOIN skill_definitions sd ON sd.id = cs.skill_id
		 WHERE cs.character_id = $1
		 ORDER BY sd.name`,
		characterID,
	)
	if err != nil {
		h.Logger.Errorf("get character skills: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list character skills")
		return
	}
	defer rows.Close()

	skills := make([]api.SkillResponse, 0)
	for rows.Next() {
		var s api.SkillResponse
		if err := rows.Scan(&s.ID, &s.SkillID, &s.Name, &s.Description, &s.BaseAbility, &s.Points); err != nil {
			h.Logger.Errorf("scan character skill: %v", err)
			continue
		}
		skills = append(skills, s)
	}

	writeJSON(w, http.StatusOK, skills)
}
