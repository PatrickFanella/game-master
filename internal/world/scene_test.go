package world

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

// --- stubs ---

// stubSceneStore records the session log passed to SaveSessionLog.
type stubSceneStore struct {
	savedLog *domain.SessionLog
	err      error
}

func (s *stubSceneStore) SaveSessionLog(_ context.Context, log domain.SessionLog) error {
	if s.err != nil {
		return s.err
	}
	s.savedLog = &log
	return nil
}

// --- helpers ---

func validSceneResponse() sceneLLMResponse {
	return sceneLLMResponse{
		Narrative: "The wind howls through the crumbling gates of Ironhold as you step into the courtyard.",
		Choices: []string{
			"Approach the guild hall",
			"Speak to the herbalist near the fountain",
			"Explore the eastern battlements",
		},
	}
}

func testSkeleton() *WorldSkeleton {
	return &WorldSkeleton{
		Factions: []SkeletonFaction{
			{Name: "Iron Guild", Description: "smiths", Agenda: "control trade", Territory: "mountains"},
		},
		Locations: []SkeletonLocation{
			{Name: "Ironhold", Description: "mountain fortress", Region: "north", LocationType: "city"},
		},
		NPCs: []SkeletonNPC{
			{Name: "Kael", Description: "guild master", Personality: "stern", Faction: "Iron Guild", Location: "Ironhold"},
		},
		WorldFacts: []SkeletonFact{
			{Fact: "Dragons were hunted to extinction", Category: "history"},
		},
		StartingLocationName: "Ironhold",
	}
}

func mustSceneJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// --- tests ---

func TestSceneGenerator_Success(t *testing.T) {
	resp := validSceneResponse()
	store := &stubSceneStore{}
	gen := NewSceneGenerator(&stubSkeletonLLM{content: mustSceneJSON(t, resp)}, store)

	result, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Narrative != resp.Narrative {
		t.Errorf("narrative: got %q, want %q", result.Narrative, resp.Narrative)
	}
	if len(result.Choices) != 3 {
		t.Errorf("choices: got %d, want 3", len(result.Choices))
	}

	// Verify session log was stored.
	if store.savedLog == nil {
		t.Fatal("expected session log to be saved")
	}
	if store.savedLog.TurnNumber != 1 {
		t.Errorf("turn number: got %d, want 1", store.savedLog.TurnNumber)
	}
	if store.savedLog.PlayerInput != "[scene_generation]" {
		t.Errorf("player input: got %q, want %q", store.savedLog.PlayerInput, "[scene_generation]")
	}
	if store.savedLog.InputType != domain.InputTypeNarrative {
		t.Errorf("input type: got %q, want %q", store.savedLog.InputType, domain.InputTypeNarrative)
	}
	if store.savedLog.LLMResponse != resp.Narrative {
		t.Errorf("llm response: got %q, want %q", store.savedLog.LLMResponse, resp.Narrative)
	}
}

func TestSceneGenerator_NilSkeleton(t *testing.T) {
	gen := NewSceneGenerator(&stubSkeletonLLM{content: "{}"}, &stubSceneStore{})

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile(), nil)
	if err == nil {
		t.Fatal("expected error for nil skeleton")
	}
}

func TestSceneGenerator_NilProfile(t *testing.T) {
	gen := NewSceneGenerator(&stubSkeletonLLM{content: "{}"}, &stubSceneStore{})

	_, err := gen.Generate(context.Background(), uuid.New(), nil, testSkeleton())
	if err == nil {
		t.Fatal("expected error for nil profile")
	}
}

func TestSceneGenerator_EmptyLLMResponse(t *testing.T) {
	gen := NewSceneGenerator(&stubSkeletonLLM{content: ""}, &stubSceneStore{})

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err == nil {
		t.Fatal("expected error for empty LLM response")
	}
}

func TestSceneGenerator_MalformedJSON(t *testing.T) {
	gen := NewSceneGenerator(&stubSkeletonLLM{content: "this is not json at all"}, &stubSceneStore{})

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSceneGenerator_MarkdownFences(t *testing.T) {
	resp := validSceneResponse()
	raw := mustSceneJSON(t, resp)
	wrapped := "```json\n" + raw + "\n```"

	store := &stubSceneStore{}
	gen := NewSceneGenerator(&stubSkeletonLLM{content: wrapped}, store)

	result, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Narrative != resp.Narrative {
		t.Errorf("narrative: got %q, want %q", result.Narrative, resp.Narrative)
	}
}

func TestSceneGenerator_StoreError(t *testing.T) {
	resp := validSceneResponse()
	storeErr := errors.New("db connection lost")
	store := &stubSceneStore{err: storeErr}
	gen := NewSceneGenerator(&stubSkeletonLLM{content: mustSceneJSON(t, resp)}, store)

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err == nil {
		t.Fatal("expected error from store failure")
	}
	if !errors.Is(err, storeErr) {
		t.Errorf("expected wrapped store error, got: %v", err)
	}
}

func TestSceneGenerator_LLMError(t *testing.T) {
	gen := NewSceneGenerator(&stubSkeletonLLM{err: errors.New("rate limited")}, &stubSceneStore{})

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
}

func TestSceneGenerator_ChoicesExtracted(t *testing.T) {
	resp := sceneLLMResponse{
		Narrative: "You arrive at the docks under a blood-red sky.",
		Choices:   []string{"Board the ship", "Search the warehouse"},
	}
	gen := NewSceneGenerator(&stubSkeletonLLM{content: mustSceneJSON(t, resp)}, &stubSceneStore{})

	result, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Choices) != 2 {
		t.Fatalf("choices count: got %d, want 2", len(result.Choices))
	}
	if result.Choices[0] != "Board the ship" {
		t.Errorf("choice[0]: got %q, want %q", result.Choices[0], "Board the ship")
	}
	if result.Choices[1] != "Search the warehouse" {
		t.Errorf("choice[1]: got %q, want %q", result.Choices[1], "Search the warehouse")
	}
}

func TestSceneGenerator_EmptyNarrative(t *testing.T) {
	resp := sceneLLMResponse{
		Narrative: "",
		Choices:   []string{"Do something"},
	}
	gen := NewSceneGenerator(&stubSkeletonLLM{content: mustSceneJSON(t, resp)}, &stubSceneStore{})

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err == nil {
		t.Fatal("expected error for empty narrative")
	}
}

func TestSceneGenerator_EmptyChoicesValid(t *testing.T) {
	resp := sceneLLMResponse{
		Narrative: "The world stretches before you, silent and waiting.",
		Choices:   []string{},
	}
	gen := NewSceneGenerator(&stubSkeletonLLM{content: mustSceneJSON(t, resp)}, &stubSceneStore{})

	result, err := gen.Generate(context.Background(), uuid.New(), testProfile(), testSkeleton())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Choices) != 0 {
		t.Errorf("choices: got %d, want 0", len(result.Choices))
	}
}


func TestBuildScenePrompt_FiltersNPCsByLocation(t *testing.T) {
	profile := testProfile()
	skeleton := &WorldSkeleton{
		Locations: []SkeletonLocation{
			{Name: "Ironhold", Description: "mountain fortress", Region: "north", LocationType: "city"},
		},
		NPCs: []SkeletonNPC{
			{Name: "Kael", Description: "guild master", Personality: "stern", Faction: "Iron Guild", Location: "Ironhold"},
			{Name: "Lira", Description: "herbalist", Personality: "kind", Faction: "", Location: "Ironhold"},
			{Name: "Orc Scout", Description: "enemy spy", Personality: "cunning", Faction: "", Location: "Forest Outpost"},
		},
		WorldFacts:           []SkeletonFact{{Fact: "test fact", Category: "lore"}},
		StartingLocationName: "Ironhold",
	}

	prompt := buildScenePrompt(profile, skeleton)

	if !strings.Contains(prompt, "Kael") {
		t.Error("expected prompt to include Kael (at starting location)")
	}
	if !strings.Contains(prompt, "Lira") {
		t.Error("expected prompt to include Lira (at starting location)")
	}
	if strings.Contains(prompt, "Orc Scout") {
		t.Error("prompt should NOT include Orc Scout (different location)")
	}
}