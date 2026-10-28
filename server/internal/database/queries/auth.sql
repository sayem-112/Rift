-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- Finds or creates the live account for an email in one statement, so two
-- concurrent first sign-ins with the same address can't create two users.
-- name: UpsertUserByEmail :one
INSERT INTO users (email, name)
VALUES (sqlc.arg(email), sqlc.narg(name))
ON CONFLICT (lower(email)) WHERE deleted_at IS NULL
DO UPDATE SET name = COALESCE(users.name, EXCLUDED.name)
RETURNING *;

-- name: GetIdentityUser :one
SELECT u.* FROM identities i
JOIN users u ON u.id = i.user_id
WHERE i.provider = $1 AND i.subject = $2 AND u.deleted_at IS NULL;

-- name: LinkIdentity :exec
INSERT INTO identities (provider, subject, user_id)
VALUES ($1, $2, $3)
ON CONFLICT (provider, subject) DO NOTHING;

-- name: CreateMagicLink :exec
INSERT INTO magic_links (token_hash, email, expires_at)
VALUES ($1, $2, $3);

-- Check and use in one statement: a link can be redeemed exactly once.
-- name: ConsumeMagicLink :one
UPDATE magic_links
SET used_at = now()
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING email;

-- name: DeleteExpiredMagicLinks :execrows
DELETE FROM magic_links
WHERE expires_at < now() - interval '1 day';

-- name: CreateSession :one
INSERT INTO sessions (family_id, device_id, refresh_token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- Locks the session row so concurrent refreshes of the same token serialize.
-- Time comparisons use the database clock, so app-server skew can't matter.
-- name: GetSessionForRefresh :one
SELECT s.id, s.family_id, s.device_id, d.user_id,
       (s.revoked_at IS NOT NULL OR d.revoked_at IS NOT NULL)::bool AS revoked,
       (s.expires_at <= now())::bool AS expired,
       (s.rotated_at IS NOT NULL)::bool AS rotated,
       (s.rotated_at IS NOT NULL
        AND s.rotated_at > now() - make_interval(secs => sqlc.arg(grace_seconds)::int))::bool AS in_grace
FROM sessions s
JOIN devices d ON d.id = s.device_id
WHERE s.refresh_token_hash = sqlc.arg(token_hash)
FOR UPDATE OF s;

-- name: MarkSessionRotated :exec
UPDATE sessions SET rotated_at = now()
WHERE id = $1 AND rotated_at IS NULL;

-- name: RevokeSessionFamily :exec
UPDATE sessions SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- Rotated rows are kept until they expire: they're what detects a replayed token.
-- name: DeleteStaleSessions :execrows
DELETE FROM sessions
WHERE expires_at < now() - interval '1 day'
   OR revoked_at < now() - interval '30 days';
