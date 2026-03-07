-- +goose Up

-- Per-grant SOW document template/configuration.
-- Controls the header, intro, costs section, and signature blocks
-- used when generating a SOW for any subaward under this grant.
CREATE TABLE sow_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    grant_id        UUID NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
    -- Header lines
    header_title    TEXT NOT NULL DEFAULT '',
    header_subtitle TEXT NOT NULL DEFAULT '',
    project_name    TEXT NOT NULL DEFAULT '',
    -- Go-template body sections (see handler for available template vars)
    intro_template  TEXT NOT NULL DEFAULT '',
    costs_template  TEXT NOT NULL DEFAULT '',
    -- JSON array: [{"name":"...","title":"...","affiliation":"..."}, ...]
    concurrence_signers JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(grant_id)
);

-- Free-form markdown description of what a person will do in a given SOW.
CREATE TABLE sow_personnel_descriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sow_id          UUID NOT NULL REFERENCES statements_of_work(id) ON DELETE CASCADE,
    personnel_id    UUID NOT NULL REFERENCES personnel(id) ON DELETE CASCADE,
    description_md  TEXT NOT NULL DEFAULT '',
    sort_order      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(sow_id, personnel_id)
);
CREATE INDEX idx_sow_pers_desc_sow ON sow_personnel_descriptions(sow_id);

-- Free-form markdown description for "atypical" budget line items in a SOW
-- (not personnel/salary, fringe, travel, or supplies).
CREATE TABLE sow_line_item_descriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sow_id          UUID NOT NULL REFERENCES statements_of_work(id) ON DELETE CASCADE,
    line_item_id    UUID NOT NULL REFERENCES budget_line_items(id) ON DELETE CASCADE,
    description_md  TEXT NOT NULL DEFAULT '',
    sort_order      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(sow_id, line_item_id)
);
CREATE INDEX idx_sow_li_desc_sow ON sow_line_item_descriptions(sow_id);

-- +goose Down
DROP TABLE IF EXISTS sow_line_item_descriptions;
DROP TABLE IF EXISTS sow_personnel_descriptions;
DROP TABLE IF EXISTS sow_configs;
