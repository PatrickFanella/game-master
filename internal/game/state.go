package game

import (
	"context"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

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

	// LoadCampaign loads a campaign and its core associated entities.
	LoadCampaign(ctx context.Context, id uuid.UUID) (*GameState, error)

	// GetGameState returns a snapshot suitable for LLM context construction.
	GetGameState(ctx context.Context, campaignID uuid.UUID) (*GameState, error)

	// GatherState assembles all relevant campaign state in one call.
	GatherState(ctx context.Context, campaignID uuid.UUID) (*GameState, error)

	// SaveSessionLog persists a turn's session log entry.
	SaveSessionLog(ctx context.Context, log domain.SessionLog) error
}
