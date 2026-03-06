-- +goose Up

-- Track automated and manual backups
CREATE TABLE backups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename    TEXT NOT NULL,
    s3_key      TEXT NOT NULL,
    s3_bucket   TEXT NOT NULL,            -- bucket where backup is stored
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'running',  -- running, completed, failed
    error_msg   TEXT,
    initiated_by TEXT NOT NULL DEFAULT 'scheduler', -- scheduler, manual
    encrypted   BOOLEAN NOT NULL DEFAULT FALSE,
    checksum    TEXT,                      -- SHA-256 of the encrypted archive
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backups_status ON backups (status);
CREATE INDEX idx_backups_started_at ON backups (started_at DESC);

-- Track SHA-256 hashes for all S3 objects we manage
CREATE TABLE object_hashes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    s3_key      TEXT NOT NULL UNIQUE,
    sha256_hash TEXT NOT NULL,
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_object_hashes_s3key ON object_hashes (s3_key);

-- +goose Down

DROP TABLE IF EXISTS object_hashes;
DROP TABLE IF EXISTS backups;
