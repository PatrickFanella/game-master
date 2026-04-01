package world

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// --- stubs ---

type stubSkeletonLLM struct {
	content string
	err     error
}

func (s *stubSkeletonLLM) Complete(_ context.Context, _ []llm.Message, _ []llm.Tool) (*llm.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &llm.Response{Content: s.content}, nil
}

func (s *stubSkeletonLLM) Stream(_ context.Context, _ []llm.Message, _ []llm.Tool) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

type storeCall struct {
	method     string
	campaignID uuid.UUID
	factionID  *uuid.UUID
	locationID *uuid.UUID
}

type stubSkeletonStore struct {
	calls []storeCall
	ids   map[string]uuid.UUID // name → deterministic ID
	err   error                // if set, every call fails
}

func newStubStore() *stubSkeletonStore {
	return &stubSkeletonStore{ids: make(map[string]uuid.UUID)}
}

func (s *stubSkeletonStore) idFor(name string) uuid.UUID {
	if id, ok := s.ids[name]; ok {
		return id
	}
	id := uuid.New()
	s.ids[name] = id
	return id
}

func (s *stubSkeletonStore) CreateFaction(_ context.Context, cid uuid.UUID, f SkeletonFaction) (uuid.UUID, error) {
	s.calls = append(s.calls, storeCall{method: "CreateFaction", campaignID: cid})
	if s.err != nil {
		return uuid.Nil, s.err
	}
	return s.idFor(f.Name), nil
}

func (s *stubSkeletonStore) CreateLocation(_ context.Context, cid uuid.UUID, l SkeletonLocation) (uuid.UUID, error) {
	s.calls = append(s.calls, storeCall{method: "CreateLocation", campaignID: cid})
	if s.err != nil {
		return uuid.Nil, s.err
	}
	return s.idFor(l.Name), nil
}

func (s *stubSkeletonStore) CreateNPC(_ context.Context, cid uuid.UUID, _ SkeletonNPC, factionID, locationID *uuid.UUID) (uuid.UUID, error) {
	s.calls = append(s.calls, storeCall{method: "CreateNPC", campaignID: cid, factionID: factionID, locationID: locationID})
	if s.err != nil {
		return uuid.Nil, s.err
	}
	return uuid.New(), nil
}

func (s *stubSkeletonStore) CreateWorldFact(_ context.Context, cid uuid.UUID, _ SkeletonFact) (uuid.UUID, error) {
	s.calls = append(s.calls, storeCall{method: "CreateWorldFact", campaignID: cid})
	if s.err != nil {
		return uuid.Nil, s.err
	}
	return uuid.New(), nil
}

// --- helpers ---

func validResponse() skeletonLLMResponse {
	return skeletonLLMResponse{
		Factions: []SkeletonFaction{
			{Name: "Iron Guild", Description: "smiths", Agenda: "control trade", Territory: "mountains"},
		},
		Locations: []SkeletonLocation{
			{Name: "Ironhold", Description: "mountain fortress", Region: "north", LocationType: "city"},
			{Name: "Shadowfen", Description: "misty swamp", Region: "south", LocationType: "wilderness"},
		},
		NPCs: []SkeletonNPC{
			{Name: "Kael", Description: "guild master", Personality: "stern", Faction: "Iron Guild", Location: "Ironhold"},
			{Name: "Mira", Description: "herbalist", Personality: "kind", Faction: "Unknown Faction", Location: "Shadowfen"},
		},
		WorldFacts: []SkeletonFact{
			{Fact: "Dragons were hunted to extinction", Category: "history"},
		},
		StartingLocation: "Ironhold",
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func testProfile() *CampaignProfile {
	return &CampaignProfile{
		Genre:               "dark fantasy",
		Tone:                "gritty",
		Themes:              []string{"survival", "betrayal"},
		WorldType:           "open wilderness",
		DangerLevel:         "high",
		PoliticalComplexity: "complex",
	}
}

// --- tests ---

func TestSkeletonGenerator_Success(t *testing.T) {
	resp := validResponse()
	store := newStubStore()
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: mustJSON(t, resp)}, store)

	skel, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify entity counts.
	if got := len(skel.Factions); got != 1 {
		t.Errorf("factions: got %d, want 1", got)
	}
	if got := len(skel.Locations); got != 2 {
		t.Errorf("locations: got %d, want 2", got)
	}
	if got := len(skel.NPCs); got != 2 {
		t.Errorf("npcs: got %d, want 2", got)
	}
	if got := len(skel.WorldFacts); got != 1 {
		t.Errorf("world_facts: got %d, want 1", got)
	}

	// Starting location resolved.
	if skel.StartingLocationID == "" {
		t.Error("starting location ID not resolved")
	}

	// Store received correct number of calls: 1 faction + 2 locations + 2 NPCs + 1 fact = 6.
	if got := len(store.calls); got != 6 {
		t.Errorf("store calls: got %d, want 6", got)
	}
}

func TestSkeletonGenerator_MalformedJSON(t *testing.T) {
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: "this is not json at all"}, newStubStore())

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSkeletonGenerator_MarkdownFences(t *testing.T) {
	resp := validResponse()
	raw := mustJSON(t, resp)
	wrapped := "```json\n" + raw + "\n```"

	store := newStubStore()
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: wrapped}, store)

	skel, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(skel.Factions); got != 1 {
		t.Errorf("factions: got %d, want 1", got)
	}
}

func TestSkeletonGenerator_NilProfile(t *testing.T) {
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: "{}"}, newStubStore())

	_, err := gen.Generate(context.Background(), uuid.New(), nil)
	if err == nil {
		t.Fatal("expected error for nil profile")
	}
}

func TestSkeletonGenerator_EmptyProfile(t *testing.T) {
	// Empty but non-nil profile: LLM still gets called, produces valid skeleton.
	resp := skeletonLLMResponse{
		Factions:         []SkeletonFaction{},
		Locations:        []SkeletonLocation{},
		NPCs:             []SkeletonNPC{},
		WorldFacts:       []SkeletonFact{},
		StartingLocation: "",
	}
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: mustJSON(t, resp)}, newStubStore())

	skel, err := gen.Generate(context.Background(), uuid.New(), &CampaignProfile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skel.StartingLocationID != "" {
		t.Errorf("expected empty starting location, got %q", skel.StartingLocationID)
	}
}

func TestSkeletonGenerator_StoreError(t *testing.T) {
	resp := validResponse()
	store := newStubStore()
	store.err = errors.New("db connection lost")
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: mustJSON(t, resp)}, store)

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err == nil {
		t.Fatal("expected error from store failure")
	}
	if !errors.Is(err, store.err) {
		t.Errorf("expected wrapped store error, got: %v", err)
	}
}

func TestSkeletonGenerator_NPCResolution(t *testing.T) {
	resp := validResponse()
	store := newStubStore()
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: mustJSON(t, resp)}, store)

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find NPC calls.
	var npcCalls []storeCall
	for _, c := range store.calls {
		if c.method == "CreateNPC" {
			npcCalls = append(npcCalls, c)
		}
	}
	if len(npcCalls) != 2 {
		t.Fatalf("expected 2 NPC calls, got %d", len(npcCalls))
	}

	// First NPC ("Kael") references "Iron Guild" and "Ironhold" — both exist.
	kael := npcCalls[0]
	if kael.factionID == nil {
		t.Error("Kael: expected faction ID to be resolved")
	}
	if kael.locationID == nil {
		t.Error("Kael: expected location ID to be resolved")
	}

	// Second NPC ("Mira") references "Unknown Faction" (doesn't exist) and "Shadowfen" (exists).
	mira := npcCalls[1]
	if mira.factionID != nil {
		t.Error("Mira: expected nil faction ID for unknown faction")
	}
	if mira.locationID == nil {
		t.Error("Mira: expected location ID to be resolved for Shadowfen")
	}
}

func TestSkeletonGenerator_LLMError(t *testing.T) {
	gen := NewSkeletonGenerator(&stubSkeletonLLM{err: errors.New("rate limited")}, newStubStore())

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
}

func TestSkeletonGenerator_EmptyLLMResponse(t *testing.T) {
	gen := NewSkeletonGenerator(&stubSkeletonLLM{content: ""}, newStubStore())

	_, err := gen.Generate(context.Background(), uuid.New(), testProfile())
	if err == nil {
		t.Fatal("expected error for empty LLM response")
	}
}
