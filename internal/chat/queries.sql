-- name: CreateMessage :one
INSERT INTO messages (room_id, user_id, content, reply_to_message_id, reply_to_username, reply_to_content)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, room_id, user_id, content, created_at;

-- name: GetReplyTarget :one
SELECT
    m.id,
    m.content,
    u.username
FROM messages m
JOIN users u ON m.user_id = u.id
WHERE m.id = $1 AND m.room_id = $2 AND m.deleted_at IS NULL;

-- name: GetMessagesByRoom :many
SELECT
    m.id,
    m.content,
    m.created_at,
    u.id AS user_id,
    u.username,
    m.reply_to_message_id AS reply_to_id,
    m.reply_to_username,
    m.reply_to_content
FROM messages m
JOIN users u ON m.user_id = u.id
WHERE m.room_id = $1 AND m.deleted_at IS NULL
ORDER BY m.created_at DESC, m.id DESC
LIMIT $2;

-- name: GetMessagesByRoomBefore :many
SELECT
    m.id,
    m.content,
    m.created_at,
    u.id AS user_id,
    u.username,
    m.reply_to_message_id AS reply_to_id,
    m.reply_to_username,
    m.reply_to_content
FROM messages m
JOIN users u ON m.user_id = u.id
WHERE m.room_id = $1
    AND m.deleted_at IS NULL
    AND (m.created_at, m.id) < (sqlc.arg(before_created_at)::timestamptz, sqlc.arg(before_id)::uuid)
ORDER BY m.created_at DESC, m.id DESC
LIMIT $2;

-- name: UpdateMessage :one
UPDATE messages
SET content = $1, updated_at = NOW()
WHERE id = $2 AND user_id = $3 AND room_id = $4 AND deleted_at IS NULL
RETURNING id, room_id, content, updated_at;

-- name: DeleteMessage :one
UPDATE messages
SET deleted_at = NOW(), content = NULL
WHERE id = $1 AND room_id = sqlc.arg(room_id) AND (user_id = $2 OR sqlc.arg(is_mod)::boolean)
RETURNING id, room_id, user_id, content, created_at;
