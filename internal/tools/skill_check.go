package tools

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const skillCheckToolName = "skill_check"
const floatIntegerTolerance = 1e-9

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
func (h *SkillCheckHandler) Handle(ctx context.Context, args map[string]any) (map[string]any, error) {
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

	return map[string]any{
		"roll":             roll,
		"rolls":            rolls,
		"modifier":         modifier,
		"total":            total,
		"dc":               dc,
		"success":          success,
		"margin":           total - dc,
		"critical_success": criticalSuccess,
		"critical_failure": criticalFailure,
	}, nil
}

func (h *SkillCheckHandler) rollD20() int {
	return h.roller.Intn(20) + 1
}

func parseUUIDArg(args map[string]any, key string) (uuid.UUID, error) {
	raw, ok := args[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("%s is required", key)
	}
	s, ok := raw.(string)
	if !ok || s == "" {
		return uuid.Nil, fmt.Errorf("%s must be a non-empty string", key)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s must be a valid UUID", key)
	}
	return id, nil
}

func parseStringArg(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := raw.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return s, nil
}

func parseBoolArg(args map[string]any, key string) (bool, error) {
	raw, ok := args[key]
	if !ok {
		return false, nil
	}
	b, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}

func parseIntArg(args map[string]any, key string) (int, error) {
	raw, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}

	switch v := raw.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		if v < int64(math.MinInt) || v > int64(math.MaxInt) {
			return 0, fmt.Errorf("%s is out of range", key)
		}
		return int(v), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("%s must be a finite integer", key)
		}
		rounded := math.Round(v)
		if math.Abs(v-rounded) > floatIntegerTolerance {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		if rounded < float64(math.MinInt) || rounded > float64(math.MaxInt) {
			return 0, fmt.Errorf("%s is out of range", key)
		}
		return int(rounded), nil
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}
