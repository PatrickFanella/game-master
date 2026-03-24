-- name: CreateMemory :one
INSERT INTO memories (
  campaign_id,
  content,
  embedding,
  memory_type,
  location_id,
  npcs_involved,
  in_game_time,
  metadata
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(content),
  sqlc.arg(embedding),
  sqlc.arg(memory_type),
  sqlc.narg(location_id),
  COALESCE(sqlc.narg(npcs_involved)::uuid[], '{}'::uuid[]),
  sqlc.narg(in_game_time),
  COALESCE(sqlc.narg(metadata)::jsonb, '{}'::jsonb)
)
RETURNING id, campaign_id, content, embedding, memory_type, location_id, npcs_involved, in_game_time, metadata, created_at;

-- name: GetMemoryByID :one
SELECT id, campaign_id, content, embedding, memory_type, location_id, npcs_involved, in_game_time, metadata, created_at
FROM memories
WHERE id = sqlc.arg(id);

-- name: ListMemoriesByCampaign :many
SELECT id, campaign_id, content, embedding, memory_type, location_id, npcs_involved, in_game_time, metadata, created_at
FROM memories
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY created_at, id;

-- name: SearchMemoriesBySimilarity :many
SELECT id, campaign_id, content, embedding, memory_type, location_id, npcs_involved, in_game_time, metadata, created_at,
       (embedding <=> sqlc.arg(query_embedding)::vector)::float8 AS distance
FROM memories
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY embedding <=> sqlc.arg(query_embedding)::vector
LIMIT sqlc.arg(limit_count);

-- name: SearchMemoriesWithFilters :many
SELECT id, campaign_id, content, embedding, memory_type, location_id, npcs_involved, in_game_time, metadata, created_at,
       (embedding <=> sqlc.arg(query_embedding)::vector)::float8 AS distance
FROM memories
WHERE campaign_id = sqlc.arg(campaign_id)
  AND (sqlc.narg(memory_type)::text IS NULL OR memory_type = sqlc.narg(memory_type))
  AND (sqlc.narg(location_id)::uuid IS NULL OR location_id = sqlc.narg(location_id))
  AND (sqlc.narg(npc_id)::uuid IS NULL OR sqlc.narg(npc_id)::uuid = ANY(npcs_involved))
  AND (sqlc.narg(start_time)::timestamptz IS NULL OR created_at >= sqlc.narg(start_time))
  AND (sqlc.narg(end_time)::timestamptz IS NULL OR created_at <= sqlc.narg(end_time))
ORDER BY embedding <=> sqlc.arg(query_embedding)::vector
LIMIT sqlc.arg(limit_count);
