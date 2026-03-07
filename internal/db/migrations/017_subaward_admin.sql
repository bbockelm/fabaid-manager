-- +goose Up

-- Mapping between users with subaward_admin role and the institutions they manage.
-- The institution is stored as a text string matching subawards.institution / grants.institution.
CREATE TABLE user_institution_access (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    institution    TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, institution)
);

CREATE INDEX idx_user_institution_access_user ON user_institution_access(user_id);
CREATE INDEX idx_user_institution_access_inst ON user_institution_access(institution);

-- +goose Down
DROP TABLE IF EXISTS user_institution_access;
