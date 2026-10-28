-- +goose Up

-- Accounts. Sign-in is OAuth or magic link only, so there is no password column.
-- wrapped_key is the per-user data key, encrypted by the server master key.
-- Destroying it (account deletion) makes every book unreadable at once.
CREATE TABLE users (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email       text NOT NULL,
    name        text,
    wrapped_key bytea,
    created_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE deleted_at IS NULL;

-- External sign-in identities (e.g. Google) linked to a user.
CREATE TABLE identities (
    provider    text NOT NULL,
    subject     text NOT NULL,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);

-- A browser or app install that signed in. Revoking one wipes its cache on next contact.
CREATE TABLE devices (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('phone', 'tablet', 'desktop')),
    client       text NOT NULL DEFAULT '',   -- e.g. "Chrome on Android"
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz
);
CREATE INDEX devices_user_idx ON devices (user_id);

-- Refresh tokens, one row per issued token; only a SHA-256 of the token is stored.
-- Each refresh rotates: the old row is marked rotated and a new row joins the same
-- family. Presenting an already-rotated token (outside a short grace window for
-- concurrent refreshes) means it was replayed, so the whole family is revoked.
CREATE TABLE sessions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    family_id          uuid NOT NULL,
    device_id          uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    refresh_token_hash bytea NOT NULL UNIQUE,
    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz NOT NULL,
    rotated_at         timestamptz,
    revoked_at         timestamptz
);
CREATE INDEX sessions_device_idx ON sessions (device_id);
CREATE INDEX sessions_family_idx ON sessions (family_id);

CREATE TYPE book_format AS ENUM ('pdf', 'epub');
CREATE TYPE book_status AS ENUM ('uploading', 'ready');

-- One row per book per user. sha256 is of the plaintext file, so the same file
-- uploaded from two devices collapses into one book.
CREATE TABLE books (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sha256           bytea NOT NULL,
    format           book_format NOT NULL,
    status           book_status NOT NULL DEFAULT 'uploading',
    title            text NOT NULL,
    author           text,
    size_bytes       bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 50 * 1024 * 1024),
    storage_key      text NOT NULL,
    wrapped_key      bytea NOT NULL,
    finished         boolean,            -- manual override; NULL means "derive from progress"
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    trashed_at       timestamptz
);
CREATE UNIQUE INDEX books_user_sha_key ON books (user_id, sha256);
CREATE INDEX books_user_idx ON books (user_id) WHERE trashed_at IS NULL;

-- Latest reading position per book per device. The newest row across devices wins.
-- locator holds {page, offset} for PDF or {cfi} for EPUB.
CREATE TABLE positions (
    book_id    uuid NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    device_id  uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    locator    jsonb NOT NULL,
    progress   real NOT NULL CHECK (progress >= 0 AND progress <= 1),
    read_at    timestamptz NOT NULL,
    PRIMARY KEY (book_id, device_id)
);
CREATE INDEX positions_book_recent_idx ON positions (book_id, read_at DESC);

-- +goose Down
DROP TABLE positions;
DROP TABLE books;
DROP TYPE book_status;
DROP TYPE book_format;
DROP TABLE sessions;
DROP TABLE devices;
DROP TABLE identities;
DROP TABLE users;
