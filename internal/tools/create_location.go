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

const createLocationToolName = "create_location"

// LocationStore persists locations and location graph connections.
type LocationStore interface {
	CreateLocation(ctx context.Context, arg statedb.CreateLocationParams) (statedb.Location, error)
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
	CreateConnection(ctx context.Context, arg statedb.CreateConnectionParams) (statedb.LocationConnection, error)
}

// CreateLocationTool returns the create_location tool definition and JSON schema.
func CreateLocationTool() llm.Tool {
	return llm.Tool{
		Name:        createLocationToolName,
		Description: "Create a location and optionally create directed or bidirectional connections to existing locations.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Location name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Location description.",
				},
				"region": map[string]any{
					"type":        "string",
					"description": "Region this location belongs to.",
				},
				"location_type": map[string]any{
					"type":        "string",
					"description": "Location type such as city, dungeon, wilderness, or building.",
				},
				"properties": map[string]any{
					"type":        "object",
					"description": "Additional JSON properties for the location.",
				},
				"connections": map[string]any{
					"type":        "array",
					"description": "Connections to existing locations.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location_id": map[string]any{
								"type":        "string",
								"description": "Connected location UUID.",
							},
							"description": map[string]any{
								"type":        "string",
								"description": "Connection description.",
							},
							"bidirectional": map[string]any{
								"type":        "boolean",
								"description": "When true, creates a reverse connection as well.",
							},
						},
						"required":             []string{"location_id", "description", "bidirectional"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"name", "description", "region", "location_type"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateLocation registers the create_location tool and handler.
func RegisterCreateLocation(reg *Registry, locationStore LocationStore, memoryStore MemoryStore, embedder Embedder) error {
	if locationStore == nil {
		return errors.New("create_location location store is required")
	}
	return reg.Register(CreateLocationTool(), NewCreateLocationHandler(locationStore, memoryStore, embedder).Handle)
}

// CreateLocationHandler executes create_location tool calls.
type CreateLocationHandler struct {
	locationStore LocationStore
	memoryStore   MemoryStore
	embedder      Embedder
}

// NewCreateLocationHandler creates a new create_location handler.
func NewCreateLocationHandler(locationStore LocationStore, memoryStore MemoryStore, embedder Embedder) *CreateLocationHandler {
	return &CreateLocationHandler{
		locationStore: locationStore,
		memoryStore:   memoryStore,
		embedder:      embedder,
	}
}

type locationConnectionInput struct {
	LocationID    uuid.UUID
	Description   string
	Bidirectional bool
}

// Handle executes the create_location tool.
func (h *CreateLocationHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_location handler is nil")
	}
	if h.locationStore == nil {
		return nil, errors.New("create_location location store is required")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	region, err := parseStringArg(args, "region")
	if err != nil {
		return nil, err
	}
	locationType, err := parseStringArg(args, "location_type")
	if err != nil {
		return nil, err
	}
	properties, err := parseOptionalJSONObjectArg(args, "properties")
	if err != nil {
		return nil, err
	}
	connections, err := parseLocationConnectionsArg(args, "connections")
	if err != nil {
		return nil, err
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("create_location requires current location id in context")
	}
	currentLocation, err := h.locationStore.GetLocationByID(ctx, dbutil.ToPgtype(currentLocationID))
	if err != nil {
		return nil, fmt.Errorf("resolve campaign from current location: %w", err)
	}
	if err := h.validateConnectionTargets(ctx, currentLocation.CampaignID, connections); err != nil {
		return nil, err
	}

	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("marshal location properties: %w", err)
	}

	createdLocation, err := h.locationStore.CreateLocation(ctx, statedb.CreateLocationParams{
		CampaignID:   currentLocation.CampaignID,
		Name:         name,
		Description:  pgtype.Text{String: description, Valid: true},
		Region:       pgtype.Text{String: region, Valid: true},
		LocationType: pgtype.Text{String: locationType, Valid: true},
		Properties:   propertiesJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create location: %w", err)
	}

	createdConnections, err := h.createConnections(ctx, createdLocation, connections)
	if err != nil {
		return nil, err
	}

	campaignID := dbutil.FromPgtype(createdLocation.CampaignID)
	locationID := dbutil.FromPgtype(createdLocation.ID)
	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedLocationMemory(ctx, campaignID, locationID, name, description, region, locationType, properties, len(createdConnections)); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":            locationID.String(),
			"campaign_id":   campaignID.String(),
			"name":          createdLocation.Name,
			"description":   createdLocation.Description.String,
			"region":        createdLocation.Region.String,
			"location_type": createdLocation.LocationType.String,
			"properties":    properties,
			"connections":   createdConnections,
		},
		Narrative: fmt.Sprintf("Location %q created successfully.", createdLocation.Name),
	}, nil
}

func (h *CreateLocationHandler) validateConnectionTargets(ctx context.Context, campaignID pgtype.UUID, connections []locationConnectionInput) error {
	for i, conn := range connections {
		location, err := h.locationStore.GetLocationByID(ctx, dbutil.ToPgtype(conn.LocationID))
		if err != nil {
			return fmt.Errorf("validate connections[%d].location_id: %w", i, err)
		}
		if dbutil.FromPgtype(location.CampaignID) != dbutil.FromPgtype(campaignID) {
			return fmt.Errorf("connections[%d].location_id must belong to active campaign", i)
		}
	}
	return nil
}

func (h *CreateLocationHandler) createConnections(ctx context.Context, location statedb.Location, connections []locationConnectionInput) ([]map[string]any, error) {
	created := make([]map[string]any, 0, len(connections))

	for i, conn := range connections {
		primary, err := h.locationStore.CreateConnection(ctx, statedb.CreateConnectionParams{
			FromLocationID: location.ID,
			ToLocationID:   dbutil.ToPgtype(conn.LocationID),
			Description:    pgtype.Text{String: conn.Description, Valid: true},
			Bidirectional:  conn.Bidirectional,
			CampaignID:     location.CampaignID,
		})
		if err != nil {
			return nil, fmt.Errorf("create connections[%d]: %w", i, err)
		}
		created = append(created, connectionToResult(primary))
	}

	return created, nil
}

func (h *CreateLocationHandler) embedLocationMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	locationID uuid.UUID,
	name string,
	description string,
	region string,
	locationType string,
	properties map[string]any,
	connectionCount int,
) error {
	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("marshal location properties for memory: %w", err)
	}

	memoryContent := fmt.Sprintf(
		"Location created: %s. Description: %s. Region: %s. Type: %s. Properties: %s.",
		name,
		description,
		region,
		locationType,
		string(propertiesJSON),
	)
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed location memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"location_id":        locationID.String(),
		"region":             region,
		"location_type":      locationType,
		"connection_count":   connectionCount,
		"properties_present": len(properties) > 0,
	})
	if err != nil {
		return fmt.Errorf("marshal location memory metadata: %w", err)
	}

	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create location memory: %w", err)
	}

	return nil
}

func parseOptionalJSONObjectArg(args map[string]any, key string) (map[string]any, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return obj, nil
}

func parseLocationConnectionsArg(args map[string]any, key string) ([]locationConnectionInput, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return []locationConnectionInput{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]locationConnectionInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}

		locationID, err := parseUUIDFromNestedObject(obj, "location_id", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		description, err := parseObjectStringArg(obj, "description", fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}

		bidirectionalRaw, exists := obj["bidirectional"]
		if !exists {
			return nil, fmt.Errorf("%s[%d].bidirectional is required", key, i)
		}
		bidirectional, ok := bidirectionalRaw.(bool)
		if !ok {
			return nil, fmt.Errorf("%s[%d].bidirectional must be a boolean", key, i)
		}

		out = append(out, locationConnectionInput{
			LocationID:    locationID,
			Description:   strings.TrimSpace(description),
			Bidirectional: bidirectional,
		})
	}

	return out, nil
}

func connectionToResult(connection statedb.LocationConnection) map[string]any {
	return map[string]any{
		"id":               dbutil.FromPgtype(connection.ID).String(),
		"from_location_id": dbutil.FromPgtype(connection.FromLocationID).String(),
		"to_location_id":   dbutil.FromPgtype(connection.ToLocationID).String(),
		"description":      connection.Description.String,
		"bidirectional":    connection.Bidirectional,
	}
}
