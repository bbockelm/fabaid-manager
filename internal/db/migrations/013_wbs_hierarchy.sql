-- +goose Up
-- Add parent-child hierarchy to WBS areas and default WBS allocations per person.

-- Allow WBS areas to have a parent for arbitrary nesting.
ALTER TABLE wbs_areas ADD COLUMN parent_id UUID REFERENCES wbs_areas(id) ON DELETE CASCADE;

-- Default WBS effort allocation per person.
-- Each person can have a default breakdown of effort across WBS areas,
-- used to pre-populate budget entries.
CREATE TABLE personnel_default_wbs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    personnel_id    UUID NOT NULL REFERENCES personnel(id) ON DELETE CASCADE,
    wbs_area_id     UUID NOT NULL REFERENCES wbs_areas(id) ON DELETE CASCADE,
    percent         NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (percent >= 0 AND percent <= 100),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(personnel_id, wbs_area_id)
);

-- +goose Down
DROP TABLE IF EXISTS personnel_default_wbs;
ALTER TABLE wbs_areas DROP COLUMN IF EXISTS parent_id;
