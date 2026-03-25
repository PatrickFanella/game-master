-- name: CreateObjective :one
INSERT INTO quest_objectives (
  quest_id,
  description,
  completed,
  order_index
) VALUES (
  sqlc.arg(quest_id),
  sqlc.arg(description),
  sqlc.arg(completed),
  sqlc.arg(order_index)
)
RETURNING id, quest_id, description, completed, order_index;

-- name: ListObjectivesByQuest :many
SELECT id, quest_id, description, completed, order_index
FROM quest_objectives
WHERE quest_id = sqlc.arg(quest_id)
ORDER BY order_index, id;

-- name: ListObjectivesByQuests :many
SELECT id, quest_id, description, completed, order_index
FROM quest_objectives
WHERE quest_id = ANY(sqlc.slice(quest_ids)::uuid[])
ORDER BY quest_id, order_index, id;

-- name: CompleteObjective :one
UPDATE quest_objectives
SET
  completed = TRUE
WHERE id = sqlc.arg(id)
RETURNING id, quest_id, description, completed, order_index;

-- name: UpdateObjective :one
UPDATE quest_objectives
SET
  description = sqlc.arg(description),
  completed = sqlc.arg(completed),
  order_index = sqlc.arg(order_index)
WHERE id = sqlc.arg(id)
RETURNING id, quest_id, description, completed, order_index;
