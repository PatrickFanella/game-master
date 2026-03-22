-- +goose Up
CREATE TABLE world_facts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  fact TEXT NOT NULL,
  category TEXT NOT NULL,
  source TEXT NOT NULL,
  superseded_by UUID REFERENCES world_facts(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS world_facts;
