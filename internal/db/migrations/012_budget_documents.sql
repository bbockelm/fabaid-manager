-- +goose Up

-- Encrypted budget documents (official budgets & budget justifications).
-- Documents are associated with an institution entity (grant or subaward)
-- and optionally an institution budget version.
-- Soft-delete: never actually delete rows; set deleted_at instead.
CREATE TABLE budget_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Association: institution entity (grant or subaward)
    entity_type     TEXT NOT NULL,           -- 'grant' or 'subaward'
    entity_id       UUID NOT NULL,
    -- Optionally linked to a specific budget version
    budget_id       UUID REFERENCES institution_budgets(id) ON DELETE SET NULL,
    -- File metadata
    doc_type        TEXT NOT NULL DEFAULT 'budget',  -- 'budget' or 'budget_justification'
    filename        TEXT NOT NULL,
    content_type    TEXT NOT NULL DEFAULT 'application/pdf',
    s3_key          TEXT NOT NULL,
    file_size       BIGINT DEFAULT 0,
    -- Encryption: per-document DEK encrypted with master-derived key
    encrypted_dek   BYTEA NOT NULL,          -- AES-256-GCM encrypted data encryption key
    dek_nonce       BYTEA NOT NULL,          -- nonce used to encrypt the DEK
    -- Audit
    uploaded_by     UUID REFERENCES users(id),
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Soft delete
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID REFERENCES users(id)
);

CREATE INDEX idx_budget_documents_entity ON budget_documents(entity_type, entity_id);
CREATE INDEX idx_budget_documents_budget ON budget_documents(budget_id) WHERE budget_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS budget_documents;
