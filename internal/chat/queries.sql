-- name: CreateMessage :one
INSERT INTO messages (room_id, user_id, content)
VALUES ($1, $2, $3)
RETURNING id, room_id, user_id, content, created_at;

-- name: GetMessagesByRoom :many
SELECT
    m.id,
    m.content,
    m.created_at,
    u.id AS user_id,
    u.username
FROM messages m
JOIN users u ON m.user_id = u.id
WHERE m.room_id = $1
ORDER BY m.created_at DESC
LIMIT $2;

-- name: ListRooms :many
SELECT id, name, type, created_at
FROM rooms
ORDER BY name ASC;

-- name: CreateRoom :one
INSERT INTO rooms (name, type)
VALUES ($1, $2)
RETURNING id, name, type, created_at;

-- name: DeleteRoom :one
DELETE FROM rooms
WHERE id = $1
RETURNING id;
