package handlers

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"tempus/internal/auth"
	"tempus/internal/db"
)

// testTmpl covers both page templates with just enough output to assert on.
var testTmpl = func() *template.Template {
	t := template.Must(template.New("day.html").Parse(
		`<html><body>{{.Date}}</body></html>`,
	))
	template.Must(t.New("week.html").Parse(
		`<html><body>{{.WeekLabel}}</body></html>`,
	))
	return t
}()

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.InitDB(":memory:")
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

// withAuth injects auth.CtxUserID and auth.CtxUserName into r's context,
// bypassing the auth middleware entirely.
func withAuth(r *http.Request, userID, userName string) *http.Request {
	ctx := context.WithValue(r.Context(), auth.CtxUserID, userID)
	ctx = context.WithValue(ctx, auth.CtxUserName, userName)
	return r.WithContext(ctx)
}

// withChiParam injects a chi URL parameter so handlers can call chi.URLParam.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestIndexRedirect(t *testing.T) {
	h := &Handler{DB: newTestDB(t), Tmpl: testTmpl}
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.Index(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", rec.Code)
	}
	today := time.Now().Format("2006-01-02")
	if loc := rec.Header().Get("Location"); loc != "/day/"+today {
		t.Errorf("expected /day/%s, got %q", today, loc)
	}
}

func TestDayBadDate(t *testing.T) {
	h := &Handler{DB: newTestDB(t), Tmpl: testTmpl}
	req := httptest.NewRequest("GET", "/day/bad-date", nil)
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "not-a-date")
	rec := httptest.NewRecorder()
	h.Day(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDayRendersOK(t *testing.T) {
	h := &Handler{DB: newTestDB(t), Tmpl: testTmpl}
	req := httptest.NewRequest("GET", "/day/2024-01-15", nil)
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "2024-01-15")
	rec := httptest.NewRecorder()
	h.Day(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "2024-01-15") {
		t.Errorf("expected date in body, got: %q", rec.Body.String())
	}
}

func TestSaveDayStoresEntries(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(db.User{ID: "u1", Email: "a@example.com", Name: "Alice"}))
	h := &Handler{DB: d, Tmpl: testTmpl}

	form := url.Values{
		"task":    {"Task A"},
		"subtask": {"Sub A"},
		"hours":   {"2.5"},
	}
	req := httptest.NewRequest("POST", "/day/2024-01-15/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "2024-01-15")
	rec := httptest.NewRecorder()
	h.SaveDay(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	entries, err := d.GetEntriesForDay("u1", "2024-01-15")
	must(t, err)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Task != "Task A" {
		t.Errorf("expected task Task A, got %q", entries[0].Task)
	}
	if entries[0].Hours != 2.5 {
		t.Errorf("expected 2.5 hours, got %v", entries[0].Hours)
	}
}

func TestSaveDaySkipsEmptyAndZeroHours(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(db.User{ID: "u1", Email: "a@example.com", Name: "Alice"}))
	h := &Handler{DB: d, Tmpl: testTmpl}

	// Two rows: one with empty task, one with a task but zero hours — both should be filtered.
	form := url.Values{
		"task":    {"", "Valid Task"},
		"subtask": {"", ""},
		"hours":   {"3", "0"},
	}
	req := httptest.NewRequest("POST", "/day/2024-01-15/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "2024-01-15")
	rec := httptest.NewRecorder()
	h.SaveDay(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rec.Code)
	}
	entries, err := d.GetEntriesForDay("u1", "2024-01-15")
	must(t, err)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (empty task and zero hours both filtered), got %d", len(entries))
	}
}

func TestWeekBadDate(t *testing.T) {
	h := &Handler{DB: newTestDB(t), Tmpl: testTmpl}
	req := httptest.NewRequest("GET", "/week/bad-date", nil)
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "not-a-date")
	rec := httptest.NewRecorder()
	h.Week(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestWeekRendersOK(t *testing.T) {
	d := newTestDB(t)
	must(t, d.UpsertUser(db.User{ID: "u1", Email: "a@example.com", Name: "Alice"}))
	must(t, d.ReplaceEntriesForDay("u1", "2024-01-15", []db.TimeEntry{
		{UserID: "u1", Date: "2024-01-15", Task: "Task A", Hours: 2},
	}))
	h := &Handler{DB: d, Tmpl: testTmpl}

	req := httptest.NewRequest("GET", "/week/2024-01-15", nil)
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "2024-01-15")
	rec := httptest.NewRecorder()
	h.Week(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	// 2024-01-15 is a Monday; the week label should contain "Jan 15".
	if !strings.Contains(rec.Body.String(), "Jan 15") {
		t.Errorf("expected week label in body, got: %q", rec.Body.String())
	}
}

func TestExportWeekContentType(t *testing.T) {
	h := &Handler{DB: newTestDB(t), Tmpl: testTmpl}
	req := httptest.NewRequest("GET", "/export/week?date=2024-01-15", nil)
	req = withAuth(req, "u1", "Alice")
	rec := httptest.NewRecorder()
	h.ExportWeek(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	const xlsxMime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct := rec.Header().Get("Content-Type"); ct != xlsxMime {
		t.Errorf("unexpected content type: %q", ct)
	}
}

func TestBuildExcel(t *testing.T) {
	monday, _ := time.Parse("2006-01-02", "2024-01-15")
	sunday := monday.AddDate(0, 0, 6)
	entries := []db.TimeEntry{
		{UserID: "u1", Date: "2024-01-15", Task: "Task A", Hours: 2},
	}

	f, filename := buildExcel(entries, monday, sunday, "Alice Smith")

	if filename != "Alice_Smith-week-2024-01-15.xlsx" {
		t.Errorf("unexpected filename: %q", filename)
	}

	sheets := f.GetSheetList()
	found := false
	for _, s := range sheets {
		if s == "Week" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected sheet named 'Week', got %v", sheets)
	}

	// Name column (D) should contain the username on data rows.
	name, err := f.GetCellValue("Week", "D5")
	if err != nil {
		t.Fatalf("reading D5: %v", err)
	}
	if name != "Alice Smith" {
		t.Errorf("expected Name column to be 'Alice Smith', got %q", name)
	}

	// Date column (A) should use M/D/YYYY format.
	date, err := f.GetCellValue("Week", "A5")
	if err != nil {
		t.Fatalf("reading A5: %v", err)
	}
	if date != "1/15/2024" {
		t.Errorf("expected date '1/15/2024', got %q", date)
	}
}

// TestDayEmptyAutocompleteNotNull ensures that when a user has no recent time
// entries, the autocomplete JS variables are serialised as [] and {} rather
// than null, which would cause a runtime error when forEach is called on them.
func TestDayEmptyAutocompleteNotNull(t *testing.T) {
	jsTmpl := template.Must(template.New("day.html").Parse(
		`{{.TaskSuggestionsJS}}|{{.SubtasksByTask}}`,
	))
	h := &Handler{DB: newTestDB(t), Tmpl: jsTmpl}
	req := httptest.NewRequest("GET", "/day/2024-01-15", nil)
	req = withAuth(req, "u1", "Alice")
	req = withChiParam(req, "date", "2024-01-15")
	rec := httptest.NewRecorder()
	h.Day(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "null") {
		t.Errorf("autocomplete data contains null — forEach will throw; got: %q", body)
	}
	if !strings.Contains(body, "[]") {
		t.Errorf("expected task suggestions to be [], got: %q", body)
	}
	if !strings.Contains(body, "{}") {
		t.Errorf("expected subtasks map to be {}, got: %q", body)
	}
}

func TestMondayOf(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"2024-01-15", "2024-01-15"}, // already Monday
		{"2024-01-16", "2024-01-15"}, // Tuesday
		{"2024-01-21", "2024-01-15"}, // Sunday
		{"2024-01-14", "2024-01-08"}, // Sunday of previous week
	}
	for _, c := range cases {
		d, _ := time.Parse("2006-01-02", c.input)
		got := mondayOf(d).Format("2006-01-02")
		if got != c.want {
			t.Errorf("mondayOf(%s) = %s, want %s", c.input, got, c.want)
		}
	}
}
