-- +goose Up
ALTER TABLE user_identities ADD COLUMN idp_name TEXT;

-- +goose Down
ALTER TABLE user_identities DROP COLUMN IF EXISTS idp_name;
