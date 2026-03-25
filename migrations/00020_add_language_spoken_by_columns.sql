-- +goose Up
ALTER TABLE languages
  ADD COLUMN spoken_by_faction_ids UUID[] NOT NULL DEFAULT '{}'::uuid[],
  ADD COLUMN spoken_by_culture_ids UUID[] NOT NULL DEFAULT '{}'::uuid[];

CREATE INDEX idx_languages_spoken_by_faction_ids ON languages USING gin (spoken_by_faction_ids);
CREATE INDEX idx_languages_spoken_by_culture_ids ON languages USING gin (spoken_by_culture_ids);

-- +goose Down
DROP INDEX IF EXISTS idx_languages_spoken_by_culture_ids;
DROP INDEX IF EXISTS idx_languages_spoken_by_faction_ids;

ALTER TABLE languages
  DROP COLUMN IF EXISTS spoken_by_culture_ids,
  DROP COLUMN IF EXISTS spoken_by_faction_ids;
