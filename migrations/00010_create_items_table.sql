-- +goose Up
CREATE TABLE items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL,
  player_character_id UUID,
  name TEXT NOT NULL,
  description TEXT,
  item_type TEXT NOT NULL CHECK (item_type IN ('weapon', 'armor', 'consumable', 'quest', 'misc')),
  rarity TEXT NOT NULL,
  properties JSONB NOT NULL DEFAULT '{}'::jsonb,
  equipped BOOLEAN NOT NULL DEFAULT false,
  quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT items_campaign_fk
    FOREIGN KEY (campaign_id)
    REFERENCES campaigns(id) ON DELETE RESTRICT,
  CONSTRAINT items_player_character_campaign_fk
    FOREIGN KEY (player_character_id, campaign_id)
    REFERENCES player_characters(id, campaign_id) ON DELETE RESTRICT
);

CREATE INDEX idx_items_campaign_id ON items(campaign_id);
CREATE INDEX idx_items_player_character_id ON items(player_character_id);

-- +goose Down
DROP TABLE IF EXISTS items;
