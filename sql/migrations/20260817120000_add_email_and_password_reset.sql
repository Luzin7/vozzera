-- +goose Up
ALTER TABLE users ADD COLUMN email VARCHAR(255) UNIQUE;
UPDATE users SET email = 'user_' || id || '@legacy.local' WHERE email IS NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);

-- +goose Down
DROP TABLE password_reset_tokens;
ALTER TABLE users DROP COLUMN email;
