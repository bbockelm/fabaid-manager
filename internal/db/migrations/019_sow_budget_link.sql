-- +goose Up

-- Associate SOW with a specific institution budget instead of storing its own amount.
ALTER TABLE statements_of_work ADD COLUMN budget_id UUID REFERENCES institution_budgets(id) ON DELETE SET NULL;
ALTER TABLE statements_of_work DROP COLUMN budget_amount;

-- +goose Down

ALTER TABLE statements_of_work ADD COLUMN budget_amount NUMERIC(14,2) NOT NULL DEFAULT 0;
ALTER TABLE statements_of_work DROP COLUMN budget_id;
