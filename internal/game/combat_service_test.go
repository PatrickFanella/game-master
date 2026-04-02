package game

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

func TestCombatService_CreateNPC_MalformedAbilities(t *testing.T) {
	svc := NewCombatService(newMockQuerier())

	_, err := svc.CreateNPC(context.Background(), tools.InitiateCombatNPCParams{
		Name:      "Goblin",
		Abilities: json.RawMessage("not valid json"),
	})
	if err == nil {
		t.Fatal("expected error for malformed abilities JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse abilities") {
		t.Errorf("expected error to contain %q, got %q", "parse abilities", err.Error())
	}
}

func TestCombatService_CreateNPC_ValidAbilities(t *testing.T) {
	svc := NewCombatService(newMockQuerier())

	_, err := svc.CreateNPC(context.Background(), tools.InitiateCombatNPCParams{
		Name:      "Mage",
		Abilities: json.RawMessage(`[{"name":"fireball"}]`),
	})
	// The mock's CreateNPC returns pgx.ErrNoRows, so an error is expected
	// from the DB layer. The key assertion: it must NOT be an abilities parse error.
	if err != nil && strings.Contains(err.Error(), "parse abilities") {
		t.Errorf("valid abilities JSON should not produce a parse error, got %q", err.Error())
	}
}

func TestCombatService_CreateNPC_NoAbilities(t *testing.T) {
	svc := NewCombatService(newMockQuerier())

	_, err := svc.CreateNPC(context.Background(), tools.InitiateCombatNPCParams{
		Name:      "Bandit",
		Abilities: nil,
	})
	// Same as above: mock DB returns ErrNoRows, but no abilities parse error should occur.
	if err != nil && strings.Contains(err.Error(), "parse abilities") {
		t.Errorf("nil abilities should not produce a parse error, got %q", err.Error())
	}
}

func TestCombatService_GetNPCByID(t *testing.T) {
	mq := newMockQuerier()
	id := [16]byte{10, 20, 30}
	mq.npcByID[id] = mockNPCRecord{
		npc: statedb.Npc{
			ID:   pgtype.UUID{Bytes: id, Valid: true},
			Name: "Troll",
		},
	}

	svc := NewCombatService(mq)

	npc, err := svc.GetNPCByID(context.Background(), uuid.UUID(id))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if npc == nil {
		t.Fatal("expected NPC, got nil")
	}
	if npc.Name != "Troll" {
		t.Errorf("expected name %q, got %q", "Troll", npc.Name)
	}
}

func TestCombatService_GetNPCByID_NotFound(t *testing.T) {
	svc := NewCombatService(newMockQuerier())

	npc, err := svc.GetNPCByID(context.Background(), uuid.UUID([16]byte{99}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if npc != nil {
		t.Errorf("expected nil NPC for unknown ID, got %+v", npc)
	}
}
