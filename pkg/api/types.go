package api

import (
	"encoding/json"
	"time"
)

// CampaignCreateRequest describes the payload used to create a campaign.
type CampaignCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Genre       string   `json:"genre"`
	Tone        string   `json:"tone"`
	Themes      []string `json:"themes"`
}

// CampaignResponse describes a campaign returned by the API.
type CampaignResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Genre       string    `json:"genre"`
	Tone        string    `json:"tone"`
	Themes      []string  `json:"themes"`
	Status      string    `json:"status"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CampaignListResponse describes the payload returned when listing campaigns.
type CampaignListResponse struct {
	Campaigns []CampaignResponse `json:"campaigns"`
}

// CharacterAbility describes a character ability.
type CharacterAbility struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CharacterResponse describes a player character returned by the API.
type CharacterResponse struct {
	ID                string             `json:"id"`
	CampaignID        string             `json:"campaign_id"`
	UserID            string             `json:"user_id"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	Stats             map[string]any     `json:"stats"`
	HP                int                `json:"hp"`
	MaxHP             int                `json:"max_hp"`
	Experience        int                `json:"experience"`
	Level             int                `json:"level"`
	Status            string             `json:"status"`
	Abilities         []CharacterAbility `json:"abilities"`
	CurrentLocationID *string            `json:"current_location_id,omitempty"`
}

// LocationConnectionResponse describes a traversable connection from a location.
type LocationConnectionResponse struct {
	ToLocationID  string `json:"to_location_id"`
	Description   string `json:"description"`
	Bidirectional bool   `json:"bidirectional"`
	TravelTime    string `json:"travel_time"`
}

// LocationResponse describes a location returned by the API.
type LocationResponse struct {
	ID           string                       `json:"id"`
	CampaignID   string                       `json:"campaign_id"`
	Name         string                       `json:"name"`
	Description  string                       `json:"description"`
	Region       string                       `json:"region"`
	LocationType string                       `json:"location_type"`
	Properties   map[string]any               `json:"properties"`
	Connections  []LocationConnectionResponse `json:"connections"`
}

// NPCResponse describes a non-player character returned by the API.
type NPCResponse struct {
	ID          string         `json:"id"`
	CampaignID  string         `json:"campaign_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Personality string         `json:"personality"`
	Disposition int            `json:"disposition"`
	FactionID   *string        `json:"faction_id,omitempty"`
	Faction     string         `json:"faction,omitempty"`
	Alive       bool           `json:"alive"`
	HP          *int           `json:"hp,omitempty"`
	Stats       map[string]any `json:"stats"`
	Properties  map[string]any `json:"properties"`
}

// QuestObjectiveResponse describes a single objective within a quest.
type QuestObjectiveResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	OrderIndex  int    `json:"order_index"`
}

// QuestResponse describes a quest returned by the API.
type QuestResponse struct {
	ID            string                   `json:"id"`
	CampaignID    string                   `json:"campaign_id"`
	ParentQuestID *string                  `json:"parent_quest_id,omitempty"`
	Title         string                   `json:"title"`
	Description   string                   `json:"description"`
	QuestType     string                   `json:"quest_type"`
	Status        string                   `json:"status"`
	Objectives    []QuestObjectiveResponse `json:"objectives"`
}

// ItemResponse describes an item returned by the API.
type ItemResponse struct {
	ID                string         `json:"id"`
	CampaignID        string         `json:"campaign_id"`
	PlayerCharacterID *string        `json:"player_character_id,omitempty"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	ItemType          string         `json:"item_type"`
	Rarity            string         `json:"rarity"`
	Properties        map[string]any `json:"properties"`
	Equipped          bool           `json:"equipped"`
	Quantity          int            `json:"quantity"`
}

// ActionRequest describes player input submitted for a turn.
type ActionRequest struct {
	Input string `json:"input"`
}

// StateChange describes a state update that occurred during a turn.
type StateChange struct {
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	ChangeType string         `json:"change_type"`
	Details    map[string]any `json:"details"`
}

// TurnResult describes the narrative and state changes produced by a turn.
type TurnResult struct {
	Narrative    string        `json:"narrative"`
	StateChanges []StateChange `json:"state_changes"`
}

// TurnResponse is an alias for TurnResult maintained for naming clarity.
type TurnResponse = TurnResult

// WebSocketMessageEnvelope describes a real-time API message wrapper.
type WebSocketMessageEnvelope struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}
