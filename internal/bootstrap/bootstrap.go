// Package bootstrap handles first-boot setup for the Game Master TUI.
// On first run it creates a default local user. On subsequent runs it returns
// existing campaigns for the user so the TUI can show a selection list.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const (
	// DefaultUserName is the name given to the auto-created local user.
	DefaultUserName = "Player"

	// DefaultCampaignName is the default name used when creating a campaign
	// through engine-level flows.
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

// Run ensures a default user exists in the database and returns all campaigns
// currently owned by that user. If no user named DefaultUserName is found, it
// creates one.
func Run(ctx context.Context, q statedb.Querier) (Result, error) {
	user, err := findOrCreateUser(ctx, q, DefaultUserName)
	if err != nil {
		return Result{}, fmt.Errorf("bootstrap user: %w", err)
	}

	campaigns, err := q.ListCampaignsByUser(ctx, user.ID)
	if err != nil {
		return Result{}, fmt.Errorf("list campaigns: %w", err)
	}

	return Result{User: user, Campaigns: campaigns}, nil
}

// CreateCampaign creates a new campaign with the given name for the user and
// adds a default starting location. It is used when the player selects "New
// campaign" from the campaign selection list.
// The name is trimmed of whitespace; an empty or whitespace-only name returns
// an error.
func CreateCampaign(ctx context.Context, q statedb.Querier, userID pgtype.UUID, name string) (statedb.Campaign, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return statedb.Campaign{}, errors.New("campaign name cannot be empty")
	}

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

// findOrCreateUser returns the user matching name, or creates one.
// It uses GetUserByName for efficiency rather than a full table scan.
func findOrCreateUser(ctx context.Context, q statedb.Querier, name string) (statedb.User, error) {
	u, err := q.GetUserByName(ctx, name)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return statedb.User{}, fmt.Errorf("get user by name: %w", err)
	}
	return q.CreateUser(ctx, name)
}
