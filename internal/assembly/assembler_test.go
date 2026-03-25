package assembly

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/prompt"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeState() *game.GameState {
	campaignID := uuid.New()
	questID := uuid.New()
	return &game.GameState{
		Campaign: domain.Campaign{
			ID:     campaignID,
			Name:   "The Lost Kingdom",
			Genre:  "Fantasy",
			Tone:   "Dark",
			Themes: []string{"redemption", "betrayal"},
		},
		Player: domain.PlayerCharacter{
			Name:  "Elara",
			Level: 5,
			HP:    32,
			MaxHP: 40,
		},
		CurrentLocation: domain.Location{
			Name:         "Thornwood Village",
			Region:       "Northern Reaches",
			LocationType: "settlement",
			Description:  "A small village shrouded in mist.",
		},
		CurrentLocationConnections: []domain.LocationConnection{
			{Description: "Forest path leading east", TravelTime: "1 hour"},
		},
		NearbyNPCs: []domain.NPC{
			{Name: "Old Merchant", Description: "A weathered trader.", Alive: true},
			{Name: "Fallen Guard", Alive: false},
		},
		ActiveQuests: []domain.Quest{
			{
				ID:          questID,
				Title:       "Find the Lost Amulet",
				QuestType:   domain.QuestTypeShortTerm,
				Description: "Recover the amulet from the forest ruins.",
			},
		},
		ActiveQuestObjectives: map[uuid.UUID][]domain.QuestObjective{
			questID: {
				{Description: "Reach the ruins", Completed: true},
				{Description: "Defeat the guardian", Completed: false},
			},
		},
		PlayerInventory: []domain.Item{
			{Name: "Iron Sword", Description: "A sturdy blade.", Quantity: 1, ItemType: domain.ItemTypeWeapon},
			{Name: "Health Potion", Quantity: 3, ItemType: domain.ItemTypeConsumable},
		},
		WorldFacts: []domain.WorldFact{
			{Fact: "The old king vanished twenty years ago."},
		},
	}
}

func makeSessionLogs(n int) []domain.SessionLog {
	logs := make([]domain.SessionLog, n)
	for i := range logs {
		logs[i] = domain.SessionLog{
			ID:          uuid.New(),
			CampaignID:  uuid.New(),
			TurnNumber:  i + 1,
			PlayerInput: "player turn " + strconv.Itoa(i+1),
			LLMResponse: "assistant turn " + strconv.Itoa(i+1),
		}
	}
	return logs
}

// ---------------------------------------------------------------------------
// NewContextAssembler
// ---------------------------------------------------------------------------

func TestNewContextAssembler_NilRegistry(t *testing.T) {
	a := NewContextAssembler(nil)
	if a == nil {
		t.Fatal("expected non-nil assembler")
	}
	if a.Tools() != nil {
		t.Error("expected nil tools for nil registry")
	}
}

func TestNewContextAssembler_WithRegistry(t *testing.T) {
	reg := tools.NewRegistry()
	err := reg.Register(llm.Tool{Name: "test_tool", Description: "a test tool"}, func(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("failed to register test tool: %v", err)
	}

	a := NewContextAssembler(reg)
	if a == nil {
		t.Fatal("expected non-nil assembler")
	}
	got := a.Tools()
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	if got[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %q", got[0].Name)
	}
}

// ---------------------------------------------------------------------------
// AssembleContext – message structure
// ---------------------------------------------------------------------------

func TestAssembleContext_AlwaysStartsWithSystemMessage(t *testing.T) {
	a := NewContextAssembler(nil)
	msgs := a.AssembleContext(makeState(), nil, "look around")

	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("first message role: got %q, want %q", msgs[0].Role, llm.RoleSystem)
	}
}

func TestAssembleContext_SystemMessageContainsGMPrompt(t *testing.T) {
	a := NewContextAssembler(nil)
	msgs := a.AssembleContext(makeState(), nil, "look around")

	sysContent := msgs[0].Content
	if !strings.Contains(sysContent, prompt.GameMaster[:50]) {
		t.Error("system message does not contain the GM prompt prefix")
	}
}

func TestAssembleContext_SystemMessageContainsStateSection(t *testing.T) {
	a := NewContextAssembler(nil)
	msgs := a.AssembleContext(makeState(), nil, "look around")

	sysContent := msgs[0].Content
	for _, want := range []string{
		"## Current Game State",
		"### Campaign",
		"### Player Character",
		"### Current Location",
	} {
		if !strings.Contains(sysContent, want) {
			t.Errorf("system message missing section %q", want)
		}
	}
}

func TestAssembleContext_PlayerInputIsLastUserMessage(t *testing.T) {
	a := NewContextAssembler(nil)
	input := "I examine the amulet closely"
	msgs := a.AssembleContext(makeState(), nil, input)

	last := msgs[len(msgs)-1]
	if last.Role != llm.RoleUser {
		t.Errorf("last message role: got %q, want %q", last.Role, llm.RoleUser)
	}
	if last.Content != input {
		t.Errorf("last message content: got %q, want %q", last.Content, input)
	}
}

func TestAssembleContext_NoHistory_TwoMessages(t *testing.T) {
	a := NewContextAssembler(nil)
	msgs := a.AssembleContext(makeState(), nil, "go north")

	// Expect: [system, user]
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("msg[0] role: got %q", msgs[0].Role)
	}
	if msgs[1].Role != llm.RoleUser {
		t.Errorf("msg[1] role: got %q", msgs[1].Role)
	}
}

func TestAssembleContext_TurnHistoryAlternates(t *testing.T) {
	a := NewContextAssembler(nil)
	logs := makeSessionLogs(3)
	msgs := a.AssembleContext(makeState(), logs, "new action")

	// Expected: [system, user1, asst1, user2, asst2, user3, asst3, user_new]
	if len(msgs) != 8 {
		t.Fatalf("expected 8 messages, got %d", len(msgs))
	}
	// system at index 0
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("msgs[0] role: got %q", msgs[0].Role)
	}
	// pairs
	for i := 0; i < 3; i++ {
		userIdx := 1 + i*2
		asstIdx := userIdx + 1
		if msgs[userIdx].Role != llm.RoleUser {
			t.Errorf("msgs[%d] role: got %q, want user", userIdx, msgs[userIdx].Role)
		}
		if msgs[asstIdx].Role != llm.RoleAssistant {
			t.Errorf("msgs[%d] role: got %q, want assistant", asstIdx, msgs[asstIdx].Role)
		}
	}
	// final user message
	if msgs[7].Role != llm.RoleUser {
		t.Errorf("msgs[7] role: got %q, want user", msgs[7].Role)
	}
	if msgs[7].Content != "new action" {
		t.Errorf("msgs[7] content: got %q, want 'new action'", msgs[7].Content)
	}
}

func TestAssembleContext_SlidingWindowCapsAt10(t *testing.T) {
	a := NewContextAssembler(nil)
	logs := makeSessionLogs(15) // 15 turns, only last 10 should be included
	msgs := a.AssembleContext(makeState(), logs, "action")

	// Expected: 1 system + 10*2 history + 1 player = 22
	want := 1 + maxRecentTurns*2 + 1
	if len(msgs) != want {
		t.Fatalf("expected %d messages, got %d", want, len(msgs))
	}

	// The first history user message should be from turn 6 (index 5, oldest of last 10).
	if msgs[1].Content != "player turn 6" {
		t.Errorf("expected first history msg to be 'player turn 6', got %q", msgs[1].Content)
	}
}

func TestAssembleContext_TurnWithoutLLMResponseOmitsAssistantMessage(t *testing.T) {
	a := NewContextAssembler(nil)
	logs := []domain.SessionLog{
		{
			ID:          uuid.New(),
			CampaignID:  uuid.New(),
			TurnNumber:  1,
			PlayerInput: "hello",
			LLMResponse: "", // no response
		},
	}
	msgs := a.AssembleContext(makeState(), logs, "next action")

	// [system, user(hello), user(next action)] – no assistant message
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[1].Role != llm.RoleUser || msgs[1].Content != "hello" {
		t.Errorf("msgs[1]: got role=%q content=%q", msgs[1].Role, msgs[1].Content)
	}
	if msgs[2].Role != llm.RoleUser || msgs[2].Content != "next action" {
		t.Errorf("msgs[2]: got role=%q content=%q", msgs[2].Role, msgs[2].Content)
	}
}

func TestAssembleContext_HistoryContentPreserved(t *testing.T) {
	a := NewContextAssembler(nil)
	logs := []domain.SessionLog{
		{
			ID:          uuid.New(),
			CampaignID:  uuid.New(),
			TurnNumber:  1,
			PlayerInput: "examine the door",
			LLMResponse: "The door is ancient and locked.",
		},
	}
	msgs := a.AssembleContext(makeState(), logs, "pick the lock")

	if msgs[1].Content != "examine the door" {
		t.Errorf("history user content: got %q", msgs[1].Content)
	}
	if msgs[2].Content != "The door is ancient and locked." {
		t.Errorf("history assistant content: got %q", msgs[2].Content)
	}
}

// ---------------------------------------------------------------------------
// serializeState – content checks
// ---------------------------------------------------------------------------

func TestSerializeState_NilState(t *testing.T) {
	got := serializeState(nil)
	if !strings.Contains(got, "no game state available") {
		t.Errorf("expected nil-state placeholder, got %q", got)
	}
}

func TestSerializeState_CampaignFields(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	for _, want := range []string{"The Lost Kingdom", "Fantasy", "Dark", "redemption", "betrayal"} {
		if !strings.Contains(got, want) {
			t.Errorf("state text missing %q", want)
		}
	}
}

func TestSerializeState_PlayerFields(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	for _, want := range []string{"Elara", "Level: 5", "HP: 32/40"} {
		if !strings.Contains(got, want) {
			t.Errorf("state text missing %q", want)
		}
	}
}

func TestSerializeState_LocationFields(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	for _, want := range []string{
		"Thornwood Village",
		"Northern Reaches",
		"settlement",
		"shrouded in mist",
		"Forest path leading east",
		"1 hour",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("state text missing %q", want)
		}
	}
}

func TestSerializeState_NPCsPresent(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	if !strings.Contains(got, "Old Merchant") {
		t.Error("state text missing alive NPC name")
	}
	if !strings.Contains(got, "Fallen Guard") {
		t.Error("state text missing dead NPC name")
	}
	if !strings.Contains(got, "(dead)") {
		t.Error("state text missing dead NPC marker")
	}
}

func TestSerializeState_ActiveQuestsAndObjectives(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	for _, want := range []string{
		"Find the Lost Amulet",
		"Recover the amulet",
		"[x] Reach the ruins",
		"[ ] Defeat the guardian",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("state text missing %q", want)
		}
	}
}

func TestSerializeState_Inventory(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	if !strings.Contains(got, "Iron Sword") {
		t.Error("state text missing item name")
	}
	// Quantity > 1 should show multiplier
	if !strings.Contains(got, "Health Potion (x3)") {
		t.Errorf("state text missing quantity notation, got:\n%s", got)
	}
}

func TestSerializeState_WorldFacts(t *testing.T) {
	s := makeState()
	got := serializeState(s)

	if !strings.Contains(got, "old king vanished") {
		t.Error("state text missing world fact")
	}
}

func TestSerializeState_EmptySectionsOmitted(t *testing.T) {
	s := &game.GameState{
		Campaign: domain.Campaign{Name: "Empty"},
		Player:   domain.PlayerCharacter{Name: "Hero", Level: 1},
		CurrentLocation: domain.Location{
			Name: "Start",
		},
		// No NPCs, quests, inventory, world facts
	}
	got := serializeState(s)

	for _, absent := range []string{"### NPCs Present", "### Active Quests", "### Player Inventory", "### World Facts"} {
		if strings.Contains(got, absent) {
			t.Errorf("state text should not contain section %q when empty", absent)
		}
	}
}

// ---------------------------------------------------------------------------
// Tools
// ---------------------------------------------------------------------------

func TestTools_NilRegistry(t *testing.T) {
	a := NewContextAssembler(nil)
	if a.Tools() != nil {
		t.Error("expected nil tools slice for nil registry")
	}
}

func TestTools_EmptyRegistry(t *testing.T) {
	a := NewContextAssembler(tools.NewRegistry())
	if a.Tools() != nil {
		t.Error("expected nil tools slice for empty registry")
	}
}

func TestTools_ReturnsRegisteredTools(t *testing.T) {
	reg := tools.NewRegistry()
	tool1 := llm.Tool{Name: "tool_a", Description: "first"}
	tool2 := llm.Tool{Name: "tool_b", Description: "second"}
	noop := func(_ context.Context, _ map[string]any) (*tools.ToolResult, error) { return nil, nil }
	if err := reg.Register(tool1, noop); err != nil {
		t.Fatalf("failed to register tool1: %v", err)
	}
	if err := reg.Register(tool2, noop); err != nil {
		t.Fatalf("failed to register tool2: %v", err)
	}

	a := NewContextAssembler(reg)
	got := a.Tools()
	if len(got) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(got))
	}
	if got[0].Name != "tool_a" || got[1].Name != "tool_b" {
		t.Errorf("unexpected tool names: %v", got)
	}
}
