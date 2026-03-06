-- +goose Up
-- Core schema for FabAID Manager

-- Grants table: top-level NSF grant/award
CREATE TABLE grants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    award_number    TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    pi_name         TEXT NOT NULL,
    agency          TEXT NOT NULL DEFAULT 'NSF',
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    total_budget    NUMERIC(14,2) NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'no-cost-extension', 'closed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- WBS (Work Breakdown Structure) areas
CREATE TABLE wbs_areas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grant_id    UUID NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
    code        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    budget      NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(grant_id, code)
);

-- Personnel / effort tracking
CREATE TABLE personnel (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grant_id        UUID NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
    wbs_area_id     UUID REFERENCES wbs_areas(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    role            TEXT NOT NULL,
    institution     TEXT,
    effort_percent  NUMERIC(5,2) DEFAULT 0,   -- FTE percentage
    annual_salary   NUMERIC(14,2) DEFAULT 0,
    funded_months   NUMERIC(5,2) DEFAULT 0,
    start_date      DATE,
    end_date        DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Budget line items (planned budget per category per WBS)
CREATE TABLE budget_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grant_id        UUID NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
    wbs_area_id     UUID REFERENCES wbs_areas(id) ON DELETE SET NULL,
    fiscal_year     INT NOT NULL,
    category        TEXT NOT NULL CHECK (category IN (
        'senior_personnel', 'other_personnel', 'fringe',
        'equipment', 'travel', 'participant_support',
        'other_direct', 'indirect', 'subaward'
    )),
    description     TEXT,
    planned_amount  NUMERIC(14,2) NOT NULL DEFAULT 0,
    actual_amount   NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Subawards
CREATE TABLE subawards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grant_id        UUID NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
    institution     TEXT NOT NULL,
    pi_name         TEXT NOT NULL,
    total_amount    NUMERIC(14,2) NOT NULL DEFAULT 0,
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending', 'closed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Invoices (uploaded PDFs from subawardees)
CREATE TABLE invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subaward_id     UUID NOT NULL REFERENCES subawards(id) ON DELETE CASCADE,
    invoice_number  TEXT,
    invoice_date    DATE NOT NULL,
    amount          NUMERIC(14,2) NOT NULL,
    period_start    DATE,
    period_end      DATE,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'paid')),
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Documents (general document storage reference -> S3)
CREATE TABLE documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type     TEXT NOT NULL,  -- 'invoice', 'statement_of_work', 'subaward', etc.
    entity_id       UUID NOT NULL,
    filename        TEXT NOT NULL,
    content_type    TEXT NOT NULL DEFAULT 'application/pdf',
    s3_key          TEXT NOT NULL,
    file_size       BIGINT DEFAULT 0,
    uploaded_by     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Statements of work (annual, per subaward)
CREATE TABLE statements_of_work (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subaward_id     UUID NOT NULL REFERENCES subawards(id) ON DELETE CASCADE,
    fiscal_year     INT NOT NULL,
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    budget_amount   NUMERIC(14,2) NOT NULL DEFAULT 0,
    scope_text      TEXT,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'signed', 'active')),
    signed_doc_id   UUID REFERENCES documents(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(subaward_id, fiscal_year)
);

-- Indexes for common queries
CREATE INDEX idx_wbs_areas_grant ON wbs_areas(grant_id);
CREATE INDEX idx_personnel_grant ON personnel(grant_id);
CREATE INDEX idx_personnel_wbs ON personnel(wbs_area_id);
CREATE INDEX idx_budget_items_grant ON budget_items(grant_id);
CREATE INDEX idx_budget_items_wbs ON budget_items(wbs_area_id);
CREATE INDEX idx_subawards_grant ON subawards(grant_id);
CREATE INDEX idx_invoices_subaward ON invoices(subaward_id);
CREATE INDEX idx_documents_entity ON documents(entity_type, entity_id);
CREATE INDEX idx_sow_subaward ON statements_of_work(subaward_id);

-- +goose Down
DROP TABLE IF EXISTS statements_of_work;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS subawards;
DROP TABLE IF EXISTS budget_items;
DROP TABLE IF EXISTS personnel;
DROP TABLE IF EXISTS wbs_areas;
DROP TABLE IF EXISTS grants;
