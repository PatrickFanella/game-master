package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const establishRelationshipToolName = "establish_relationship"

// EstablishRelationshipStore persists and updates entity relationships while validating entity references.
type EstablishRelationshipStore interface {
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
	GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error)
	GetNPCByID(ctx context.Context, id pgtype.UUID) (statedb.Npc, error)
	GetPlayerCharacterByID(ctx context.Context, id pgtype.UUID) (statedb.PlayerCharacter, error)
	GetItemByID(ctx context.Context, id pgtype.UUID) (statedb.Item, error)
	GetRelationshipsBetween(ctx context.Context, arg statedb.GetRelationshipsBetweenParams) ([]statedb.EntityRelationship, error)
	CreateRelationship(ctx context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error)
	UpdateRelationship(ctx context.Context, arg statedb.UpdateRelationshipParams) (statedb.EntityRelationship, error)
}

// EstablishRelationshipTool returns the establish_relationship tool definition and JSON schema.
func EstablishRelationshipTool() llm.Tool {
	return llm.Tool{
		Name:        establishRelationshipToolName,
		Description: "Create or update a relationship between two existing entities.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source_entity_type": map[string]any{
					"type":        "string",
					"description": "Source entity type (npc, location, faction, player_character, item). You may also provide player as an alias for player_character.",
				},
				"source_entity_id": map[string]any{
					"type":        "string",
					"description": "Source entity UUID.",
				},
				"target_entity_type": map[string]any{
					"type":        "string",
					"description": "Target entity type (npc, location, faction, player_character, item). You may also provide player as an alias for player_character.",
				},
				"target_entity_id": map[string]any{
					"type":        "string",
					"description": "Target entity UUID.",
				},
				"relationship_type": map[string]any{
					"type":        "string",
					"description": "Free-form relationship category (mentor, rival, lover, employer, etc.).",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Narrative details about this relationship.",
				},
				"strength": map[string]any{
					"type":        "integer",
					"description": "Optional relationship strength from 1-10. Defaults to 5. This tool intentionally uses a 1-10 scale even though the underlying database column supports a broader integer range.",
					"minimum":     1,
					"maximum":     10,
				},
			},
			"required": []string{
				"source_entity_type",
				"source_entity_id",
				"target_entity_type",
				"target_entity_id",
				"relationship_type",
				"description",
			},
			"additionalProperties": false,
		},
	}
}

// RegisterEstablishRelationship registers the establish_relationship tool and handler.
func RegisterEstablishRelationship(reg *Registry, store EstablishRelationshipStore) error {
	if store == nil {
		return errors.New("establish_relationship store is required")
	}
	return reg.Register(EstablishRelationshipTool(), NewEstablishRelationshipHandler(store).Handle)
}

// EstablishRelationshipHandler executes establish_relationship tool calls.
type EstablishRelationshipHandler struct {
	store EstablishRelationshipStore
}

// NewEstablishRelationshipHandler creates a new establish_relationship handler.
func NewEstablishRelationshipHandler(store EstablishRelationshipStore) *EstablishRelationshipHandler {
	return &EstablishRelationshipHandler{store: store}
}

// Handle executes the establish_relationship tool.
func (h *EstablishRelationshipHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("establish_relationship handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("establish_relationship store is required")
	}

	sourceEntityType, err := parseRelationshipEntityTypeArg(args, "source_entity_type")
	if err != nil {
		return nil, err
	}
	sourceEntityID, err := parseUUIDArg(args, "source_entity_id")
	if err != nil {
		return nil, err
	}
	targetEntityType, err := parseRelationshipEntityTypeArg(args, "target_entity_type")
	if err != nil {
		return nil, err
	}
	targetEntityID, err := parseUUIDArg(args, "target_entity_id")
	if err != nil {
		return nil, err
	}
	relationshipType, err := parseStringArg(args, "relationship_type")
	if err != nil {
		return nil, err
	}
	trimmedRelationshipType := strings.TrimSpace(relationshipType)
	if trimmedRelationshipType == "" {
		return nil, errors.New("relationship_type must not be empty or whitespace")
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	trimmedDescription := strings.TrimSpace(description)
	if trimmedDescription == "" {
		return nil, errors.New("description must not be empty or whitespace")
	}

	strength, strengthProvided, err := parseRelationshipStrengthArg(args, "strength")
	if err != nil {
		return nil, err
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("establish_relationship requires current location id in context")
	}
	currentLocation, err := h.store.GetLocationByID(ctx, dbutil.ToPgtype(currentLocationID))
	if err != nil {
		return nil, fmt.Errorf("resolve campaign from current location: %w", err)
	}
	campaignID := dbutil.FromPgtype(currentLocation.CampaignID)

	if err := h.validateEntityInCampaign(ctx, campaignID, sourceEntityType, sourceEntityID, "source"); err != nil {
		return nil, err
	}
	if err := h.validateEntityInCampaign(ctx, campaignID, targetEntityType, targetEntityID, "target"); err != nil {
		return nil, err
	}

	existing, err := h.store.GetRelationshipsBetween(ctx, statedb.GetRelationshipsBetweenParams{
		CampaignID:       dbutil.ToPgtype(campaignID),
		FirstEntityType:  sourceEntityType,
		FirstEntityID:    dbutil.ToPgtype(sourceEntityID),
		SecondEntityType: targetEntityType,
		SecondEntityID:   dbutil.ToPgtype(targetEntityID),
	})
	if err != nil {
		return nil, fmt.Errorf("lookup existing relationships: %w", err)
	}

	var relationship statedb.EntityRelationship
	updated := false
	for _, existingRelationship := range existing {
		if strings.EqualFold(strings.TrimSpace(existingRelationship.RelationshipType), trimmedRelationshipType) {
			updateStrength := existingRelationship.Strength
			if strengthProvided || !updateStrength.Valid {
				updateStrength = pgtype.Int4{Int32: int32(strength), Valid: true}
			}
			relationship, err = h.store.UpdateRelationship(ctx, statedb.UpdateRelationshipParams{
				RelationshipType: trimmedRelationshipType,
				Description:      pgtype.Text{String: trimmedDescription, Valid: true},
				Strength:         updateStrength,
				ID:               existingRelationship.ID,
				CampaignID:       dbutil.ToPgtype(campaignID),
			})
			if err != nil {
				return nil, fmt.Errorf("update existing relationship: %w", err)
			}
			updated = true
			break
		}
	}
	if !updated {
		relationship, err = h.store.CreateRelationship(ctx, statedb.CreateRelationshipParams{
			CampaignID:       dbutil.ToPgtype(campaignID),
			SourceEntityType: sourceEntityType,
			SourceEntityID:   dbutil.ToPgtype(sourceEntityID),
			TargetEntityType: targetEntityType,
			TargetEntityID:   dbutil.ToPgtype(targetEntityID),
			RelationshipType: trimmedRelationshipType,
			Description:      pgtype.Text{String: trimmedDescription, Valid: true},
			Strength:         pgtype.Int4{Int32: int32(strength), Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("create relationship: %w", err)
		}
	}

	verb := "established"
	if updated {
		verb = "updated"
	}
	summary := fmt.Sprintf("Relationship %q %s between %s %s and %s %s.",
		trimmedRelationshipType,
		verb,
		sourceEntityType,
		sourceEntityID.String(),
		targetEntityType,
		targetEntityID.String(),
	)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":                 dbutil.FromPgtype(relationship.ID).String(),
			"campaign_id":        campaignID.String(),
			"source_entity_type": relationship.SourceEntityType,
			"source_entity_id":   dbutil.FromPgtype(relationship.SourceEntityID).String(),
			"target_entity_type": relationship.TargetEntityType,
			"target_entity_id":   dbutil.FromPgtype(relationship.TargetEntityID).String(),
			"relationship_type":  relationship.RelationshipType,
			"description":        relationship.Description.String,
			"strength":           relationship.Strength.Int32,
			"updated":            updated,
			"summary":            summary,
		},
		Narrative: summary,
	}, nil
}

func parseRelationshipEntityTypeArg(args map[string]any, key string) (string, error) {
	value, err := parseStringArg(args, key)
	if err != nil {
		return "", err
	}
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "player" {
		normalized = string(domain.EntityTypePlayerCharacter)
	}
	switch domain.EntityType(normalized) {
	case domain.EntityTypeNPC, domain.EntityTypeLocation, domain.EntityTypeFaction, domain.EntityTypePlayerCharacter, domain.EntityTypeItem:
		return normalized, nil
	default:
		return "", fmt.Errorf("%s must be one of: npc, location, faction, player_character, player, item", key)
	}
}

func parseRelationshipStrengthArg(args map[string]any, key string) (int, bool, error) {
	if _, ok := args[key]; !ok {
		return 5, false, nil
	}
	strength, err := parseIntArg(args, key)
	if err != nil {
		return 0, false, err
	}
	if strength < 1 || strength > 10 {
		return 0, false, fmt.Errorf("%s must be between 1 and 10", key)
	}
	return strength, true, nil
}

func (h *EstablishRelationshipHandler) validateEntityInCampaign(ctx context.Context, campaignID uuid.UUID, entityType string, entityID uuid.UUID, role string) error {
	switch domain.EntityType(entityType) {
	case domain.EntityTypeNPC:
		entity, err := h.store.GetNPCByID(ctx, dbutil.ToPgtype(entityID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%s entity not found: npc %s", role, entityID)
			}
			return fmt.Errorf("lookup %s npc: %w", role, err)
		}
		if dbutil.FromPgtype(entity.CampaignID) != campaignID {
			return fmt.Errorf("%s entity does not belong to current campaign: npc %s", role, entityID)
		}
	case domain.EntityTypeLocation:
		entity, err := h.store.GetLocationByID(ctx, dbutil.ToPgtype(entityID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%s entity not found: location %s", role, entityID)
			}
			return fmt.Errorf("lookup %s location: %w", role, err)
		}
		if dbutil.FromPgtype(entity.CampaignID) != campaignID {
			return fmt.Errorf("%s entity does not belong to current campaign: location %s", role, entityID)
		}
	case domain.EntityTypeFaction:
		entity, err := h.store.GetFactionByID(ctx, dbutil.ToPgtype(entityID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%s entity not found: faction %s", role, entityID)
			}
			return fmt.Errorf("lookup %s faction: %w", role, err)
		}
		if dbutil.FromPgtype(entity.CampaignID) != campaignID {
			return fmt.Errorf("%s entity does not belong to current campaign: faction %s", role, entityID)
		}
	case domain.EntityTypePlayerCharacter:
		entity, err := h.store.GetPlayerCharacterByID(ctx, dbutil.ToPgtype(entityID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%s entity not found: player_character %s", role, entityID)
			}
			return fmt.Errorf("lookup %s player character: %w", role, err)
		}
		if dbutil.FromPgtype(entity.CampaignID) != campaignID {
			return fmt.Errorf("%s entity does not belong to current campaign: player_character %s", role, entityID)
		}
	case domain.EntityTypeItem:
		entity, err := h.store.GetItemByID(ctx, dbutil.ToPgtype(entityID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%s entity not found: item %s", role, entityID)
			}
			return fmt.Errorf("lookup %s item: %w", role, err)
		}
		if dbutil.FromPgtype(entity.CampaignID) != campaignID {
			return fmt.Errorf("%s entity does not belong to current campaign: item %s", role, entityID)
		}
	default:
		return fmt.Errorf("%s_entity_type must be one of: npc, location, faction, player_character, player, item", role)
	}

	return nil
}
