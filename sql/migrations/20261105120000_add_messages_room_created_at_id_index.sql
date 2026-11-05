-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_room_created_at_id
    ON messages (room_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_messages_room_created_at_id;
