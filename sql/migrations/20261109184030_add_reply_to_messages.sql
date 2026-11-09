-- +goose Up
ALTER TABLE messages ADD COLUMN reply_to_message_id UUID REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN reply_to_username TEXT;
ALTER TABLE messages ADD COLUMN reply_to_content TEXT;
CREATE INDEX idx_messages_reply_to_message_id ON messages(reply_to_message_id);

-- +goose Down
DROP INDEX idx_messages_reply_to_message_id;
ALTER TABLE messages DROP COLUMN reply_to_content;
ALTER TABLE messages DROP COLUMN reply_to_username;
ALTER TABLE messages DROP COLUMN reply_to_message_id;
