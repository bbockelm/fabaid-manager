-- +goose Up
-- Rearchitect budgeting: institution → budget (versioned) → line items → WBS splits.
-- Replaces grant-level budget_items with institution-scoped line items.

-- Flexible overhead rates per institution (replaces overhead_on_campus/off_campus columns).
-- An institution may have multiple rates, e.g. "MTDC On-Campus", "MTDC Off-Campus", "TDC".
CREATE TABLE institution_overhead_rates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('grant', 'subaward')),
    entity_id   UUID NOT NULL,
    rate_name   TEXT NOT NULL,
    rate        NUMERIC(7,4) NOT NULL DEFAULT 0,
    description TEXT DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entity_type, entity_id, rate_name)
);

CREATE INDEX idx_overhead_rates_entity ON institution_overhead_rates(entity_type, entity_id);

-- Budget line items belong to a versioned institution_budget.
-- Each line item is one cost in the budget: personnel effort, travel, equipment, etc.
CREATE TABLE budget_line_items (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    institution_budget_id  UUID NOT NULL REFERENCES institution_budgets(id) ON DELETE CASCADE,
    line_type              TEXT NOT NULL CHECK (line_type IN (
        'personnel', 'fringe', 'travel', 'equipment', 'supplies',
        'contractual', 'participant_support', 'tuition', 'other'
    )),
    description            TEXT DEFAULT '',
    -- For personnel lines: link to person and effort
    personnel_id           UUID REFERENCES personnel(id) ON DELETE SET NULL,
    effort_months          NUMERIC(5,2) DEFAULT 0,
    -- Direct cost amount (for personnel: salary portion; for others: the cost)
    amount                 NUMERIC(14,2) NOT NULL DEFAULT 0,
    -- If this line incurs overhead, which rate applies
    overhead_rate_id       UUID REFERENCES institution_overhead_rates(id) ON DELETE SET NULL,
    notes                  TEXT DEFAULT '',
    sort_order             INT NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_line_items_budget ON budget_line_items(institution_budget_id);
CREATE INDEX idx_line_items_personnel ON budget_line_items(personnel_id) WHERE personnel_id IS NOT NULL;

-- WBS cost allocation for line items (many-to-many with split percentages).
-- A line item's cost can be split across multiple WBS areas.
CREATE TABLE budget_line_item_wbs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    line_item_id      UUID NOT NULL REFERENCES budget_line_items(id) ON DELETE CASCADE,
    wbs_area_id       UUID NOT NULL REFERENCES wbs_areas(id) ON DELETE CASCADE,
    allocation_percent NUMERIC(5,2) NOT NULL DEFAULT 100,
    UNIQUE(line_item_id, wbs_area_id)
);

CREATE INDEX idx_line_item_wbs_item ON budget_line_item_wbs(line_item_id);
CREATE INDEX idx_line_item_wbs_wbs ON budget_line_item_wbs(wbs_area_id);

-- Drop the old overhead columns from grants/subawards (replaced by institution_overhead_rates).
ALTER TABLE grants DROP COLUMN IF EXISTS overhead_on_campus;
ALTER TABLE grants DROP COLUMN IF EXISTS overhead_off_campus;
ALTER TABLE subawards DROP COLUMN IF EXISTS overhead_on_campus;
ALTER TABLE subawards DROP COLUMN IF EXISTS overhead_off_campus;

-- Drop old grant-level budget tables (replaced by institution-scoped line items).
DROP TABLE IF EXISTS budget_items;
DROP TABLE IF EXISTS personnel_year_budgets;
DROP TABLE IF EXISTS wbs_year_budgets;
DROP TABLE IF EXISTS subaward_year_budgets;

-- +goose Down
-- Restore old tables
CREATE TABLE budget_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grant_id        UUID NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
    wbs_area_id     UUID REFERENCES wbs_areas(id) ON DELETE SET NULL,
    fiscal_year     INT NOT NULL,
    category        TEXT NOT NULL,
    description     TEXT,
    planned_amount  NUMERIC(14,2) NOT NULL DEFAULT 0,
    actual_amount   NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE personnel_year_budgets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    personnel_id    UUID NOT NULL REFERENCES personnel(id) ON DELETE CASCADE,
    fiscal_year     INT NOT NULL CHECK (fiscal_year > 0),
    effort_percent  NUMERIC(5,2) DEFAULT 0,
    funded_months   NUMERIC(5,2) DEFAULT 0,
    salary          NUMERIC(14,2) DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(personnel_id, fiscal_year)
);

CREATE TABLE wbs_year_budgets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wbs_area_id UUID NOT NULL REFERENCES wbs_areas(id) ON DELETE CASCADE,
    fiscal_year INT NOT NULL CHECK (fiscal_year > 0),
    budget      NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(wbs_area_id, fiscal_year)
);

CREATE TABLE subaward_year_budgets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subaward_id UUID NOT NULL REFERENCES subawards(id) ON DELETE CASCADE,
    fiscal_year INT NOT NULL CHECK (fiscal_year > 0),
    budget      NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(subaward_id, fiscal_year)
);

ALTER TABLE grants ADD COLUMN overhead_on_campus  NUMERIC(5,4) NOT NULL DEFAULT 0;
ALTER TABLE grants ADD COLUMN overhead_off_campus NUMERIC(5,4) NOT NULL DEFAULT 0;
ALTER TABLE subawards ADD COLUMN overhead_on_campus  NUMERIC(5,4) NOT NULL DEFAULT 0;
ALTER TABLE subawards ADD COLUMN overhead_off_campus NUMERIC(5,4) NOT NULL DEFAULT 0;

DROP TABLE IF EXISTS budget_line_item_wbs;
DROP TABLE IF EXISTS budget_line_items;
DROP TABLE IF EXISTS institution_overhead_rates;
