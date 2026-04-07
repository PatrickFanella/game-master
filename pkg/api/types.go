package api

import (
	"encoding/json"
	"time"

	"github.com/PatrickFanella/game-master/internal/world"
)

// CampaignCreateRequest describes the payload used to create a campaign.
type CampaignCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Genre       string   `json:"genre"`
	Tone        string   `json:"tone"`
	Themes      []string `json:"themes"`
	RulesMode   string   `json:"rules_mode,omitempty"`
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
	RulesMode   string    `json:"rules_mode"`
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
	FromLocationID string `json:"from_location_id,omitempty"`
	ToLocationID   string `json:"to_location_id"`
	Description    string `json:"description"`
	Bidirectional  bool   `json:"bidirectional"`
	TravelTime     string `json:"travel_time"`
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

// SessionLogEntry describes a single turn in the campaign history.
type SessionLogEntry struct {
	TurnNumber  int       `json:"turn_number"`
	PlayerInput string    `json:"player_input"`
	InputType   string    `json:"input_type"`
	LLMResponse string    `json:"llm_response"`
	CreatedAt   time.Time `json:"created_at"`
}

// SessionHistoryResponse returns the turn history for a campaign.
type SessionHistoryResponse struct {
	Entries []SessionLogEntry `json:"entries"`
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
	CombatActive bool          `json:"combat_active"`
}

// TurnResponse is an alias for TurnResult maintained for naming clarity.
type TurnResponse = TurnResult

// StatusPayload describes a processing stage event sent over the WebSocket.
type StatusPayload struct {
	Stage       string `json:"stage"`
	Tool        string `json:"tool,omitempty"`
	Description string `json:"description"`
}

// WebSocketMessageEnvelope describes a real-time API message wrapper.
type WebSocketMessageEnvelope struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// CampaignProfile mirrors the startup workflow's campaign profile payload.
type CampaignProfile = world.CampaignProfile

// CharacterProfile mirrors the startup workflow's character profile payload.
type CharacterProfile = world.CharacterProfile

// InterviewStepRequest submits one reply into an active startup interview session.
type InterviewStepRequest struct {
	Input string `json:"input"`
}

// CampaignInterviewResponse describes one campaign-interview turn.
type CampaignInterviewResponse struct {
	SessionID string           `json:"session_id"`
	Message   string           `json:"message"`
	Done      bool             `json:"done"`
	Profile   *CampaignProfile `json:"profile,omitempty"`
}

// CampaignProposalsRequest asks the backend to generate campaign proposals.
type CampaignProposalsRequest struct {
	Genre        string `json:"genre"`
	SettingStyle string `json:"setting_style"`
	Tone         string `json:"tone"`
}

// CampaignProposal describes one generated startup proposal.
type CampaignProposal struct {
	Name    string          `json:"name"`
	Summary string          `json:"summary"`
	Profile CampaignProfile `json:"profile"`
}

// CampaignProposalsResponse returns generated campaign proposals.
type CampaignProposalsResponse struct {
	Proposals []CampaignProposal `json:"proposals"`
}

// CampaignNameRequest asks the backend to name a campaign from its profile.
type CampaignNameRequest struct {
	Profile *CampaignProfile `json:"profile"`
}

// CampaignNameResponse returns one generated campaign name.
type CampaignNameResponse struct {
	Name string `json:"name"`
}

// CharacterInterviewStartRequest starts a character interview for a campaign profile.
type CharacterInterviewStartRequest struct {
	CampaignProfile *CampaignProfile `json:"campaign_profile"`
}

// CharacterInterviewResponse describes one character-interview turn.
type CharacterInterviewResponse struct {
	SessionID string            `json:"session_id"`
	Message   string            `json:"message"`
	Done      bool              `json:"done"`
	Profile   *CharacterProfile `json:"profile,omitempty"`
}

// OpeningSceneResponse contains the generated opening scene and initial choices.
type OpeningSceneResponse struct {
	Narrative string   `json:"narrative"`
	Choices   []string `json:"choices"`
}

// WorldBuildRequest finalizes startup choices and creates the campaign world.
type WorldBuildRequest struct {
	Name             string            `json:"name"`
	Summary          string            `json:"summary"`
	Profile          *CampaignProfile  `json:"profile"`
	CharacterProfile *CharacterProfile `json:"character_profile"`
	RulesMode        string            `json:"rules_mode,omitempty"`
}

// FeatResponse describes a feat granted to a character.
type FeatResponse struct {
	ID            string `json:"id"`
	FeatID        string `json:"feat_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	BonusType     string `json:"bonus_type"`
	BonusValue    int    `json:"bonus_value"`
	Prerequisites string `json:"prerequisites,omitempty"`
}

// SkillResponse describes a skill allocated to a character.
type SkillResponse struct {
	ID          string `json:"id"`
	SkillID     string `json:"skill_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	BaseAbility string `json:"base_ability"`
	Points      int    `json:"points"`
}

// WorldBuildResponse returns the created campaign plus its opening scene.
type WorldBuildResponse struct {
	Campaign     CampaignResponse     `json:"campaign"`
	OpeningScene OpeningSceneResponse `json:"opening_scene"`
}

// FactResponse represents a world fact visible to the API consumer.
type FactResponse struct {
	ID           string  `json:"id"`
	CampaignID   string  `json:"campaign_id"`
	Fact         string  `json:"fact"`
	Category     string  `json:"category"`
	Source       string  `json:"source"`
	SupersededBy *string `json:"superseded_by,omitempty"`
	PlayerKnown  bool    `json:"player_known"`
	CreatedAt    string  `json:"created_at"`
}

// RelationshipResponse represents an entity relationship.
type RelationshipResponse struct {
	ID               string `json:"id"`
	CampaignID       string `json:"campaign_id"`
	SourceEntityType string `json:"source_entity_type"`
	SourceEntityID   string `json:"source_entity_id"`
	TargetEntityType string `json:"target_entity_type"`
	TargetEntityID   string `json:"target_entity_id"`
	RelationshipType string `json:"relationship_type"`
	Description      string `json:"description"`
	Strength         *int   `json:"strength,omitempty"`
	PlayerAware      bool   `json:"player_aware"`
	CreatedAt        string `json:"created_at"`
}

// LanguageResponse represents a player-known language.
type LanguageResponse struct {
	ID          string `json:"id"`
	CampaignID  string `json:"campaign_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PlayerKnown bool   `json:"player_known"`
	CreatedAt   string `json:"created_at"`
}

// CultureResponse represents a player-known culture.
type CultureResponse struct {
	ID             string  `json:"id"`
	CampaignID     string  `json:"campaign_id"`
	Name           string  `json:"name"`
	LanguageID     *string `json:"language_id,omitempty"`
	BeliefSystemID *string `json:"belief_system_id,omitempty"`
	PlayerKnown    bool    `json:"player_known"`
	CreatedAt      string  `json:"created_at"`
}

// BeliefSystemResponse represents a player-known belief system.
type BeliefSystemResponse struct {
	ID          string `json:"id"`
	CampaignID  string `json:"campaign_id"`
	Name        string `json:"name"`
	PlayerKnown bool   `json:"player_known"`
	CreatedAt   string `json:"created_at"`
}

// EconomicSystemResponse represents a player-known economic system.
type EconomicSystemResponse struct {
	ID          string `json:"id"`
	CampaignID  string `json:"campaign_id"`
	Name        string `json:"name"`
	PlayerKnown bool   `json:"player_known"`
	CreatedAt   string `json:"created_at"`
}

// MapLocationResponse represents a location on the player's map.
type MapLocationResponse struct {
	ID            string `json:"id"`
	CampaignID    string `json:"campaign_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Region        string `json:"region"`
	LocationType  string `json:"location_type"`
	PlayerVisited bool   `json:"player_visited"`
	PlayerKnown   bool   `json:"player_known"`
}

// MapDataResponse is the consolidated map data for a campaign.
type MapDataResponse struct {
	Locations   []MapLocationResponse        `json:"locations"`
	Connections []LocationConnectionResponse `json:"connections"`
}

// EncounteredNPCResponse represents an NPC the player has encountered.
type EncounteredNPCResponse struct {
	ID          string  `json:"id"`
	CampaignID  string  `json:"campaign_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Disposition *int    `json:"disposition,omitempty"`
	Alive       bool    `json:"alive"`
	FactionID   *string `json:"faction_id,omitempty"`
}

// DialogueEntry represents one turn of dialogue involving an NPC.
type DialogueEntry struct {
	TurnNumber  int    `json:"turn_number"`
	PlayerInput string `json:"player_input"`
	LLMResponse string `json:"llm_response"`
	CreatedAt   string `json:"created_at"`
}

// QuestNoteResponse represents a player note on a quest.
type QuestNoteResponse struct {
	ID        string `json:"id"`
	QuestID   string `json:"quest_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// QuestHistoryEntry represents a snapshot of quest state at a point in time.
type QuestHistoryEntry struct {
	ID        string `json:"id"`
	QuestID   string `json:"quest_id"`
	Snapshot  string `json:"snapshot"`
	CreatedAt string `json:"created_at"`
}
