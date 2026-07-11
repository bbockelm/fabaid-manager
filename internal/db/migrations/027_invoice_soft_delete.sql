-- +goose Up

-- Invoices are soft-deleted so they can be recovered / audited.
ALTER TABLE invoices ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_invoices_active ON invoices(entity_type, entity_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_invoices_active;
ALTER TABLE invoices DROP COLUMN IF EXISTS deleted_at;
