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

const createCityToolName = "create_city"

// CityStore persists city locations and allows campaign-scoped location queries.
type CityStore interface {
	CreateLocation(ctx context.Context, arg statedb.CreateLocationParams) (statedb.Location, error)
	UpdateLocation(ctx context.Context, arg statedb.UpdateLocationParams) (statedb.Location, error)
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
	ListLocationsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Location, error)
}

// CreateCityTool returns the create_city tool definition and JSON schema.
func CreateCityTool() llm.Tool {
	return llm.Tool{
		Name:        createCityToolName,
		Description: "Create or update a city location with rich properties such as districts, landmarks, governance, economy, and demographics.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "City name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "City description.",
				},
				"population": map[string]any{
					"type":        "integer",
					"description": "City population.",
				},
				"districts": map[string]any{
					"type":        "array",
					"description": "District names within the city.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"landmarks": map[string]any{
					"type":        "array",
					"description": "Landmark names within the city.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"governance": map[string]any{
					"type":        "string",
					"description": "Governance structure summary.",
				},
				"economy_summary": map[string]any{
					"type":        "string",
					"description": "Economic summary of the city.",
				},
				"demographics": map[string]any{
					"type":        "object",
					"description": "Demographics object for the city.",
				},
				"location_id": map[string]any{
					"type":        "string",
					"description": "Parent location UUID that contains this city.",
				},
				"create_district_locations": map[string]any{
					"type":        "boolean",
					"description": "When true, creates district sub-locations for district names not already present.",
				},
			},
			"required": []string{
				"name",
				"description",
				"population",
				"districts",
				"landmarks",
				"governance",
				"economy_summary",
				"demographics",
				"location_id",
			},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateCity registers the create_city tool and handler.
func RegisterCreateCity(reg *Registry, cityStore CityStore, memoryStore MemoryStore, embedder Embedder) error {
	if cityStore == nil {
		return errors.New("create_city city store is required")
	}
	return reg.Register(CreateCityTool(), NewCreateCityHandler(cityStore, memoryStore, embedder).Handle)
}

// CreateCityHandler executes create_city tool calls.
type CreateCityHandler struct {
	cityStore   CityStore
	memoryStore MemoryStore
	embedder    Embedder
}

// NewCreateCityHandler creates a new create_city handler.
func NewCreateCityHandler(cityStore CityStore, memoryStore MemoryStore, embedder Embedder) *CreateCityHandler {
	return &CreateCityHandler{
		cityStore:   cityStore,
		memoryStore: memoryStore,
		embedder:    embedder,
	}
}

// Handle executes the create_city tool.
func (h *CreateCityHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_city handler is nil")
	}
	if h.cityStore == nil {
		return nil, errors.New("create_city city store is required")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	population, err := parseIntArg(args, "population")
	if err != nil {
		return nil, err
	}
	districts, err := parseStringArrayArg(args, "districts")
	if err != nil {
		return nil, err
	}
	landmarks, err := parseStringArrayArg(args, "landmarks")
	if err != nil {
		return nil, err
	}
	governance, err := parseStringArg(args, "governance")
	if err != nil {
		return nil, err
	}
	economySummary, err := parseStringArg(args, "economy_summary")
	if err != nil {
		return nil, err
	}
	demographics, err := parseJSONObjectArg(args, "demographics")
	if err != nil {
		return nil, err
	}
	parentLocationID, err := parseUUIDArg(args, "location_id")
	if err != nil {
		return nil, err
	}
	createDistrictLocations, err := parseBoolArg(args, "create_district_locations")
	if err != nil {
		return nil, err
	}

	parentLocation, err := h.cityStore.GetLocationByID(ctx, dbutil.ToPgtype(parentLocationID))
	if err != nil {
		return nil, fmt.Errorf("validate location_id: %w", err)
	}

	properties := map[string]any{
		"population":         population,
		"districts":          districts,
		"landmarks":          landmarks,
		"governance":         governance,
		"economy_summary":    economySummary,
		"demographics":       demographics,
		"parent_location_id": parentLocationID.String(),
	}
	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("marshal city properties: %w", err)
	}

	locations, err := h.cityStore.ListLocationsByCampaign(ctx, parentLocation.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("list campaign locations: %w", err)
	}

	var (
		city   statedb.Location
		action string
	)
	if existing, ok := findExistingCity(locations, name, parentLocationID); ok {
		city, err = h.cityStore.UpdateLocation(ctx, statedb.UpdateLocationParams{
			Name:         name,
			Description:  pgtype.Text{String: description, Valid: true},
			Region:       deriveCityRegion(parentLocation),
			LocationType: pgtype.Text{String: "city", Valid: true},
			Properties:   propertiesJSON,
			ID:           existing.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("update city location: %w", err)
		}
		action = "updated"
	} else {
		city, err = h.cityStore.CreateLocation(ctx, statedb.CreateLocationParams{
			CampaignID:   parentLocation.CampaignID,
			Name:         name,
			Description:  pgtype.Text{String: description, Valid: true},
			Region:       deriveCityRegion(parentLocation),
			LocationType: pgtype.Text{String: "city", Valid: true},
			Properties:   propertiesJSON,
		})
		if err != nil {
			return nil, fmt.Errorf("create city location: %w", err)
		}
		action = "created"
	}

	cityID := dbutil.FromPgtype(city.ID)
	createdDistrictLocationIDs := []string{}
	if createDistrictLocations {
		createdDistrictLocationIDs, err = h.createDistrictLocations(ctx, city.CampaignID, cityID, districts, deriveCityRegion(parentLocation))
		if err != nil {
			return nil, err
		}
	}

	campaignID := dbutil.FromPgtype(city.CampaignID)
	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedCityMemory(ctx, campaignID, cityID, parentLocationID, name, description, population, districts, landmarks, governance, economySummary, demographics); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":                    cityID.String(),
			"campaign_id":           campaignID.String(),
			"name":                  name,
			"description":           description,
			"population":            population,
			"districts":             districts,
			"landmarks":             landmarks,
			"governance":            governance,
			"economy_summary":       economySummary,
			"demographics":          demographics,
			"location_id":           parentLocationID.String(),
			"action":                action,
			"district_location_ids": createdDistrictLocationIDs,
		},
		Narrative: fmt.Sprintf("City %q %s successfully.", name, action),
	}, nil
}

func (h *CreateCityHandler) createDistrictLocations(
	ctx context.Context,
	campaignID pgtype.UUID,
	cityID uuid.UUID,
	districts []string,
	region pgtype.Text,
) ([]string, error) {
	existing, err := h.cityStore.ListLocationsByCampaign(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list campaign locations for districts: %w", err)
	}

	existingSet := make(map[string]struct{})
	for _, location := range existing {
		if !location.LocationType.Valid || !strings.EqualFold(location.LocationType.String, "district") {
			continue
		}
		parentID, ok := extractUUIDProperty(location.Properties, "parent_city_id")
		if !ok || parentID != cityID {
			continue
		}
		existingSet[strings.ToLower(strings.TrimSpace(location.Name))] = struct{}{}
	}

	created := make([]string, 0, len(districts))
	for _, districtName := range districts {
		key := strings.ToLower(strings.TrimSpace(districtName))
		if _, exists := existingSet[key]; exists {
			continue
		}

		propsJSON, err := json.Marshal(map[string]any{
			"parent_city_id": cityID.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("marshal district properties: %w", err)
		}

		district, err := h.cityStore.CreateLocation(ctx, statedb.CreateLocationParams{
			CampaignID:   campaignID,
			Name:         districtName,
			Description:  pgtype.Text{String: "A district within the city.", Valid: true},
			Region:       region,
			LocationType: pgtype.Text{String: "district", Valid: true},
			Properties:   propsJSON,
		})
		if err != nil {
			return nil, fmt.Errorf("create district location %q: %w", districtName, err)
		}
		existingSet[key] = struct{}{}
		created = append(created, dbutil.FromPgtype(district.ID).String())
	}

	return created, nil
}

func (h *CreateCityHandler) embedCityMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	cityID uuid.UUID,
	parentLocationID uuid.UUID,
	name string,
	description string,
	population int,
	districts []string,
	landmarks []string,
	governance string,
	economySummary string,
	demographics map[string]any,
) error {
	demographicsJSON, err := json.Marshal(demographics)
	if err != nil {
		return fmt.Errorf("marshal city demographics for memory: %w", err)
	}

	memoryContent := fmt.Sprintf(
		"City created or updated: %s. Description: %s. Population: %d. Districts: %s. Landmarks: %s. Governance: %s. Economy: %s. Demographics: %s.",
		name,
		description,
		population,
		strings.Join(districts, ", "),
		strings.Join(landmarks, ", "),
		governance,
		economySummary,
		string(demographicsJSON),
	)

	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed city memory: %w", err)
	}

	metadata, err := json.Marshal(map[string]any{
		"city_id":              cityID.String(),
		"parent_location_id":   parentLocationID.String(),
		"districts":            districts,
		"landmarks":            landmarks,
		"population":           population,
		"governance":           governance,
		"economy_summary":      economySummary,
		"demographics_present": len(demographics) > 0,
	})
	if err != nil {
		return fmt.Errorf("marshal city memory metadata: %w", err)
	}

	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create city memory: %w", err)
	}

	return nil
}

func findExistingCity(locations []statedb.Location, cityName string, parentLocationID uuid.UUID) (statedb.Location, bool) {
	for _, location := range locations {
		if !location.LocationType.Valid || !strings.EqualFold(location.LocationType.String, "city") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(location.Name), strings.TrimSpace(cityName)) {
			continue
		}
		parentID, ok := extractUUIDProperty(location.Properties, "parent_location_id")
		if !ok || parentID != parentLocationID {
			continue
		}
		return location, true
	}
	return statedb.Location{}, false
}

func extractUUIDProperty(rawJSON []byte, key string) (uuid.UUID, bool) {
	if len(rawJSON) == 0 {
		return uuid.Nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal(rawJSON, &obj); err != nil {
		return uuid.Nil, false
	}
	value, ok := obj[key]
	if !ok {
		return uuid.Nil, false
	}
	s, ok := value.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func deriveCityRegion(parentLocation statedb.Location) pgtype.Text {
	if parentLocation.Region.Valid && strings.TrimSpace(parentLocation.Region.String) != "" {
		return parentLocation.Region
	}
	return pgtype.Text{String: parentLocation.Name, Valid: true}
}
