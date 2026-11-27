package web

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/befund"
)

// openAssessment ist eine Messung, deren Nachkontrolle fällig ist.
type openAssessment struct {
	PatientID int64
	Patient   string
	Test      string
	Date      string
	Days      int
}

// dashboard ist die Startseite: Tagesplan, zuletzt gesehene Patienten, fällige
// Nachkontrollen, zuletzt geänderte Wissenseinträge, Suche und Schnellzugriffe.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// Alte Links auf die Wissensliste weiterleiten (die Startseite war früher die Liste).
	if q.Get("q") != "" || q.Get("art") != "" || q.Get("region") != "" {
		redirect(w, r, "/wissen?%s", q.Encode())
		return
	}
	ctx := r.Context()
	now := time.Now()
	cal, err := s.buildCalendar(ctx, r, calParams{View: "tag", Day: startOfDay(now)})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recent, err := s.st.RecentSessions(ctx, startOfDay(now).AddDate(0, 0, -7), 8)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	open, err := s.openAssessments(ctx, now)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	entries, err := s.st.RecentlyUpdatedEntries(ctx, 6)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "dashboard", http.StatusOK, map[string]any{
		"Cal": cal, "Recent": recent, "Open": open, "Entries": entries, "Today": now,
	})
}

// openAssessments findet Messungen, deren letzte Durchführung mindestens 14 Tage
// zurückliegt und für die kein künftiger Termin eingetragen ist. Reine Auswertung
// über die jüngsten Daten, damit die Startseite nicht die ganze Datenbank liest.
func (s *Server) openAssessments(ctx context.Context, now time.Time) ([]openAssessment, error) {
	const followUpDays = 14
	since := now.AddDate(0, 0, -120)
	runs, err := s.st.RecentTestRuns(ctx, since, 500)
	if err != nil {
		return nil, err
	}
	bs, err := s.st.BefundeSince(ctx, since, 500)
	if err != nil {
		return nil, err
	}
	qes, err := s.questionnaireEntries(ctx)
	if err != nil {
		return nil, err
	}
	titles := make(map[int64]string, len(qes))
	for _, q := range qes {
		titles[q.ID] = q.Title
	}
	type key struct {
		pid  int64
		test string
	}
	latest := map[key]openAssessment{}
	consider := func(pid int64, patient, test, date string) {
		if date == "" || test == "" {
			return
		}
		k := key{pid, test}
		if cur, ok := latest[k]; !ok || date > cur.Date {
			latest[k] = openAssessment{PatientID: pid, Patient: patient, Test: test, Date: date}
		}
	}
	for _, r := range runs {
		consider(r.PatientID, r.Patient, r.Title, r.Date)
	}
	for _, b := range bs {
		d, err := befund.Parse(b.Data)
		if err != nil || len(d.Q) == 0 {
			continue
		}
		for _, idStr := range d.QList() {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			consider(b.PatientID, b.Patient, titles[id], b.Date)
		}
	}
	upcoming, err := s.st.UpcomingPatientIDs(ctx, now, now.AddDate(0, 0, 30))
	if err != nil {
		return nil, err
	}
	cutoff := now.AddDate(0, 0, -followUpDays).Format("2006-01-02")
	var out []openAssessment
	for _, a := range latest {
		if a.Date > cutoff || upcoming[a.PatientID] {
			continue
		}
		if t, err := time.ParseInLocation("2006-01-02", a.Date, time.Local); err == nil {
			a.Days = int(now.Sub(t).Hours() / 24)
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Patient < out[j].Patient
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out, nil
}

// toggleSidebar klappt die Navigation ein oder aus (Cookie, pro Gerät gemerkt).
// Ohne JavaScript funktioniert es über dieses Formular.
func (s *Server) toggleSidebar(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	state := "1"
	if c, err := r.Cookie(sidebarCookie); err == nil && c.Value == "1" {
		state = "0"
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: reines UI-Cookie, kein Geheimnis
		Name: sidebarCookie, Value: state, Path: "/", MaxAge: 365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode, HttpOnly: true,
	})
	back := r.FormValue("zurueck")
	if back == "" || back[0] != '/' || len(back) > 512 {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

const sidebarCookie = "physiobase_sidebar"

// sidebarCollapsed liest den Zustand der Navigation aus dem Cookie.
func sidebarCollapsed(r *http.Request) bool {
	c, err := r.Cookie(sidebarCookie)
	return err == nil && c.Value == "1"
}

// areaFor ordnet eine Anfrage einem Navigationsbereich zu (für die Hervorhebung).
func areaFor(r *http.Request) string {
	path := r.URL.Path
	switch {
	case path == "/":
		return "dashboard"
	case path == "/wissen" && r.URL.Query().Get("art") == "test":
		return "tests"
	case path == "/wissen" || strings.HasPrefix(path, "/eintrag"):
		return "wissen"
	case path == "/patienten" || strings.HasPrefix(path, "/patient/"):
		return "patienten"
	case path == "/kalender" || path == "/termine" || strings.HasPrefix(path, "/termin/"):
		return "termine"
	case path == "/handouts" || strings.HasPrefix(path, "/handout/"):
		return "handouts"
	case path == "/konto":
		return "konto"
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return "admin"
	}
	return ""
}
