// Package bootstrap handles first-boot setup for the Game Master TUI.
// On first run it creates a default local user and a starter campaign so
// the player can begin immediately. On subsequent runs it returns the
// existing campaigns for the user so the TUI can show a selection list.
package bootstrap

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const (
	// DefaultUserName is the name given to the auto-created local user.
	DefaultUserName = "Player"

	// DefaultCampaignName is the name of the auto-created starter campaign.
	DefaultCampaignName = "The Beginning"

	// DefaultLocationName is the name of the starter campaign's first location.
	DefaultLocationName = "The Crossroads Tavern"

	// DefaultLocationDesc is the description of the starter location.
	DefaultLocationDesc = "A warm tavern at the crossroads of adventure. Every journey starts somewhere."

	// DefaultLocationType is the type tag for the starter location.
	DefaultLocationType = "tavern"
)

// Result holds the user and available campaigns returned by Run.
type Result struct {
	User      statedb.User
	Campaigns []statedb.Campaign
}

// Run ensures a default user and at least one campaign exist in the database.
// If no user named DefaultUserName is found, it creates one. If the user has
// no campaigns, a starter campaign with a default starting location is created.
// The returned Result always contains at least one campaign.
func Run(ctx context.Context, q statedb.Querier) (Result, error) {
	user, err := findOrCreateUser(ctx, q, DefaultUserName)
	if err != nil {
		return Result{}, fmt.Errorf("bootstrap user: %w", err)
	}

	campaigns, err := q.ListCampaignsByUser(ctx, user.ID)
	if err != nil {
		return Result{}, fmt.Errorf("list campaigns: %w", err)
	}

	if len(campaigns) == 0 {
		campaign, err := createStarterCampaign(ctx, q, user.ID)
		if err != nil {
			return Result{}, fmt.Errorf("create starter campaign: %w", err)
		}
		campaigns = []statedb.Campaign{campaign}
	}

	return Result{User: user, Campaigns: campaigns}, nil
}

// CreateCampaign creates a new campaign with the given name for the user and
// adds a default starting location. It is used when the player selects "New
// campaign" from the campaign selection list.
func CreateCampaign(ctx context.Context, q statedb.Querier, userID pgtype.UUID, name string) (statedb.Campaign, error) {
	campaign, err := q.CreateCampaign(ctx, statedb.CreateCampaignParams{
		Name:      name,
		Status:    "active",
		CreatedBy: userID,
	})
	if err != nil {
		return statedb.Campaign{}, fmt.Errorf("create campaign: %w", err)
	}

	_, err = q.CreateLocation(ctx, statedb.CreateLocationParams{
		CampaignID:   campaign.ID,
		Name:         DefaultLocationName,
		Description:  pgtype.Text{String: DefaultLocationDesc, Valid: true},
		LocationType: pgtype.Text{String: DefaultLocationType, Valid: true},
	})
	if err != nil {
		return statedb.Campaign{}, fmt.Errorf("create starting location: %w", err)
	}

	return campaign, nil
}

// findOrCreateUser returns the first user matching name, or creates one.
func findOrCreateUser(ctx context.Context, q statedb.Querier, name string) (statedb.User, error) {
	users, err := q.ListUsers(ctx)
	if err != nil {
		return statedb.User{}, fmt.Errorf("list users: %w", err)
	}
	for _, u := range users {
		if u.Name == name {
			return u, nil
		}
	}
	return q.CreateUser(ctx, name)
}

// createStarterCampaign creates a new campaign named DefaultCampaignName with
// a default starting location for the given user.
func createStarterCampaign(ctx context.Context, q statedb.Querier, userID pgtype.UUID) (statedb.Campaign, error) {
	return CreateCampaign(ctx, q, userID, DefaultCampaignName)
}
