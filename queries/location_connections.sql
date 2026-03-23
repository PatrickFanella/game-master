-- name: CreateConnection :one
INSERT INTO location_connections (
  from_location_id,
  to_location_id,
  description,
  bidirectional,
  travel_time,
  campaign_id
) VALUES (
  sqlc.arg(from_location_id),
  sqlc.arg(to_location_id),
  sqlc.arg(description),
  sqlc.arg(bidirectional),
  sqlc.arg(travel_time),
  sqlc.arg(campaign_id)
)
RETURNING id, from_location_id, to_location_id, description, bidirectional, travel_time, campaign_id;

-- name: GetConnectionsFromLocation :many
SELECT DISTINCT ON (connected_location_id)
  *
FROM (
  SELECT
    lc.id,
    lc.from_location_id,
    lc.to_location_id,
    lc.description,
    lc.bidirectional,
    lc.travel_time,
    lc.campaign_id,
    cl.id AS connected_location_id,
    cl.name AS connected_location_name,
    cl.description AS connected_location_description
  FROM location_connections lc
  JOIN locations cl ON cl.id = lc.to_location_id
  WHERE lc.campaign_id = sqlc.arg(campaign_id)
    AND lc.from_location_id = sqlc.arg(location_id)

  UNION ALL

  SELECT
    lc.id,
    -- Normalize row shape so returned edges are always from the requested location to the connected location.
    lc.to_location_id AS from_location_id,
    lc.from_location_id AS to_location_id,
    lc.description,
    lc.bidirectional,
    lc.travel_time,
    lc.campaign_id,
    cl.id AS connected_location_id,
    cl.name AS connected_location_name,
    cl.description AS connected_location_description
  FROM location_connections lc
  JOIN locations cl ON cl.id = lc.from_location_id
  WHERE lc.campaign_id = sqlc.arg(campaign_id)
    AND lc.to_location_id = sqlc.arg(location_id)
    AND lc.bidirectional = TRUE
) AS connections
ORDER BY connected_location_id, connected_location_name;

-- name: DeleteConnection :exec
DELETE FROM location_connections
WHERE id = sqlc.arg(id)
  AND campaign_id = sqlc.arg(campaign_id);
