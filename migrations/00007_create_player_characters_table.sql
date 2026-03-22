-- +goose Up
CREATE TABLE player_characters (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  description TEXT,
  stats JSONB NOT NULL DEFAULT '{}'::jsonb,
  hp INTEGER NOT NULL DEFAULT 0,
  max_hp INTEGER NOT NULL DEFAULT 0,
  experience INTEGER NOT NULL DEFAULT 0,
  level INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'active',
  abilities JSONB NOT NULL DEFAULT '[]'::jsonb,
  current_location_id UUID REFERENCES locations(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_player_characters_campaign_id ON player_characters(campaign_id);
CREATE INDEX idx_player_characters_user_id ON player_characters(user_id);
CREATE INDEX idx_player_characters_current_location_id ON player_characters(current_location_id);

-- +goose Down
DROP TABLE IF EXISTS player_characters;
