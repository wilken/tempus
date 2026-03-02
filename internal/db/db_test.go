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

func TestGetRecentTasks(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(User{ID: "u1", Email: "a@example.com", Name: "Alice"}))

	must(t, d.ReplaceEntriesForDay("u1", "2024-01-10", []TimeEntry{
		{UserID: "u1", Date: "2024-01-10", Task: "Old Task", Hours: 1},
	}))
	must(t, d.ReplaceEntriesForDay("u1", "2024-01-15", []TimeEntry{
		{UserID: "u1", Date: "2024-01-15", Task: "Recent Task", Hours: 1},
	}))

	// since=2024-01-13 should include only the recent entry.
	tasks, err := d.GetRecentTasks("u1", "2024-01-13")
	must(t, err)
	if len(tasks) != 1 || tasks[0] != "Recent Task" {
		t.Errorf("expected [Recent Task], got %v", tasks)
	}
}

func TestGetRecentSubtasksByTask(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(User{ID: "u1", Email: "a@example.com", Name: "Alice"}))

	must(t, d.ReplaceEntriesForDay("u1", "2024-01-15", []TimeEntry{
		{UserID: "u1", Date: "2024-01-15", Task: "Project", Subtask: "Design", Hours: 1},
		{UserID: "u1", Date: "2024-01-15", Task: "Project", Subtask: "Dev", Hours: 2},
		{UserID: "u1", Date: "2024-01-15", Task: "Other", Subtask: "Review", Hours: 1},
	}))

	m, err := d.GetRecentSubtasksByTask("u1", "2024-01-14")
	must(t, err)

	if len(m["Project"]) != 2 {
		t.Errorf("expected 2 subtasks for Project, got %v", m["Project"])
	}
	if len(m["Other"]) != 1 || m["Other"][0] != "Review" {
		t.Errorf("expected [Review] for Other, got %v", m["Other"])
	}
	// Entries with empty subtask should not appear.
	must(t, d.ReplaceEntriesForDay("u1", "2024-01-15", []TimeEntry{
		{UserID: "u1", Date: "2024-01-15", Task: "Project", Subtask: "", Hours: 3},
	}))
	m2, err := d.GetRecentSubtasksByTask("u1", "2024-01-14")
	must(t, err)
	if len(m2["Project"]) != 0 {
		t.Errorf("expected no subtasks for empty subtask entry, got %v", m2["Project"])
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
