-- +goose Up

-- Every change to a user's data gets the next number from this per-user
-- counter. Taking it locks the user's row, which serializes that user's
-- writes; per-user write rates are tiny, and it makes versions gapless and
-- ordered.
ALTER TABLE users ADD COLUMN version_seq bigint NOT NULL DEFAULT 0;

-- Server version of the book row's last metadata change.
ALTER TABLE books ADD COLUMN version bigint NOT NULL DEFAULT 0;

-- An in-flight S3 multipart upload. The book row exists from the start (in
-- status 'uploading') so its size counts against quota while bytes are still
-- arriving; it becomes visible to other devices only when completed.
CREATE TYPE upload_state AS ENUM ('uploading', 'completed', 'abandoned');

CREATE TABLE uploads (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id       uuid NOT NULL UNIQUE REFERENCES books (id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    device_id     uuid REFERENCES devices (id) ON DELETE SET NULL,
    s3_upload_id  text NOT NULL,
    part_count    int NOT NULL CHECK (part_count BETWEEN 1 AND 10000),
    state         upload_state NOT NULL DEFAULT 'uploading',
    created_at    timestamptz NOT NULL DEFAULT now(),
    completed_at  timestamptz
);
CREATE INDEX uploads_stale_idx ON uploads (created_at) WHERE state = 'uploading';

-- positions (from 00001) keeps each device's latest position per book, so a
-- write that loses a conflict is still on record. current_positions holds the
-- winner that every device syncs to.
ALTER TABLE positions ADD COLUMN label text NOT NULL DEFAULT '';

CREATE TABLE current_positions (
    book_id    uuid PRIMARY KEY REFERENCES books (id) ON DELETE CASCADE,
    device_id  uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    locator    jsonb NOT NULL,
    progress   real NOT NULL CHECK (progress >= 0 AND progress <= 1),
    label      text NOT NULL DEFAULT '',
    read_at    timestamptz NOT NULL,   -- corrected for the writing device's clock offset
    version    bigint NOT NULL
);

-- The change feed. One row per entity, holding its latest state: writing an
-- entity again replaces its row (and its version), so the feed compacts
-- itself and a new device never replays 500 page turns for one book.
-- Deletions stay as tombstones (op 'deleted') so offline devices learn of them.
CREATE TYPE entity_type AS ENUM ('book', 'position');

CREATE TABLE changes (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    version    bigint NOT NULL,
    entity     entity_type NOT NULL,
    entity_id  uuid NOT NULL,
    op         text NOT NULL CHECK (op IN ('upsert', 'deleted')),
    data       jsonb NOT NULL,
    PRIMARY KEY (user_id, version)
);
CREATE UNIQUE INDEX changes_entity_key ON changes (user_id, entity, entity_id);

-- Responses to mutating requests, keyed by the client's Idempotency-Key, so a
-- retried request replays the original result instead of applying twice.
-- status is NULL while the first request is still running.
CREATE TABLE idempotency_keys (
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key           text NOT NULL CHECK (length(key) BETWEEN 8 AND 128),
    request_hash  bytea NOT NULL,
    status        int,
    response      bytea,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);
CREATE INDEX idempotency_keys_created_idx ON idempotency_keys (created_at);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE changes;
DROP TYPE entity_type;
DROP TABLE current_positions;
ALTER TABLE positions DROP COLUMN label;
DROP TABLE uploads;
DROP TYPE upload_state;
ALTER TABLE books DROP COLUMN version;
ALTER TABLE users DROP COLUMN version_seq;
