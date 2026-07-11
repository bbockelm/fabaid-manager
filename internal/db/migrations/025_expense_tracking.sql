-- +goose Up

-- Expense tracking: invoices from institutions, coded per billed line to a
-- budget category + WBS areas, so actual expenditure can be compared to budget.

-- 1. Generalize invoices to any billing institution (lead grant OR subaward),
--    mirroring the (entity_type, entity_id) pattern used elsewhere, and add
--    expense-coding status plus a link to the uploaded invoice document.
ALTER TABLE invoices
    ADD COLUMN entity_type   TEXT,
    ADD COLUMN entity_id     UUID,
    ADD COLUMN coding_status TEXT NOT NULL DEFAULT 'uncoded'
        CHECK (coding_status IN ('uncoded', 'draft', 'final')),
    ADD COLUMN document_id   UUID REFERENCES documents(id) ON DELETE SET NULL,
    ADD COLUMN fiscal_year   INT;

-- Backfill existing subaward invoices into the polymorphic columns.
UPDATE invoices SET entity_type = 'subaward', entity_id = subaward_id WHERE entity_id IS NULL;

-- Require the entity columns going forward; relax the legacy subaward_id to
-- nullable (lead-institution invoices have no subaward).
ALTER TABLE invoices
    ALTER COLUMN entity_type SET NOT NULL,
    ALTER COLUMN entity_id SET NOT NULL,
    ALTER COLUMN subaward_id DROP NOT NULL;

CREATE INDEX idx_invoices_entity ON invoices(entity_type, entity_id);

-- 2. Billed expense lines within an invoice. Each line is coded to a category
--    (line_type) and split across WBS areas via invoice_expense_wbs. line_type
--    may be 'uncategorized' when a cost cannot be mapped confidently — allowed
--    but surfaced prominently so it gets cleaned up.
CREATE TABLE invoice_expenses (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id          UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    line_type           TEXT NOT NULL DEFAULT 'uncategorized',
    description         TEXT NOT NULL DEFAULT '',
    amount              NUMERIC(14,2) NOT NULL DEFAULT 0,
    personnel_id        UUID REFERENCES personnel(id) ON DELETE SET NULL,
    budget_line_item_id UUID REFERENCES budget_line_items(id) ON DELETE SET NULL,
    is_capital          BOOLEAN NOT NULL DEFAULT false,
    notes               TEXT NOT NULL DEFAULT '',
    sort_order          INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_invoice_expenses_invoice ON invoice_expenses(invoice_id);

-- 3. WBS allocation for an expense line. Unlike budget_line_item_wbs, these are
--    NOT required to sum to 100% — any remainder is WBS-uncategorized.
CREATE TABLE invoice_expense_wbs (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_expense_id UUID NOT NULL REFERENCES invoice_expenses(id) ON DELETE CASCADE,
    wbs_area_id        UUID NOT NULL REFERENCES wbs_areas(id) ON DELETE CASCADE,
    allocation_percent NUMERIC(6,3) NOT NULL DEFAULT 0
);
CREATE INDEX idx_invoice_expense_wbs_expense ON invoice_expense_wbs(invoice_expense_id);

-- 4. Reuse the AI processing-run machinery for invoice coding. Invoice PDFs live
--    in the plain `documents` table (not budget_documents), so allow a NULL
--    document_id, add an invoice_id link, and a run_type discriminator.
ALTER TABLE document_processing_runs
    ALTER COLUMN document_id DROP NOT NULL,
    ADD COLUMN invoice_id UUID REFERENCES invoices(id) ON DELETE CASCADE,
    ADD COLUMN run_type   TEXT NOT NULL DEFAULT 'budget_extraction';
CREATE INDEX idx_dpr_invoice_id ON document_processing_runs(invoice_id);

-- +goose Down
DROP INDEX IF EXISTS idx_dpr_invoice_id;
ALTER TABLE document_processing_runs
    DROP COLUMN IF EXISTS run_type,
    DROP COLUMN IF EXISTS invoice_id;
-- (document_id NOT NULL is intentionally not restored to avoid failing on invoice rows)

DROP TABLE IF EXISTS invoice_expense_wbs;
DROP TABLE IF EXISTS invoice_expenses;

DROP INDEX IF EXISTS idx_invoices_entity;
ALTER TABLE invoices
    DROP COLUMN IF EXISTS fiscal_year,
    DROP COLUMN IF EXISTS document_id,
    DROP COLUMN IF EXISTS coding_status,
    DROP COLUMN IF EXISTS entity_id,
    DROP COLUMN IF EXISTS entity_type;
