-- +goose Up

-- Allow a fringe (or any) line item's amount to be manually overridden, e.g. to enter
-- a composite rate for institutions whose fringe rate changes mid-year. Override lines
-- are excluded from automatic recomputation (effort cascades) and are flagged in the UI.
ALTER TABLE budget_line_items
    ADD COLUMN is_manual_override BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE budget_line_items DROP COLUMN IF EXISTS is_manual_override;
