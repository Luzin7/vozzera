-- name: CreateUser :one
INSERT INTO users (username, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, username, created_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, created_at, email
FROM users
WHERE username = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT id, username, password_hash, created_at, role, email
FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT id, username, email
FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByIdentifier :one
SELECT id, username, password_hash, created_at, role, email
FROM users
WHERE username = $1 OR email = $2
ORDER BY (username = $1) DESC
LIMIT 1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = sqlc.arg(password_hash) WHERE id = sqlc.arg(id);

-- name: UpdateUserEmail :exec
UPDATE users SET email = sqlc.arg(email) WHERE id = sqlc.arg(id);

-- name: InsertPasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, created_at;

-- name: GetPasswordResetTokenByHash :one
SELECT id, user_id, expires_at
FROM password_reset_tokens
WHERE token_hash = $1 AND expires_at > NOW()
LIMIT 1;

-- name: DeletePasswordResetToken :exec
DELETE FROM password_reset_tokens WHERE id = $1;

-- name: CleanupExpiredPasswordResetTokens :exec
DELETE FROM password_reset_tokens WHERE expires_at < NOW();

-- name: InsertSession :one
INSERT INTO sessions (user_id, expires_at)
VALUES ($1, $2)
RETURNING id, expires_at;

-- name: GetSessionByID :one
SELECT s.id AS id, s.expires_at AS expires_at, u.id AS user_id, u.username AS username, u.role AS role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = $1;

-- name: TouchSession :exec
UPDATE sessions
SET expires_at = $2
WHERE id = $1 AND expires_at < $2;

-- name: DeleteSessionByID :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionsByUser :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: CleanupExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();
