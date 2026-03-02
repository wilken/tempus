# Tempus — Project Notes

## Stack

- **Language:** Go (standard library `net/http` + `chi` router)
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGo required; driver name is `"sqlite"`)
- **Auth:** Google OAuth 2.0 / OpenID Connect via `golang.org/x/oauth2/google`; user info fetched from `https://www.googleapis.com/oauth2/v3/userinfo`
- **Sessions:** `gorilla/sessions` CookieStore; session name is `"tempus-session"`
- **Excel export:** `github.com/xuri/excelize/v2`
- **Templates:** Go `html/template`, loaded at startup from `templates/*.html` (relative path — binary must be run from project root)

## Architecture decisions

- **Save strategy:** Replace-all for a day — on POST `/day/{date}/save`, all existing entries for that user+date are deleted and the submitted rows are inserted in a single transaction. Simplest possible approach.
- **No CGo:** `modernc.org/sqlite` was chosen specifically to avoid CGo dependencies, making cross-compilation easy.
- **Flat package structure:** All Go files live in `package main` (no sub-packages) because the app is small.
- **Context keys:** Typed `contextKey` string aliases (`ctxUserID`, `ctxUserName`) are used to avoid collisions.

## Key files

| File             | Purpose                                      |
|------------------|----------------------------------------------|
| `main.go`        | Server setup, routes, config from env vars   |
| `db.go`          | SQLite schema + CRUD helpers                 |
| `auth.go`        | Google OAuth flow, session middleware        |
| `handlers.go`    | HTTP handlers + Excel builder                |
| `templates/`     | HTML templates (login.html, day.html)        |

## Running

```bash
source .env   # set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, SESSION_SECRET
go run .
```

## Schema

```sql
users(id TEXT PK, email TEXT, name TEXT, created_at)
time_entries(id INT PK, user_id TEXT, date TEXT, task TEXT, subtask TEXT, hours REAL, created_at)
```

Date is stored as `YYYY-MM-DD` text.

## Week export

Monday–Sunday. Entries are grouped by day with daily totals and a week total row at the bottom. File name: `tempus-week-YYYY-MM-DD.xlsx` (Monday date).
