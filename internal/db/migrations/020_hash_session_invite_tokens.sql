-- +goose Up
-- Hash session and invite tokens instead of storing plaintext.
-- Existing sessions are invalidated (users must re-login).
-- Existing unused invites are invalidated (admins must re-create).

-- Sessions: add token_hash column, look up by hash instead of plaintext id
ALTER TABLE sessions ADD COLUMN token_hash BYTEA;

-- Clear existing sessions (we cannot retroactively hash DB-generated UUIDs
-- that were used as cookie values — there is no stored raw token to hash).
DELETE FROM sessions;

ALTER TABLE sessions ALTER COLUMN token_hash SET NOT NULL;
CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions (token_hash);

-- Invites: add token_hash column, stop storing plaintext token
ALTER TABLE invites ADD COLUMN token_hash BYTEA;

-- Clear existing unused invites; used invites don't need the raw token
DELETE FROM invites WHERE used = false;
-- For used invites, set a placeholder hash so NOT NULL works
UPDATE invites SET token_hash = decode(md5(token), 'hex') WHERE token_hash IS NULL;

ALTER TABLE invites ALTER COLUMN token_hash SET NOT NULL;
CREATE UNIQUE INDEX idx_invites_token_hash ON invites (token_hash);

-- Clear the plaintext token from all existing invite rows
UPDATE invites SET token = '';

-- Drop the old UNIQUE constraint on plaintext token (we use token_hash now)
ALTER TABLE invites DROP CONSTRAINT IF EXISTS invites_token_key;
-- Allow NULL/empty tokens since we no longer store them
ALTER TABLE invites ALTER COLUMN token DROP NOT NULL;
ALTER TABLE invites ALTER COLUMN token SET DEFAULT '';

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_token_hash;
ALTER TABLE sessions DROP COLUMN IF EXISTS token_hash;

DROP INDEX IF EXISTS idx_invites_token_hash;
ALTER TABLE invites DROP COLUMN IF EXISTS token_hash;
ALTER TABLE invites ALTER COLUMN token SET NOT NULL;
ALTER TABLE invites ALTER COLUMN token DROP DEFAULT;
ALTER TABLE invites ADD CONSTRAINT invites_token_key UNIQUE (token);
