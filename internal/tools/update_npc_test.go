package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

type stubUpdateNPCStore struct {
	npcs      map[uuid.UUID]*domain.NPC
	locations map[uuid.UUID]uuid.UUID

	lastUpdated *domain.NPC

	getErr      error
	locationErr error
	updateErr   error

	lastLocationCheckCampaignID uuid.UUID
}

var _ UpdateNPCStore = (*stubUpdateNPCStore)(nil)

func (s *stubUpdateNPCStore) GetNPCByID(_ context.Context, npcID uuid.UUID) (*domain.NPC, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	npc, ok := s.npcs[npcID]
	if !ok {
		return nil, nil
	}
	copy := *npc
	return &copy, nil
}

func (s *stubUpdateNPCStore) LocationExistsInCampaign(_ context.Context, locationID, campaignID uuid.UUID) (bool, error) {
	if s.locationErr != nil {
		return false, s.locationErr
	}
	s.lastLocationCheckCampaignID = campaignID
	locationCampaignID, ok := s.locations[locationID]
	if !ok {
		return false, nil
	}
	return locationCampaignID == campaignID, nil
}

func (s *stubUpdateNPCStore) UpdateNPC(_ context.Context, npc domain.NPC) (*domain.NPC, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	copy := npc
	s.lastUpdated = &copy
	s.npcs[npc.ID] = &copy
	return &copy, nil
}

func TestRegisterUpdateNPC(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterUpdateNPC(reg, &stubUpdateNPCStore{}); err != nil {
		t.Fatalf("register update_npc: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != updateNPCToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, updateNPCToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 1 || required[0] != "npc_id" {
		t.Fatalf("required schema = %#v, want [npc_id]", required)
	}
}

func TestUpdateNPCHandleDescriptionOnlyUpdatesProvidedField(t *testing.T) {
	npcID := uuid.New()
	locationID := uuid.New()
	store := &stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {
				ID:          npcID,
				CampaignID:  uuid.New(),
				Name:        "Scout Nyra",
				Description: "Original description",
				Disposition: 12,
				LocationID:  &locationID,
				Alive:       true,
			},
		},
	}
	h := NewUpdateNPCHandler(store)

	got, err := h.Handle(context.Background(), map[string]any{
		"npc_id":      npcID.String(),
		"description": "Updated description",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.lastUpdated == nil {
		t.Fatal("expected UpdateNPC call")
	}
	if store.lastUpdated.Description != "Updated description" {
		t.Fatalf("description = %q, want updated value", store.lastUpdated.Description)
	}
	if store.lastUpdated.Disposition != 12 {
		t.Fatalf("disposition changed unexpectedly: %d", store.lastUpdated.Disposition)
	}
	if !store.lastUpdated.Alive {
		t.Fatal("alive changed unexpectedly")
	}
	if store.lastUpdated.LocationID == nil || *store.lastUpdated.LocationID != locationID {
		t.Fatalf("location changed unexpectedly: %v", store.lastUpdated.LocationID)
	}
	if got.Data["description"] != "Updated description" {
		t.Fatalf("result description = %v, want updated value", got.Data["description"])
	}
}

func TestUpdateNPCHandleDispositionClamped(t *testing.T) {
	t.Run("clamps high values to 100", func(t *testing.T) {
		npcID := uuid.New()
		store := &stubUpdateNPCStore{
			npcs: map[uuid.UUID]*domain.NPC{
				npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Guard", Disposition: 0, Alive: true},
			},
		}
		h := NewUpdateNPCHandler(store)

		got, err := h.Handle(context.Background(), map[string]any{
			"npc_id":      npcID.String(),
			"disposition": 500,
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if store.lastUpdated == nil {
			t.Fatal("expected UpdateNPC call")
		}
		if store.lastUpdated.Disposition != 100 {
			t.Fatalf("disposition = %d, want 100", store.lastUpdated.Disposition)
		}
		if got.Data["disposition"] != 100 {
			t.Fatalf("result disposition = %v, want 100", got.Data["disposition"])
		}
	})

	t.Run("clamps low values to -100", func(t *testing.T) {
		npcID := uuid.New()
		store := &stubUpdateNPCStore{
			npcs: map[uuid.UUID]*domain.NPC{
				npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Guard", Disposition: 0, Alive: true},
			},
		}
		h := NewUpdateNPCHandler(store)

		got, err := h.Handle(context.Background(), map[string]any{
			"npc_id":      npcID.String(),
			"disposition": -500,
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if store.lastUpdated == nil {
			t.Fatal("expected UpdateNPC call")
		}
		if store.lastUpdated.Disposition != -100 {
			t.Fatalf("disposition = %d, want -100", store.lastUpdated.Disposition)
		}
		if got.Data["disposition"] != -100 {
			t.Fatalf("result disposition = %v, want -100", got.Data["disposition"])
		}
	})
}

func TestUpdateNPCHandleLocationUpdate(t *testing.T) {
	npcID := uuid.New()
	oldLocationID := uuid.New()
	newLocationID := uuid.New()
	campaignID := uuid.New()
	store := &stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {ID: npcID, CampaignID: campaignID, Name: "Guide", LocationID: &oldLocationID, Alive: true},
		},
		locations: map[uuid.UUID]uuid.UUID{newLocationID: campaignID},
	}
	h := NewUpdateNPCHandler(store)

	got, err := h.Handle(context.Background(), map[string]any{
		"npc_id":      npcID.String(),
		"location_id": newLocationID.String(),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.lastUpdated == nil || store.lastUpdated.LocationID == nil || *store.lastUpdated.LocationID != newLocationID {
		t.Fatalf("location_id = %v, want %s", store.lastUpdated.LocationID, newLocationID)
	}
	if got.Data["location_id"] != newLocationID.String() {
		t.Fatalf("result location_id = %v, want %s", got.Data["location_id"], newLocationID)
	}
	if store.lastLocationCheckCampaignID != store.npcs[npcID].CampaignID {
		t.Fatalf("campaign_id passed to location check = %s, want %s", store.lastLocationCheckCampaignID, store.npcs[npcID].CampaignID)
	}
}

func TestUpdateNPCHandleOmitsLocationWhenUnset(t *testing.T) {
	npcID := uuid.New()
	store := &stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {
				ID:          npcID,
				CampaignID:  uuid.New(),
				Name:        "Hermit",
				Description: "A lone figure.",
				Disposition: 5,
				LocationID:  nil,
				Alive:       true,
			},
		},
	}
	h := NewUpdateNPCHandler(store)

	got, err := h.Handle(context.Background(), map[string]any{
		"npc_id":      npcID.String(),
		"description": "A quiet lone figure.",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if _, exists := got.Data["location_id"]; exists {
		t.Fatalf("did not expect location_id in result data when npc has no location: %#v", got.Data)
	}
}

func TestUpdateNPCHandleAliveFalseKillsNPC(t *testing.T) {
	npcID := uuid.New()
	store := &stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Bandit", Alive: true},
		},
	}
	h := NewUpdateNPCHandler(store)

	got, err := h.Handle(context.Background(), map[string]any{
		"npc_id": npcID.String(),
		"alive":  false,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.lastUpdated == nil {
		t.Fatal("expected UpdateNPC call")
	}
	if store.lastUpdated.Alive {
		t.Fatal("expected alive=false update")
	}
	if got.Data["alive"] != false {
		t.Fatalf("result alive = %v, want false", got.Data["alive"])
	}
}

func TestUpdateNPCHandleUnknownNPC(t *testing.T) {
	h := NewUpdateNPCHandler(&stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{},
	})

	_, err := h.Handle(context.Background(), map[string]any{
		"npc_id": uuid.New().String(),
	})
	if err == nil {
		t.Fatal("expected unknown npc error")
	}
	if !strings.Contains(err.Error(), "npc_id does not reference an existing npc") {
		t.Fatalf("error = %v, want unknown npc message", err)
	}
}

func TestUpdateNPCHandleRequiresAtLeastOneUpdateField(t *testing.T) {
	npcID := uuid.New()
	h := NewUpdateNPCHandler(&stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Watcher", Alive: true},
		},
	})

	_, err := h.Handle(context.Background(), map[string]any{
		"npc_id": npcID.String(),
	})
	if err == nil {
		t.Fatal("expected no-fields-provided error")
	}
	if !strings.Contains(err.Error(), "at least one field must be provided") {
		t.Fatalf("error = %v, want no-fields-provided message", err)
	}
}

func TestUpdateNPCHandleUnknownLocationRejected(t *testing.T) {
	npcID := uuid.New()
	store := &stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Merchant", Alive: true},
		},
		locations: map[uuid.UUID]uuid.UUID{},
	}
	h := NewUpdateNPCHandler(store)

	_, err := h.Handle(context.Background(), map[string]any{
		"npc_id":      npcID.String(),
		"location_id": uuid.New().String(),
	})
	if err == nil {
		t.Fatal("expected unknown location error")
	}
	if !strings.Contains(err.Error(), "location_id does not reference an existing location in npc campaign") {
		t.Fatalf("error = %v, want unknown location message", err)
	}
	if store.lastUpdated != nil {
		t.Fatal("did not expect UpdateNPC to be called")
	}
}

func TestUpdateNPCHandleLocationInDifferentCampaignRejected(t *testing.T) {
	npcID := uuid.New()
	locationID := uuid.New()
	store := &stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Merchant", Alive: true},
		},
		locations: map[uuid.UUID]uuid.UUID{
			locationID: uuid.New(),
		},
	}
	h := NewUpdateNPCHandler(store)

	_, err := h.Handle(context.Background(), map[string]any{
		"npc_id":      npcID.String(),
		"location_id": locationID.String(),
	})
	if err == nil {
		t.Fatal("expected cross-campaign location error")
	}
	if !strings.Contains(err.Error(), "location_id does not reference an existing location in npc campaign") {
		t.Fatalf("error = %v, want campaign-scoped location error", err)
	}
}

func TestUpdateNPCHandleNilReceiver(t *testing.T) {
	var h *UpdateNPCHandler
	_, err := h.Handle(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected nil receiver error")
	}
	if !strings.Contains(err.Error(), "update_npc handler is required") {
		t.Fatalf("error = %v, want nil receiver message", err)
	}
}

func TestUpdateNPCHandleStoreErrorWrapped(t *testing.T) {
	npcID := uuid.New()
	h := NewUpdateNPCHandler(&stubUpdateNPCStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {ID: npcID, CampaignID: uuid.New(), Name: "Sentinel", Alive: true},
		},
		updateErr: errors.New("db down"),
	})

	_, err := h.Handle(context.Background(), map[string]any{
		"npc_id": npcID.String(),
		"alive":  false,
	})
	if err == nil {
		t.Fatal("expected update error")
	}
	if !strings.Contains(err.Error(), "update npc: db down") {
		t.Fatalf("error = %v, want wrapped update error", err)
	}
}
