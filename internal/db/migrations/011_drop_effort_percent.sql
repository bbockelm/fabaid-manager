-- +goose Up
-- effort_percent is no longer tracked on personnel; effort is only in budget line items.
ALTER TABLE personnel DROP COLUMN effort_percent;

-- +goose Down
ALTER TABLE personnel ADD COLUMN effort_percent NUMERIC DEFAULT 0;
