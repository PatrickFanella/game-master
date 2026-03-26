package tools

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const rollDiceToolName = "roll_dice"
const maxRollDiceCount = 100

var rollDiceNotationPattern = regexp.MustCompile(`^([1-9]\d*)d(4|6|8|10|12|20|100)([+-]\d+)?$`)

type diceExpression struct {
	Count    int
	Sides    int
	Modifier int
}

// RollDiceTool returns the roll_dice tool definition and JSON schema.
func RollDiceTool() llm.Tool {
	return llm.Tool{
		Name:        rollDiceToolName,
		Description: "Roll dice using standard notation (NdS, NdS+M, NdS-M) for random outcomes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dice": map[string]any{
					"type":        "string",
					"description": "Dice notation in NdS, NdS+M, or NdS-M format (e.g., 2d6+3).",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Why this roll is being made.",
				},
			},
			"required":             []string{"dice", "reason"},
			"additionalProperties": false,
		},
	}
}

// RegisterRollDice registers the roll_dice tool and handler.
func RegisterRollDice(reg *Registry) error {
	return reg.Register(RollDiceTool(), NewRollDiceHandler().Handle)
}

// RollDiceHandler executes roll_dice tool calls.
type RollDiceHandler struct{}

// NewRollDiceHandler creates a new roll_dice handler.
func NewRollDiceHandler() *RollDiceHandler {
	return &RollDiceHandler{}
}

// Handle executes the roll_dice tool.
func (h *RollDiceHandler) Handle(_ context.Context, args map[string]any) (*ToolResult, error) {
	dice, err := parseStringArg(args, "dice")
	if err != nil {
		return nil, err
	}
	reason, err := parseStringArg(args, "reason")
	if err != nil {
		return nil, err
	}

	expr, err := parseDiceNotation(dice)
	if err != nil {
		return nil, err
	}

	rolls := make([]int, 0, expr.Count)
	sum := 0
	for range expr.Count {
		roll, err := rollDie(expr.Sides)
		if err != nil {
			return nil, fmt.Errorf("roll die: %w", err)
		}
		rolls = append(rolls, roll)
		sum += roll
	}
	total := sum + expr.Modifier

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"dice":     dice,
			"rolls":    rolls,
			"modifier": expr.Modifier,
			"total":    total,
			"reason":   reason,
		},
		Narrative: fmt.Sprintf("Rolled %s for %s: %v, modifier %+d, total %d.", dice, reason, rolls, expr.Modifier, total),
	}, nil
}

func parseDiceNotation(dice string) (diceExpression, error) {
	normalized := strings.ToLower(strings.TrimSpace(dice))
	matches := rollDiceNotationPattern.FindStringSubmatch(normalized)
	if matches == nil {
		return diceExpression{}, errors.New("dice must use NdS, NdS+M, or NdS-M notation with supported sides: 4, 6, 8, 10, 12, 20, 100")
	}

	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return diceExpression{}, fmt.Errorf("parse dice count: %w", err)
	}
	if count > maxRollDiceCount {
		return diceExpression{}, fmt.Errorf("dice count must be at most %d", maxRollDiceCount)
	}

	sides, err := strconv.Atoi(matches[2])
	if err != nil {
		return diceExpression{}, fmt.Errorf("parse dice sides: %w", err)
	}

	modifier := 0
	if matches[3] != "" {
		modifier, err = strconv.Atoi(matches[3])
		if err != nil {
			return diceExpression{}, fmt.Errorf("parse dice modifier: %w", err)
		}
	}
	return diceExpression{
		Count:    count,
		Sides:    sides,
		Modifier: modifier,
	}, nil
}

func rollDie(sides int) (int, error) {
	if sides < 1 {
		return 0, errors.New("sides must be greater than zero")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + 1, nil
}
