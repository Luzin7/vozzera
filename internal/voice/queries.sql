-- name: GetRoomByID :one
SELECT id, name, type, created_at, updated_at FROM rooms WHERE id = $1 LIMIT 1;

-- name: ListVoiceRooms :many
SELECT id, name, type, created_at, updated_at FROM rooms WHERE type = 'voice' ORDER BY name ASC;
