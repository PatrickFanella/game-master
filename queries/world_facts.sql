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
  RETURNING id, campaign_id, fact, category, source, superseded_by, created_at
),
updated_previous AS (
  UPDATE world_facts
  SET superseded_by = (SELECT id FROM new_fact)
  WHERE world_facts.id = (SELECT id FROM previous_fact)
  RETURNING id
)
SELECT
  new_fact.id,
  new_fact.campaign_id,
  new_fact.fact,
  new_fact.category,
  new_fact.source,
  new_fact.superseded_by,
  new_fact.created_at
FROM new_fact
JOIN updated_previous ON TRUE;
