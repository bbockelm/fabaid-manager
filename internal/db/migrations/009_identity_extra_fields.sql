-- +goose Up

-- Add extra fields for CILogon userinfo claims and OIDC display name
ALTER TABLE user_identities ADD COLUMN eppn TEXT;
ALTER TABLE user_identities ADD COLUMN oidc TEXT;
ALTER TABLE user_identities ADD COLUMN cilogon_id TEXT;
ALTER TABLE user_identities ADD COLUMN display_name TEXT;

-- +goose Down
ALTER TABLE user_identities DROP COLUMN IF EXISTS display_name;
ALTER TABLE user_identities DROP COLUMN IF EXISTS cilogon_id;
ALTER TABLE user_identities DROP COLUMN IF EXISTS oidc;
ALTER TABLE user_identities DROP COLUMN IF EXISTS eppn;
