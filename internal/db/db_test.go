package db

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("newTestDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertUser(t *testing.T) {
	d := newTestDB(t)
	u := User{ID: "u1", Email: "a@example.com", Name: "Alice"}
	must(t, d.UpsertUser(u))

	// Upsert again with a different name — should not fail.
	u.Name = "Alice Smith"
	must(t, d.UpsertUser(u))
}

func TestGetEntriesForDay_Empty(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(User{ID: "u1", Email: "a@example.com", Name: "Alice"}))

	entries, err := d.GetEntriesForDay("u1", "2024-01-15")
	must(t, err)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestReplaceEntriesForDay(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(User{ID: "u1", Email: "a@example.com", Name: "Alice"}))

	day := "2024-01-15"
	first := []TimeEntry{
		{UserID: "u1", Date: day, Task: "Task A", Hours: 2},
		{UserID: "u1", Date: day, Task: "Task B", Hours: 3},
	}
	must(t, d.ReplaceEntriesForDay("u1", day, first))

	entries, err := d.GetEntriesForDay("u1", day)
	must(t, err)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Replace with just one entry — old rows must be gone.
	second := []TimeEntry{
		{UserID: "u1", Date: day, Task: "Task C", Hours: 4},
	}
	must(t, d.ReplaceEntriesForDay("u1", day, second))

	entries, err = d.GetEntriesForDay("u1", day)
	must(t, err)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after replace, got %d", len(entries))
	}
	if entries[0].Task != "Task C" {
		t.Errorf("expected Task C, got %q", entries[0].Task)
	}
}

func TestGetEntriesForWeek(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(User{ID: "u1", Email: "a@example.com", Name: "Alice"}))

	days := []string{"2024-01-14", "2024-01-15", "2024-01-20"}
	for _, day := range days {
		must(t, d.ReplaceEntriesForDay("u1", day, []TimeEntry{
			{UserID: "u1", Date: day, Task: "Work", Hours: 1},
		}))
	}

	// Range 2024-01-14 to 2024-01-15 should return only the first two.
	entries, err := d.GetEntriesForWeek("u1", "2024-01-14", "2024-01-15")
	must(t, err)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries in range, got %d", len(entries))
	}
}
