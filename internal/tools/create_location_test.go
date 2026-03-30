package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubLocationStore struct {
	locationsByID      map[[16]byte]statedb.Location
	createLocationCall []statedb.CreateLocationParams
	createConnection   []statedb.CreateConnectionParams
	createLocationRes  statedb.Location
	connectionResults  []statedb.LocationConnection
	getLocationErr     error
	createLocationErr  error
	createConnErr      error
}

var _ LocationStore = (*stubLocationStore)(nil)

func (s *stubLocationStore) CreateLocation(_ context.Context, arg statedb.CreateLocationParams) (statedb.Location, error) {
	if s.createLocationErr != nil {
		return statedb.Location{}, s.createLocationErr
	}
	s.createLocationCall = append(s.createLocationCall, arg)
	if s.createLocationRes.ID.Valid {
		return s.createLocationRes, nil
	}
	return statedb.Location{
		ID:           dbutil.ToPgtype(uuid.New()),
		CampaignID:   arg.CampaignID,
		Name:         arg.Name,
		Description:  arg.Description,
		Region:       arg.Region,
		LocationType: arg.LocationType,
		Properties:   arg.Properties,
	}, nil
}

func (s *stubLocationStore) GetLocationByID(_ context.Context, id pgtype.UUID) (statedb.Location, error) {
	if s.getLocationErr != nil {
		return statedb.Location{}, s.getLocationErr
	}
	location, ok := s.locationsByID[id.Bytes]
	if !ok {
		return statedb.Location{}, errors.New("location not found")
	}
	return location, nil
}

func (s *stubLocationStore) CreateConnection(_ context.Context, arg statedb.CreateConnectionParams) (statedb.LocationConnection, error) {
	if s.createConnErr != nil {
		return statedb.LocationConnection{}, s.createConnErr
	}
	s.createConnection = append(s.createConnection, arg)
	if len(s.connectionResults) > 0 {
		out := s.connectionResults[0]
		s.connectionResults = s.connectionResults[1:]
		return out, nil
	}
	return statedb.LocationConnection{
		ID:             dbutil.ToPgtype(uuid.New()),
		FromLocationID: arg.FromLocationID,
		ToLocationID:   arg.ToLocationID,
		Description:    arg.Description,
		Bidirectional:  arg.Bidirectional,
		CampaignID:     arg.CampaignID,
	}, nil
}

func TestRegisterCreateLocation(t *testing.T) {
	reg := NewRegistry()
	store := &stubLocationStore{}
	if err := RegisterCreateLocation(reg, store, nil, nil); err != nil {
		t.Fatalf("register create_location: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createLocationToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createLocationToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"name", "description", "region", "location_type"} {
		if _, exists := requiredSet[field]; !exists {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateLocationHandleSuccessWithoutConnections(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	newLocationID := uuid.New()

	store := &stubLocationStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Current",
			},
		},
		createLocationRes: statedb.Location{
			ID:           dbutil.ToPgtype(newLocationID),
			CampaignID:   dbutil.ToPgtype(campaignID),
			Name:         "Whispering Grove",
			Description:  pgtype.Text{String: "A mossy grove", Valid: true},
			Region:       pgtype.Text{String: "Emerald Wilds", Valid: true},
			LocationType: pgtype.Text{String: "wilderness", Valid: true},
		},
	}

	h := NewCreateLocationHandler(store, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)
	got, err := h.Handle(ctx, map[string]any{
		"name":          "Whispering Grove",
		"description":   "A mossy grove",
		"region":        "Emerald Wilds",
		"location_type": "wilderness",
		"properties": map[string]any{
			"danger_level": "low",
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.createLocationCall) != 1 {
		t.Fatalf("CreateLocation call count = %d, want 1", len(store.createLocationCall))
	}
	if store.createLocationCall[0].Name != "Whispering Grove" {
		t.Fatalf("CreateLocation name = %q, want Whispering Grove", store.createLocationCall[0].Name)
	}
	if len(store.createConnection) != 0 {
		t.Fatalf("CreateConnection call count = %d, want 0", len(store.createConnection))
	}

	if got.Data["id"] != newLocationID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], newLocationID)
	}
	if got.Data["location_type"] != "wilderness" {
		t.Fatalf("result location_type = %v, want wilderness", got.Data["location_type"])
	}
	if got.Data["region"] != "Emerald Wilds" {
		t.Fatalf("result region = %v, want Emerald Wilds", got.Data["region"])
	}
}

func TestCreateLocationHandleSuccessWithBidirectionalConnectionsAndMemory(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	targetLocationID := uuid.New()
	newLocationID := uuid.New()
	conn1ID := uuid.New()

	store := &stubLocationStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Current",
			},
			dbutil.ToPgtype(targetLocationID).Bytes: {
				ID:         dbutil.ToPgtype(targetLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Old Keep",
			},
		},
		createLocationRes: statedb.Location{
			ID:           dbutil.ToPgtype(newLocationID),
			CampaignID:   dbutil.ToPgtype(campaignID),
			Name:         "Old Road",
			Description:  pgtype.Text{String: "A weathered road", Valid: true},
			Region:       pgtype.Text{String: "Borderlands", Valid: true},
			LocationType: pgtype.Text{String: "wilderness", Valid: true},
		},
		connectionResults: []statedb.LocationConnection{
			{
				ID:             dbutil.ToPgtype(conn1ID),
				FromLocationID: dbutil.ToPgtype(newLocationID),
				ToLocationID:   dbutil.ToPgtype(targetLocationID),
				Description:    pgtype.Text{String: "A worn trail", Valid: true},
				Bidirectional:  true,
				CampaignID:     dbutil.ToPgtype(campaignID),
			},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}

	h := NewCreateLocationHandler(store, memStore, embedder)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)
	got, err := h.Handle(ctx, map[string]any{
		"name":          "Old Road",
		"description":   "A weathered road",
		"region":        "Borderlands",
		"location_type": "wilderness",
		"connections": []any{
			map[string]any{
				"location_id":   targetLocationID.String(),
				"description":   "A worn trail",
				"bidirectional": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.createConnection) != 1 {
		t.Fatalf("CreateConnection call count = %d, want 1", len(store.createConnection))
	}
	conn := store.createConnection[0]
	if dbutil.FromPgtype(conn.FromLocationID) != newLocationID {
		t.Fatalf("forward connection from_location_id mismatch")
	}
	if dbutil.FromPgtype(conn.ToLocationID) != targetLocationID {
		t.Fatalf("forward connection to_location_id mismatch")
	}

	connectionsData, ok := got.Data["connections"].([]map[string]any)
	if !ok {
		t.Fatalf("result connections type = %T, want []map[string]any", got.Data["connections"])
	}
	if len(connectionsData) != 1 {
		t.Fatalf("result connections count = %d, want 1", len(connectionsData))
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("memory_type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeWorldFact)
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
}

func TestCreateLocationValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	targetLocationID := uuid.New()

	baseStore := &stubLocationStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Current",
			},
			dbutil.ToPgtype(targetLocationID).Bytes: {
				ID:         dbutil.ToPgtype(targetLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Target",
			},
		},
	}

	baseArgs := map[string]any{
		"name":          "Old Tower",
		"description":   "A crumbling watchtower",
		"region":        "Western March",
		"location_type": "building",
	}

	t.Run("missing required field", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, nil, nil)
		for _, tc := range []struct {
			field string
			want  string
		}{
			{field: "name", want: "name is required"},
			{field: "description", want: "description is required"},
			{field: "region", want: "region is required"},
			{field: "location_type", want: "location_type is required"},
		} {
			args := copyArgs(baseArgs)
			delete(args, tc.field)

			_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("field %s: error = %v, want %q", tc.field, err, tc.want)
			}
		}
	})

	t.Run("missing current location context", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, nil, nil)
		_, err := h.Handle(context.Background(), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "requires current location id in context") {
			t.Fatalf("error = %v, want current location context error", err)
		}
	})

	t.Run("invalid connection target", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, nil, nil)
		args := copyArgs(baseArgs)
		args["connections"] = []any{map[string]any{
			"location_id":   uuid.New().String(),
			"description":   "Path",
			"bidirectional": false,
		}}
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "validate connections[0].location_id") {
			t.Fatalf("error = %v, want connected-location validation error", err)
		}
		if len(baseStore.createLocationCall) != 0 {
			t.Fatalf("CreateLocation call count = %d, want 0", len(baseStore.createLocationCall))
		}
	})

	t.Run("connection target outside campaign", func(t *testing.T) {
		otherCampaign := uuid.New()
		store := &stubLocationStore{
			locationsByID: map[[16]byte]statedb.Location{
				dbutil.ToPgtype(currentLocationID).Bytes: {
					ID:         dbutil.ToPgtype(currentLocationID),
					CampaignID: dbutil.ToPgtype(campaignID),
					Name:       "Current",
				},
				dbutil.ToPgtype(targetLocationID).Bytes: {
					ID:         dbutil.ToPgtype(targetLocationID),
					CampaignID: dbutil.ToPgtype(otherCampaign),
					Name:       "Target",
				},
			},
		}
		h := NewCreateLocationHandler(store, nil, nil)
		args := copyArgs(baseArgs)
		args["connections"] = []any{map[string]any{
			"location_id":   targetLocationID.String(),
			"description":   "Path",
			"bidirectional": false,
		}}
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "must belong to active campaign") {
			t.Fatalf("error = %v, want campaign-scoped validation", err)
		}
		if len(store.createLocationCall) != 0 {
			t.Fatalf("CreateLocation call count = %d, want 0", len(store.createLocationCall))
		}
	})

	t.Run("embedder error", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, &stubMemoryStore{}, &stubEmbedder{err: errors.New("embed failed")})
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "embed location memory") {
			t.Fatalf("error = %v, want embed context", err)
		}
	})

	t.Run("memory store error", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, &stubMemoryStore{err: errors.New("insert failed")}, &stubEmbedder{vector: []float32{0.1}})
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "create location memory") {
			t.Fatalf("error = %v, want memory creation context", err)
		}
	})

	t.Run("properties must be object", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, nil, nil)
		args := copyArgs(baseArgs)
		args["properties"] = "bad"
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "properties must be an object") {
			t.Fatalf("error = %v, want properties object validation", err)
		}
	})

	t.Run("connections must be array", func(t *testing.T) {
		h := NewCreateLocationHandler(baseStore, nil, nil)
		args := copyArgs(baseArgs)
		args["connections"] = "bad"
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "connections must be an array") {
			t.Fatalf("error = %v, want connections array validation", err)
		}
	})
}

func TestCreateLocationMemoryMetadata(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	newLocationID := uuid.New()

	store := &stubLocationStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Current",
			},
		},
		createLocationRes: statedb.Location{
			ID:           dbutil.ToPgtype(newLocationID),
			CampaignID:   dbutil.ToPgtype(campaignID),
			Name:         "Ancient Vault",
			Description:  pgtype.Text{String: "Sealed chamber", Valid: true},
			Region:       pgtype.Text{String: "Underdeep", Valid: true},
			LocationType: pgtype.Text{String: "dungeon", Valid: true},
		},
	}
	memStore := &stubMemoryStore{}
	h := NewCreateLocationHandler(store, memStore, &stubEmbedder{vector: []float32{1.0}})

	_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), map[string]any{
		"name":          "Ancient Vault",
		"description":   "Sealed chamber",
		"region":        "Underdeep",
		"location_type": "dungeon",
		"properties": map[string]any{
			"locked": true,
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["location_id"] != newLocationID.String() {
		t.Fatalf("metadata.location_id = %v, want %s", metadata["location_id"], newLocationID)
	}
}
