-- Locks the user's row for the rest of the transaction. Quota checks and
-- version allocation both take this lock, so they serialize per user.
-- name: LockUser :one
SELECT id, wrapped_key FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;

-- name: SetUserKey :exec
UPDATE users SET wrapped_key = $2 WHERE id = $1;

-- Everything still stored counts: ready books, uploads in flight, and trashed
-- books until they are purged.
-- name: UserUsedBytes :one
SELECT COALESCE(SUM(size_bytes), 0)::bigint AS used
FROM books
WHERE user_id = $1;

-- name: NextVersion :one
UPDATE users SET version_seq = version_seq + 1
WHERE id = $1
RETURNING version_seq;

-- name: CurrentVersion :one
SELECT version_seq FROM users WHERE id = $1;

-- name: UpsertChange :exec
INSERT INTO changes (user_id, version, entity, entity_id, op, data)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, entity, entity_id)
DO UPDATE SET version = EXCLUDED.version, op = EXCLUDED.op, data = EXCLUDED.data;

-- name: ListChanges :many
SELECT version, entity, entity_id, op, data
FROM changes
WHERE user_id = $1 AND version > $2
ORDER BY version
LIMIT $3;

-- Wakes every API instance's listener once the transaction commits.
-- name: NotifyChange :exec
SELECT pg_notify('rift_changes', sqlc.arg(user_id)::text);

-- name: GetBookBySHA :one
SELECT * FROM books
WHERE user_id = $1 AND sha256 = $2;

-- name: CreateBook :one
INSERT INTO books (id, user_id, sha256, format, title, author, size_bytes, storage_key, wrapped_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetBook :one
SELECT * FROM books
WHERE id = $1 AND user_id = $2;

-- name: LockBook :one
SELECT * FROM books
WHERE id = $1 AND user_id = $2
FOR UPDATE;

-- name: MarkBookReady :one
UPDATE books SET status = 'ready', version = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- Passing set_finished=false leaves the manual finished override untouched;
-- true sets it to the given value, which may be NULL ("derive from progress").
-- name: UpdateBookMeta :one
UPDATE books SET
    title    = COALESCE(sqlc.narg(title), title),
    author   = COALESCE(sqlc.narg(author), author),
    finished = CASE WHEN sqlc.arg(set_finished)::bool THEN sqlc.narg(finished)::bool ELSE finished END,
    version  = sqlc.arg(version),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetBookTrashed :one
UPDATE books SET
    trashed_at = CASE WHEN sqlc.arg(trashed)::bool THEN now() ELSE NULL END,
    version = sqlc.arg(version),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteBook :exec
DELETE FROM books WHERE id = $1;

-- name: ListPurgeableBooks :many
SELECT id, user_id, storage_key FROM books
WHERE trashed_at < now() - make_interval(days => sqlc.arg(after_days)::int)
LIMIT sqlc.arg(batch_size)::int;

-- name: CreateUpload :one
INSERT INTO uploads (book_id, user_id, device_id, s3_upload_id, part_count)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUploadByBook :one
SELECT * FROM uploads WHERE book_id = $1;

-- name: LockUpload :one
SELECT * FROM uploads
WHERE id = $1 AND user_id = $2
FOR UPDATE;

-- name: SetUploadState :exec
UPDATE uploads SET
    state = sqlc.arg(state),
    completed_at = CASE WHEN sqlc.arg(state) = 'completed'::upload_state THEN now() ELSE completed_at END
WHERE id = sqlc.arg(id);

-- name: ListStaleUploads :many
SELECT u.id, u.book_id, u.user_id, u.s3_upload_id, b.storage_key
FROM uploads u
JOIN books b ON b.id = u.book_id
WHERE u.state = 'uploading'
  AND u.created_at < now() - make_interval(hours => sqlc.arg(after_hours)::int)
LIMIT sqlc.arg(batch_size)::int;

-- name: UpsertDevicePosition :exec
INSERT INTO positions (book_id, device_id, locator, progress, label, read_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (book_id, device_id)
DO UPDATE SET locator = EXCLUDED.locator, progress = EXCLUDED.progress,
              label = EXCLUDED.label, read_at = EXCLUDED.read_at;

-- name: GetCurrentPosition :one
SELECT * FROM current_positions WHERE book_id = $1;

-- name: SetCurrentPosition :one
INSERT INTO current_positions (book_id, device_id, locator, progress, label, read_at, version)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (book_id)
DO UPDATE SET device_id = EXCLUDED.device_id, locator = EXCLUDED.locator,
              progress = EXCLUDED.progress, label = EXCLUDED.label,
              read_at = EXCLUDED.read_at, version = EXCLUDED.version
RETURNING *;
