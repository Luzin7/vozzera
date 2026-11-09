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
WHERE m.room_id = $1 AND m.deleted_at IS NULL
ORDER BY m.created_at DESC, m.id DESC
LIMIT $2;

-- name: GetMessagesByRoomBefore :many
SELECT
    m.id,
    m.content,
    m.created_at,
    u.id AS user_id,
    u.username
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

-- name: LockMessage :one
SELECT id FROM messages WHERE id = sqlc.arg(message_id) FOR UPDATE;

-- name: GetReactionState :one
SELECT
    (SELECT COUNT(DISTINCT emoji) FROM message_reactions WHERE message_id = m.id)::int AS distinct_emoji_count,
    EXISTS (
        SELECT 1 FROM message_reactions mr
        WHERE mr.message_id = m.id AND mr.emoji = sqlc.arg(reaction_emoji)
    ) AS emoji_exists
FROM messages m
WHERE m.id = sqlc.arg(message_id)
    AND m.room_id = sqlc.arg(room_id)
    AND m.deleted_at IS NULL;

-- name: AddReaction :execrows
INSERT INTO message_reactions (message_id, user_id, emoji)
VALUES (sqlc.arg(message_id), sqlc.arg(user_id), sqlc.arg(emoji))
ON CONFLICT DO NOTHING;

-- name: RemoveReaction :execrows
DELETE FROM message_reactions
WHERE message_id = sqlc.arg(message_id)
    AND user_id = sqlc.arg(user_id)
    AND emoji = sqlc.arg(emoji);

-- name: GetReactionsByMessageIDs :many
SELECT
    r.message_id,
    r.emoji,
    COUNT(*)::int AS count,
    array_agg(r.user_id ORDER BY r.created_at, r.user_id)::uuid[] AS user_ids
FROM message_reactions r
WHERE r.message_id = ANY(sqlc.slice('message_ids')::uuid[])
GROUP BY r.message_id, r.emoji
ORDER BY r.message_id, MIN(r.created_at), r.emoji;
