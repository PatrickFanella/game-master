-- name: CreatePlayerCharacter :one
INSERT INTO player_characters (
  campaign_id,
  user_id,
  name,
  description,
  stats,
  hp,
  max_hp,
  experience,
  level,
  status,
  abilities,
  current_location_id
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(user_id),
  sqlc.arg(name),
  sqlc.arg(description),
  COALESCE(sqlc.narg(stats)::jsonb, '{}'::jsonb),
  sqlc.arg(hp),
  sqlc.arg(max_hp),
  sqlc.arg(experience),
  sqlc.arg(level),
  sqlc.arg(status),
  COALESCE(sqlc.narg(abilities)::jsonb, '[]'::jsonb),
  sqlc.narg(current_location_id)
)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: GetPlayerCharacterByID :one
SELECT id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at
FROM player_characters
WHERE id = sqlc.arg(id);

-- name: GetPlayerCharacterByCampaign :many
SELECT id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at
FROM player_characters
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdatePlayerCharacter :one
UPDATE player_characters
SET
  name = sqlc.arg(name),
  description = sqlc.arg(description),
  stats = COALESCE(sqlc.narg(stats)::jsonb, '{}'::jsonb),
  hp = sqlc.arg(hp),
  max_hp = sqlc.arg(max_hp),
  experience = sqlc.arg(experience),
  level = sqlc.arg(level),
  status = sqlc.arg(status),
  abilities = COALESCE(sqlc.narg(abilities)::jsonb, '[]'::jsonb),
  current_location_id = sqlc.narg(current_location_id),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerStats :one
UPDATE player_characters
SET
  stats = COALESCE(sqlc.narg(stats)::jsonb, '{}'::jsonb),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerAbilities :one
UPDATE player_characters
SET
  abilities = COALESCE(sqlc.narg(abilities)::jsonb, '[]'::jsonb),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerHP :one
UPDATE player_characters
SET
  hp = sqlc.arg(hp),
  max_hp = sqlc.arg(max_hp),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerExperience :one
UPDATE player_characters
SET
  experience = sqlc.arg(experience),
  level = sqlc.arg(level),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerLevel :one
UPDATE player_characters
SET
  level = sqlc.arg(level),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerAbilities :one
UPDATE player_characters
SET
  abilities = COALESCE(sqlc.narg(abilities)::jsonb, '[]'::jsonb),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerLocation :one
UPDATE player_characters
SET
  current_location_id = sqlc.narg(current_location_id),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;

-- name: UpdatePlayerStatus :one
UPDATE player_characters
SET
  status = sqlc.arg(status),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, user_id, name, description, stats, hp, max_hp, experience, level, status, abilities, current_location_id, created_at, updated_at;
