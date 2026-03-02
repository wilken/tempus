# Tempus

A simple time registration app built with Go, SQLite, and plain HTML.

## Features

- Per-day view with task / subtask / hours lines
- Previous / Next day navigation
- Google sign-in (your name is pulled from your Google account)
- Save entries (replaces all entries for that day)
- Export a full week to an Excel file (.xlsx)

## Requirements

- Go 1.21+
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
go run .
```

Then open <http://localhost:8080> in your browser.

## Environment variables

| Variable              | Default                                | Required |
|-----------------------|----------------------------------------|----------|
| `GOOGLE_CLIENT_ID`    | —                                      | ✅       |
| `GOOGLE_CLIENT_SECRET`| —                                      | ✅       |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/auth/callback`  |          |
| `SESSION_SECRET`      | `dev-secret-change-in-production`      |          |
| `DATABASE_PATH`       | `tempus.db`                            |          |
| `PORT`                | `8080`                                 |          |

## Building a binary

```bash
go build -o tempus .
./tempus
```

> **Note:** The binary must be run from the same directory that contains the `templates/` folder.
