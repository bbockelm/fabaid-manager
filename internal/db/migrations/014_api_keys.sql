-- +goose Up

CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    -- Only the bcrypt hash is stored, never the raw key.
    key_hash        TEXT NOT NULL,
    -- Fixed prefix (e.g. "fabaid_") + first 4 random chars; safe to display.
    key_prefix      TEXT NOT NULL,      -- e.g. "fabaid_Ab3x"
    -- Roles this key is authorized for (Postgres text array).
    roles           TEXT[] NOT NULL DEFAULT '{}',
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ,
    -- Idle expiration: key expires if unused for this many seconds.
    idle_timeout_s  INT,                -- NULL = no idle expiration
    -- Hard expiration: key becomes invalid after this date regardless of use.
    expires_at      TIMESTAMPTZ,        -- NULL = no hard expiration
    revoked_at      TIMESTAMPTZ         -- NULL = active; set to soft-revoke
);

CREATE INDEX idx_api_keys_prefix ON api_keys (key_prefix);

-- +goose Down

DROP TABLE IF EXISTS api_keys;
