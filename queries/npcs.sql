-- name: CreateNPC :one
INSERT INTO npcs (
  campaign_id,
  name,
  description,
  personality,
  disposition,
  location_id,
  faction_id,
  alive,
  hp,
  stats,
  properties
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(name),
  sqlc.arg(description),
  sqlc.arg(personality),
  sqlc.arg(disposition),
  sqlc.narg(location_id),
  sqlc.narg(faction_id),
  sqlc.arg(alive),
  sqlc.narg(hp),
  COALESCE(sqlc.narg(stats)::jsonb, '{}'::jsonb),
  COALESCE(sqlc.narg(properties)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at;

-- name: GetNPCByID :one
SELECT id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at
FROM npcs
WHERE id = sqlc.arg(id);

-- name: ListNPCsByCampaign :many
SELECT id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at
FROM npcs
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: ListNPCsByLocation :many
SELECT id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at
FROM npcs
WHERE campaign_id = sqlc.arg(campaign_id)
  AND location_id = sqlc.arg(location_id)
ORDER BY created_at, id;

-- name: ListNPCsByFaction :many
SELECT id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at
FROM npcs
WHERE campaign_id = sqlc.arg(campaign_id)
  AND faction_id = sqlc.arg(faction_id)
ORDER BY created_at, id;

-- name: ListAliveNPCsByLocation :many
SELECT id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at
FROM npcs
WHERE campaign_id = sqlc.arg(campaign_id)
  AND location_id = sqlc.arg(location_id)
  AND alive = TRUE
ORDER BY created_at, id;

-- name: UpdateNPC :one
UPDATE npcs
SET
  name = sqlc.arg(name),
  description = sqlc.arg(description),
  personality = sqlc.arg(personality),
  disposition = sqlc.arg(disposition),
  location_id = sqlc.narg(location_id),
  faction_id = sqlc.narg(faction_id),
  alive = sqlc.arg(alive),
  hp = sqlc.narg(hp),
  stats = COALESCE(sqlc.narg(stats)::jsonb, stats),
  properties = COALESCE(sqlc.narg(properties)::jsonb, properties),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at;

-- name: UpdateNPCDisposition :one
UPDATE npcs
SET
  disposition = sqlc.arg(disposition),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at;

-- name: UpdateNPCLocation :one
UPDATE npcs
SET
  location_id = sqlc.narg(location_id),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at;

-- name: KillNPC :one
UPDATE npcs
SET
  alive = FALSE,
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, description, personality, disposition, location_id, faction_id, alive, hp, stats, properties, created_at, updated_at;
