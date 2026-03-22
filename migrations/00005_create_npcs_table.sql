-- +goose Up
CREATE TABLE npcs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  description TEXT,
  personality TEXT,
  disposition INTEGER NOT NULL DEFAULT 0 CHECK (disposition >= -100 AND disposition <= 100),
  location_id UUID REFERENCES locations(id) ON DELETE RESTRICT,
  faction_id UUID REFERENCES factions(id) ON DELETE RESTRICT,
  alive BOOLEAN NOT NULL DEFAULT true,
  hp INTEGER,
  stats JSONB NOT NULL DEFAULT '{}'::jsonb,
  properties JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_npcs_campaign_id ON npcs(campaign_id);
CREATE INDEX idx_npcs_location_id ON npcs(location_id);
CREATE INDEX idx_npcs_faction_id ON npcs(faction_id);

-- +goose Down
DROP TABLE IF EXISTS npcs;
