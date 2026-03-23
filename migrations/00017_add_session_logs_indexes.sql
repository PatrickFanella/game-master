-- +goose Up
CREATE INDEX idx_session_logs_campaign_turn_id_desc
  ON session_logs(campaign_id, turn_number DESC, id DESC);

CREATE INDEX idx_session_logs_campaign_location_turn_id
  ON session_logs(campaign_id, location_id, turn_number, id);

-- +goose Down
DROP INDEX IF EXISTS idx_session_logs_campaign_location_turn_id;
DROP INDEX IF EXISTS idx_session_logs_campaign_turn_id_desc;
