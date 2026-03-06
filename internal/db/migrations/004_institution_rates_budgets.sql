-- +goose Up
-- Institution overhead rates, fringe rates (per year), and versioned budgets.

-- Lead institution overhead rates (on grants table)
ALTER TABLE grants ADD COLUMN overhead_on_campus  NUMERIC(5,4) NOT NULL DEFAULT 0;
ALTER TABLE grants ADD COLUMN overhead_off_campus NUMERIC(5,4) NOT NULL DEFAULT 0;

-- Subaward institution overhead rates
ALTER TABLE subawards ADD COLUMN overhead_on_campus  NUMERIC(5,4) NOT NULL DEFAULT 0;
ALTER TABLE subawards ADD COLUMN overhead_off_campus NUMERIC(5,4) NOT NULL DEFAULT 0;

-- Per-institution, per-year fringe rates.
-- entity_type is 'grant' or 'subaward'; entity_id is the grants.id or subawards.id.
CREATE TABLE institution_fringe_rates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('grant', 'subaward')),
    entity_id   UUID NOT NULL,
    fiscal_year INT  NOT NULL CHECK (fiscal_year > 0),
    rate_name   TEXT NOT NULL DEFAULT 'default',
    rate        NUMERIC(5,4) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entity_type, entity_id, fiscal_year, rate_name)
);

CREATE INDEX idx_fringe_rates_entity ON institution_fringe_rates(entity_type, entity_id);

-- Versioned institution budgets (per institution per year).
-- Each row is one version; is_latest marks the active one.
CREATE TABLE institution_budgets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('grant', 'subaward')),
    entity_id   UUID NOT NULL,
    fiscal_year INT  NOT NULL CHECK (fiscal_year > 0),
    version     INT  NOT NULL DEFAULT 1,
    is_latest   BOOLEAN NOT NULL DEFAULT true,
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'final')),
    budget      NUMERIC(14,2) NOT NULL DEFAULT 0,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entity_type, entity_id, fiscal_year, version)
);

CREATE INDEX idx_inst_budgets_entity ON institution_budgets(entity_type, entity_id);
CREATE INDEX idx_inst_budgets_latest ON institution_budgets(entity_type, entity_id, fiscal_year) WHERE is_latest = true;

-- Remove fringe_rate from personnel_year_budgets (now on the institution)
ALTER TABLE personnel_year_budgets DROP COLUMN fringe_rate;

-- +goose Down
ALTER TABLE personnel_year_budgets ADD COLUMN fringe_rate NUMERIC(5,4) DEFAULT 0;
DROP TABLE IF EXISTS institution_budgets;
DROP TABLE IF EXISTS institution_fringe_rates;
ALTER TABLE subawards DROP COLUMN overhead_off_campus;
ALTER TABLE subawards DROP COLUMN overhead_on_campus;
ALTER TABLE grants DROP COLUMN overhead_off_campus;
ALTER TABLE grants DROP COLUMN overhead_on_campus;
