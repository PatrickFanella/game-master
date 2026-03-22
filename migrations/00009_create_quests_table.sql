-- +goose Up
CREATE TABLE quests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  parent_quest_id UUID REFERENCES quests(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  description TEXT,
  quest_type TEXT NOT NULL CHECK (quest_type IN ('short_term', 'medium_term', 'long_term')),
  status TEXT NOT NULL CHECK (status IN ('active', 'completed', 'failed', 'abandoned')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE quest_objectives (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quest_id UUID NOT NULL REFERENCES quests(id) ON DELETE CASCADE,
  description TEXT NOT NULL,
  completed BOOLEAN NOT NULL DEFAULT false,
  order_index INTEGER NOT NULL CHECK (order_index >= 0),
  UNIQUE (quest_id, order_index)
);

CREATE INDEX idx_quests_campaign_id ON quests(campaign_id);
CREATE INDEX idx_quests_parent_quest_id ON quests(parent_quest_id);
CREATE INDEX idx_quest_objectives_quest_id ON quest_objectives(quest_id);

-- +goose Down
DROP TABLE IF EXISTS quest_objectives;
DROP TABLE IF EXISTS quests;
