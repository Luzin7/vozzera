-- +goose Up
ALTER TABLE messages ALTER COLUMN content DROP NOT NULL;
ALTER TABLE messages ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_messages_deleted_at ON messages(deleted_at) WHERE deleted_at IS NOT NULL;


-- +goose Down
DROP INDEX idx_messages_deleted_at;
ALTER TABLE messages DROP COLUMN deleted_at;
ALTER TABLE messages ALTER COLUMN content SET NOT NULL;
