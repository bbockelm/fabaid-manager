-- +goose Up
ALTER TABLE invites DROP COLUMN IF EXISTS role;

-- +goose Down
ALTER TABLE invites ADD COLUMN role TEXT NOT NULL DEFAULT 'read_only';
