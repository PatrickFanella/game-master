package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubEstablishRelationshipStore struct {
	locationsByID        map[[16]byte]statedb.Location
	factionsByID         map[[16]byte]statedb.Faction
	npcsByID             map[[16]byte]statedb.Npc
	playerCharactersByID map[[16]byte]statedb.PlayerCharacter
	itemsByID            map[[16]byte]statedb.Item

	relationshipsBetween []statedb.EntityRelationship

	createRelationshipCalls []statedb.CreateRelationshipParams
	updateRelationshipCalls []statedb.UpdateRelationshipParams
}

func (s *stubEstablishRelationshipStore) GetLocationByID(_ context.Context, id pgtype.UUID) (statedb.Location, error) {
	location, ok := s.locationsByID[id.Bytes]
	if !ok {
		return statedb.Location{}, pgx.ErrNoRows
	}
	return location, nil
}

func (s *stubEstablishRelationshipStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	faction, ok := s.factionsByID[id.Bytes]
	if !ok {
		return statedb.Faction{}, pgx.ErrNoRows
	}
	return faction, nil
}

func (s *stubEstablishRelationshipStore) GetNPCByID(_ context.Context, id pgtype.UUID) (statedb.Npc, error) {
	npc, ok := s.npcsByID[id.Bytes]
	if !ok {
		return statedb.Npc{}, pgx.ErrNoRows
	}
	return npc, nil
}

func (s *stubEstablishRelationshipStore) GetPlayerCharacterByID(_ context.Context, id pgtype.UUID) (statedb.PlayerCharacter, error) {
	player, ok := s.playerCharactersByID[id.Bytes]
	if !ok {
		return statedb.PlayerCharacter{}, pgx.ErrNoRows
	}
	return player, nil
}

func (s *stubEstablishRelationshipStore) GetItemByID(_ context.Context, id pgtype.UUID) (statedb.Item, error) {
	item, ok := s.itemsByID[id.Bytes]
	if !ok {
		return statedb.Item{}, pgx.ErrNoRows
	}
	return item, nil
}

func (s *stubEstablishRelationshipStore) GetRelationshipsBetween(_ context.Context, _ statedb.GetRelationshipsBetweenParams) ([]statedb.EntityRelationship, error) {
	return s.relationshipsBetween, nil
}

func (s *stubEstablishRelationshipStore) CreateRelationship(_ context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	s.createRelationshipCalls = append(s.createRelationshipCalls, arg)
	return statedb.EntityRelationship{
		ID:               dbutil.ToPgtype(uuid.New()),
		CampaignID:       arg.CampaignID,
		SourceEntityType: arg.SourceEntityType,
		SourceEntityID:   arg.SourceEntityID,
		TargetEntityType: arg.TargetEntityType,
		TargetEntityID:   arg.TargetEntityID,
		RelationshipType: arg.RelationshipType,
		Description:      arg.Description,
		Strength:         arg.Strength,
	}, nil
}

func (s *stubEstablishRelationshipStore) UpdateRelationship(_ context.Context, arg statedb.UpdateRelationshipParams) (statedb.EntityRelationship, error) {
	s.updateRelationshipCalls = append(s.updateRelationshipCalls, arg)
	return statedb.EntityRelationship{
		ID:               arg.ID,
		CampaignID:       arg.CampaignID,
		RelationshipType: arg.RelationshipType,
		Description:      arg.Description,
		Strength:         arg.Strength,
	}, nil
}

func TestRegisterEstablishRelationship(t *testing.T) {
	reg := NewRegistry()
	store := &stubEstablishRelationshipStore{}

	if err := RegisterEstablishRelationship(reg, store); err != nil {
		t.Fatalf("register establish_relationship: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != establishRelationshipToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, establishRelationshipToolName)
	}
}

func TestEstablishRelationshipHandleVariousEntityTypes(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	npcID := uuid.New()
	factionID := uuid.New()
	playerID := uuid.New()
	itemID := uuid.New()

	store := &stubEstablishRelationshipStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {ID: dbutil.ToPgtype(currentLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		npcsByID: map[[16]byte]statedb.Npc{
			dbutil.ToPgtype(npcID).Bytes: {ID: dbutil.ToPgtype(npcID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		factionsByID: map[[16]byte]statedb.Faction{
			dbutil.ToPgtype(factionID).Bytes: {ID: dbutil.ToPgtype(factionID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		playerCharactersByID: map[[16]byte]statedb.PlayerCharacter{
			dbutil.ToPgtype(playerID).Bytes: {ID: dbutil.ToPgtype(playerID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		itemsByID: map[[16]byte]statedb.Item{
			dbutil.ToPgtype(itemID).Bytes: {ID: dbutil.ToPgtype(itemID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
	}
	h := NewEstablishRelationshipHandler(store)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)

	testCases := []struct {
		name       string
		sourceType string
		sourceID   uuid.UUID
		targetType string
		targetID   uuid.UUID
	}{
		{name: "npc to faction", sourceType: "npc", sourceID: npcID, targetType: "faction", targetID: factionID},
		{name: "location to npc", sourceType: "location", sourceID: currentLocationID, targetType: "npc", targetID: npcID},
		{name: "player alias to item", sourceType: "player", sourceID: playerID, targetType: "item", targetID: itemID},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store.createRelationshipCalls = nil

			result, err := h.Handle(ctx, map[string]any{
				"source_entity_type": tc.sourceType,
				"source_entity_id":   tc.sourceID.String(),
				"target_entity_type": tc.targetType,
				"target_entity_id":   tc.targetID.String(),
				"relationship_type":  "mentor",
				"description":        "Long-standing bond.",
			})
			if err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(store.createRelationshipCalls) != 1 {
				t.Fatalf("CreateRelationship call count = %d, want 1", len(store.createRelationshipCalls))
			}
			createCall := store.createRelationshipCalls[0]
			if createCall.Strength.Int32 != 5 || !createCall.Strength.Valid {
				t.Fatalf("default strength = %v (valid=%v), want 5 and valid", createCall.Strength.Int32, createCall.Strength.Valid)
			}
			if result.Data["relationship_type"] != "mentor" {
				t.Fatalf("relationship_type = %v, want mentor", result.Data["relationship_type"])
			}
		})
	}
}

func TestEstablishRelationshipHandleUpdatesDuplicateRelationship(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	sourceID := uuid.New()
	targetID := uuid.New()
	relationshipID := uuid.New()

	store := &stubEstablishRelationshipStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {ID: dbutil.ToPgtype(currentLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
			dbutil.ToPgtype(targetID).Bytes:          {ID: dbutil.ToPgtype(targetID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		npcsByID: map[[16]byte]statedb.Npc{
			dbutil.ToPgtype(sourceID).Bytes: {ID: dbutil.ToPgtype(sourceID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		relationshipsBetween: []statedb.EntityRelationship{
			{
				ID:               dbutil.ToPgtype(relationshipID),
				CampaignID:       dbutil.ToPgtype(campaignID),
				SourceEntityType: "npc",
				SourceEntityID:   dbutil.ToPgtype(sourceID),
				TargetEntityType: "location",
				TargetEntityID:   dbutil.ToPgtype(targetID),
				RelationshipType: "rival",
				Strength:         pgtype.Int4{Int32: 4, Valid: true},
			},
		},
	}
	h := NewEstablishRelationshipHandler(store)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)

	result, err := h.Handle(ctx, map[string]any{
		"source_entity_type": "npc",
		"source_entity_id":   sourceID.String(),
		"target_entity_type": "location",
		"target_entity_id":   targetID.String(),
		"relationship_type":  "RIVAL",
		"description":        "Their feud just escalated.",
		"strength":           9,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(store.createRelationshipCalls) != 0 {
		t.Fatalf("CreateRelationship call count = %d, want 0", len(store.createRelationshipCalls))
	}
	if len(store.updateRelationshipCalls) != 1 {
		t.Fatalf("UpdateRelationship call count = %d, want 1", len(store.updateRelationshipCalls))
	}
	if store.updateRelationshipCalls[0].ID != dbutil.ToPgtype(relationshipID) {
		t.Fatalf("updated relationship id mismatch")
	}
	if store.updateRelationshipCalls[0].Strength.Int32 != 9 || !store.updateRelationshipCalls[0].Strength.Valid {
		t.Fatalf("updated strength = %d (valid=%v), want 9 valid", store.updateRelationshipCalls[0].Strength.Int32, store.updateRelationshipCalls[0].Strength.Valid)
	}
	if updated, ok := result.Data["updated"].(bool); !ok || !updated {
		t.Fatalf("result updated = %v, want true", result.Data["updated"])
	}
}

func TestEstablishRelationshipHandleUpdatesDuplicateRelationshipPreservesStrengthWhenOmitted(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	sourceID := uuid.New()
	targetID := uuid.New()
	relationshipID := uuid.New()

	store := &stubEstablishRelationshipStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {ID: dbutil.ToPgtype(currentLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
			dbutil.ToPgtype(targetID).Bytes:          {ID: dbutil.ToPgtype(targetID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		npcsByID: map[[16]byte]statedb.Npc{
			dbutil.ToPgtype(sourceID).Bytes: {ID: dbutil.ToPgtype(sourceID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		relationshipsBetween: []statedb.EntityRelationship{
			{
				ID:               dbutil.ToPgtype(relationshipID),
				CampaignID:       dbutil.ToPgtype(campaignID),
				SourceEntityType: "npc",
				SourceEntityID:   dbutil.ToPgtype(sourceID),
				TargetEntityType: "location",
				TargetEntityID:   dbutil.ToPgtype(targetID),
				RelationshipType: "rival",
				Strength:         pgtype.Int4{Int32: 8, Valid: true},
			},
		},
	}
	h := NewEstablishRelationshipHandler(store)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)

	_, err := h.Handle(ctx, map[string]any{
		"source_entity_type": "npc",
		"source_entity_id":   sourceID.String(),
		"target_entity_type": "location",
		"target_entity_id":   targetID.String(),
		"relationship_type":  "rival",
		"description":        "Still rivals.",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(store.updateRelationshipCalls) != 1 {
		t.Fatalf("UpdateRelationship call count = %d, want 1", len(store.updateRelationshipCalls))
	}
	if store.updateRelationshipCalls[0].Strength.Int32 != 8 || !store.updateRelationshipCalls[0].Strength.Valid {
		t.Fatalf("updated strength = %d (valid=%v), want existing 8 valid", store.updateRelationshipCalls[0].Strength.Int32, store.updateRelationshipCalls[0].Strength.Valid)
	}
}

func TestEstablishRelationshipHandleValidatesEntityExistenceByType(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	store := &stubEstablishRelationshipStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {ID: dbutil.ToPgtype(currentLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
	}
	h := NewEstablishRelationshipHandler(store)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)

	_, err := h.Handle(ctx, map[string]any{
		"source_entity_type": "npc",
		"source_entity_id":   uuid.New().String(),
		"target_entity_type": "location",
		"target_entity_id":   currentLocationID.String(),
		"relationship_type":  "ally",
		"description":        "Trusted companion.",
	})
	if err == nil || !strings.Contains(err.Error(), "source entity not found: npc") {
		t.Fatalf("error = %v, want source npc not found", err)
	}
}

func TestEstablishRelationshipHandleStrengthValidation(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	sourceID := uuid.New()
	targetID := uuid.New()

	store := &stubEstablishRelationshipStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {ID: dbutil.ToPgtype(currentLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		npcsByID: map[[16]byte]statedb.Npc{
			dbutil.ToPgtype(sourceID).Bytes: {ID: dbutil.ToPgtype(sourceID), CampaignID: dbutil.ToPgtype(campaignID)},
			dbutil.ToPgtype(targetID).Bytes: {ID: dbutil.ToPgtype(targetID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
	}
	h := NewEstablishRelationshipHandler(store)
	ctx := WithCurrentLocationID(context.Background(), currentLocationID)

	_, err := h.Handle(ctx, map[string]any{
		"source_entity_type": "npc",
		"source_entity_id":   sourceID.String(),
		"target_entity_type": "npc",
		"target_entity_id":   targetID.String(),
		"relationship_type":  "ally",
		"description":        "Old friends.",
		"strength":           11,
	})
	if err == nil || !strings.Contains(err.Error(), "strength must be between 1 and 10") {
		t.Fatalf("error = %v, want strength range error", err)
	}
}

func TestRegisterEstablishRelationshipRequiresStore(t *testing.T) {
	err := RegisterEstablishRelationship(NewRegistry(), nil)
	if err == nil || !strings.Contains(err.Error(), "establish_relationship store is required") {
		t.Fatalf("error = %v, want missing store error", err)
	}
}

var _ EstablishRelationshipStore = (*stubEstablishRelationshipStore)(nil)
