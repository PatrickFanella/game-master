-- +goose Up
ALTER TABLE languages
  ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE languages
  DROP COLUMN IF EXISTS description;
