package tools

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
)

// PostValidationStore provides entity lookup operations for post-validation.
type PostValidationStore interface {
	// FindNPCByNameAndLocation checks if an NPC with the given name exists
	// at the specified location within a campaign. Returns the ID if found.
	FindNPCByNameAndLocation(ctx context.Context, campaignID uuid.UUID, name string, locationID *uuid.UUID) (*uuid.UUID, error)
	// FindLocationByNameAndRegion checks if a location with the given name
	// exists in the specified region within a campaign.
	FindLocationByNameAndRegion(ctx context.Context, campaignID uuid.UUID, name string, region string) (*uuid.UUID, error)
	// EntityExists checks if an entity of the given type and ID exists.
	EntityExists(ctx context.Context, entityType string, id uuid.UUID) (bool, error)
}

// PostValidator validates and corrects tool results after successful execution.
// It detects duplicates, verifies FK references, and fills defaults.
type PostValidator struct {
	store  PostValidationStore
	logger *log.Logger
}

// NewPostValidator constructs a PostValidator. If logger is nil, the default
// logger is used.
func NewPostValidator(store PostValidationStore, logger *log.Logger) *PostValidator {
	if logger == nil {
		logger = log.Default()
	}
	return &PostValidator{store: store, logger: logger}
}

// Validate runs post-creation validation on a tool result. It may modify
// the result to correct duplicates or fill defaults.
func (pv *PostValidator) Validate(ctx context.Context, toolName string, result *ToolResult) (*ToolResult, error) {
	if result == nil || result.Data == nil {
		return result, nil
	}

	switch toolName {
	case "create_npc":
		return pv.validateNPC(ctx, result)
	case "create_location":
		return pv.validateLocation(ctx, result)
	default:
		return result, nil
	}
}

// validateNPC deduplicates NPCs and fills missing defaults.
func (pv *PostValidator) validateNPC(ctx context.Context, result *ToolResult) (*ToolResult, error) {
	campaignID, err := extractUUID(result.Data, "campaign_id")
	if err != nil {
		// Cannot dedup without campaign_id; still fill defaults.
		pv.fillNPCDefaults(result)
		return result, nil
	}

	name, _ := result.Data["name"].(string)
	if name == "" {
		pv.fillNPCDefaults(result)
		return result, nil
	}

	var locationID *uuid.UUID
	if lid, err := extractUUID(result.Data, "location_id"); err == nil {
		locationID = &lid
	}

	existingID, err := pv.store.FindNPCByNameAndLocation(ctx, campaignID, name, locationID)
	if err != nil {
		return nil, fmt.Errorf("post-validate NPC lookup: %w", err)
	}
	if existingID != nil {
		result.Data["id"] = existingID.String()
		result.Data["deduplicated"] = true
	}

	pv.fillNPCDefaults(result)
	return result, nil
}

// fillNPCDefaults sets missing optional NPC fields to their zero values.
func (pv *PostValidator) fillNPCDefaults(result *ToolResult) {
	if _, ok := result.Data["disposition"]; !ok {
		result.Data["disposition"] = 0
	}
	if _, ok := result.Data["alive"]; !ok {
		result.Data["alive"] = true
	}
	if _, ok := result.Data["properties"]; !ok {
		result.Data["properties"] = map[string]any{}
	}
}

// validateLocation deduplicates locations by name+region.
func (pv *PostValidator) validateLocation(ctx context.Context, result *ToolResult) (*ToolResult, error) {
	campaignID, err := extractUUID(result.Data, "campaign_id")
	if err != nil {
		return result, nil
	}

	name, _ := result.Data["name"].(string)
	if name == "" {
		return result, nil
	}

	region, _ := result.Data["region"].(string)

	existingID, err := pv.store.FindLocationByNameAndRegion(ctx, campaignID, name, region)
	if err != nil {
		return nil, fmt.Errorf("post-validate location lookup: %w", err)
	}
	if existingID != nil {
		result.Data["id"] = existingID.String()
		result.Data["deduplicated"] = true
	}

	return result, nil
}

// Wrap returns a new handler that delegates to handler and, on success,
// runs post-validation. Validation errors are logged but never propagated;
// the original result is returned.
func (pv *PostValidator) Wrap(toolName string, handler func(ctx context.Context, args map[string]any) (*ToolResult, error)) func(ctx context.Context, args map[string]any) (*ToolResult, error) {
	return func(ctx context.Context, args map[string]any) (*ToolResult, error) {
		result, err := handler(ctx, args)
		if err != nil || result == nil || !result.Success {
			return result, err
		}

		validated, vErr := pv.Validate(ctx, toolName, result)
		if vErr != nil {
			pv.logger.Error("post-validation failed", "tool", toolName, "err", vErr)
			return result, nil
		}
		return validated, nil
	}
}

// extractUUID pulls a uuid.UUID from a map field, accepting string values.
func extractUUID(data map[string]any, key string) (uuid.UUID, error) {
	raw, ok := data[key]
	if !ok {
		return uuid.UUID{}, fmt.Errorf("missing key %q", key)
	}
	switch v := raw.(type) {
	case string:
		return uuid.Parse(v)
	case uuid.UUID:
		return v, nil
	default:
		return uuid.UUID{}, fmt.Errorf("unsupported type %T for key %q", raw, key)
	}
}
