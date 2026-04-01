package memory

import (
	"testing"

	"github.com/google/uuid"
)

func TestExtractMetadata_CombatTools(t *testing.T) {
	calls := []ToolCallInfo{
		{ToolName: "initiate_combat", Arguments: nil},
		{ToolName: "combat_round", Arguments: nil},
		{ToolName: "apply_damage", Arguments: nil},
	}
	md := ExtractMetadata(calls, nil)
	if md.MemoryType != "combat" {
		t.Fatalf("expected combat, got %s", md.MemoryType)
	}
}

func TestExtractMetadata_QuestTools(t *testing.T) {
	calls := []ToolCallInfo{
		{ToolName: "create_quest", Arguments: nil},
		{ToolName: "complete_objective", Arguments: nil},
	}
	md := ExtractMetadata(calls, nil)
	if md.MemoryType != "quest_update" {
		t.Fatalf("expected quest_update, got %s", md.MemoryType)
	}
}

func TestExtractMetadata_NPCTools(t *testing.T) {
	npcID := uuid.New()
	calls := []ToolCallInfo{
		{ToolName: "npc_dialogue", Arguments: map[string]any{"npc_id": npcID.String()}},
		{ToolName: "update_npc", Arguments: map[string]any{"id": npcID.String()}},
	}
	md := ExtractMetadata(calls, nil)
	if md.MemoryType != "dialogue" {
		t.Fatalf("expected dialogue, got %s", md.MemoryType)
	}
	if len(md.NPCsInvolved) != 1 {
		t.Fatalf("expected 1 unique NPC, got %d", len(md.NPCsInvolved))
	}
	if md.NPCsInvolved[0] != npcID {
		t.Fatalf("expected %s, got %s", npcID, md.NPCsInvolved[0])
	}
}

func TestExtractMetadata_MixedTools(t *testing.T) {
	calls := []ToolCallInfo{
		{ToolName: "initiate_combat", Arguments: nil},
		{ToolName: "combat_round", Arguments: nil},
		{ToolName: "create_quest", Arguments: nil},
	}
	md := ExtractMetadata(calls, nil)
	// combat has 2 vs quest 1 → combat wins
	if md.MemoryType != "combat" {
		t.Fatalf("expected combat, got %s", md.MemoryType)
	}
}

func TestExtractMetadata_LocationFromState(t *testing.T) {
	locID := uuid.New()
	state := &GameStateSnapshot{CurrentLocationID: &locID}
	md := ExtractMetadata(nil, state)
	if md.LocationID == nil {
		t.Fatal("expected LocationID to be set")
	}
	if *md.LocationID != locID {
		t.Fatalf("expected %s, got %s", locID, *md.LocationID)
	}
}

func TestExtractMetadata_NilState(t *testing.T) {
	md := ExtractMetadata(nil, nil)
	if md.LocationID != nil {
		t.Fatalf("expected nil LocationID, got %v", md.LocationID)
	}
}

func TestExtractMetadata_NoToolCalls(t *testing.T) {
	md := ExtractMetadata(nil, nil)
	if md.MemoryType != "turn_summary" {
		t.Fatalf("expected turn_summary, got %s", md.MemoryType)
	}
	if md.InGameTime != "" {
		t.Fatalf("expected empty InGameTime, got %q", md.InGameTime)
	}
}

func TestExtractMetadata_NPCIDExtraction(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	calls := []ToolCallInfo{
		{ToolName: "create_npc", Arguments: map[string]any{"id": id1.String()}},
		{ToolName: "update_npc", Arguments: map[string]any{"npc_id": id2.String()}},
	}
	md := ExtractMetadata(calls, nil)
	if len(md.NPCsInvolved) != 2 {
		t.Fatalf("expected 2 NPCs, got %d", len(md.NPCsInvolved))
	}
	got := make(map[uuid.UUID]bool)
	for _, id := range md.NPCsInvolved {
		got[id] = true
	}
	if !got[id1] || !got[id2] {
		t.Fatalf("missing expected IDs: got %v", md.NPCsInvolved)
	}
}

func TestExtractMetadata_InvalidNPCID(t *testing.T) {
	calls := []ToolCallInfo{
		{ToolName: "create_npc", Arguments: map[string]any{"id": "not-a-uuid"}},
		{ToolName: "update_npc", Arguments: map[string]any{"npc_id": 12345}}, // wrong type
	}
	md := ExtractMetadata(calls, nil)
	if len(md.NPCsInvolved) != 0 {
		t.Fatalf("expected 0 NPCs, got %d", len(md.NPCsInvolved))
	}
}

func TestExtractMetadata_TieBreakPriority(t *testing.T) {
	// One combat, one quest → tied at 1 each. Combat has higher priority.
	calls := []ToolCallInfo{
		{ToolName: "initiate_combat", Arguments: nil},
		{ToolName: "create_quest", Arguments: nil},
	}
	md := ExtractMetadata(calls, nil)
	if md.MemoryType != "combat" {
		t.Fatalf("expected combat on tie-break, got %s", md.MemoryType)
	}
}

func TestExtractMetadata_UnknownToolsIgnored(t *testing.T) {
	calls := []ToolCallInfo{
		{ToolName: "some_unknown_tool", Arguments: nil},
		{ToolName: "another_mystery", Arguments: nil},
	}
	md := ExtractMetadata(calls, nil)
	if md.MemoryType != "turn_summary" {
		t.Fatalf("expected turn_summary for unknown tools, got %s", md.MemoryType)
	}
}
