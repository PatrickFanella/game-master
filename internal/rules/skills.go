package rules

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SkillDefinition describes a skill that can be allocated to a character.
type SkillDefinition struct {
	Name        string
	Description string
	BaseAbility string
}

// DefaultSkills returns the standard D&D 5e skills seeded for crunch mode campaigns.
var DefaultSkills = []SkillDefinition{
	{Name: "Acrobatics", Description: "Balance, tumbling, and aerial maneuvers", BaseAbility: "DEX"},
	{Name: "Animal Handling", Description: "Calm, control, or intuit animal behavior", BaseAbility: "WIS"},
	{Name: "Arcana", Description: "Recall lore about spells, magic items, and planes", BaseAbility: "INT"},
	{Name: "Athletics", Description: "Climb, jump, swim, and feats of raw strength", BaseAbility: "STR"},
	{Name: "Deception", Description: "Mislead through words, disguise, or misdirection", BaseAbility: "CHA"},
	{Name: "History", Description: "Recall historical events, people, and civilizations", BaseAbility: "INT"},
	{Name: "Insight", Description: "Read body language and detect lies or intentions", BaseAbility: "WIS"},
	{Name: "Intimidation", Description: "Influence through threats and forceful presence", BaseAbility: "CHA"},
	{Name: "Investigation", Description: "Search for clues, deduce, and analyze evidence", BaseAbility: "INT"},
	{Name: "Medicine", Description: "Stabilize the dying and diagnose ailments", BaseAbility: "WIS"},
	{Name: "Nature", Description: "Recall lore about terrain, plants, and animals", BaseAbility: "INT"},
	{Name: "Perception", Description: "Spot, hear, or detect hidden things", BaseAbility: "WIS"},
	{Name: "Performance", Description: "Entertain through music, dance, or oratory", BaseAbility: "CHA"},
	{Name: "Persuasion", Description: "Influence through tact, diplomacy, and social grace", BaseAbility: "CHA"},
	{Name: "Religion", Description: "Recall lore about deities, rites, and holy symbols", BaseAbility: "INT"},
	{Name: "Sleight of Hand", Description: "Pick pockets, plant objects, and manual trickery", BaseAbility: "DEX"},
	{Name: "Stealth", Description: "Move silently and avoid detection", BaseAbility: "DEX"},
	{Name: "Survival", Description: "Track, forage, navigate, and endure the wild", BaseAbility: "WIS"},
}

// SeedDefaultSkills inserts the standard skill definitions for a crunch mode campaign.
func SeedDefaultSkills(ctx context.Context, db DBTX, campaignID uuid.UUID) error {
	const insertSQL = `INSERT INTO skill_definitions (campaign_id, name, description, base_ability)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING`

	for _, skill := range DefaultSkills {
		if _, err := db.Exec(ctx, insertSQL,
			campaignID, skill.Name, skill.Description, skill.BaseAbility,
		); err != nil {
			return fmt.Errorf("seed skill %q: %w", skill.Name, err)
		}
	}
	return nil
}
