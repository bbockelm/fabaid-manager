-- +goose Up
-- Year-based budgeting: track personnel effort and WBS budgets per project year.
-- FabAID is a 5-year project starting 1 May 2026. Fiscal years are 1-5.

-- Per-year personnel effort and salary
CREATE TABLE personnel_year_budgets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    personnel_id    UUID NOT NULL REFERENCES personnel(id) ON DELETE CASCADE,
    fiscal_year     INT NOT NULL CHECK (fiscal_year > 0),
    effort_percent  NUMERIC(5,2) DEFAULT 0,
    funded_months   NUMERIC(5,2) DEFAULT 0,
    salary          NUMERIC(14,2) DEFAULT 0,
    fringe_rate     NUMERIC(5,4) DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(personnel_id, fiscal_year)
);

-- Per-year WBS area budget allocations
CREATE TABLE wbs_year_budgets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wbs_area_id UUID NOT NULL REFERENCES wbs_areas(id) ON DELETE CASCADE,
    fiscal_year INT NOT NULL CHECK (fiscal_year > 0),
    budget      NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(wbs_area_id, fiscal_year)
);

-- Per-year subaward budget allocations (complements statements_of_work)
CREATE TABLE subaward_year_budgets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subaward_id UUID NOT NULL REFERENCES subawards(id) ON DELETE CASCADE,
    fiscal_year INT NOT NULL CHECK (fiscal_year > 0),
    budget      NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(subaward_id, fiscal_year)
);

CREATE INDEX idx_personnel_year_budgets_person ON personnel_year_budgets(personnel_id);
CREATE INDEX idx_wbs_year_budgets_wbs ON wbs_year_budgets(wbs_area_id);
CREATE INDEX idx_subaward_year_budgets_sub ON subaward_year_budgets(subaward_id);

-- +goose Down
DROP TABLE IF EXISTS subaward_year_budgets;
DROP TABLE IF EXISTS wbs_year_budgets;
DROP TABLE IF EXISTS personnel_year_budgets;
