package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const allocateSkillToolName = "allocate_skill"

// AllocateSkillDB is the minimal database interface for the allocate_skill tool.
type AllocateSkillDB interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// AllocateSkillTool returns the allocate_skill tool definition and JSON schema.
func AllocateSkillTool() llm.Tool {
	return llm.Tool{
		Name:        allocateSkillToolName,
		Description: "Allocate skill points to a character's skill, incrementing their proficiency.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Name of the skill to allocate points to (must match a skill_definition in the campaign).",
				},
				"points": map[string]any{
					"type":        "integer",
					"description": "Number of points to allocate (1-3).",
					"minimum":     1,
					"maximum":     3,
				},
			},
			"required":             []string{"skill_name", "points"},
			"additionalProperties": false,
		},
	}
}

// RegisterAllocateSkill registers the allocate_skill tool and handler.
func RegisterAllocateSkill(reg *Registry, db AllocateSkillDB) error {
	if db == nil {
		return errors.New("allocate_skill database is required")
	}
	handler := &AllocateSkillHandler{db: db}
	return reg.Register(AllocateSkillTool(), handler.Handle)
}

// AllocateSkillHandler executes allocate_skill tool calls.
type AllocateSkillHandler struct {
	db AllocateSkillDB
}

// Handle executes the allocate_skill tool.
func (h *AllocateSkillHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("allocate_skill handler is not configured")
	}

	skillName, err := parseStringArg(args, "skill_name")
	if err != nil {
		return nil, err
	}
	points, err := parseIntArg(args, "points")
	if err != nil {
		return nil, err
	}
	if points < 1 || points > 3 {
		return nil, errors.New("points must be between 1 and 3")
	}

	campaignID, ok := CurrentCampaignIDFromContext(ctx)
	if !ok {
		return nil, errors.New("allocate_skill requires a current campaign in context")
	}

	characterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("allocate_skill requires a current player character in context")
	}

	// Look up skill definition by name in the campaign.
	var skillID uuid.UUID
	err = h.db.QueryRow(ctx,
		`SELECT id FROM skill_definitions WHERE campaign_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1`,
		campaignID, strings.TrimSpace(skillName),
	).Scan(&skillID)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found in this campaign", skillName)
	}

	// Upsert character_skills — increment points.
	_, err = h.db.Exec(ctx,
		`INSERT INTO character_skills (character_id, skill_id, points)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (character_id, skill_id) DO UPDATE SET points = character_skills.points + $3`,
		characterID, skillID, points,
	)
	if err != nil {
		return nil, fmt.Errorf("allocate skill: %w", err)
	}

	narrative := fmt.Sprintf("You've improved your %s skill (+%d)", skillName, points)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"skill_name":   skillName,
			"skill_id":     skillID.String(),
			"character_id": characterID.String(),
			"points_added": points,
		},
		Narrative: narrative,
	}, nil
}
