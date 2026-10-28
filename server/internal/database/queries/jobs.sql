-- A job with a dedupe key is skipped while another with the same kind and key
-- is still pending or running; the query then returns no row.
-- name: EnqueueJob :one
INSERT INTO jobs (kind, payload, run_at, dedupe_key, max_attempts)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (kind, dedupe_key) WHERE state IN ('pending', 'running') AND dedupe_key IS NOT NULL
DO NOTHING
RETURNING id;

-- Claims due jobs, plus running jobs whose worker's lease expired (it crashed).
-- SKIP LOCKED lets concurrent workers take disjoint batches without waiting.
-- name: ClaimJobs :many
UPDATE jobs
SET state = 'running',
    attempts = attempts + 1,
    locked_until = now() + make_interval(secs => sqlc.arg(lease_seconds)::int)
WHERE id IN (
    SELECT id FROM jobs
    WHERE (state = 'pending' AND run_at <= now())
       OR (state = 'running' AND locked_until < now())
    ORDER BY run_at
    LIMIT sqlc.arg(batch_size)::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteJob :exec
UPDATE jobs
SET state = 'done', finished_at = now(), locked_until = NULL
WHERE id = $1;

-- name: RetryJob :exec
UPDATE jobs
SET state = 'pending', run_at = $2, last_error = $3, locked_until = NULL
WHERE id = $1;

-- name: FailJob :exec
UPDATE jobs
SET state = 'failed', finished_at = now(), last_error = $2, locked_until = NULL
WHERE id = $1;

-- name: DeleteFinishedJobs :execrows
DELETE FROM jobs
WHERE state = 'done' AND finished_at < now() - interval '7 days';

-- name: CountUnfinishedJobs :many
SELECT kind, state, count(*)::int AS total
FROM jobs
WHERE state IN ('pending', 'running', 'failed')
GROUP BY kind, state;
