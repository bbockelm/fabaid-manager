-- +goose Up

-- Records each AI-driven processing run on an uploaded budget document.
CREATE TABLE document_processing_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL REFERENCES budget_documents(id) ON DELETE CASCADE,
    entity_type     TEXT NOT NULL,                     -- 'grant' or 'subaward'
    entity_id       UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',    -- pending, extracting, processing, applying, completed, failed
    status_detail   TEXT NOT NULL DEFAULT '',           -- human-readable status line
    summary_md      TEXT NOT NULL DEFAULT '',           -- AI-generated markdown summary
    conversation    JSONB NOT NULL DEFAULT '[]'::jsonb, -- full LLM conversation (messages array)
    actions_taken   JSONB NOT NULL DEFAULT '[]'::jsonb, -- list of tool calls & results applied
    error_msg       TEXT NOT NULL DEFAULT '',
    llm_model       TEXT NOT NULL DEFAULT '',
    prompt_tokens   INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dpr_document_id ON document_processing_runs(document_id);
CREATE INDEX idx_dpr_entity ON document_processing_runs(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS document_processing_runs;
