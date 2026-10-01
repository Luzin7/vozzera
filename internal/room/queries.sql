-- name: ListRooms :many
SELECT id, name, created_at, updated_at, created_by, has_voice, staff_only
FROM rooms
ORDER BY name ASC;

-- name: GetRoomByID :one
SELECT id, name, created_at, updated_at, created_by, has_voice, staff_only
FROM rooms
WHERE id = $1;

-- name: CreateRoom :one
INSERT INTO rooms (name, created_by, has_voice)
VALUES ($1, $2, $3)
RETURNING id, name, created_at, updated_at, created_by, has_voice, staff_only;

-- name: UpdateRoom :one
UPDATE rooms
SET name = sqlc.arg(name),
    staff_only = COALESCE(sqlc.narg(staff_only)::boolean, staff_only),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, name, created_at, updated_at, created_by, has_voice, staff_only;

-- name: DeleteRoom :one
DELETE FROM rooms
WHERE id = $1
RETURNING id, name, created_at, updated_at, created_by, has_voice, staff_only;

-- name: GetUserRole :one
SELECT role
FROM users
WHERE id = $1;
