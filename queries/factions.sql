-- name: CreateFaction :one
INSERT INTO factions (
  campaign_id,
  name,
  description,
  agenda,
  territory,
  properties
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(name),
  sqlc.arg(description),
  sqlc.arg(agenda),
  sqlc.arg(territory),
  COALESCE(sqlc.narg(properties)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, name, description, agenda, territory, properties, created_at, updated_at;

-- name: GetFactionByID :one
SELECT id, campaign_id, name, description, agenda, territory, properties, created_at, updated_at
FROM factions
WHERE id = sqlc.arg(id);

-- name: ListFactionsByCampaign :many
SELECT id, campaign_id, name, description, agenda, territory, properties, created_at, updated_at
FROM factions
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdateFaction :one
UPDATE factions
SET
  name = sqlc.arg(name),
  description = sqlc.arg(description),
  agenda = sqlc.arg(agenda),
  territory = sqlc.arg(territory),
  properties = COALESCE(sqlc.narg(properties)::jsonb, properties),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, description, agenda, territory, properties, created_at, updated_at;

-- name: CreateFactionRelationship :one
INSERT INTO faction_relationships (
  faction_id,
  related_faction_id,
  relationship_type,
  description
) VALUES (
  sqlc.arg(faction_id),
  sqlc.arg(related_faction_id),
  sqlc.arg(relationship_type),
  sqlc.arg(description)
)
RETURNING id, faction_id, related_faction_id, relationship_type, description, created_at, updated_at;

-- name: GetFactionRelationships :many
SELECT id, faction_id, related_faction_id, relationship_type, description, created_at, updated_at
FROM faction_relationships
WHERE faction_id = sqlc.arg(faction_id)
   OR related_faction_id = sqlc.arg(faction_id)
ORDER BY created_at, id;

-- name: UpdateFactionRelationship :one
UPDATE faction_relationships
SET
  relationship_type = sqlc.arg(relationship_type),
  description = sqlc.arg(description),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, faction_id, related_faction_id, relationship_type, description, created_at, updated_at;
