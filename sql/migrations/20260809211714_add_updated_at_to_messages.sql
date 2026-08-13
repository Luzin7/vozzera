-- +goose Up
ALTER TABLE messages ADD COLUMN updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS updated_at;