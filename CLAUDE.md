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
- **Auto-save:** The day page saves automatically via `fetch` (no page reload) when the user tabs out of the hours field or deletes a row. There is no manual save button.
- **No CGo:** `modernc.org/sqlite` was chosen specifically to avoid CGo dependencies, making cross-compilation easy.
- **Standard Go layout:** `cmd/` for the entry point; `internal/` sub-packages for domain logic. Dependencies injected via struct fields — no global variables.
- **Context keys:** `auth.ContextKey` exported string type; constants `auth.CtxUserID` / `auth.CtxUserName` set by middleware and read by handlers without circular imports.

## Key files

| File                              | Purpose                                                  |
|-----------------------------------|----------------------------------------------------------|
| `cmd/main.go`                     | Server setup, routes, config from env vars               |
| `internal/db/db.go`               | SQLite schema, DB/User/TimeEntry types, CRUD             |
| `internal/auth/auth.go`           | Google OAuth flow, session middleware, context keys      |
| `internal/handlers/handlers.go`   | HTTP handlers + Excel builder                            |
| `templates/`                      | HTML templates (login.html, day.html, week.html)         |
| `internal/db/db_test.go`          | DB layer tests (in-memory SQLite)                        |
| `internal/auth/auth_test.go`      | Auth middleware and login handler tests                  |
| `internal/handlers/handlers_test.go` | Handler tests (httptest + chi + in-memory DB)         |

## Running

```bash
source .env   # exports GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, SESSION_SECRET (use export VAR=value in .env)
go run ./cmd
```

## Testing

```bash
go test ./...
```

Uses in-memory SQLite — no external dependencies required.

## Schema

```sql
users(id TEXT PK, email TEXT, name TEXT, created_at)
time_entries(id INT PK, user_id TEXT, date TEXT, task TEXT, subtask TEXT, hours REAL, created_at)
```

Date is stored as `YYYY-MM-DD` text.

## Week view (`/week/{date}`)

Read-only single table with columns Task | Subtask | Hours, with a blue "Daily total" row after each day and a "Week total" at the bottom. Any date in the week resolves to Monday. Day headings (links back to the day edit page) are shown as a row spanning all columns above each day's entries. Previous/Next week navigation at top; "Go to today" and "Export to Excel" links at bottom.

## Week export

Monday–Sunday. Columns: Date | Task | Subtask | Name | Hours. Entries are grouped by day with daily totals and a week total row at the bottom. File name: `{username}-week-YYYY-MM-DD.xlsx` (Monday date; spaces in username replaced with `_`).

## Day page UX

- Tab out of the hours field auto-saves and adds a new row (last row only).
- Deleting a row auto-saves immediately.
- Task and subtask fields show autocomplete suggestions from the 5 days prior to the viewed date, plus anything entered earlier in the current session.
- Subtask suggestions are filtered to match the task in the same row.
- A toast notification confirms saves and shows errors if the connection is lost.
- "View week" link navigates to `/week/{date}`.
