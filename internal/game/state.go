package game

import (
	"context"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

// CampaignTime represents the in-game clock for a campaign.
type CampaignTime struct {
	Day    int
	Hour   int
	Minute int
}

// GameState is a snapshot of campaign data needed for the LLM context window.
type GameState struct {
	Campaign                   domain.Campaign
	Player                     domain.PlayerCharacter
	CurrentLocation            domain.Location
	CurrentLocationConnections []domain.LocationConnection
	NearbyNPCs                 []domain.NPC
	ActiveQuests               []domain.Quest
	ActiveQuestObjectives      map[uuid.UUID][]domain.QuestObjective
	PlayerInventory            []domain.Item
	WorldFacts                 []domain.WorldFact
	Time                       *CampaignTime
	RulesMode                  string
	CombatActive               bool
}

// CreateCampaignParams holds parameters for creating a new campaign.
type CreateCampaignParams struct {
	Name        string
	Description string
	Genre       string
	Tone        string
	Themes      []string
	UserID      uuid.UUID
}

// StateManager provides campaign-level composite operations over the database.
type StateManager interface {
	// GetOrCreateDefaultUser returns the default single-player user,
	// creating one if none exists.
	GetOrCreateDefaultUser(ctx context.Context) (*domain.User, error)

	// CreateCampaign creates a new campaign.
	CreateCampaign(ctx context.Context, params CreateCampaignParams) (*domain.Campaign, error)

	// GatherState assembles all relevant campaign state in one call,
	// returning a snapshot suitable for LLM context construction.
	GatherState(ctx context.Context, campaignID uuid.UUID) (*GameState, error)

	// SaveSessionLog persists a turn's session log entry.
	SaveSessionLog(ctx context.Context, log domain.SessionLog) error

	// ListRecentSessionLogs returns the most recent session log entries for a campaign.
	ListRecentSessionLogs(ctx context.Context, campaignID uuid.UUID, limit int) ([]domain.SessionLog, error)

	// GetCampaignByID returns the campaign for the given ID.
	GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (*domain.Campaign, error)
}
