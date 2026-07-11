-- +goose Up

-- Capital is not a separate flag: an expense is capital iff its category is
-- 'equipment'. Burn-rate rollups derive "non-capital" from line_type, so the
-- explicit column is unnecessary.
ALTER TABLE invoice_expenses DROP COLUMN IF EXISTS is_capital;

-- +goose Down
ALTER TABLE invoice_expenses ADD COLUMN is_capital BOOLEAN NOT NULL DEFAULT false;
