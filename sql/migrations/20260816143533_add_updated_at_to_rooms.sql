-- +goose Up
ALTER TABLE rooms ADD COLUMN updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE rooms DROP COLUMN updated_at;
