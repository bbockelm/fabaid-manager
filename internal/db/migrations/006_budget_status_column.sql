-- +goose Up
ALTER TABLE institution_budgets
    ADD COLUMN status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'final'));

-- +goose Down
ALTER TABLE institution_budgets DROP COLUMN IF EXISTS status;
