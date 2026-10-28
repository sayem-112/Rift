-- Claims a key for a new request. Returns no row when the key already exists.
-- name: ClaimIdempotencyKey :one
INSERT INTO idempotency_keys (user_id, key, request_hash)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, key) DO NOTHING
RETURNING user_id;

-- name: GetIdempotencyKey :one
SELECT request_hash, status, response FROM idempotency_keys
WHERE user_id = $1 AND key = $2;

-- name: FinishIdempotencyKey :exec
UPDATE idempotency_keys SET status = $3, response = $4
WHERE user_id = $1 AND key = $2;

-- Releases a key whose request failed with a server error, so it can be retried.
-- name: ReleaseIdempotencyKey :exec
DELETE FROM idempotency_keys
WHERE user_id = $1 AND key = $2 AND status IS NULL;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys
WHERE created_at < now() - interval '24 hours';
