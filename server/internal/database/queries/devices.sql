-- name: CreateDevice :one
INSERT INTO devices (user_id, name, kind, client)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListDevices :many
SELECT * FROM devices
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY last_seen_at DESC;

-- name: GetActiveDevice :one
SELECT * FROM devices
WHERE id = $1 AND revoked_at IS NULL;

-- Throttled so a busy device writes at most once every five minutes.
-- name: TouchDevice :exec
UPDATE devices SET last_seen_at = now()
WHERE id = $1 AND last_seen_at < now() - interval '5 minutes';

-- name: RevokeDevice :execrows
UPDATE devices SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: RevokeDeviceSessions :exec
UPDATE sessions SET revoked_at = now()
WHERE device_id = $1 AND revoked_at IS NULL;
