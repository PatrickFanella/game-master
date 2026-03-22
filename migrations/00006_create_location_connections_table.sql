-- +goose Up
ALTER TABLE locations
  ADD CONSTRAINT locations_id_campaign_id_unique UNIQUE (id, campaign_id);

CREATE TABLE location_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  from_location_id UUID NOT NULL,
  to_location_id UUID NOT NULL,
  description TEXT,
  bidirectional BOOLEAN NOT NULL DEFAULT FALSE,
  travel_time TEXT,
  campaign_id UUID NOT NULL,
  CONSTRAINT location_connections_campaign_fk
    FOREIGN KEY (campaign_id)
    REFERENCES campaigns(id) ON DELETE RESTRICT,
  CONSTRAINT location_connections_from_location_fk
    FOREIGN KEY (from_location_id, campaign_id)
    REFERENCES locations(id, campaign_id) ON DELETE RESTRICT,
  CONSTRAINT location_connections_to_location_fk
    FOREIGN KEY (to_location_id, campaign_id)
    REFERENCES locations(id, campaign_id) ON DELETE RESTRICT,
  CONSTRAINT location_connections_unique_direction UNIQUE (from_location_id, to_location_id)
);

-- +goose Down
DROP TABLE IF EXISTS location_connections;
ALTER TABLE locations
  DROP CONSTRAINT IF EXISTS locations_id_campaign_id_unique;
