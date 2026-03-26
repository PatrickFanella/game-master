package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

const innkeeperName = "Innkeeper Toma"

func TestNPCDialogueStoreGetNPCByID(t *testing.T) {
	q := newMockQuerier()
	npcID := uuid.New()
	campaignID := uuid.New()
	locationID := uuid.New()
	q.npcByID[npcID] = mockNPCRecord{
		npc: statedb.Npc{
			ID:         dbutil.ToPgtype(npcID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Name:       innkeeperName,
			LocationID: dbutil.ToPgtype(locationID),
			Alive:      true,
		},
	}
	store := NewNPCDialogueStore(q)

	got, err := store.GetNPCByID(context.Background(), npcID)
	if err != nil {
		t.Fatalf("GetNPCByID() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected NPC, got nil")
	}
	if got.ID != npcID {
		t.Fatalf("npc id = %s, want %s", got.ID, npcID)
	}
	if got.Name != innkeeperName {
		t.Fatalf("npc name = %q, want %s", got.Name, innkeeperName)
	}
	if got.LocationID == nil || *got.LocationID != locationID {
		t.Fatalf("npc location = %v, want %s", got.LocationID, locationID)
	}
}

func TestNPCDialogueStoreGetNPCByIDNotFound(t *testing.T) {
	q := newMockQuerier()
	store := NewNPCDialogueStore(q)

	got, err := store.GetNPCByID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetNPCByID() error = %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil npc, got %+v", got)
	}
}

func TestNPCDialogueStoreLogNPCDialoguePersistsSessionLog(t *testing.T) {
	q := newMockQuerier()
	store := NewNPCDialogueStore(q)
	campaignID := uuid.New()
	locationID := uuid.New()
	npcID := uuid.New()
	q.recentSessionLogs = []statedb.SessionLog{{TurnNumber: 7}}

	err := store.LogNPCDialogue(context.Background(), tools.NPCDialogueLogEntry{
		CampaignID:        campaignID,
		LocationID:        locationID,
		NPCID:             npcID,
		Dialogue:          "Welcome, traveler.",
		FormattedDialogue: "Innkeeper Toma: Welcome, traveler.",
	})
	if err != nil {
		t.Fatalf("LogNPCDialogue() error = %v", err)
	}
	if q.lastSessionLog == nil {
		t.Fatal("expected CreateSessionLog to be called")
	}
	if q.lastSessionLog.TurnNumber != 8 {
		t.Fatalf("turn_number = %d, want 8", q.lastSessionLog.TurnNumber)
	}
	if q.lastSessionLog.InputType != string(domain.Narrative) {
		t.Fatalf("input_type = %q, want %q", q.lastSessionLog.InputType, domain.Narrative)
	}
	if q.lastSessionLog.PlayerInput != "Innkeeper Toma: Welcome, traveler." {
		t.Fatalf("player_input = %q", q.lastSessionLog.PlayerInput)
	}
	if q.lastSessionLog.LlmResponse != "Innkeeper Toma: Welcome, traveler." {
		t.Fatalf("llm_response = %q", q.lastSessionLog.LlmResponse)
	}
	if string(q.lastSessionLog.ToolCalls) != "[]" {
		t.Fatalf("tool_calls = %s, want []", q.lastSessionLog.ToolCalls)
	}
	if got := dbutil.FromPgtype(q.lastSessionLog.LocationID); got != locationID {
		t.Fatalf("location_id = %s, want %s", got, locationID)
	}
	if len(q.lastSessionLog.NpcsInvolved) != 1 || dbutil.FromPgtype(q.lastSessionLog.NpcsInvolved[0]) != npcID {
		t.Fatalf("npcs_involved = %+v, want [%s]", q.lastSessionLog.NpcsInvolved, npcID)
	}
}

func TestNPCDialogueStoreLogNPCDialogueErrors(t *testing.T) {
	t.Run("list recent session logs", func(t *testing.T) {
		q := newMockQuerier()
		q.listRecentSessionLogsErr = errors.New("query failed")
		store := NewNPCDialogueStore(q)

		err := store.LogNPCDialogue(context.Background(), tools.NPCDialogueLogEntry{
			CampaignID:        uuid.New(),
			LocationID:        uuid.New(),
			NPCID:             uuid.New(),
			FormattedDialogue: "Guard: Halt!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "list recent session logs: query failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("create session log", func(t *testing.T) {
		q := newMockQuerier()
		q.createSessionLogErr = errors.New("insert failed")
		store := NewNPCDialogueStore(q)

		err := store.LogNPCDialogue(context.Background(), tools.NPCDialogueLogEntry{
			CampaignID:        uuid.New(),
			LocationID:        uuid.New(),
			NPCID:             uuid.New(),
			FormattedDialogue: "Guard: Halt!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "create session log: insert failed") {
			t.Fatalf("error = %v", err)
		}
	})
}

var _ tools.NPCDialogueStore = (*npcDialogueStore)(nil)
