-- name: CreateSessionLog :one
INSERT INTO session_logs (
  campaign_id,
  turn_number,
  player_input,
  input_type,
  llm_response,
  tool_calls,
  location_id,
  npcs_involved
) VALUES (
  sqlc.arg(campaign_id),
  sqlc.arg(turn_number),
  sqlc.arg(player_input),
  sqlc.arg(input_type),
  sqlc.arg(llm_response),
  COALESCE(sqlc.narg(tool_calls)::jsonb, '[]'::jsonb),
  sqlc.narg(location_id),
  COALESCE(sqlc.narg(npcs_involved)::uuid[], '{}'::uuid[])
)
RETURNING id, campaign_id, turn_number, player_input, input_type, llm_response, tool_calls, location_id, npcs_involved, created_at;

-- name: GetSessionLogByID :one
SELECT id, campaign_id, turn_number, player_input, input_type, llm_response, tool_calls, location_id, npcs_involved, created_at
FROM session_logs
WHERE id = sqlc.arg(id);

-- name: ListSessionLogsByCampaign :many
SELECT id, campaign_id, turn_number, player_input, input_type, llm_response, tool_calls, location_id, npcs_involved, created_at
FROM session_logs
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY turn_number, id;

-- name: ListRecentSessionLogs :many
SELECT id, campaign_id, turn_number, player_input, input_type, llm_response, tool_calls, location_id, npcs_involved, created_at
FROM session_logs
WHERE campaign_id = sqlc.arg(campaign_id)
ORDER BY turn_number DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListSessionLogsByLocation :many
SELECT id, campaign_id, turn_number, player_input, input_type, llm_response, tool_calls, location_id, npcs_involved, created_at
FROM session_logs
WHERE campaign_id = sqlc.arg(campaign_id)
  AND location_id = sqlc.arg(location_id)
ORDER BY turn_number, id;
