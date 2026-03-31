package tools

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

type createQuestObjectiveInput struct {
	Description string
	OrderIndex  int
}

type createQuestRelatedEntityInput struct {
	EntityType string
	EntityID   uuid.UUID
}

func parseQuestTypeArg(args map[string]any, key string) (string, error) {
	value, err := parseStringArg(args, key)
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(value) {
	case string(domain.QuestTypeShortTerm), string(domain.QuestTypeMediumTerm), string(domain.QuestTypeLongTerm):
		return strings.TrimSpace(value), nil
	default:
		return "", fmt.Errorf("%s must be one of: short_term, medium_term, long_term", key)
	}
}

func parseQuestObjectivesArg(args map[string]any, key string) ([]createQuestObjectiveInput, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]createQuestObjectiveInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		description, err := parseObjectStringArg(obj, "description", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		orderIndex, err := parseObjectIntArg(obj, "order_index", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		if orderIndex < 0 {
			return nil, fmt.Errorf("%s[%d].order_index must be greater than or equal to 0", key, i)
		}
		out = append(out, createQuestObjectiveInput{
			Description: strings.TrimSpace(description),
			OrderIndex:  orderIndex,
		})
	}

	return out, nil
}

func parseQuestRelatedEntitiesArg(args map[string]any, key string) ([]createQuestRelatedEntityInput, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return []createQuestRelatedEntityInput{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]createQuestRelatedEntityInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		entityType, err := parseObjectStringArg(obj, "entity_type", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		normalizedEntityType, err := normalizeRelatedEntityType(entityType, key, i)
		if err != nil {
			return nil, err
		}
		entityID, err := parseUUIDFromNestedObject(obj, "entity_id", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		out = append(out, createQuestRelatedEntityInput{
			EntityType: normalizedEntityType,
			EntityID:   entityID,
		})
	}

	return out, nil
}

func normalizeRelatedEntityType(entityType, key string, index int) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(entityType))
	if normalized == "player" {
		normalized = string(domain.EntityTypePlayerCharacter)
	}
	switch domain.EntityType(normalized) {
	case domain.EntityTypeNPC, domain.EntityTypeLocation, domain.EntityTypeFaction, domain.EntityTypePlayerCharacter, domain.EntityTypeItem:
		return normalized, nil
	default:
		return "", fmt.Errorf("%s[%d].entity_type must be one of: npc, location, faction, player_character, player, item", key, index)
	}
}

func parseObjectIntArg(obj map[string]any, key, prefix string) (int, error) {
	raw, ok := obj[key]
	if !ok {
		return 0, fmt.Errorf("%s.%s is required", prefix, key)
	}
	return parseIntArg(map[string]any{key: raw}, key)
}
