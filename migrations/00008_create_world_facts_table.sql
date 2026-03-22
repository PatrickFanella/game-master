-- +goose Up
CREATE TABLE world_facts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
  fact TEXT NOT NULL,
  category TEXT NOT NULL,
  source TEXT NOT NULL,
  superseded_by UUID REFERENCES world_facts(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION validate_world_facts_superseded_campaign()
RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'UPDATE'
    AND NEW.campaign_id <> OLD.campaign_id
    AND EXISTS (
      SELECT 1
      FROM world_facts wf
      WHERE wf.superseded_by = NEW.id
    ) THEN
    RAISE EXCEPTION 'cannot change campaign_id of a world_facts row that is referenced by superseded_by';
  END IF;

  IF NEW.superseded_by IS NULL THEN
    RETURN NEW;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM world_facts wf
    WHERE wf.id = NEW.superseded_by
  ) THEN
    RAISE EXCEPTION 'superseded_by must reference an existing world_facts row';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM world_facts wf
    WHERE wf.id = NEW.superseded_by
      AND wf.campaign_id = NEW.campaign_id
  ) THEN
    RAISE EXCEPTION 'superseded_by must reference a world_facts row in the same campaign';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER world_facts_superseded_campaign_trigger
BEFORE INSERT OR UPDATE OF campaign_id, superseded_by ON world_facts
FOR EACH ROW
EXECUTE FUNCTION validate_world_facts_superseded_campaign();

CREATE INDEX idx_world_facts_campaign_id ON world_facts(campaign_id);

-- +goose Down
DROP TRIGGER IF EXISTS world_facts_superseded_campaign_trigger ON world_facts;
DROP FUNCTION IF EXISTS validate_world_facts_superseded_campaign();
DROP TABLE IF EXISTS world_facts;
