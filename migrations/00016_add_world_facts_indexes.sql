-- +goose Up
CREATE INDEX idx_world_facts_campaign_category_created_id
  ON world_facts(campaign_id, category, created_at, id);

CREATE INDEX idx_world_facts_active_campaign_created_id
  ON world_facts(campaign_id, created_at, id)
  WHERE superseded_by IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_world_facts_active_campaign_created_id;
DROP INDEX IF EXISTS idx_world_facts_campaign_category_created_id;
