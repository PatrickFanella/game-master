package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const createFactionToolName = "create_faction"

var allowedFactionRelationshipTypes = map[string]struct{}{
	"allied":        {},
	"hostile":       {},
	"neutral":       {},
	"vassal":        {},
	"rival":         {},
	"trade_partner": {},
}

type factionRelationshipInput struct {
	FactionID   uuid.UUID
	Type        string
	Description string
}

// FactionStore persists factions and faction relationships.
type FactionStore interface {
	CreateFaction(ctx context.Context, arg statedb.CreateFactionParams) (statedb.Faction, error)
	CreateFactionRelationship(ctx context.Context, arg statedb.CreateFactionRelationshipParams) (statedb.FactionRelationship, error)
	GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error)
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
}

// CreateFactionTool returns the create_faction tool definition and JSON schema.
func CreateFactionTool() llm.Tool {
	return llm.Tool{
		Name:        createFactionToolName,
		Description: "Create a faction and establish relationships to existing factions in the same campaign.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Faction name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Faction description.",
				},
				"agenda": map[string]any{
					"type":        "string",
					"description": "Faction agenda and goals.",
				},
				"territory": map[string]any{
					"type":        "string",
					"description": "Faction territory.",
				},
				"properties": map[string]any{
					"type":        "object",
					"description": "Additional JSON properties for the faction.",
				},
				"relationships": map[string]any{
					"type":        "array",
					"description": "Relationships to existing factions.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"faction_id": map[string]any{
								"type":        "string",
								"description": "Existing related faction UUID.",
							},
							"type": map[string]any{
								"type":        "string",
								"description": "Relationship type between the new faction and the referenced faction (case-insensitive input; stored as lowercase canonical value).",
								"enum":        []string{"allied", "hostile", "neutral", "vassal", "rival", "trade_partner"},
							},
							"description": map[string]any{
								"type":        "string",
								"description": "Relationship details.",
							},
						},
						"required":             []string{"faction_id", "type", "description"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"name", "description", "agenda", "territory", "properties", "relationships"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateFaction registers the create_faction tool and handler.
func RegisterCreateFaction(reg *Registry, factionStore FactionStore, memoryStore MemoryStore, embedder Embedder) error {
	if factionStore == nil {
		return errors.New("create_faction faction store is required")
	}
	return reg.Register(CreateFactionTool(), NewCreateFactionHandler(factionStore, memoryStore, embedder).Handle)
}

// CreateFactionHandler executes create_faction tool calls.
type CreateFactionHandler struct {
	factionStore FactionStore
	memoryStore  MemoryStore
	embedder     Embedder
}

// NewCreateFactionHandler creates a new create_faction handler.
func NewCreateFactionHandler(factionStore FactionStore, memoryStore MemoryStore, embedder Embedder) *CreateFactionHandler {
	return &CreateFactionHandler{
		factionStore: factionStore,
		memoryStore:  memoryStore,
		embedder:     embedder,
	}
}

// Handle executes the create_faction tool.
func (h *CreateFactionHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_faction handler is nil")
	}
	if h.factionStore == nil {
		return nil, errors.New("create_faction faction store is required")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	agenda, err := parseStringArg(args, "agenda")
	if err != nil {
		return nil, err
	}
	territory, err := parseStringArg(args, "territory")
	if err != nil {
		return nil, err
	}
	properties, err := parseJSONObjectArg(args, "properties")
	if err != nil {
		return nil, err
	}
	relationships, err := parseFactionRelationshipsArg(args, "relationships")
	if err != nil {
		return nil, err
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("create_faction requires current location id in context")
	}
	currentLocation, err := h.factionStore.GetLocationByID(ctx, dbutil.ToPgtype(currentLocationID))
	if err != nil {
		return nil, fmt.Errorf("resolve campaign from current location: %w", err)
	}

	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("marshal faction properties: %w", err)
	}

	if err := h.validateRelatedFactionIDs(ctx, currentLocation.CampaignID, relationships); err != nil {
		return nil, err
	}

	faction, err := h.factionStore.CreateFaction(ctx, statedb.CreateFactionParams{
		CampaignID:  currentLocation.CampaignID,
		Name:        name,
		Description: pgtype.Text{String: description, Valid: true},
		Agenda:      pgtype.Text{String: agenda, Valid: true},
		Territory:   pgtype.Text{String: territory, Valid: true},
		Properties:  propertiesJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create faction: %w", err)
	}

	createdRelationships, err := h.createFactionRelationships(ctx, faction.ID, relationships)
	if err != nil {
		return nil, err
	}

	factionID := dbutil.FromPgtype(faction.ID)
	campaignID := dbutil.FromPgtype(faction.CampaignID)
	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedFactionMemory(ctx, campaignID, factionID, name, description, agenda, territory, properties, createdRelationships); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":            factionID.String(),
			"campaign_id":   campaignID.String(),
			"name":          name,
			"description":   description,
			"agenda":        agenda,
			"territory":     territory,
			"properties":    properties,
			"relationships": createdRelationships,
		},
		Narrative: fmt.Sprintf("Faction %q created successfully.", name),
	}, nil
}

func (h *CreateFactionHandler) validateRelatedFactionIDs(ctx context.Context, campaignID pgtype.UUID, relationships []factionRelationshipInput) error {
	for i, relationship := range relationships {
		faction, err := h.factionStore.GetFactionByID(ctx, dbutil.ToPgtype(relationship.FactionID))
		if err != nil {
			return fmt.Errorf("validate relationships[%d].faction_id: %w", i, err)
		}
		if faction.CampaignID != campaignID {
			return fmt.Errorf("relationships[%d].faction_id must belong to active campaign", i)
		}
	}
	return nil
}

func (h *CreateFactionHandler) createFactionRelationships(ctx context.Context, factionID pgtype.UUID, relationships []factionRelationshipInput) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(relationships))
	for i, relationship := range relationships {
		created, err := h.factionStore.CreateFactionRelationship(ctx, statedb.CreateFactionRelationshipParams{
			FactionID:        factionID,
			RelatedFactionID: dbutil.ToPgtype(relationship.FactionID),
			RelationshipType: relationship.Type,
			Description:      pgtype.Text{String: relationship.Description, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("create relationships[%d]: %w", i, err)
		}
		out = append(out, map[string]any{
			"id":                 dbutil.FromPgtype(created.ID).String(),
			"faction_id":         dbutil.FromPgtype(created.FactionID).String(),
			"related_faction_id": dbutil.FromPgtype(created.RelatedFactionID).String(),
			"type":               created.RelationshipType,
			"description":        created.Description.String,
		})
	}
	return out, nil
}

func (h *CreateFactionHandler) embedFactionMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	factionID uuid.UUID,
	name string,
	description string,
	agenda string,
	territory string,
	properties map[string]any,
	relationships []map[string]any,
) error {
	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("marshal faction properties for memory: %w", err)
	}
	relationshipsJSON, err := json.Marshal(relationships)
	if err != nil {
		return fmt.Errorf("marshal faction relationships for memory: %w", err)
	}
	memoryContent := fmt.Sprintf(
		"Faction created: %s. Description: %s. Agenda: %s. Territory: %s. Properties: %s. Relationships: %s.",
		name,
		description,
		agenda,
		territory,
		string(propertiesJSON),
		string(relationshipsJSON),
	)
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed faction memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"faction_id":          factionID.String(),
		"territory":           territory,
		"relationship_count":  len(relationships),
		"properties_present":  len(properties) > 0,
	})
	if err != nil {
		return fmt.Errorf("marshal faction memory metadata: %w", err)
	}
	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create faction memory: %w", err)
	}
	return nil
}

func parseFactionRelationshipsArg(args map[string]any, key string) ([]factionRelationshipInput, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]factionRelationshipInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		prefix := fmt.Sprintf("%s[%d]", key, i)
		factionID, err := parseUUIDFromNestedObject(obj, "faction_id", prefix)
		if err != nil {
			return nil, err
		}
		relationshipType, err := parseObjectStringArg(obj, "type", prefix)
		if err != nil {
			return nil, err
		}
		relationshipType = strings.ToLower(strings.TrimSpace(relationshipType))
		if _, allowed := allowedFactionRelationshipTypes[relationshipType]; !allowed {
			return nil, fmt.Errorf("%s.type must be one of allied, hostile, neutral, vassal, rival, trade_partner", prefix)
		}
		description, err := parseObjectStringArg(obj, "description", prefix)
		if err != nil {
			return nil, err
		}
		out = append(out, factionRelationshipInput{
			FactionID:   factionID,
			Type:        relationshipType,
			Description: strings.TrimSpace(description),
		})
	}

	return out, nil
}
