-- +goose Up
CREATE TABLE entity_relationships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  source_entity_type TEXT NOT NULL CHECK (source_entity_type IN ('npc', 'location', 'faction', 'player_character', 'item')),
  source_entity_id UUID NOT NULL,
  target_entity_type TEXT NOT NULL CHECK (target_entity_type IN ('npc', 'location', 'faction', 'player_character', 'item')),
  target_entity_id UUID NOT NULL,
  relationship_type TEXT NOT NULL,
  description TEXT,
  strength INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_entity_relationships_source ON entity_relationships(source_entity_type, source_entity_id);
CREATE INDEX idx_entity_relationships_target ON entity_relationships(target_entity_type, target_entity_id);
CREATE INDEX idx_entity_relationships_campaign_id ON entity_relationships(campaign_id);

-- +goose Down
DROP TABLE IF EXISTS entity_relationships;
