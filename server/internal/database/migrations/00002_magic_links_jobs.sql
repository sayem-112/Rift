-- +goose Up

-- Email sign-in links. Single use: redeeming sets used_at in the same statement
-- that checks it, so two clicks can't both sign in.
CREATE TABLE magic_links (
    token_hash  bytea PRIMARY KEY,
    email       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);
CREATE INDEX magic_links_expires_idx ON magic_links (expires_at);

-- Durable background jobs. Workers claim rows with FOR UPDATE SKIP LOCKED, so
-- any number of workers and instances share the queue without double-running.
-- dedupe_key keeps at most one pending job per key (used by periodic jobs).
CREATE TYPE job_state AS ENUM ('pending', 'running', 'done', 'failed');

CREATE TABLE jobs (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind         text NOT NULL,
    payload      jsonb NOT NULL DEFAULT '{}',
    state        job_state NOT NULL DEFAULT 'pending',
    dedupe_key   text,
    attempts     int NOT NULL DEFAULT 0,
    max_attempts int NOT NULL DEFAULT 8,
    run_at       timestamptz NOT NULL DEFAULT now(),
    locked_until timestamptz,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz
);
-- The claim query scans only runnable rows, oldest due first.
CREATE INDEX jobs_runnable_idx ON jobs (run_at) WHERE state IN ('pending', 'running');
CREATE UNIQUE INDEX jobs_dedupe_key ON jobs (kind, dedupe_key) WHERE state IN ('pending', 'running') AND dedupe_key IS NOT NULL;

-- +goose Down
DROP TABLE jobs;
DROP TYPE job_state;
DROP TABLE magic_links;
