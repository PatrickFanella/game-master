package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const presentChoicesToolName = "present_choices"

const maxPresentChoices = 6

var allowedChoiceTypes = map[string]struct{}{
	"action":   {},
	"dialogue": {},
	"movement": {},
}

// PresentChoicesTool returns the present_choices tool definition and JSON schema.
func PresentChoicesTool() llm.Tool {
	return llm.Tool{
		Name:        presentChoicesToolName,
		Description: "Present selectable next-step choices for the player.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"choices": map[string]any{
					"type":        "array",
					"description": "Suggested options the player can select from.",
					"minItems":    1,
					"maxItems":    maxPresentChoices,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{
								"type":        "string",
								"description": "Stable identifier for the choice.",
							},
							"text": map[string]any{
								"type":        "string",
								"description": "Human-readable text shown in the UI.",
							},
							"type": map[string]any{
								"type":        "string",
								"description": "Choice category. One of: action, dialogue, movement.",
							},
						},
						"required":             []string{"id", "text", "type"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"choices"},
			"additionalProperties": false,
		},
	}
}

// RegisterPresentChoices registers the present_choices tool and handler.
func RegisterPresentChoices(reg *Registry) error {
	return reg.Register(PresentChoicesTool(), NewPresentChoicesHandler().Handle)
}

// PresentChoicesHandler executes present_choices tool calls.
type PresentChoicesHandler struct{}

// NewPresentChoicesHandler creates a new present_choices handler.
func NewPresentChoicesHandler() *PresentChoicesHandler {
	return &PresentChoicesHandler{}
}

// Handle executes the present_choices tool.
func (h *PresentChoicesHandler) Handle(_ context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("present_choices handler is nil")
	}

	rawChoices, ok := args["choices"]
	if !ok {
		return nil, errors.New("choices is required")
	}
	choices, ok := rawChoices.([]any)
	if !ok {
		return nil, errors.New("choices must be an array")
	}
	if len(choices) == 0 {
		return nil, errors.New("choices must contain at least one entry")
	}
	if len(choices) > maxPresentChoices {
		return nil, fmt.Errorf("choices must contain at most %d entries", maxPresentChoices)
	}

	validated := make([]map[string]any, 0, len(choices))
	for i, raw := range choices {
		choice, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("choices[%d] must be an object", i)
		}

		id, err := parseChoiceField(choice, "id", i)
		if err != nil {
			return nil, err
		}
		text, err := parseChoiceField(choice, "text", i)
		if err != nil {
			return nil, err
		}
		choiceType, err := parseChoiceField(choice, "type", i)
		if err != nil {
			return nil, err
		}
		if _, allowed := allowedChoiceTypes[choiceType]; !allowed {
			return nil, fmt.Errorf("choices[%d].type must be one of: action, dialogue, movement", i)
		}

		validated = append(validated, map[string]any{
			"id":   id,
			"text": text,
			"type": choiceType,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"choices": validated,
		},
		Narrative: "Choices prepared for display.",
	}, nil
}

func parseChoiceField(choice map[string]any, key string, index int) (string, error) {
	raw, ok := choice[key]
	if !ok {
		return "", fmt.Errorf("choices[%d].%s is required", index, key)
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", fmt.Errorf("choices[%d].%s must be a non-empty string", index, key)
	}
	return value, nil
}
