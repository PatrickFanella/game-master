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

const createEconomicSystemToolName = "create_economic_system"

// EconomicSystemStore persists economic systems and related world facts.
type EconomicSystemStore interface {
	CreateEconomicSystem(ctx context.Context, arg statedb.CreateEconomicSystemParams) (statedb.EconomicSystem, error)
	CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error)
	GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error)
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
}

// CreateEconomicSystemTool returns the create_economic_system tool definition and JSON schema.
func CreateEconomicSystemTool() llm.Tool {
	return llm.Tool{
		Name:        createEconomicSystemToolName,
		Description: "Create a world economic system with currency, resources, trade routes, class structure, and campaign scope.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"campaign_id": map[string]any{
					"type":        "string",
					"description": "Campaign UUID that owns this economic system.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Economic system name.",
				},
				"currency": map[string]any{
					"type":        "object",
					"description": "Currency definition including denominations.",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
						"denominations": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"required":             []string{"name", "denominations"},
					"additionalProperties": false,
				},
				"primary_resources": map[string]any{
					"type":        "array",
					"description": "Primary resources produced by the economy.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"trade_routes": map[string]any{
					"type":        "array",
					"description": "Trade routes linked to location IDs.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"from_location_id": map[string]any{
								"type": "string",
							},
							"to_location_id": map[string]any{
								"type": "string",
							},
							"goods": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
							},
						},
						"required":             []string{"from_location_id", "to_location_id"},
						"additionalProperties": false,
					},
				},
				"class_structure": map[string]any{
					"type":        "object",
					"description": "Class structure and socioeconomic tiers.",
				},
				"economic_type": map[string]any{
					"type":        "string",
					"description": "Economic archetype such as mercantile, agrarian, command, or mixed.",
				},
				"scope": map[string]any{
					"type":        "object",
					"description": "Region/faction scope linked to this economy.",
					"properties": map[string]any{
						"faction_ids": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
						"region_names": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"additionalProperties": false,
				},
			},
			"required": []string{
				"campaign_id",
				"name",
				"currency",
				"primary_resources",
				"trade_routes",
				"class_structure",
				"economic_type",
				"scope",
			},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateEconomicSystem registers the create_economic_system tool and handler.
// The memoryStore and embedder parameters are optional; when nil, embedding is skipped.
func RegisterCreateEconomicSystem(reg *Registry, economicStore EconomicSystemStore, memoryStore MemoryStore, embedder Embedder) error {
	if economicStore == nil {
		return errors.New("create_economic_system economic store is required")
	}
	return reg.Register(CreateEconomicSystemTool(), NewCreateEconomicSystemHandler(economicStore, memoryStore, embedder).Handle)
}

// CreateEconomicSystemHandler executes create_economic_system tool calls.
type CreateEconomicSystemHandler struct {
	economicStore EconomicSystemStore
	memoryStore   MemoryStore
	embedder      Embedder
}

// NewCreateEconomicSystemHandler creates a new create_economic_system handler.
func NewCreateEconomicSystemHandler(economicStore EconomicSystemStore, memoryStore MemoryStore, embedder Embedder) *CreateEconomicSystemHandler {
	return &CreateEconomicSystemHandler{
		economicStore: economicStore,
		memoryStore:   memoryStore,
		embedder:      embedder,
	}
}

// Handle executes the create_economic_system tool.
func (h *CreateEconomicSystemHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_economic_system handler is nil")
	}
	if h.economicStore == nil {
		return nil, errors.New("create_economic_system economic store is required")
	}

	campaignID, err := parseUUIDArg(args, "campaign_id")
	if err != nil {
		return nil, err
	}
	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	currency, err := parseJSONObjectArg(args, "currency")
	if err != nil {
		return nil, err
	}
	currencyName, err := parseObjectStringArg(currency, "name", "currency")
	if err != nil {
		return nil, err
	}
	currencyDenominations, err := parseRequiredObjectStringArrayArg(currency, "denominations", "currency")
	if err != nil {
		return nil, err
	}
	primaryResources, err := parseStringArrayArg(args, "primary_resources")
	if err != nil {
		return nil, err
	}
	tradeRoutes, err := parseTradeRoutesArg(args, "trade_routes")
	if err != nil {
		return nil, err
	}
	classStructure, err := parseJSONObjectArg(args, "class_structure")
	if err != nil {
		return nil, err
	}
	economicType, err := parseStringArg(args, "economic_type")
	if err != nil {
		return nil, err
	}
	scope, err := parseJSONObjectArg(args, "scope")
	if err != nil {
		return nil, err
	}
	scopeFactionIDs, err := parseUUIDArrayFromNestedObject(scope, "faction_ids", "scope")
	if err != nil {
		return nil, err
	}
	regionNames, err := parseObjectStringArrayArg(scope, "region_names", "scope")
	if err != nil {
		return nil, err
	}

	dbCampaignID := dbutil.ToPgtype(campaignID)
	if err := h.validateScopeAndTradeRouteIDs(ctx, dbCampaignID, scopeFactionIDs, tradeRoutes); err != nil {
		return nil, err
	}

	tradeRoutePayload := make([]map[string]any, 0, len(tradeRoutes))
	for _, route := range tradeRoutes {
		tradeRoutePayload = append(tradeRoutePayload, map[string]any{
			"from_location_id": route.FromLocationID.String(),
			"to_location_id":   route.ToLocationID.String(),
			"goods":            route.Goods,
		})
	}

	details := map[string]any{
		"currency": map[string]any{
			"name":          currencyName,
			"denominations": currencyDenominations,
		},
		"primary_resources": primaryResources,
		"trade_routes":      tradeRoutePayload,
		"class_structure":   classStructure,
		"economic_type":     economicType,
		"scope": map[string]any{
			"faction_ids":  dbutil.PgUUIDsToStrings(scopeFactionIDs),
			"region_names": regionNames,
		},
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return nil, fmt.Errorf("marshal economic system details: %w", err)
	}

	economicSystem, err := h.economicStore.CreateEconomicSystem(ctx, statedb.CreateEconomicSystemParams{
		CampaignID: dbCampaignID,
		Name:       name,
		Details:    detailsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create economic system: %w", err)
	}

	for i, fact := range buildEconomicSystemFacts(name, currencyName, currencyDenominations, primaryResources, economicType, tradeRoutePayload) {
		if _, err := h.economicStore.CreateFact(ctx, statedb.CreateFactParams{
			CampaignID: dbCampaignID,
			Fact:       fact,
			Category:   "economic_system",
			Source:     fmt.Sprintf("create_economic_system:%s", dbutil.FromPgtype(economicSystem.ID).String()),
		}); err != nil {
			return nil, fmt.Errorf("create economic system world_fact[%d]: %w", i, err)
		}
	}

	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedEconomicSystemMemory(ctx, campaignID, dbutil.FromPgtype(economicSystem.ID), name, details, dbutil.PgUUIDsToStrings(scopeFactionIDs), regionNames); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":                dbutil.FromPgtype(economicSystem.ID).String(),
			"campaign_id":       dbutil.FromPgtype(economicSystem.CampaignID).String(),
			"name":              economicSystem.Name,
			"currency":          details["currency"],
			"primary_resources": primaryResources,
			"trade_routes":      tradeRoutePayload,
			"class_structure":   classStructure,
			"economic_type":     economicType,
			"scope": map[string]any{
				"faction_ids":  dbutil.PgUUIDsToStrings(scopeFactionIDs),
				"region_names": regionNames,
			},
		},
		Narrative: fmt.Sprintf("Economic system %q created successfully.", economicSystem.Name),
	}, nil
}

func (h *CreateEconomicSystemHandler) embedEconomicSystemMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	economicSystemID uuid.UUID,
	name string,
	details map[string]any,
	factionIDs []string,
	regionNames []string,
) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal economic system memory details: %w", err)
	}

	memoryContent := fmt.Sprintf("Economic system created: %s. Details: %s.", name, string(detailsJSON))
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed economic system memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"economic_system_id": economicSystemID.String(),
		"faction_ids":        factionIDs,
		"region_names":       regionNames,
	})
	if err != nil {
		return fmt.Errorf("marshal economic system memory metadata: %w", err)
	}
	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create economic system memory: %w", err)
	}
	return nil
}

func (h *CreateEconomicSystemHandler) validateScopeAndTradeRouteIDs(ctx context.Context, campaignID pgtype.UUID, scopeFactionIDs []pgtype.UUID, tradeRoutes []tradeRoute) error {
	for i, factionID := range scopeFactionIDs {
		faction, err := h.economicStore.GetFactionByID(ctx, factionID)
		if err != nil {
			return fmt.Errorf("validate scope.faction_ids[%d]: %w", i, err)
		}
		if faction.CampaignID != campaignID {
			return fmt.Errorf("scope.faction_ids[%d] must belong to campaign_id", i)
		}
	}

	for i, route := range tradeRoutes {
		fromLocation, err := h.economicStore.GetLocationByID(ctx, dbutil.ToPgtype(route.FromLocationID))
		if err != nil {
			return fmt.Errorf("validate trade_routes[%d].from_location_id: %w", i, err)
		}
		if fromLocation.CampaignID != campaignID {
			return fmt.Errorf("trade_routes[%d].from_location_id must belong to campaign_id", i)
		}

		toLocation, err := h.economicStore.GetLocationByID(ctx, dbutil.ToPgtype(route.ToLocationID))
		if err != nil {
			return fmt.Errorf("validate trade_routes[%d].to_location_id: %w", i, err)
		}
		if toLocation.CampaignID != campaignID {
			return fmt.Errorf("trade_routes[%d].to_location_id must belong to campaign_id", i)
		}
	}

	return nil
}

type tradeRoute struct {
	FromLocationID uuid.UUID
	ToLocationID   uuid.UUID
	Goods          []string
}

func parseTradeRoutesArg(args map[string]any, key string) ([]tradeRoute, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]tradeRoute, 0, len(items))
	for i, item := range items {
		routeObj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}

		fromID, err := parseUUIDFromNestedObject(routeObj, "from_location_id", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		toID, err := parseUUIDFromNestedObject(routeObj, "to_location_id", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}

		goods, err := parseObjectStringArrayArg(routeObj, "goods", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}

		out = append(out, tradeRoute{
			FromLocationID: fromID,
			ToLocationID:   toID,
			Goods:          goods,
		})
	}
	return out, nil
}

func parseObjectStringArg(obj map[string]any, key, prefix string) (string, error) {
	raw, ok := obj[key]
	if !ok {
		return "", fmt.Errorf("%s.%s is required", prefix, key)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s.%s must be a non-empty string", prefix, key)
	}
	return value, nil
}

func parseObjectStringArrayArg(obj map[string]any, key, prefix string) ([]string, error) {
	raw, ok := obj[key]
	if !ok {
		return []string{}, nil
	}
	if raw == nil {
		return []string{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s.%s must be an array", prefix, key)
	}

	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s.%s[%d] must be a non-empty string", prefix, key, i)
		}
		out = append(out, s)
	}
	return out, nil
}

func parseRequiredObjectStringArrayArg(obj map[string]any, key, prefix string) ([]string, error) {
	raw, ok := obj[key]
	if !ok || raw == nil {
		return nil, fmt.Errorf("%s.%s is required", prefix, key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s.%s must be an array", prefix, key)
	}

	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s.%s[%d] must be a non-empty string", prefix, key, i)
		}
		out = append(out, s)
	}
	return out, nil
}

func parseUUIDFromNestedObject(obj map[string]any, key, prefix string) (uuid.UUID, error) {
	raw, ok := obj[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("%s.%s is required", prefix, key)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return uuid.Nil, fmt.Errorf("%s.%s must be a non-empty string UUID", prefix, key)
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s.%s must be a valid UUID", prefix, key)
	}
	return id, nil
}

func parseUUIDArrayFromNestedObject(obj map[string]any, key, prefix string) ([]pgtype.UUID, error) {
	raw, ok := obj[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s.%s must be an array", prefix, key)
	}

	out := make([]pgtype.UUID, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s.%s[%d] must be a non-empty string UUID", prefix, key, i)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("%s.%s[%d] must be a valid UUID", prefix, key, i)
		}
		out = append(out, dbutil.ToPgtype(id))
	}
	return out, nil
}

func buildEconomicSystemFacts(name, currencyName string, currencyDenominations, primaryResources []string, economicType string, tradeRoutePayload []map[string]any) []string {
	facts := make([]string, 0, 4)
	facts = append(facts, fmt.Sprintf("%s currency: %s (%s).", name, currencyName, strings.Join(currencyDenominations, ", ")))
	if len(primaryResources) > 0 {
		facts = append(facts, fmt.Sprintf("%s primary_resources: %s.", name, strings.Join(primaryResources, ", ")))
	}
	facts = append(facts, fmt.Sprintf("%s economic_type: %s.", name, economicType))
	if len(tradeRoutePayload) > 0 {
		facts = append(facts, fmt.Sprintf("%s trade_routes_count: %d.", name, len(tradeRoutePayload)))
	}
	return facts
}
