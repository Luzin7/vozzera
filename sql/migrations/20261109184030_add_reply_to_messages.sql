-- +goose Up
ALTER TABLE messages ADD COLUMN reply_to_message_id UUID REFERENCES messages(id);
ALTER TABLE messages ADD COLUMN reply_to_username TEXT;
ALTER TABLE messages ADD COLUMN reply_to_content TEXT;

-- +goose Down
ALTER TABLE messages DROP COLUMN reply_to_content;
ALTER TABLE messages DROP COLUMN reply_to_username;
ALTER TABLE messages DROP COLUMN reply_to_message_id;
