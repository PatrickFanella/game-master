-- +goose Up
DO $$
DECLARE
  embedding_dimension_setting TEXT := COALESCE(
    NULLIF(current_setting('app.embedding_dimension', true), ''),
    NULLIF(current_setting('app.embedding_dimensions', true), ''),
    '1536'
  );
  embedding_dimension INTEGER;
BEGIN
  IF embedding_dimension_setting !~ '^[0-9]+$' THEN
    RAISE EXCEPTION 'app.embedding_dimension/app.embedding_dimensions must be a positive integer';
  END IF;

  embedding_dimension := embedding_dimension_setting::INTEGER;

  IF embedding_dimension <= 0 THEN
    RAISE EXCEPTION 'app.embedding_dimension/app.embedding_dimensions must be a positive integer';
  END IF;

  EXECUTE format($migration$
    CREATE TABLE memories (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      campaign_id UUID NOT NULL,
      content TEXT NOT NULL,
      embedding VECTOR(%1$s) NOT NULL,
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
  $migration$, embedding_dimension);
END
$$;

CREATE INDEX idx_memories_campaign_id ON memories(campaign_id);
CREATE INDEX idx_memories_location_id ON memories(location_id);
CREATE INDEX idx_memories_embedding_hnsw ON memories USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- +goose Down
DROP INDEX IF EXISTS idx_memories_embedding_hnsw;
DROP INDEX IF EXISTS idx_memories_location_id;
DROP INDEX IF EXISTS idx_memories_campaign_id;
DROP TABLE IF EXISTS memories;
