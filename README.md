# rift

A reader for the PDFs and EPUBs you already own. Upload a book once and it's on your phone, tablet and laptop, and whichever one you pick up opens at the page you left off on the others.

![The rift library on desktop](web/public/landing/library-desktop.webp)

It's a Go backend and a React web app, and it's still in progress: the backend works end to end, but the web app runs on a sample library until it's wired to the API.

## How it works

Most of the work is in the server, so most of this is about the server.

**Sync.** Every change to a library (a book added, renamed, trashed, a new reading position) gets a per-user version number. Versions are taken under a row lock on the user, so they commit in order and a client can never skip one. A device catches up with `GET /api/sync?since=<version>`, and the change feed compacts itself so it holds one row per item, not one per edit. Deletes leave tombstones so devices that were offline still hear about them.

**Live updates.** When a change commits, Postgres `NOTIFY` fires. Each server instance holds one `LISTEN` connection and fans the signal out to its open Server-Sent Events streams. Clients then fetch the delta themselves, so the notification carries no data and a missed one costs nothing. Streams resume with `Last-Event-ID`.

**Reading positions.** Phone clocks are wrong more often than you'd think. The server corrects each position's timestamp by the difference between the client's clock and its own, and when two devices disagree the later corrected time wins. The losing position is kept, so the app can offer to jump back.

**Uploads.** Books go straight from the browser to S3-compatible storage in 8 MiB parts, each encrypted in the browser. Part URLs are presigned with an exact `Content-Length`, quota is reserved under a lock before anything is sent, and the server lists the parts itself when the upload completes rather than trusting the client. The same file is never stored twice, and an interrupted upload resumes where it stopped.

**Encryption.** Envelope encryption: a master key wraps a key per user, which wraps a key per book. Books are encrypted with AES-GCM, with the owner's ID bound in as associated data so a file can't be moved between accounts.

**Sign-in.** No passwords: a one-time email link or Google (OIDC with PKCE). Access tokens last 15 minutes. Refresh tokens live in an HttpOnly cookie, are stored hashed, and rotate on every use; reusing an old one revokes the whole session. Signing out a device takes effect on its next request.

**Background jobs.** A small job queue in Postgres (`SKIP LOCKED`, leases, retries with backoff) handles cleanup: expired sessions, abandoned uploads, emptying the trash after 30 days.

Things I chose not to add: Kafka, microservices, Kubernetes, gRPC. One Go binary and one Postgres database cover everything above, and running several copies of the binary works because the coordination lives in Postgres.

## Stack

- **Server:** Go 1.26 with the standard library's HTTP router, Postgres 17 (pgx, sqlc, goose migrations), S3 or Cloudflare R2, Prometheus metrics.
- **Web:** React 19, TypeScript, Vite, Motion; Catppuccin Mocha colors on a pure black background for OLED screens.

## Running it locally

You need Go 1.26, Node 22 and Docker.

```bash
cp .env.example .env
```

Fill in `RIFT_TOKEN_SECRET` and `RIFT_MASTER_KEY` in `.env`; `openssl rand -base64 32` gives you a value for each. Then start Postgres (on port 5433, so it won't clash with a local install) and an S3 stand-in (on port 7070):

```bash
docker compose up -d
```

Run the API on port 8080. It applies migrations on start.

```bash
cd server && set -a && . ../.env && set +a && go run ./cmd/rift
```

Without an email API key, sign-in links are printed to the server log. Google sign-in stays off until you set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.

Run the web app on port 5173:

```bash
npm --prefix web install && npm --prefix web run dev
```

The landing page is at `/welcome`. To see the app with books in it, turn on the sample library in Settings.

## Tests

The server's integration tests run against the Postgres and S3 containers, with a fresh database and bucket per test:

```bash
cd server && TEST_DATABASE_URL="postgres://rift:rift@localhost:5433/postgres?sslmode=disable" TEST_S3_ENDPOINT=http://localhost:7070 go test -race ./...
```

Web:

```bash
npm --prefix web run typecheck && npm --prefix web test
```

CI runs both on every push, along with `golangci-lint` and a check that the sqlc output matches the SQL. If you change a query in `server/internal/database/queries/`, run `sqlc generate` in `server/`.
