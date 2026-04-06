-- name: CreateLocation :one
INSERT INTO locations (
  campaign_id,
  name,
  description,
  region,
  location_type,
  properties
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(name),
  sqlc.arg(description),
  sqlc.arg(region),
  sqlc.arg(location_type),
  COALESCE(sqlc.narg(properties)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known;

-- name: GetLocationByID :one
SELECT id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known
FROM locations
WHERE id = sqlc.arg(id)
  AND (sqlc.narg(campaign_id)::uuid IS NULL OR campaign_id = sqlc.narg(campaign_id));

-- name: ListLocationsByCampaign :many
SELECT id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known
FROM locations
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: ListLocationsByRegion :many
SELECT id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known
FROM locations
WHERE campaign_id = sqlc.arg(campaign_id)
  AND region = sqlc.arg(region)
ORDER BY created_at, id;

-- name: UpdateLocation :one
UPDATE locations
SET
  name = sqlc.arg(name),
  description = sqlc.arg(description),
  region = sqlc.arg(region),
  location_type = sqlc.arg(location_type),
  properties = COALESCE(sqlc.narg(properties)::jsonb, '{}'::jsonb),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known;

-- name: ListPlayerVisitedLocations :many
SELECT id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known
FROM locations
WHERE campaign_id = sqlc.arg(campaign_id)
  AND player_visited = TRUE
ORDER BY created_at, id;

-- name: ListPlayerKnownLocations :many
SELECT id, campaign_id, name, description, region, location_type, properties, created_at, updated_at, player_visited, player_known
FROM locations
WHERE campaign_id = sqlc.arg(campaign_id)
  AND player_known = TRUE
ORDER BY created_at, id;

-- name: SetLocationPlayerVisited :exec
UPDATE locations
SET player_visited = TRUE
WHERE id = sqlc.arg(id);

-- name: SetLocationPlayerKnown :exec
UPDATE locations
SET player_known = TRUE
WHERE id = sqlc.arg(id);
