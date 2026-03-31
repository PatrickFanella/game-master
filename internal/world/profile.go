// Package world provides the campaign-creation interview flow and related
// world-building orchestration.
package world

// CampaignProfile captures the player's preferences gathered during the
// campaign-creation interview. Each field corresponds to one of the topics
// the LLM explores with the player.
type CampaignProfile struct {
	// Genre is the high-level genre of the campaign (e.g. "dark fantasy",
	// "sci-fi", "steampunk").
	Genre string `json:"genre"`
	// Tone describes the overall mood (e.g. "gritty and dark", "light-hearted and
	// humorous").
	Tone string `json:"tone"`
	// Themes lists narrative themes the player wants to explore (e.g.
	// "redemption", "survival", "political intrigue").
	Themes []string `json:"themes"`
	// WorldType describes the kind of world the campaign is set in (e.g.
	// "open wilderness", "urban sprawl", "island archipelago").
	WorldType string `json:"world_type"`
	// DangerLevel indicates how lethal the world should feel (e.g. "low",
	// "moderate", "high", "brutal").
	DangerLevel string `json:"danger_level"`
	// PoliticalComplexity indicates how much political intrigue and faction
	// dynamics the player wants (e.g. "simple", "moderate", "complex").
	PoliticalComplexity string `json:"political_complexity"`
}

// Complete returns true when every field has been populated, indicating the
// interview has gathered sufficient information.
func (p *CampaignProfile) Complete() bool {
	return p.Genre != "" &&
		p.Tone != "" &&
		len(p.Themes) > 0 &&
		p.WorldType != "" &&
		p.DangerLevel != "" &&
		p.PoliticalComplexity != ""
}
