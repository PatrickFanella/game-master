-- +goose Up
CREATE TABLE languages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  phonology JSONB NOT NULL DEFAULT '{}'::jsonb,
  naming JSONB NOT NULL DEFAULT '{}'::jsonb,
  vocabulary JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE belief_systems (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE economic_systems (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cultures (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  language_id UUID REFERENCES languages(id) ON DELETE RESTRICT,
  belief_system_id UUID REFERENCES belief_systems(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_languages_campaign_id ON languages(campaign_id);
CREATE INDEX idx_belief_systems_campaign_id ON belief_systems(campaign_id);
CREATE INDEX idx_economic_systems_campaign_id ON economic_systems(campaign_id);
CREATE INDEX idx_cultures_campaign_id ON cultures(campaign_id);
CREATE INDEX idx_cultures_language_id ON cultures(language_id);
CREATE INDEX idx_cultures_belief_system_id ON cultures(belief_system_id);

-- +goose Down
DROP TABLE IF EXISTS cultures;
DROP TABLE IF EXISTS economic_systems;
DROP TABLE IF EXISTS belief_systems;
DROP TABLE IF EXISTS languages;
