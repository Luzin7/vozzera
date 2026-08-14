-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES ($1, $2)
RETURNING id, username, created_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, created_at
FROM users
WHERE username = $1 LIMIT 1;

-- name: InsertSession :one
INSERT INTO sessions (user_id, expires_at)
VALUES ($1, $2)
RETURNING id, expires_at;

-- name: GetSessionByID :one
SELECT s.id AS id, s.expires_at AS expires_at, u.id AS user_id, u.username AS username
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
