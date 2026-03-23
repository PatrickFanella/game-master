-- name: CreateFact :one
INSERT INTO world_facts (
  campaign_id,
  fact,
  category,
  source
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(fact),
  sqlc.arg(category),
  sqlc.arg(source)
)
RETURNING id, campaign_id, fact, category, source, superseded_by, created_at;

-- name: GetFactByID :one
SELECT id, campaign_id, fact, category, source, superseded_by, created_at
FROM world_facts
WHERE id = sqlc.arg(id);

-- name: ListFactsByCampaign :many
SELECT id, campaign_id, fact, category, source, superseded_by, created_at
FROM world_facts
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: ListFactsByCategory :many
SELECT id, campaign_id, fact, category, source, superseded_by, created_at
FROM world_facts
WHERE campaign_id = sqlc.arg(campaign_id)
  AND category = sqlc.arg(category)
ORDER BY created_at, id;

-- name: ListActiveFactsByCampaign :many
SELECT id, campaign_id, fact, category, source, superseded_by, created_at
FROM world_facts
WHERE campaign_id = sqlc.arg(campaign_id)
  AND superseded_by IS NULL
ORDER BY created_at, id;

-- name: SupersedeFact :one
WITH previous_fact AS (
  SELECT world_facts.id, world_facts.campaign_id
  FROM world_facts
  WHERE world_facts.id = sqlc.arg(old_fact_id)
    AND world_facts.superseded_by IS NULL
),
new_fact AS (
  INSERT INTO world_facts (
    campaign_id,
    fact,
    category,
    source
  )
  SELECT
    campaign_id,
    sqlc.arg(fact),
    sqlc.arg(category),
    sqlc.arg(source)
  FROM previous_fact
  RETURNING id
),
updated_previous AS (
  UPDATE world_facts
  SET superseded_by = (SELECT id FROM new_fact)
  WHERE world_facts.id = (SELECT id FROM previous_fact)
  RETURNING id
)
SELECT
  world_facts.id,
  world_facts.campaign_id,
  world_facts.fact,
  world_facts.category,
  world_facts.source,
  world_facts.superseded_by,
  world_facts.created_at
FROM world_facts
WHERE world_facts.id = (SELECT id FROM new_fact)
  AND EXISTS (SELECT 1 FROM updated_previous);
