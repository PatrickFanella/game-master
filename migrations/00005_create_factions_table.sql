-- +goose Up
CREATE TABLE factions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  description TEXT,
  agenda TEXT,
  territory TEXT,
  properties JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE faction_relationships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  faction_id UUID NOT NULL REFERENCES factions(id) ON DELETE CASCADE,
  related_faction_id UUID NOT NULL REFERENCES factions(id) ON DELETE CASCADE,
  relationship_type TEXT NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT faction_relationships_distinct_factions CHECK (faction_id <> related_faction_id)
);

CREATE UNIQUE INDEX faction_relationships_unique_pair
  ON faction_relationships (LEAST(faction_id, related_faction_id), GREATEST(faction_id, related_faction_id));

-- +goose Down
DROP TABLE IF EXISTS faction_relationships;
DROP TABLE IF EXISTS factions;
