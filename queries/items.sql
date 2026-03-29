-- name: CreateItem :one
INSERT INTO items (
  campaign_id,
  player_character_id,
  name,
  description,
  item_type,
  rarity,
  properties,
  equipped,
  quantity
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.narg(player_character_id),
  sqlc.arg(name),
  sqlc.narg(description),
  sqlc.arg(item_type),
  sqlc.arg(rarity),
  COALESCE(sqlc.narg(properties)::jsonb, '{}'::jsonb),
  sqlc.arg(equipped),
  sqlc.arg(quantity)
)
RETURNING id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at;

-- name: GetItemByID :one
SELECT id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at
FROM items
WHERE id = sqlc.arg(id);

-- name: ListItemsByPlayer :many
SELECT id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at
FROM items
WHERE campaign_id = sqlc.arg(campaign_id)
  AND player_character_id IS NOT DISTINCT FROM sqlc.narg(player_character_id)
ORDER BY created_at, id;

-- name: ListItemsByType :many
SELECT id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at
FROM items
WHERE campaign_id = sqlc.arg(campaign_id)
  AND item_type = sqlc.arg(item_type)
ORDER BY created_at, id;

-- name: UpdateItem :one
UPDATE items
SET
  name = sqlc.arg(name),
  description = sqlc.narg(description),
  item_type = sqlc.arg(item_type),
  rarity = sqlc.arg(rarity),
  properties = COALESCE(sqlc.narg(properties)::jsonb, properties),
  equipped = sqlc.arg(equipped),
  quantity = sqlc.arg(quantity),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at;

-- name: UpdateItemEquipped :one
UPDATE items
SET
  equipped = sqlc.arg(equipped),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at;

-- name: UpdateItemQuantity :one
UPDATE items
SET
  quantity = sqlc.arg(quantity),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at;

-- name: UpdateItemProperties :one
UPDATE items
SET
  properties = COALESCE(sqlc.narg(properties)::jsonb, properties),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at;

-- name: DeleteItem :exec
DELETE FROM items
WHERE id = sqlc.arg(id);

-- name: TransferItem :one
UPDATE items
SET
  player_character_id = sqlc.narg(player_character_id),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, player_character_id, name, description, item_type, rarity, properties, equipped, quantity, created_at, updated_at;
