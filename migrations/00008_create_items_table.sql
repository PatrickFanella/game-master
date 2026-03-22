-- +goose Up
CREATE TABLE items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  player_character_id UUID REFERENCES player_characters(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  description TEXT,
  item_type TEXT NOT NULL CHECK (item_type IN ('weapon', 'armor', 'consumable', 'quest', 'misc')),
  rarity TEXT NOT NULL,
  properties JSONB NOT NULL DEFAULT '{}'::jsonb,
  equipped BOOLEAN NOT NULL DEFAULT false,
  quantity INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_items_campaign_id ON items(campaign_id);
CREATE INDEX idx_items_player_character_id ON items(player_character_id);

-- +goose Down
DROP TABLE IF EXISTS items;
