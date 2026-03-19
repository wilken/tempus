package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps the sql.DB connection.
type DB struct {
	conn *sql.DB
}

// User represents a logged-in Google user.
type User struct {
	ID    string
	Email string
	Name  string
}

// TimeEntry is a single task/subtask/hours line for a given day.
type TimeEntry struct {
	ID      int64
	UserID  string
	Date    string
	Task    string
	Subtask string
	Hours   float64
}

func InitDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id         TEXT PRIMARY KEY,
		email      TEXT UNIQUE NOT NULL,
		name       TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS time_entries (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    TEXT    NOT NULL,
		date       TEXT    NOT NULL,
		task       TEXT    NOT NULL,
		subtask    TEXT    NOT NULL DEFAULT '',
		hours      REAL    NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) UpsertUser(u User) error {
	_, err := db.conn.Exec(`
		INSERT INTO users (id, email, name) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, email=excluded.email`,
		u.ID, u.Email, u.Name)
	return err
}

func (db *DB) DeleteUser(userID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM time_entries WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) GetEntriesForDay(userID, date string) ([]TimeEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, user_id, date, task, subtask, hours
		FROM time_entries
		WHERE user_id = ? AND date = ?
		ORDER BY id`, userID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (db *DB) GetEntriesForWeek(userID, startDate, endDate string) ([]TimeEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, user_id, date, task, subtask, hours
		FROM time_entries
		WHERE user_id = ? AND date >= ? AND date <= ?
		ORDER BY date, id`, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// ReplaceEntriesForDay deletes all entries for the day then inserts the new set.
func (db *DB) ReplaceEntriesForDay(userID, date string, entries []TimeEntry) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM time_entries WHERE user_id = ? AND date = ?`, userID, date,
	); err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := tx.Exec(`
			INSERT INTO time_entries (user_id, date, task, subtask, hours)
			VALUES (?, ?, ?, ?, ?)`,
			userID, date, e.Task, e.Subtask, e.Hours,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetRecentTasks returns distinct task names used by the user on or after since.
func (db *DB) GetRecentTasks(userID, since string) ([]string, error) {
	rows, err := db.conn.Query(`
		SELECT task FROM time_entries
		WHERE user_id = ? AND date >= ? AND task != ''
		GROUP BY task
		ORDER BY MAX(date) DESC, MAX(created_at) DESC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStrings(rows)
}

// GetRecentSubtasksByTask returns distinct subtasks grouped by task used on or after since.
func (db *DB) GetRecentSubtasksByTask(userID, since string) (map[string][]string, error) {
	rows, err := db.conn.Query(`
		SELECT task, subtask FROM time_entries
		WHERE user_id = ? AND date >= ? AND subtask != ''
		GROUP BY task, subtask
		ORDER BY task, MAX(date) DESC, MAX(created_at) DESC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string][]string{}
	for rows.Next() {
		var task, subtask string
		if err := rows.Scan(&task, &subtask); err != nil {
			return nil, err
		}
		result[task] = append(result[task], subtask)
	}
	return result, rows.Err()
}

func scanStrings(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanEntries(rows *sql.Rows) ([]TimeEntry, error) {
	var entries []TimeEntry
	for rows.Next() {
		var e TimeEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Date, &e.Task, &e.Subtask, &e.Hours); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
