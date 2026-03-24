-- name: CreateRelationship :one
INSERT INTO entity_relationships (
  campaign_id,
  source_entity_type,
  source_entity_id,
  target_entity_type,
  target_entity_id,
  relationship_type,
  description,
  strength
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(source_entity_type),
  sqlc.arg(source_entity_id),
  sqlc.arg(target_entity_type),
  sqlc.arg(target_entity_id),
  sqlc.arg(relationship_type),
  sqlc.narg(description),
  sqlc.narg(strength)
)
RETURNING id, campaign_id, source_entity_type, source_entity_id, target_entity_type, target_entity_id, relationship_type, description, strength, created_at, updated_at;

-- name: GetRelationshipsByEntity :many
SELECT id, campaign_id, source_entity_type, source_entity_id, target_entity_type, target_entity_id, relationship_type, description, strength, created_at, updated_at
FROM entity_relationships
WHERE campaign_id = sqlc.arg(campaign_id)
  AND (
    (source_entity_type = sqlc.arg(entity_type) AND source_entity_id = sqlc.arg(entity_id))
    OR
    (target_entity_type = sqlc.arg(entity_type) AND target_entity_id = sqlc.arg(entity_id))
  )
ORDER BY created_at, id;

-- name: GetRelationshipsBetween :many
SELECT id, campaign_id, source_entity_type, source_entity_id, target_entity_type, target_entity_id, relationship_type, description, strength, created_at, updated_at
FROM entity_relationships
WHERE campaign_id = sqlc.arg(campaign_id)
  AND (
    (
      source_entity_type = sqlc.arg(first_entity_type)
      AND source_entity_id = sqlc.arg(first_entity_id)
      AND target_entity_type = sqlc.arg(second_entity_type)
      AND target_entity_id = sqlc.arg(second_entity_id)
    )
    OR
    (
      source_entity_type = sqlc.arg(second_entity_type)
      AND source_entity_id = sqlc.arg(second_entity_id)
      AND target_entity_type = sqlc.arg(first_entity_type)
      AND target_entity_id = sqlc.arg(first_entity_id)
    )
  )
ORDER BY created_at, id;

-- name: ListRelationshipsByCampaign :many
SELECT id, campaign_id, source_entity_type, source_entity_id, target_entity_type, target_entity_id, relationship_type, description, strength, created_at, updated_at
FROM entity_relationships
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdateRelationship :one
UPDATE entity_relationships
SET
  relationship_type = sqlc.arg(relationship_type),
  description = sqlc.narg(description),
  strength = sqlc.narg(strength),
  updated_at = now()
WHERE id = sqlc.arg(id)
  AND campaign_id = sqlc.arg(campaign_id)
RETURNING id, campaign_id, source_entity_type, source_entity_id, target_entity_type, target_entity_id, relationship_type, description, strength, created_at, updated_at;

-- name: DeleteRelationship :exec
DELETE FROM entity_relationships
WHERE id = sqlc.arg(id)
  AND campaign_id = sqlc.arg(campaign_id);
