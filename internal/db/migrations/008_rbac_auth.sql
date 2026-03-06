-- +goose Up

-- Application configuration store (OIDC settings, etc.)
CREATE TABLE app_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users of the application
CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active',  -- active, disabled
    last_login   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Roles assigned to users (a user may have multiple roles)
CREATE TABLE user_roles (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    TEXT NOT NULL,  -- admin, grant_admin, read_only
    UNIQUE(user_id, role)
);

-- Federated identities (OIDC issuer + subject)
CREATE TABLE user_identities (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer  TEXT NOT NULL,
    subject TEXT NOT NULL,
    email   TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(issuer, subject)
);

-- Login sessions (cookie-based)
CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,  -- the effective role for this session
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Invite links
CREATE TABLE invites (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token      TEXT NOT NULL UNIQUE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS app_config;
