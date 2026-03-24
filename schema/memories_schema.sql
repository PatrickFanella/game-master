-- Static schema definition for sqlc code generation.
-- The actual table is created by migration 00012 with a configurable
-- embedding dimension via dynamic DDL; sqlc cannot parse that PL/pgSQL
-- block, so this file provides the table definition it needs.
CREATE TABLE IF NOT EXISTS memories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL,
  content TEXT NOT NULL,
  embedding VECTOR(1536) NOT NULL,
  memory_type TEXT NOT NULL CHECK (memory_type IN ('turn_summary', 'lore', 'npc_interaction', 'scene', 'world_fact')),
  location_id UUID,
  npcs_involved UUID[] NOT NULL DEFAULT '{}'::UUID[],
  in_game_time TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT memories_campaign_fk
    FOREIGN KEY (campaign_id)
    REFERENCES campaigns(id) ON DELETE RESTRICT,
  CONSTRAINT memories_location_fk
    FOREIGN KEY (location_id)
    REFERENCES locations(id) ON DELETE SET NULL
);
