-- +goose Up
-- Per-institution salary escalation rate (applied compounding each year).
-- e.g. 0.03 = 3% annual salary increase.
ALTER TABLE grants    ADD COLUMN salary_escalation_rate NUMERIC(5,4) NOT NULL DEFAULT 0;
ALTER TABLE subawards ADD COLUMN salary_escalation_rate NUMERIC(5,4) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE subawards DROP COLUMN IF EXISTS salary_escalation_rate;
ALTER TABLE grants    DROP COLUMN IF EXISTS salary_escalation_rate;
