-- name: CreateQuest :one
INSERT INTO quests (
  campaign_id,
  parent_quest_id,
  title,
  description,
  quest_type,
  status
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.narg(parent_quest_id),
  sqlc.arg(title),
  sqlc.narg(description),
  sqlc.arg(quest_type),
  sqlc.arg(status)
)
RETURNING id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at;

-- name: GetQuestByID :one
SELECT id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at
FROM quests
WHERE id = sqlc.arg(id);

-- name: ListQuestsByCampaign :many
SELECT id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at
FROM quests
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: ListActiveQuests :many
SELECT id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at
FROM quests
WHERE campaign_id = sqlc.arg(campaign_id)
  AND status = 'active'
ORDER BY created_at, id;

-- name: ListQuestsByType :many
SELECT id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at
FROM quests
WHERE campaign_id = sqlc.arg(campaign_id)
  AND quest_type = sqlc.arg(quest_type)
ORDER BY created_at, id;

-- name: ListSubquestsByParentQuest :many
SELECT id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at
FROM quests
WHERE parent_quest_id = sqlc.arg(parent_quest_id)
ORDER BY created_at, id;

-- name: UpdateQuest :one
UPDATE quests
SET
  parent_quest_id = sqlc.narg(parent_quest_id),
  title = sqlc.arg(title),
  description = sqlc.narg(description),
  quest_type = sqlc.arg(quest_type),
  status = sqlc.arg(status),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at;

-- name: UpdateQuestStatus :one
UPDATE quests
SET
  status = sqlc.arg(status),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, parent_quest_id, title, description, quest_type, status, created_at, updated_at;
