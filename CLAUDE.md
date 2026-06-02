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
- **Auto-save:** The day page saves automatically via `fetch` (no page reload) when the user tabs out of the hours field or deletes a row. A manual Save button is also present but disabled until the form is dirty and has at least one complete row.
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
| `templates/`                      | HTML templates (login.html, day.html, week.html, range.html, header.html) |
| `templates/header.html`           | `{{define "header"}}` partial — nav dropdown + delete modal  |
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

## Range view (`/range/{start}/{end}`)

Generic read-only view for any inclusive date range. Renders the same table (Task | Subtask | Hours) with daily totals and a grand total. Day headings link back to the day edit page. Previous/Next navigation shifts by the range duration, except when the range is a whole calendar month — in that case it navigates by calendar month. Export link downloads an Excel file for the range.

- `/week/{date}` — redirects to the Mon–Sun range containing the date
- `/month/{date}` — redirects to the first–last day of the calendar month containing the date

## Range export (`/export/range?start=…&end=…`)

Columns: Date | Task | Subtask | Name | Hours. Grouped by day with a "Total" row at the bottom. File name: `{username}-{start}-{end}.xlsx` (spaces and unsafe characters in username replaced with `_`). `/export/week?date=…` redirects to this endpoint.

## Security

- `SESSION_SECRET` is required (no default) — generate with `openssl rand -base64 32`.
- Session cookie is `Secure` (HTTPS only) and `HttpOnly`. Local HTTP dev requires unsetting `Secure` or using a TLS proxy.
- `securityHeaders` middleware in `cmd/main.go` sets HSTS, CSP, X-Frame-Options, X-Content-Type-Options, and Referrer-Policy on every response.
- `json.Marshal` is used deliberately in the Day handler for autocomplete data — it escapes `<`, `>`, `&` as unicode escapes, which is required for safe injection into `<script>` blocks via `template.JS`. Do not replace it with an encoder that skips this escaping.
- `maxRangeDays = 366` caps date ranges in `DateRange` and `ExportRange` to prevent oversized allocations and runaway Excel generation.
- Usernames in Excel filenames are sanitised with `strings.Map` to strip characters that could inject into `Content-Disposition` headers (`"`, `\`, `\r`, `\n`, `/`, `:`).

## Docker

- Uses a named volume `storage` mounted at `/storage` for the SQLite database.
- The app is exposed on host port 80 (mapped to container port 8080).
- The container runs as a non-root `tempus` user — files copied into `/storage` via `docker compose cp` will be owned by root and cause a "readonly database" error on startup. Always copy all three SQLite files (`.db`, `.db-wal`, `.db-shm`) together to keep WAL state consistent.
- To migrate an existing database, copy all three SQLite files together (`.db`, `.db-wal`, `.db-shm`) to keep WAL state consistent, then fix ownership:
  ```bash
  docker compose cp tempus.db app:/storage/tempus.db
  docker compose cp tempus.db-wal app:/storage/tempus.db-wal
  docker compose cp tempus.db-shm app:/storage/tempus.db-shm
  docker run --rm -v tempus_storage:/storage alpine chown -R 100:101 /storage
  docker compose restart app
  ```
- To delete files inside the volume while the container is running: `docker compose exec app sh -c "rm -f /storage/tempus.db*"`

## Header / user menu

- The header is defined once in `templates/header.html` as `{{define "header"}}` and included in `day.html`, `week.html`, and `range.html` via `{{template "header" .}}`.
- The username is a dropdown toggle (`▾`) revealing "Sign out" and "Delete account…".
- "Delete account…" opens a confirmation modal (POST `/account/delete`) that permanently deletes all time entries and the user row, then expires the session.
- `db.DeleteUser` deletes entries before the user row to satisfy the FK constraint (no CASCADE set).

## Day page UX

- Tab out of the hours field auto-saves and adds a new row (last row only).
- Deleting a row auto-saves immediately.
- Task and subtask fields show autocomplete suggestions from the 10 days prior to the viewed date, plus anything entered earlier in the current session.
- Subtask suggestions are filtered to match the task in the same row.
- A toast notification confirms saves and shows errors if the connection is lost.
- "View week" link navigates to `/week/{date}`; "View month" navigates to `/month/{date}`.
