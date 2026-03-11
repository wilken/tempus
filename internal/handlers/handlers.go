package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"tempus/internal/auth"
	"tempus/internal/db"
)

// Handler holds handler dependencies and exposes HTTP handlers.
type Handler struct {
	DB   *db.DB
	Tmpl *template.Template
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/day/"+time.Now().Format("2006-01-02"), http.StatusFound)
}

// DayPageData is the view model for the day page.
type DayPageData struct {
	Date            string
	DateFormatted   string
	PrevDate        string
	NextDate        string
	UserName        string
	Entries         []db.TimeEntry
	TotalHours      float64
	TaskSuggestions []string
	SubtasksByTask  template.JS
}

func (h *Handler) Day(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(auth.CtxUserID).(string)
	userName, _ := r.Context().Value(auth.CtxUserName).(string)

	entries, err := h.DB.GetEntriesForDay(userID, dateStr)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	var total float64
	for _, e := range entries {
		total += e.Hours
	}

	since := date.AddDate(0, 0, -10).Format("2006-01-02")
	tasks, _ := h.DB.GetRecentTasks(userID, since)
	subtaskMap, _ := h.DB.GetRecentSubtasksByTask(userID, since)
	subtaskJSON, _ := json.Marshal(subtaskMap)

	h.renderTemplate(w, "day.html", DayPageData{
		Date:            dateStr,
		DateFormatted:   date.Format("Monday, January 2, 2006"),
		PrevDate:        date.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:        date.AddDate(0, 0, 1).Format("2006-01-02"),
		UserName:        userName,
		Entries:         entries,
		TotalHours:      total,
		TaskSuggestions: tasks,
		SubtasksByTask:  template.JS(subtaskJSON),
	})
}

func (h *Handler) SaveDay(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(auth.CtxUserID).(string)
	tasks := r.Form["task"]
	subtasks := r.Form["subtask"]
	hoursStrs := r.Form["hours"]

	var entries []db.TimeEntry
	for i, task := range tasks {
		task = strings.TrimSpace(task)
		if task == "" {
			continue
		}
		subtask := ""
		if i < len(subtasks) {
			subtask = strings.TrimSpace(subtasks[i])
		}
		hours := 0.0
		if i < len(hoursStrs) {
			hours, _ = strconv.ParseFloat(hoursStrs[i], 64)
		}
		if hours <= 0 {
			continue
		}
		entries = append(entries, db.TimeEntry{
			UserID:  userID,
			Date:    dateStr,
			Task:    task,
			Subtask: subtask,
			Hours:   hours,
		})
	}

	if err := h.DB.ReplaceEntriesForDay(userID, dateStr, entries); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/day/"+dateStr, http.StatusSeeOther)
}

// DayGroup holds entries for a single day, used by the week view.
type DayGroup struct {
	Date          string
	DateFormatted string
	Entries       []db.TimeEntry
	Total         float64
}

// WeekPageData is the view model for the week view page.
type WeekPageData struct {
	WeekLabel  string
	Monday     string
	PrevMonday string
	NextMonday string
	Today      string
	Days       []DayGroup
	WeekTotal  float64
	UserName   string
}

func (h *Handler) Week(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	monday := mondayOf(date)
	sunday := monday.AddDate(0, 0, 6)

	userID := r.Context().Value(auth.CtxUserID).(string)
	userName, _ := r.Context().Value(auth.CtxUserName).(string)

	entries, err := h.DB.GetEntriesForWeek(userID, monday.Format("2006-01-02"), sunday.Format("2006-01-02"))
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Build a DayGroup for every day Mon–Sun, then fill in entries.
	dayMap := make(map[string]*DayGroup, 7)
	days := make([]DayGroup, 7)
	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i)
		ds := d.Format("2006-01-02")
		days[i] = DayGroup{Date: ds, DateFormatted: d.Format("Monday, January 2")}
		dayMap[ds] = &days[i]
	}

	var weekTotal float64
	for _, e := range entries {
		if dg, ok := dayMap[e.Date]; ok {
			dg.Entries = append(dg.Entries, e)
			dg.Total += e.Hours
			weekTotal += e.Hours
		}
	}

	h.renderTemplate(w, "week.html", WeekPageData{
		WeekLabel:  fmt.Sprintf("%s – %s", monday.Format("Jan 2"), sunday.Format("Jan 2, 2006")),
		Monday:     monday.Format("2006-01-02"),
		PrevMonday: monday.AddDate(0, 0, -7).Format("2006-01-02"),
		NextMonday: monday.AddDate(0, 0, 7).Format("2006-01-02"),
		Today:      time.Now().Format("2006-01-02"),
		Days:       days,
		WeekTotal:  weekTotal,
		UserName:   userName,
	})
}

func (h *Handler) ExportWeek(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	monday := mondayOf(date)
	sunday := monday.AddDate(0, 0, 6)

	userID := r.Context().Value(auth.CtxUserID).(string)
	userName, _ := r.Context().Value(auth.CtxUserName).(string)

	entries, err := h.DB.GetEntriesForWeek(
		userID,
		monday.Format("2006-01-02"),
		sunday.Format("2006-01-02"),
	)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	f, filename := buildExcel(entries, monday, sunday, userName)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if err := f.Write(w); err != nil {
		http.Error(w, "failed to write excel file", http.StatusInternalServerError)
	}
}

// mondayOf returns the Monday of the week containing date.
func mondayOf(date time.Time) time.Time {
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return date.AddDate(0, 0, -(weekday - 1))
}

func buildExcel(entries []db.TimeEntry, monday, sunday time.Time, userName string) (*excelize.File, string) {
	f := excelize.NewFile()
	sheet := "Week"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	// Meta header
	f.SetCellValue(sheet, "A1", "Week")
	f.SetCellValue(sheet, "B1", fmt.Sprintf("%s – %s", monday.Format("Jan 2"), sunday.Format("Jan 2, 2006")))
	f.SetCellValue(sheet, "A2", "Name")
	f.SetCellValue(sheet, "B2", userName)
	f.SetCellStyle(sheet, "A1", "A2", bold)

	// Column headers
	f.SetCellValue(sheet, "A4", "Date")
	f.SetCellValue(sheet, "B4", "Task")
	f.SetCellValue(sheet, "C4", "Subtask")
	f.SetCellValue(sheet, "D4", "Name")
	f.SetCellValue(sheet, "E4", "Hours")
	f.SetCellStyle(sheet, "A4", "E4", bold)

	row := 5
	var weekTotal float64
	prevDate := ""
	var dateFormatted string

	for _, e := range entries {
		if e.Date != prevDate {
			prevDate = e.Date
			d, _ := time.Parse("2006-01-02", e.Date)
			dateFormatted = d.Format("1/2/2006")
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), dateFormatted)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), e.Task)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), e.Subtask)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), userName)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), e.Hours)
		weekTotal += e.Hours
		row++
	}

	// Week total
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "Week total")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", row), weekTotal)
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), bold)

	// Column widths
	f.SetColWidth(sheet, "A", "A", 14)
	f.SetColWidth(sheet, "B", "B", 32)
	f.SetColWidth(sheet, "C", "C", 22)
	f.SetColWidth(sheet, "D", "D", 20)
	f.SetColWidth(sheet, "E", "E", 8)

	safeName := strings.ReplaceAll(userName, " ", "_")
	filename := fmt.Sprintf("%s-week-%s.xlsx", safeName, monday.Format("2006-01-02"))
	return f, filename
}
