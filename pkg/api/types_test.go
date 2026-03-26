package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCampaignTypesJSONShape(t *testing.T) {
	t.Parallel()

	createReq := CampaignCreateRequest{
		Name:        "The Iron Road",
		Description: "A frontier campaign",
		Genre:       "fantasy",
		Tone:        "gritty",
		Themes:      []string{"exploration", "survival"},
	}
	assertJSONKeys(t, createReq, "name", "description", "genre", "tone", "themes")

	response := CampaignResponse{
		ID:          "camp-1",
		Name:        "The Iron Road",
		Description: "A frontier campaign",
		Genre:       "fantasy",
		Tone:        "gritty",
		Themes:      []string{"exploration"},
		Status:      "active",
		CreatedBy:   "user-1",
		CreatedAt:   time.Unix(100, 0).UTC(),
		UpdatedAt:   time.Unix(200, 0).UTC(),
	}
	assertJSONKeys(t, response, "id", "name", "description", "genre", "tone", "themes", "status", "created_by", "created_at", "updated_at")

	list := CampaignListResponse{Campaigns: []CampaignResponse{response}}
	assertJSONKeys(t, list, "campaigns")
}

func TestEntityResponseJSONShape(t *testing.T) {
	t.Parallel()

	locationID := "loc-1"
	npcHP := 22
	character := CharacterResponse{
		ID:                "pc-1",
		CampaignID:        "camp-1",
		UserID:            "user-1",
		Name:              "Vera",
		Description:       "Scout",
		Stats:             map[string]any{"strength": 12},
		HP:                20,
		MaxHP:             25,
		Experience:        120,
		Level:             3,
		Status:            "healthy",
		Abilities:         []CharacterAbility{{Name: "Track", Description: "Follow signs"}},
		CurrentLocationID: &locationID,
	}
	assertJSONKeys(t, character, "id", "campaign_id", "user_id", "name", "description", "stats", "hp", "max_hp", "experience", "level", "status", "abilities", "current_location_id")

	location := LocationResponse{
		ID:           "loc-1",
		CampaignID:   "camp-1",
		Name:         "Gatehouse",
		Description:  "Stone fort",
		Region:       "North",
		LocationType: "city",
		Properties:   map[string]any{"danger": "low"},
		Connections: []LocationConnectionResponse{{
			ToLocationID:  "loc-2",
			Description:   "Road east",
			Bidirectional: true,
			TravelTime:    "15m",
		}},
	}
	assertJSONKeys(t, location, "id", "campaign_id", "name", "description", "region", "location_type", "properties", "connections")

	npc := NPCResponse{
		ID:          "npc-1",
		CampaignID:  "camp-1",
		Name:        "Captain Rowan",
		Description: "Guard commander",
		Personality: "stern",
		Disposition: 40,
		FactionID:   ptr("faction-1"),
		Faction:     "City Watch",
		Alive:       true,
		HP:          &npcHP,
		Stats:       map[string]any{"intelligence": 14},
		Properties:  map[string]any{"rank": "captain"},
	}
	assertJSONKeys(t, npc, "id", "campaign_id", "name", "description", "personality", "disposition", "faction_id", "faction", "alive", "hp", "stats", "properties")

	quest := QuestResponse{
		ID:            "quest-1",
		CampaignID:    "camp-1",
		ParentQuestID: ptr("quest-parent"),
		Title:         "Secure the pass",
		Description:   "Defeat raiders",
		QuestType:     "short_term",
		Status:        "active",
		Objectives: []QuestObjectiveResponse{{
			ID:          "obj-1",
			Description: "Scout the pass",
			Completed:   false,
			OrderIndex:  1,
		}},
	}
	assertJSONKeys(t, quest, "id", "campaign_id", "parent_quest_id", "title", "description", "quest_type", "status", "objectives")

	item := ItemResponse{
		ID:                "item-1",
		CampaignID:        "camp-1",
		PlayerCharacterID: ptr("pc-1"),
		Name:              "Lantern",
		Description:       "Oil lantern",
		ItemType:          "misc",
		Rarity:            "common",
		Properties:        map[string]any{"weight": 2},
		Equipped:          false,
		Quantity:          1,
	}
	assertJSONKeys(t, item, "id", "campaign_id", "player_character_id", "name", "description", "item_type", "rarity", "properties", "equipped", "quantity")
}

func TestActionTurnAndEnvelopeTypesJSONShape(t *testing.T) {
	t.Parallel()

	action := ActionRequest{Input: "Move to the gatehouse"}
	assertJSONKeys(t, action, "input")

	result := TurnResult{
		Narrative: "You arrive at the gatehouse.",
		StateChanges: []StateChange{{
			EntityType: "character",
			EntityID:   "pc-1",
			ChangeType: "location_updated",
			Details:    map[string]any{"location_id": "loc-1"},
		}},
	}
	assertJSONKeys(t, result, "narrative", "state_changes")

	alias := TurnResponse(result)
	assertJSONKeys(t, alias, "narrative", "state_changes")

	envelope := WebSocketMessageEnvelope{
		Type:      "turn_result",
		Payload:   json.RawMessage(`{"narrative":"You arrive at the gatehouse."}`),
		Timestamp: time.Unix(1234, 0).UTC(),
	}
	assertJSONKeys(t, envelope, "type", "payload", "timestamp")
}

func TestOmitEmptyPointerFieldsJSONShape(t *testing.T) {
	t.Parallel()

	character := CharacterResponse{
		ID:          "pc-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		Name:        "Vera",
		Description: "Scout",
		Stats:       map[string]any{"strength": 12},
		HP:          20,
		MaxHP:       25,
		Experience:  120,
		Level:       3,
		Status:      "healthy",
		Abilities:   []CharacterAbility{{Name: "Track"}},
	}
	assertJSONKeys(t, character, "id", "campaign_id", "user_id", "name", "description", "stats", "hp", "max_hp", "experience", "level", "status", "abilities")

	npc := NPCResponse{
		ID:          "npc-1",
		CampaignID:  "camp-1",
		Name:        "Captain Rowan",
		Description: "Guard commander",
		Personality: "stern",
		Disposition: 40,
		Alive:       true,
		Stats:       map[string]any{"intelligence": 14},
		Properties:  map[string]any{"rank": "captain"},
	}
	assertJSONKeys(t, npc, "id", "campaign_id", "name", "description", "personality", "disposition", "alive", "stats", "properties")

	quest := QuestResponse{
		ID:          "quest-1",
		CampaignID:  "camp-1",
		Title:       "Secure the pass",
		Description: "Defeat raiders",
		QuestType:   "short_term",
		Status:      "active",
		Objectives: []QuestObjectiveResponse{{
			ID:          "obj-1",
			Description: "Scout the pass",
			Completed:   false,
			OrderIndex:  1,
		}},
	}
	assertJSONKeys(t, quest, "id", "campaign_id", "title", "description", "quest_type", "status", "objectives")

	item := ItemResponse{
		ID:          "item-1",
		CampaignID:  "camp-1",
		Name:        "Lantern",
		Description: "Oil lantern",
		ItemType:    "misc",
		Rarity:      "common",
		Properties:  map[string]any{"weight": 2},
		Equipped:    false,
		Quantity:    1,
	}
	assertJSONKeys(t, item, "id", "campaign_id", "name", "description", "item_type", "rarity", "properties", "equipped", "quantity")
}

func assertJSONKeys(t *testing.T, v any, keys ...string) {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expected := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		expected[key] = struct{}{}
	}

	for _, key := range keys {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %q in JSON: %s", key, string(b))
		}
	}

	for key := range m {
		if _, ok := expected[key]; !ok {
			t.Fatalf("unexpected key %q in JSON: %s", key, string(b))
		}
	}
}

func ptr(v string) *string {
	return &v
}
