-- +goose Up
CREATE TABLE location_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  from_location_id UUID NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
  to_location_id UUID NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
  description TEXT,
  bidirectional BOOLEAN NOT NULL DEFAULT FALSE,
  travel_time TEXT,
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  CONSTRAINT location_connections_unique_direction UNIQUE (from_location_id, to_location_id)
);

-- +goose Down
DROP TABLE IF EXISTS location_connections;
