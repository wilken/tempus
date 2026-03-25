# Tempus

A simple time registration app built with Go, SQLite, and plain HTML.

## Features

- Per-day view with task / subtask / hours lines
- Previous / Next day navigation
- Google sign-in (your name is pulled from your Google account)
- Auto-save when tabbing out of a field or deleting a row
- Weekly read-only summary view
- Export a full week to an Excel file (.xlsx)

## Requirements

- Go 1.21+ (for running locally)
- A Google OAuth 2.0 credential (see below)

## Google OAuth setup

1. Go to <https://console.cloud.google.com/> → **APIs & Services** → **Credentials**
2. Create an **OAuth 2.0 Client ID** (Application type: **Web application**)
3. Under **Authorised redirect URIs** add: `http://localhost:8080/auth/callback`
4. Copy the **Client ID** and **Client Secret**

## Running locally

```bash
cp .env.example .env
# Edit .env and fill in your Google credentials and a random SESSION_SECRET
source .env
go run ./cmd
```

Then open <http://localhost:8080> in your browser.

## Running with Docker

```bash
cp .env.docker.example .env.docker
# Edit .env.docker and fill in your Google credentials and a random SESSION_SECRET
docker compose up -d
```

The app will be available at <http://localhost> (port 80).

The database is stored in a Docker named volume called `storage`, mounted at `/storage` inside the container.

### Migrating an existing database into Docker

Copy all three SQLite files together to preserve WAL state, then fix ownership so the container user can write to them:

```bash
docker compose cp tempus.db app:/storage/tempus.db
docker compose cp tempus.db-wal app:/storage/tempus.db-wal
docker compose cp tempus.db-shm app:/storage/tempus.db-shm
docker run --rm --user root -v tempus_storage:/storage alpine chown -R 100:101 /storage
docker compose restart app
```

## Environment variables

| Variable              | Default                                | Required |
|-----------------------|----------------------------------------|----------|
| `GOOGLE_CLIENT_ID`    | —                                      | ✅       |
| `GOOGLE_CLIENT_SECRET`| —                                      | ✅       |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/auth/callback`  |          |
| `SESSION_SECRET`      | `dev-secret-change-in-production`      |          |
| `DATABASE_PATH`       | `tempus.db`                            |          |
| `PORT`                | `8080`                                 |          |

## Health check

`GET /up` returns `200 ok` — useful for uptime monitoring or load balancer checks.

## Building a binary

```bash
go build -o tempus ./cmd
./tempus
```

> **Note:** The binary must be run from the same directory that contains the `templates/` folder.

## Docker image

The GitHub Actions workflow in `.github/workflows/docker.yml` automatically builds and pushes the image to GitHub Container Registry (`ghcr.io`) on every push to `main`.
