package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
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
	TaskSuggestions   []string
	TaskSuggestionsJS template.JS
	SubtasksByTask    template.JS
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
		log.Printf("GetEntriesForDay error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	var total float64
	for _, e := range entries {
		total += e.Hours
	}

	tasks, _ := h.DB.GetRecentTasks(userID, date)
	if tasks == nil {
		tasks = []string{}
	}
	taskJSON, _ := json.Marshal(tasks)
	subtaskMap, _ := h.DB.GetRecentSubtasksByTask(userID, date)
	if subtaskMap == nil {
		subtaskMap = map[string][]string{}
	}
	subtaskJSON, _ := json.Marshal(subtaskMap)

	h.renderTemplate(w, "day.html", DayPageData{
		Date:              dateStr,
		DateFormatted:     date.Format("Monday, January 2, 2006"),
		PrevDate:          date.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:          date.AddDate(0, 0, 1).Format("2006-01-02"),
		UserName:          userName,
		Entries:           entries,
		TotalHours:        total,
		TaskSuggestions:   tasks,
		TaskSuggestionsJS: template.JS(taskJSON),
		SubtasksByTask:    template.JS(subtaskJSON),
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
		log.Printf("ReplaceEntriesForDay error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/day/"+dateStr, http.StatusSeeOther)
}

// DayGroup holds entries for a single day, used by the range view.
type DayGroup struct {
	Date          string
	DateFormatted string
	Entries       []db.TimeEntry
	Total         float64
}

// RangePageData is the view model for the date-range view page.
type RangePageData struct {
	RangeLabel string
	Start      string
	End        string
	PrevStart  string
	PrevEnd    string
	NextStart  string
	NextEnd    string
	Days       []DayGroup
	RangeTotal float64
	UserName   string
}

// Week redirects to the generic range view for the Mon–Sun week containing date.
func (h *Handler) Week(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	monday := mondayOf(date)
	sunday := monday.AddDate(0, 0, 6)
	http.Redirect(w, r, fmt.Sprintf("/range/%s/%s",
		monday.Format("2006-01-02"), sunday.Format("2006-01-02")),
		http.StatusFound)
}

// Month redirects to the generic range view for the calendar month containing date.
func (h *Handler) Month(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	first := firstOfMonth(date)
	last := lastOfMonth(date)
	http.Redirect(w, r, fmt.Sprintf("/range/%s/%s",
		first.Format("2006-01-02"), last.Format("2006-01-02")),
		http.StatusFound)
}

// DateRange renders any inclusive date range.
func (h *Handler) DateRange(w http.ResponseWriter, r *http.Request) {
	startStr := chi.URLParam(r, "start")
	endStr := chi.URLParam(r, "end")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		http.Error(w, "invalid start date", http.StatusBadRequest)
		return
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		http.Error(w, "invalid end date", http.StatusBadRequest)
		return
	}
	if end.Before(start) {
		http.Error(w, "end date must not be before start date", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(auth.CtxUserID).(string)
	userName, _ := r.Context().Value(auth.CtxUserName).(string)

	entries, err := h.DB.GetEntriesInRange(userID, startStr, endStr)
	if err != nil {
		log.Printf("GetEntriesInRange error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	numDays := int(end.Sub(start).Hours()/24) + 1
	dayMap := make(map[string]*DayGroup, numDays)
	days := make([]DayGroup, numDays)
	for i := 0; i < numDays; i++ {
		d := start.AddDate(0, 0, i)
		ds := d.Format("2006-01-02")
		days[i] = DayGroup{Date: ds, DateFormatted: d.Format("Monday, January 2")}
		dayMap[ds] = &days[i]
	}

	var rangeTotal float64
	for _, e := range entries {
		if dg, ok := dayMap[e.Date]; ok {
			dg.Entries = append(dg.Entries, e)
			dg.Total += e.Hours
			rangeTotal += e.Hours
		}
	}

	var prevStart, prevEnd, nextStart, nextEnd time.Time
	if isWholeMonth(start, end) {
		prevStart = firstOfMonth(start.AddDate(0, -1, 0))
		prevEnd = lastOfMonth(start.AddDate(0, -1, 0))
		nextStart = firstOfMonth(start.AddDate(0, 1, 0))
		nextEnd = lastOfMonth(start.AddDate(0, 1, 0))
	} else {
		prevStart = start.AddDate(0, 0, -numDays)
		prevEnd = end.AddDate(0, 0, -numDays)
		nextStart = start.AddDate(0, 0, numDays)
		nextEnd = end.AddDate(0, 0, numDays)
	}

	h.renderTemplate(w, "range.html", RangePageData{
		RangeLabel: rangeLabel(start, end),
		Start:      startStr,
		End:        endStr,
		PrevStart:  prevStart.Format("2006-01-02"),
		PrevEnd:    prevEnd.Format("2006-01-02"),
		NextStart:  nextStart.Format("2006-01-02"),
		NextEnd:    nextEnd.Format("2006-01-02"),
		Days:       days,
		RangeTotal: rangeTotal,
		UserName:   userName,
	})
}

// ExportWeek redirects to the generic range export for the week containing date.
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
	http.Redirect(w, r, fmt.Sprintf("/export/range?start=%s&end=%s",
		monday.Format("2006-01-02"), sunday.Format("2006-01-02")),
		http.StatusFound)
}

// ExportRange streams an Excel file for any inclusive date range.
func (h *Handler) ExportRange(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		http.Error(w, "invalid start date", http.StatusBadRequest)
		return
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		http.Error(w, "invalid end date", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value(auth.CtxUserID).(string)
	userName, _ := r.Context().Value(auth.CtxUserName).(string)

	entries, err := h.DB.GetEntriesInRange(userID, startStr, endStr)
	if err != nil {
		log.Printf("GetEntriesInRange (export) error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	f, filename := buildExcel(entries, start, end, userName)
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

func isWholeMonth(start, end time.Time) bool {
	return start == firstOfMonth(start) && end == lastOfMonth(start)
}

func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func lastOfMonth(t time.Time) time.Time {
	return firstOfMonth(t).AddDate(0, 1, -1)
}

func rangeLabel(start, end time.Time) string {
	if start.Year() == end.Year() {
		return fmt.Sprintf("%s – %s", start.Format("Jan 2"), end.Format("Jan 2, 2006"))
	}
	return fmt.Sprintf("%s – %s", start.Format("Jan 2, 2006"), end.Format("Jan 2, 2006"))
}

func buildExcel(entries []db.TimeEntry, start, end time.Time, userName string) (*excelize.File, string) {
	f := excelize.NewFile()
	sheet := "Week"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	// Meta header
	f.SetCellValue(sheet, "A1", "Period")
	f.SetCellValue(sheet, "B1", rangeLabel(start, end))
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
	var total float64
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
		total += e.Hours
		row++
	}

	// Total row
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "Total")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", row), total)
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), bold)

	// Column widths
	f.SetColWidth(sheet, "A", "A", 14)
	f.SetColWidth(sheet, "B", "B", 32)
	f.SetColWidth(sheet, "C", "C", 22)
	f.SetColWidth(sheet, "D", "D", 20)
	f.SetColWidth(sheet, "E", "E", 8)

	safeName := strings.ReplaceAll(userName, " ", "_")
	filename := fmt.Sprintf("%s-%s-%s.xlsx", safeName, start.Format("2006-01-02"), end.Format("2006-01-02"))
	return f, filename
}
