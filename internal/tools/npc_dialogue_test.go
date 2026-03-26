package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

type stubNPCDialogueStore struct {
	npcs      map[uuid.UUID]*domain.NPC
	lastEntry *NPCDialogueLogEntry
	err       error
}

func (s *stubNPCDialogueStore) GetNPCByID(_ context.Context, npcID uuid.UUID) (*domain.NPC, error) {
	npc, ok := s.npcs[npcID]
	if !ok {
		return nil, nil
	}
	return npc, nil
}

func (s *stubNPCDialogueStore) LogNPCDialogue(_ context.Context, entry NPCDialogueLogEntry) error {
	if s.err != nil {
		return s.err
	}
	copy := entry
	s.lastEntry = &copy
	return nil
}

func TestRegisterNPCDialogue(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterNPCDialogue(reg, &stubNPCDialogueStore{}); err != nil {
		t.Fatalf("register npc_dialogue: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != npcDialogueToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, npcDialogueToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 2 || required[0] != "npc_id" || required[1] != "dialogue" {
		t.Fatalf("required schema = %#v, want [npc_id dialogue]", required)
	}
}

func TestNPCDialogueHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	npcID := uuid.New()
	store := &stubNPCDialogueStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {
				ID:         npcID,
				CampaignID: campaignID,
				Name:       "Captain Mira",
				LocationID: &locationID,
				Alive:      true,
			},
		},
	}
	h := NewNPCDialogueHandler(store)
	ctx := WithCurrentLocationID(context.Background(), locationID)

	got, err := h.Handle(ctx, map[string]any{
		"npc_id":   npcID.String(),
		"dialogue": "Hold the line!",
		"emotion":  "urgent",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if store.lastEntry == nil {
		t.Fatal("expected npc dialogue log entry to be persisted")
	}
	if store.lastEntry.NPCID != npcID {
		t.Fatalf("logged npc_id = %s, want %s", store.lastEntry.NPCID, npcID)
	}
	if store.lastEntry.LocationID != locationID {
		t.Fatalf("logged location_id = %s, want %s", store.lastEntry.LocationID, locationID)
	}
	if store.lastEntry.FormattedDialogue != "Captain Mira (urgent): Hold the line!" {
		t.Fatalf("logged formatted_dialogue = %q, want expected format", store.lastEntry.FormattedDialogue)
	}
	if !got.Success {
		t.Fatal("result success = false, want true")
	}
	if got.Narrative != "Captain Mira (urgent): Hold the line!" {
		t.Fatalf("narrative = %q, want expected formatted dialogue", got.Narrative)
	}
	if got.Data["npc_name"] != "Captain Mira" {
		t.Fatalf("result npc_name = %v, want Captain Mira", got.Data["npc_name"])
	}
}

func TestNPCDialogueHandleInvalidNPCID(t *testing.T) {
	store := &stubNPCDialogueStore{npcs: map[uuid.UUID]*domain.NPC{}}
	h := NewNPCDialogueHandler(store)
	ctx := WithCurrentLocationID(context.Background(), uuid.New())

	_, err := h.Handle(ctx, map[string]any{
		"npc_id":   uuid.New().String(),
		"dialogue": "You shall not pass.",
	})
	if err == nil {
		t.Fatal("expected error for unknown npc_id")
	}
	if !strings.Contains(err.Error(), "npc_id does not reference an existing npc") {
		t.Fatalf("error = %v, want unknown npc_id message", err)
	}
}

func TestNPCDialogueHandleWrongLocation(t *testing.T) {
	npcID := uuid.New()
	npcLocationID := uuid.New()
	ctxLocationID := uuid.New()
	store := &stubNPCDialogueStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {
				ID:         npcID,
				CampaignID: uuid.New(),
				Name:       "Archivist Sol",
				LocationID: &npcLocationID,
				Alive:      true,
			},
		},
	}
	h := NewNPCDialogueHandler(store)
	ctx := WithCurrentLocationID(context.Background(), ctxLocationID)

	_, err := h.Handle(ctx, map[string]any{
		"npc_id":   npcID.String(),
		"dialogue": "This archive is closed.",
	})
	if err == nil {
		t.Fatal("expected error for npc at wrong location")
	}
	if !strings.Contains(err.Error(), "npc is not at current location") {
		t.Fatalf("error = %v, want wrong-location message", err)
	}
}

func TestNPCDialogueHandleNPCMustBeAlive(t *testing.T) {
	locationID := uuid.New()
	npcID := uuid.New()
	store := &stubNPCDialogueStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {
				ID:         npcID,
				CampaignID: uuid.New(),
				Name:       "Fallen Knight",
				LocationID: &locationID,
				Alive:      false,
			},
		},
	}
	h := NewNPCDialogueHandler(store)
	ctx := WithCurrentLocationID(context.Background(), locationID)

	_, err := h.Handle(ctx, map[string]any{
		"npc_id":   npcID.String(),
		"dialogue": "...",
	})
	if err == nil {
		t.Fatal("expected error for dead npc")
	}
	if !strings.Contains(err.Error(), "npc must be alive") {
		t.Fatalf("error = %v, want alive-validation message", err)
	}
}

func TestNPCDialogueHandleStoreErrorWrapped(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	npcID := uuid.New()
	store := &stubNPCDialogueStore{
		npcs: map[uuid.UUID]*domain.NPC{
			npcID: {
				ID:         npcID,
				CampaignID: campaignID,
				Name:       "Elder Rowan",
				LocationID: &locationID,
				Alive:      true,
			},
		},
		err: errors.New("insert failed"),
	}
	h := NewNPCDialogueHandler(store)
	ctx := WithCurrentLocationID(context.Background(), locationID)

	_, err := h.Handle(ctx, map[string]any{
		"npc_id":   npcID.String(),
		"dialogue": "Listen carefully.",
	})
	if err == nil {
		t.Fatal("expected store error")
	}
	if !strings.Contains(err.Error(), "log npc dialogue: insert failed") {
		t.Fatalf("error = %v, want wrapped store error", err)
	}
}

var _ NPCDialogueStore = (*stubNPCDialogueStore)(nil)
