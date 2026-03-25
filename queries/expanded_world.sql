-- name: CreateLanguage :one
INSERT INTO languages (
  campaign_id,
  name,
  description,
  phonology,
  naming,
  vocabulary,
  spoken_by_faction_ids,
  spoken_by_culture_ids
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(name),
  COALESCE(sqlc.narg(description)::text, ''),
  COALESCE(sqlc.narg(phonology)::jsonb, '{}'::jsonb),
  COALESCE(sqlc.narg(naming)::jsonb, '{}'::jsonb),
  COALESCE(sqlc.narg(vocabulary)::jsonb, '{}'::jsonb),
  COALESCE(sqlc.narg(spoken_by_faction_ids)::uuid[], '{}'::uuid[]),
  COALESCE(sqlc.narg(spoken_by_culture_ids)::uuid[], '{}'::uuid[])
)
RETURNING id, campaign_id, name, phonology, naming, vocabulary, created_at, updated_at, spoken_by_faction_ids, spoken_by_culture_ids, description;

-- name: GetLanguageByID :one
SELECT id, campaign_id, name, phonology, naming, vocabulary, created_at, updated_at, spoken_by_faction_ids, spoken_by_culture_ids, description
FROM languages
WHERE id = sqlc.arg(id);

-- name: ListLanguagesByCampaign :many
SELECT id, campaign_id, name, phonology, naming, vocabulary, created_at, updated_at, spoken_by_faction_ids, spoken_by_culture_ids, description
FROM languages
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdateLanguage :one
UPDATE languages
SET
  name = sqlc.arg(name),
  description = COALESCE(sqlc.narg(description)::text, description),
  phonology = COALESCE(sqlc.narg(phonology)::jsonb, phonology),
  naming = COALESCE(sqlc.narg(naming)::jsonb, naming),
  vocabulary = COALESCE(sqlc.narg(vocabulary)::jsonb, vocabulary),
  spoken_by_faction_ids = COALESCE(sqlc.narg(spoken_by_faction_ids)::uuid[], spoken_by_faction_ids),
  spoken_by_culture_ids = COALESCE(sqlc.narg(spoken_by_culture_ids)::uuid[], spoken_by_culture_ids),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, phonology, naming, vocabulary, created_at, updated_at, spoken_by_faction_ids, spoken_by_culture_ids, description;

-- name: DeleteLanguage :exec
DELETE FROM languages
WHERE id = sqlc.arg(id);

-- name: CreateBeliefSystem :one
INSERT INTO belief_systems (
  campaign_id,
  name,
  details
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(name),
  COALESCE(sqlc.narg(details)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, name, details, created_at, updated_at;

-- name: GetBeliefSystemByID :one
SELECT id, campaign_id, name, details, created_at, updated_at
FROM belief_systems
WHERE id = sqlc.arg(id);

-- name: ListBeliefSystemsByCampaign :many
SELECT id, campaign_id, name, details, created_at, updated_at
FROM belief_systems
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdateBeliefSystem :one
UPDATE belief_systems
SET
  name = sqlc.arg(name),
  details = COALESCE(sqlc.narg(details)::jsonb, details),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, details, created_at, updated_at;

-- name: DeleteBeliefSystem :exec
DELETE FROM belief_systems
WHERE id = sqlc.arg(id);

-- name: CreateEconomicSystem :one
INSERT INTO economic_systems (
  campaign_id,
  name,
  details
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(name),
  COALESCE(sqlc.narg(details)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, name, details, created_at, updated_at;

-- name: GetEconomicSystemByID :one
SELECT id, campaign_id, name, details, created_at, updated_at
FROM economic_systems
WHERE id = sqlc.arg(id);

-- name: ListEconomicSystemsByCampaign :many
SELECT id, campaign_id, name, details, created_at, updated_at
FROM economic_systems
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdateEconomicSystem :one
UPDATE economic_systems
SET
  name = sqlc.arg(name),
  details = COALESCE(sqlc.narg(details)::jsonb, details),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, name, details, created_at, updated_at;

-- name: DeleteEconomicSystem :exec
DELETE FROM economic_systems
WHERE id = sqlc.arg(id);

-- name: CreateCulture :one
INSERT INTO cultures (
  campaign_id,
  language_id,
  belief_system_id,
  name,
  details
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.narg(language_id),
  sqlc.narg(belief_system_id),
  sqlc.arg(name),
  COALESCE(sqlc.narg(details)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, language_id, belief_system_id, name, details, created_at, updated_at;

-- name: GetCultureByID :one
SELECT id, campaign_id, language_id, belief_system_id, name, details, created_at, updated_at
FROM cultures
WHERE id = sqlc.arg(id);

-- name: ListCulturesByCampaign :many
SELECT id, campaign_id, language_id, belief_system_id, name, details, created_at, updated_at
FROM cultures
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: UpdateCulture :one
UPDATE cultures
SET
  language_id = sqlc.narg(language_id),
  belief_system_id = sqlc.narg(belief_system_id),
  name = sqlc.arg(name),
  details = COALESCE(sqlc.narg(details)::jsonb, details),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, campaign_id, language_id, belief_system_id, name, details, created_at, updated_at;

-- name: DeleteCulture :exec
DELETE FROM cultures
WHERE id = sqlc.arg(id);

-- name: ListLanguagesByFaction :many
SELECT l.id, l.campaign_id, l.name, l.phonology, l.naming, l.vocabulary, l.created_at, l.updated_at, l.spoken_by_faction_ids, l.spoken_by_culture_ids, l.description
FROM languages l
INNER JOIN factions f
  ON f.id = sqlc.arg(faction_id)::uuid
 AND f.campaign_id = l.campaign_id
WHERE l.spoken_by_faction_ids @> ARRAY[f.id]
ORDER BY l.created_at, l.id;

-- name: GetBeliefSystemByCulture :one
SELECT b.id, b.campaign_id, b.name, b.details, b.created_at, b.updated_at
FROM belief_systems b
INNER JOIN cultures c
  ON c.belief_system_id = b.id
WHERE c.id = sqlc.arg(culture_id);

-- name: ListCulturesByLanguage :many
SELECT id, campaign_id, language_id, belief_system_id, name, details, created_at, updated_at
FROM cultures
WHERE language_id = sqlc.arg(language_id)
ORDER BY created_at, id;
