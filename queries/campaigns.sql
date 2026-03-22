-- name: CreateCampaign :one
INSERT INTO campaigns (
  name,
  description,
  genre,
  tone,
  themes,
  status,
  created_by
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7
)
RETURNING id, name, description, genre, tone, themes, status, created_by, created_at, updated_at;

-- name: GetCampaignByID :one
SELECT id, name, description, genre, tone, themes, status, created_by, created_at, updated_at
FROM campaigns
WHERE id = $1;

-- name: ListCampaignsByUser :many
SELECT id, name, description, genre, tone, themes, status, created_by, created_at, updated_at
FROM campaigns
WHERE created_by = $1
ORDER BY created_at, id;

-- name: UpdateCampaign :one
UPDATE campaigns
SET
  name = $2,
  description = $3,
  genre = $4,
  tone = $5,
  themes = $6,
  updated_at = now()
WHERE id = $1
RETURNING id, name, description, genre, tone, themes, status, created_by, created_at, updated_at;

-- name: UpdateCampaignStatus :one
UPDATE campaigns
SET
  status = $2,
  updated_at = now()
WHERE id = $1
RETURNING id, name, description, genre, tone, themes, status, created_by, created_at, updated_at;

-- name: DeleteCampaign :exec
DELETE FROM campaigns
WHERE id = $1;
