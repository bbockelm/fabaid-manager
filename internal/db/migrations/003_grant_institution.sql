-- +goose Up
-- Add institution column to grants for the lead/PI institution.
ALTER TABLE grants ADD COLUMN institution TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE grants DROP COLUMN institution;
