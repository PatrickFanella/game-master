package rules

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal database interface used by the rules package.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// FeatDefinition describes a feat that can be granted to a character.
type FeatDefinition struct {
	Name          string
	Description   string
	Prerequisites string
	BonusType     string
	BonusValue    int
}

// DefaultFeats returns the standard feats seeded for crunch mode campaigns.
var DefaultFeats = []FeatDefinition{
	{Name: "Alert", Description: "+5 to initiative, can't be surprised", BonusType: "initiative", BonusValue: 5},
	{Name: "Tough", Description: "+2 HP per level", BonusType: "hp_per_level", BonusValue: 2},
	{Name: "Lucky", Description: "Reroll 3 dice per long rest", BonusType: "reroll", BonusValue: 3},
	{Name: "Sharpshooter", Description: "No disadvantage at long range", BonusType: "ranged_attack", BonusValue: 0},
	{Name: "Great Weapon Master", Description: "Extra attack on crit/kill", BonusType: "melee_attack", BonusValue: 0},
	{Name: "Sentinel", Description: "Opportunity attacks stop movement", BonusType: "opportunity_attack", BonusValue: 0},
	{Name: "War Caster", Description: "Advantage on concentration saves", BonusType: "concentration", BonusValue: 0},
	{Name: "Resilient", Description: "+1 to chosen ability, proficiency in saves", BonusType: "saving_throw", BonusValue: 1},
	{Name: "Mobile", Description: "+10 speed, no opportunity attacks after melee", BonusType: "speed", BonusValue: 10},
	{Name: "Observant", Description: "+5 passive perception", BonusType: "perception", BonusValue: 5},
	{Name: "Healer", Description: "Stabilize + heal with healer's kit", BonusType: "medicine", BonusValue: 5},
	{Name: "Magic Initiate", Description: "Learn 2 cantrips and 1 level-1 spell", BonusType: "spellcasting", BonusValue: 0},
}

// SeedDefaultFeats inserts the standard feat definitions for a crunch mode campaign.
func SeedDefaultFeats(ctx context.Context, db DBTX, campaignID uuid.UUID) error {
	const insertSQL = `INSERT INTO feat_definitions (campaign_id, name, description, prerequisites, bonus_type, bonus_value)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`

	for _, feat := range DefaultFeats {
		if _, err := db.Exec(ctx, insertSQL,
			campaignID, feat.Name, feat.Description, feat.Prerequisites, feat.BonusType, feat.BonusValue,
		); err != nil {
			return fmt.Errorf("seed feat %q: %w", feat.Name, err)
		}
	}
	return nil
}
