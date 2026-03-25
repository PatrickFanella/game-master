package tools

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const skillCheckToolName = "skill_check"

// StatModifierResolver resolves a character's modifier for a given skill/stat.
type StatModifierResolver interface {
	GetStatModifier(ctx context.Context, characterID uuid.UUID, skill string) (int, error)
}

// DiceRoller provides pseudo-random integer generation.
type DiceRoller interface {
	Intn(n int) int
}

func newRandomRoller() DiceRoller {
	return &randomRoller{}
}

type randomRoller struct{}

func (r *randomRoller) Intn(n int) int {
	return rand.IntN(n)
}

// SkillCheckTool returns the skill_check tool definition and JSON schema.
func SkillCheckTool() llm.Tool {
	return llm.Tool{
		Name:        skillCheckToolName,
		Description: "Resolve an uncertain action by rolling d20 plus a character skill/stat modifier against a difficulty class.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"character_id": map[string]any{
					"type":        "string",
					"description": "Character UUID performing the check.",
				},
				"skill": map[string]any{
					"type":        "string",
					"description": "Skill or stat key to use for the modifier.",
				},
				"difficulty": map[string]any{
					"type":        "integer",
					"description": "Difficulty class (DC).",
				},
				"advantage": map[string]any{
					"type":        "boolean",
					"description": "If true, roll twice and use the higher result.",
				},
				"disadvantage": map[string]any{
					"type":        "boolean",
					"description": "If true, roll twice and use the lower result.",
				},
			},
			"required":             []string{"character_id", "skill", "difficulty"},
			"additionalProperties": false,
		},
	}
}

// RegisterSkillCheck registers the skill_check tool and handler.
func RegisterSkillCheck(reg *Registry, resolver StatModifierResolver, roller DiceRoller) error {
	if resolver == nil {
		return errors.New("skill_check resolver is required")
	}
	handler := NewSkillCheckHandler(resolver, roller)
	return reg.Register(SkillCheckTool(), handler.Handle)
}

// SkillCheckHandler executes skill_check tool calls.
type SkillCheckHandler struct {
	resolver StatModifierResolver
	roller   DiceRoller
}

// NewSkillCheckHandler creates a new skill check handler.
func NewSkillCheckHandler(resolver StatModifierResolver, roller DiceRoller) *SkillCheckHandler {
	if roller == nil {
		roller = newRandomRoller()
	}
	return &SkillCheckHandler{resolver: resolver, roller: roller}
}

// Handle executes the skill_check tool.
func (h *SkillCheckHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("skill_check handler is nil")
	}
	if h.resolver == nil {
		return nil, errors.New("skill_check resolver is required")
	}
	if h.roller == nil {
		return nil, errors.New("skill_check roller is required")
	}

	characterID, err := parseUUIDArg(args, "character_id")
	if err != nil {
		return nil, err
	}
	skill, err := parseStringArg(args, "skill")
	if err != nil {
		return nil, err
	}
	dc, err := parseIntArg(args, "difficulty")
	if err != nil {
		return nil, err
	}
	advantage, err := parseBoolArg(args, "advantage")
	if err != nil {
		return nil, err
	}
	disadvantage, err := parseBoolArg(args, "disadvantage")
	if err != nil {
		return nil, err
	}
	if advantage && disadvantage {
		return nil, errors.New("advantage and disadvantage cannot both be true")
	}

	modifier, err := h.resolver.GetStatModifier(ctx, characterID, skill)
	if err != nil {
		return nil, fmt.Errorf("resolve stat modifier: %w", err)
	}

	rolls := []int{h.rollD20()}
	roll := rolls[0]
	if advantage || disadvantage {
		rolls = append(rolls, h.rollD20())
		if advantage && rolls[1] > roll {
			roll = rolls[1]
		}
		if disadvantage && rolls[1] < roll {
			roll = rolls[1]
		}
	}

	total := roll + modifier
	criticalSuccess := roll == 20
	criticalFailure := roll == 1
	success := total >= dc
	if criticalSuccess {
		success = true
	}
	if criticalFailure {
		success = false
	}

	data := map[string]any{
		"roll":             roll,
		"rolls":            rolls,
		"modifier":         modifier,
		"total":            total,
		"dc":               dc,
		"success":          success,
		"margin":           total - dc,
		"critical_success": criticalSuccess,
		"critical_failure": criticalFailure,
	}

	return &ToolResult{
		Success:   success,
		Data:      data,
		Narrative: buildSkillCheckNarrative(skill, roll, modifier, total, dc, success, criticalSuccess, criticalFailure),
	}, nil
}

func buildSkillCheckNarrative(skill string, roll, modifier, total, dc int, success, criticalSuccess, criticalFailure bool) string {
	sign := "+"
	if modifier < 0 {
		sign = ""
	}
	result := "Failure"
	switch {
	case criticalSuccess:
		result = "Critical Success"
	case criticalFailure:
		result = "Critical Failure"
	case success:
		result = "Success"
	}
	return fmt.Sprintf("%s check: d20 roll %d %s%d = %d vs DC %d — %s.", skill, roll, sign, modifier, total, dc, result)
}

func (h *SkillCheckHandler) rollD20() int {
	return h.roller.Intn(20) + 1
}

