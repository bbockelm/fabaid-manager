-- +goose Up
ALTER TABLE backups ADD COLUMN status_detail TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE backups DROP COLUMN status_detail;
