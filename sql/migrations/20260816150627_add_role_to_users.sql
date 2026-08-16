-- +goose Up
ALTER TABLE users ADD COLUMN role VARCHAR(5) NOT NULL DEFAULT 'user';
ALTER TABLE users ADD CONSTRAINT role_check CHECK (role IN ('user', 'mod', 'admin'));

-- +goose Down
ALTER TABLE users DROP CONSTRAINT role_check;
ALTER TABLE users DROP COLUMN role;
