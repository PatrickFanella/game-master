-- +goose Up
CREATE INDEX idx_factions_campaign_id ON factions(campaign_id);
CREATE INDEX idx_faction_relationships_faction_id ON faction_relationships(faction_id);
CREATE INDEX idx_faction_relationships_related_faction_id ON faction_relationships(related_faction_id);

-- +goose Down
DROP INDEX IF EXISTS idx_faction_relationships_related_faction_id;
DROP INDEX IF EXISTS idx_faction_relationships_faction_id;
DROP INDEX IF EXISTS idx_factions_campaign_id;
