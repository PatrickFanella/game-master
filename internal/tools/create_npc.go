package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const createNPCToolName = "create_npc"

// CreateNPCParams holds the parameters for creating an NPC.
type CreateNPCParams struct {
	CampaignID  uuid.UUID
	Name        string
	Description string
	Personality string
	Disposition int
	LocationID  *uuid.UUID
	FactionID   *uuid.UUID
	Stats       json.RawMessage
	Properties  json.RawMessage
}

// CreateNPCStore persists and retrieves NPC records and supporting entities.
type CreateNPCStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	LocationExistsInCampaign(ctx context.Context, locationID, campaignID uuid.UUID) (bool, error)
	CreateNPC(ctx context.Context, params CreateNPCParams) (*domain.NPC, error)
	ListNPCsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]domain.NPC, error)
}

// CreateNPCTool returns the create_npc tool definition and JSON schema.
func CreateNPCTool() llm.Tool {
	return llm.Tool{
		Name:        createNPCToolName,
		Description: "Create a new NPC in the world with personality, disposition, and optional faction/stat properties.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "NPC name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "NPC description and role summary.",
				},
				"personality": map[string]any{
					"type":        "string",
					"description": "NPC personality profile.",
				},
				"disposition": map[string]any{
					"type":        "integer",
					"description": "Optional NPC disposition toward the player (-100 to 100). Defaults to 0.",
				},
				"location_id": map[string]any{
					"type":        "string",
					"description": "Optional location UUID. Defaults to the current player's location.",
				},
				"faction_id": map[string]any{
					"type":        "string",
					"description": "Optional faction UUID for NPC affiliation.",
				},
				"stats": map[string]any{
					"type":        "object",
					"description": "Optional structured NPC stats JSON object.",
				},
				"properties": map[string]any{
					"type":        "object",
					"description": "Optional structured NPC properties JSON object.",
				},
			},
			"required":             []string{"name", "description", "personality"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateNPC registers the create_npc tool and handler.
func RegisterCreateNPC(reg *Registry, store CreateNPCStore, memoryStore MemoryStore, embedder Embedder) error {
	if store == nil {
		return errors.New("create_npc store is required")
	}
	return reg.Register(CreateNPCTool(), NewCreateNPCHandler(store, memoryStore, embedder).Handle)
}

// CreateNPCHandler executes create_npc tool calls.
type CreateNPCHandler struct {
	store       CreateNPCStore
	memoryStore MemoryStore
	embedder    Embedder
}

// NewCreateNPCHandler creates a new create_npc handler.
func NewCreateNPCHandler(store CreateNPCStore, memoryStore MemoryStore, embedder Embedder) *CreateNPCHandler {
	return &CreateNPCHandler{
		store:       store,
		memoryStore: memoryStore,
		embedder:    embedder,
	}
}

// Handle executes the create_npc tool.
func (h *CreateNPCHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_npc handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("create_npc store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("create_npc requires current player character id in context")
	}

	playerCharacter, err := h.store.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return nil, fmt.Errorf("get player character: %w", err)
	}
	if playerCharacter == nil {
		return nil, errors.New("current player character not found")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	personality, err := parseStringArg(args, "personality")
	if err != nil {
		return nil, err
	}

	disposition, dispositionSet, err := parseOptionalIntArg(args, "disposition")
	if err != nil {
		return nil, err
	}
	if !dispositionSet {
		disposition = 0
	}
	disposition = clampDispositionValue(disposition)

	locationID, locationSet, err := parseOptionalUUIDArg(args, "location_id")
	if err != nil {
		return nil, err
	}
	if !locationSet {
		if playerCharacter.CurrentLocationID == nil {
			return nil, errors.New("create_npc requires location_id or player current location")
		}
		locationID = *playerCharacter.CurrentLocationID
	}

	locationExists, err := h.store.LocationExistsInCampaign(ctx, locationID, playerCharacter.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("check location exists in campaign: %w", err)
	}
	if !locationExists {
		return nil, errors.New("location_id does not reference an existing location in player campaign")
	}

	factionID, factionSet, err := parseOptionalUUIDArg(args, "faction_id")
	if err != nil {
		return nil, err
	}
	var factionIDPtr *uuid.UUID
	if factionSet {
		factionIDPtr = &factionID
	}

	statsObj, statsSet, err := parseOptionalJSONObjectArgWithSet(args, "stats")
	if err != nil {
		return nil, err
	}
	propertiesObj, propertiesSet, err := parseOptionalJSONObjectArgWithSet(args, "properties")
	if err != nil {
		return nil, err
	}

	statsJSON, err := marshalOptionalJSONObject(statsObj, statsSet, "stats")
	if err != nil {
		return nil, err
	}
	propertiesJSON, err := marshalOptionalJSONObject(propertiesObj, propertiesSet, "properties")
	if err != nil {
		return nil, err
	}

	npcs, err := h.store.ListNPCsByCampaign(ctx, playerCharacter.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("list npcs by campaign: %w", err)
	}
	if hasDuplicateNPCNameAtLocation(npcs, name, locationID) {
		return nil, fmt.Errorf("npc with name %q already exists at location %s", name, locationID)
	}

	locationIDPtr := &locationID
	npc, err := h.store.CreateNPC(ctx, CreateNPCParams{
		CampaignID:  playerCharacter.CampaignID,
		Name:        name,
		Description: description,
		Personality: personality,
		Disposition: disposition,
		LocationID:  locationIDPtr,
		FactionID:   factionIDPtr,
		Stats:       statsJSON,
		Properties:  propertiesJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create npc: %w", err)
	}

	role := deriveNPCRole(description, propertiesObj)
	if h.embedder != nil && h.memoryStore != nil {
		_ = h.embedNPCMemory(ctx, npc, role)
	}

	statsData, err := rawJSONToObject(npc.Stats, "stats")
	if err != nil {
		return nil, err
	}
	propertiesData, err := rawJSONToObject(npc.Properties, "properties")
	if err != nil {
		return nil, err
	}

	data := map[string]any{
		"id":          npc.ID.String(),
		"campaign_id": npc.CampaignID.String(),
		"name":        npc.Name,
		"description": npc.Description,
		"personality": npc.Personality,
		"disposition": npc.Disposition,
		"alive":       npc.Alive,
		"stats":       statsData,
		"properties":  propertiesData,
	}
	if npc.LocationID != nil {
		data["location_id"] = npc.LocationID.String()
	}
	if npc.FactionID != nil {
		data["faction_id"] = npc.FactionID.String()
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: fmt.Sprintf("NPC %q created successfully.", npc.Name),
	}, nil
}

func (h *CreateNPCHandler) embedNPCMemory(ctx context.Context, npc *domain.NPC, role string) error {
	memoryContent := fmt.Sprintf("NPC introduced: %s. Personality: %s. Role: %s.", npc.Name, npc.Personality, role)
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed npc memory: %w", err)
	}

	metadata := map[string]any{
		"npc_id": npc.ID.String(),
	}
	if npc.LocationID != nil {
		metadata["location_id"] = npc.LocationID.String()
	}
	if npc.FactionID != nil {
		metadata["faction_id"] = npc.FactionID.String()
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal npc memory metadata: %w", err)
	}

	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: npc.CampaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadataJSON,
	}); err != nil {
		return fmt.Errorf("create npc memory: %w", err)
	}
	return nil
}

func parseOptionalJSONObjectArgWithSet(args map[string]any, key string) (map[string]any, bool, error) {
	raw, ok := args[key]
	if !ok {
		return nil, false, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("%s must be an object", key)
	}
	return obj, true, nil
}

func marshalOptionalJSONObject(obj map[string]any, set bool, key string) (json.RawMessage, error) {
	if !set {
		return nil, nil
	}
	value, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", key, err)
	}
	return value, nil
}

func rawJSONToObject(value json.RawMessage, key string) (map[string]any, error) {
	if len(value) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(value, &out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

func hasDuplicateNPCNameAtLocation(npcs []domain.NPC, name string, locationID uuid.UUID) bool {
	needle := strings.TrimSpace(name)
	for _, existing := range npcs {
		if existing.LocationID == nil || *existing.LocationID != locationID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(existing.Name), needle) {
			return true
		}
	}
	return false
}

func deriveNPCRole(description string, properties map[string]any) string {
	if properties != nil {
		if value, ok := properties["role"].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return description
}
