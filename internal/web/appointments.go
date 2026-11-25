package web

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsc.io/qr"

	"forge.gpl-nas.de/physiobase/internal/ical"
	"forge.gpl-nas.de/physiobase/internal/store"
)

var weekdays = [...]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}

func dayLabel(t time.Time) string {
	return weekdays[t.Weekday()] + ", " + t.Format("02.01.2006")
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// parseAppt liest die Terminfelder mit Präfix ("" für das Bearbeiten-Formular,
// "t.appt.3." für Zeile 3 der Sammel-Eingabe). Der Ort gilt für alle Zeilen.
func (s *Server) parseAppt(r *http.Request, p string) (store.Appointment, error) {
	start, err := time.ParseInLocation("2006-01-02 15:04", formText(r, p+"date")+" "+formText(r, p+"time"), time.Local)
	if err != nil {
		return store.Appointment{}, errors.New("Datum oder Uhrzeit fehlt bzw. ist ungültig") //nolint:staticcheck // ST1005: Meldung wird direkt angezeigt
	}
	dur, err := strconv.Atoi(formText(r, p+"duration"))
	if err != nil || dur < 5 || dur > 480 {
		dur = s.defaultDuration()
	}
	loc := formText(r, p+"location")
	if loc == "" {
		loc = formText(r, "location")
	}
	a := store.Appointment{Start: start, Duration: dur, Title: formText(r, p+"title"), Location: loc, Note: formText(r, p+"note")}
	if a.Title == "" {
		a.Title = s.text("appt_default_type")
	}
	a.TherapistID, _ = formID(r, p+"therapist")
	return a, nil
}

func (s *Server) defaultDuration() int {
	if n, err := strconv.Atoi(s.rawText("appt_default_duration")); err == nil && n >= 5 && n <= 480 {
		return n
	}
	return 30
}

// apptFormData füllt die Sammel-Eingabe (Vorgaben aus den Einstellungen).
type apptFormData struct {
	Rows                             []struct{}
	Therapists                       []store.Therapist
	Types                            []string
	Today, Time, Type, Note, Address string
	Duration                         int
}

func (s *Server) apptForm(therapists []store.Therapist) apptFormData {
	var active []store.Therapist
	for _, t := range therapists {
		if t.Active {
			active = append(active, t)
		}
	}
	return apptFormData{
		Rows: make([]struct{}, 3), Therapists: active, Types: s.lines("appt_types"),
		Today: time.Now().Format("2006-01-02"), Time: s.rawText("appt_default_time"), Duration: s.defaultDuration(),
		Type: s.text("appt_default_type"), Note: s.text("appt_default_note"), Address: s.address(),
	}
}

// ---------- Praxisübersicht ----------

type apptDay struct {
	Label string
	Today bool
	Appts []store.Appointment
}

func (s *Server) listAppointments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	from := startOfDay(time.Now())
	if v := r.URL.Query().Get("von"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			from = t
		}
	}
	days := 14
	therapist, _ := strconv.ParseInt(r.URL.Query().Get("behandler"), 10, 64)
	as, err := s.st.AppointmentsBetween(ctx, from, from.AddDate(0, 0, days), therapist)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var grouped []apptDay
	for _, a := range as {
		label := dayLabel(a.Start)
		if len(grouped) == 0 || grouped[len(grouped)-1].Label != label {
			grouped = append(grouped, apptDay{Label: label, Today: startOfDay(a.Start).Equal(startOfDay(time.Now()))})
		}
		grouped[len(grouped)-1].Appts = append(grouped[len(grouped)-1].Appts, a)
	}
	therapists, err := s.st.Therapists(ctx, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	patients, err := s.st.Patients(ctx, "", false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "appointments", http.StatusOK, map[string]any{
		"Days": grouped, "From": from.Format("2006-01-02"), "To": from.AddDate(0, 0, days-1),
		"Prev": from.AddDate(0, 0, -days).Format("2006-01-02"), "Next": from.AddDate(0, 0, days).Format("2006-01-02"),
		"Therapists": therapists, "Therapist": therapist, "Patients": patients, "Count": len(as),
		"Today": time.Now().Format("2006-01-02"), "Address": s.address(), "Error": r.URL.Query().Get("fehler"),
		"AF": s.apptForm(therapists),
	})
}

// createAppointment legt einen Termin oder eine Serie an (z. B. 6× wöchentlich für eine Verordnung).
func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	ctx := r.Context()
	pid, ok := formID(r, "patient")
	back := func(msg string) {
		if r.FormValue("back") == "patient" && ok {
			redirect(w, r, "/patient/%d?fehler=%s#termine", pid, urlPathEscape(msg))
			return
		}
		redirect(w, r, "/termine?fehler=%s", urlPathEscape(msg))
	}
	if !ok {
		back("Bitte einen Patienten auswählen.")
		return
	}
	if _, err := s.st.Patient(ctx, pid); err != nil {
		s.fail(w, r, err)
		return
	}
	// Sammel-Eingabe: Zeilen t.appt.<i>.*, leere Zeilen (ohne Datum) werden übersprungen
	if n, err := strconv.Atoi(r.FormValue("t.appt.n")); err == nil && n > 0 {
		var items []store.Appointment
		for i := range min(n, 100) {
			p := fmt.Sprintf("t.appt.%d.", i)
			if formText(r, p+"date") == "" {
				continue
			}
			a, err := s.parseAppt(r, p)
			if err != nil {
				back(fmt.Sprintf("Zeile %d: %s", i+1, err.Error()))
				return
			}
			a.PatientID = pid
			items = append(items, a)
		}
		if len(items) == 0 {
			back("Bitte mindestens ein Datum eintragen.")
			return
		}
		for _, a := range items { // erst alles prüfen, dann speichern
			if _, err := s.st.CreateAppointment(ctx, a); err != nil {
				s.fail(w, r, err)
				return
			}
		}
		if r.FormValue("back") == "patient" {
			redirect(w, r, "/patient/%d#termine", pid)
			return
		}
		redirect(w, r, "/termine?von=%s", startOfDay(items[0].Start).Format("2006-01-02"))
		return
	}
	a, err := s.parseAppt(r, "")
	if err != nil {
		back(err.Error())
		return
	}
	a.PatientID = pid
	count, _ := strconv.Atoi(r.FormValue("repeat"))
	count = max(1, min(count, 40))
	every, _ := strconv.Atoi(r.FormValue("every"))
	every = max(1, min(every, 60))
	for i := range count {
		x := a
		x.Start = a.Start.AddDate(0, 0, i*every) // AddDate hält die Uhrzeit auch über die Zeitumstellung
		if _, err := s.st.CreateAppointment(ctx, x); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	if r.FormValue("back") == "patient" {
		redirect(w, r, "/patient/%d#termine", pid)
		return
	}
	redirect(w, r, "/termine?von=%s", startOfDay(a.Start).Format("2006-01-02"))
}

func (s *Server) showAppointment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	a, err := s.st.Appointment(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	therapists, err := s.st.Therapists(r.Context(), false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "appointment", http.StatusOK, map[string]any{
		"A": a, "Therapists": therapists, "Error": r.URL.Query().Get("fehler"),
		"AskDelete": r.URL.Query().Get("loeschen") == "1", // Löschblock gleich geöffnet
	})
}

func (s *Server) saveAppointment(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	old, err := s.st.Appointment(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	a, err := s.parseAppt(r, "")
	if err != nil {
		redirect(w, r, "/termin/%d?fehler=%s", id, urlPathEscape(err.Error()))
		return
	}
	a.ID, a.PatientID, a.Cancelled = id, old.PatientID, r.FormValue("cancelled") == "1"
	if err := s.st.UpdateAppointment(r.Context(), a); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#termine", a.PatientID)
}

// toggleCancel sagt einen Termin ab bzw. wieder zu; im Kalender erscheint er als abgesagt.
func (s *Server) toggleCancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	a, err := s.st.Appointment(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	a.Cancelled = !a.Cancelled
	if err := s.st.UpdateAppointment(r.Context(), a); err != nil {
		s.fail(w, r, err)
		return
	}
	if to := r.FormValue("back"); to == "termine" {
		redirect(w, r, "/termine?von=%s", startOfDay(a.Start).Format("2006-01-02"))
		return
	}
	redirect(w, r, "/patient/%d#termine", a.PatientID)
}

func (s *Server) deleteAppointment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pid, err := s.st.DeleteAppointment(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#termine", pid)
}

// ---------- Behandler ----------

func (s *Server) createTherapist(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	if name := formText(r, "name"); name != "" {
		if err := s.st.CreateTherapist(r.Context(), name); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	redirect(w, r, "/termine#behandler")
}

func (s *Server) updateTherapist(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	name := formText(r, "name")
	if !ok || name == "" {
		http.Error(w, "Name fehlt", http.StatusBadRequest)
		return
	}
	if err := s.st.UpdateTherapist(r.Context(), store.Therapist{ID: id, Name: name, Active: r.FormValue("active") == "1"}); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/termine#behandler")
}

// ---------- Kalender-Abo ----------

// calURL: Abo-Adresse. Ohne PHYSIOBASE_PUBLIC_URL nur im lokalen Netz erreichbar.
func (s *Server) calURL(r *http.Request, token string) (string, bool) {
	base, public := s.cfg.PublicURL, true
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base, public = scheme+"://"+r.Host, false
	}
	return base + "/kalender/" + token + ".ics", public
}

func webcal(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		return "webcal" + u[i:]
	}
	return u
}

// calendarFeed ist öffentlich (Kalender-Apps können sich nicht anmelden) und nur über den
// geheimen Token erreichbar. Er enthält bewusst keine Diagnosen und keinen Patientennamen.
func (s *Server) calendarFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := strings.TrimSuffix(r.PathValue("file"), ".ics")
	pid, err := s.st.PatientByCalToken(ctx, token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.pause(500 * time.Millisecond) // Raten unattraktiv machen
			http.NotFound(w, r)
			return
		}
		s.fail(w, r, err)
		return
	}
	as, err := s.st.PatientAppointments(ctx, pid, time.Now().AddDate(0, 0, -90))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	host := "physiobase"
	if i := strings.LastIndex(s.cfg.SMTP.From, "@"); i >= 0 {
		host = s.cfg.SMTP.From[i+1:]
	}
	cal := ical.Calendar{Name: s.practice() + " – Termine", ProdID: "-//PhysioBase//Termine//DE"}
	for _, a := range as {
		var desc []string
		if a.TherapistName != "" {
			desc = append(desc, "Behandlung bei: "+a.TherapistName)
		}
		if a.Note != "" {
			desc = append(desc, "Hinweis: "+a.Note)
		}
		if a.Cancelled {
			desc = append(desc, "Dieser Termin wurde abgesagt.")
		}
		summary := a.Title + " – " + s.practice()
		if a.Cancelled {
			summary = "Abgesagt: " + summary
		}
		loc := a.Location
		if loc == "" {
			loc = s.address()
		}
		cal.Events = append(cal.Events, ical.Event{
			UID: a.UID + "@" + host, Start: a.Start, End: a.End(), Stamp: a.Updated,
			Summary: summary, Description: strings.Join(desc, "\n"), Location: loc,
			Sequence: a.Sequence, Cancelled: a.Cancelled,
		})
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="termine.ics"`)
	w.Header().Set("Cache-Control", "private, max-age=900")
	w.Header().Set("X-Robots-Tag", "noindex")
	_, _ = w.Write(cal.Bytes())
}

// qrSVG zeichnet einen QR-Code als SVG (scharf in jeder Druckgröße).
func qrSVG(text string) ([]byte, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil, err
	}
	const quiet = 4
	n := code.Size + 2*quiet
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := range code.Size {
		for x := range code.Size {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.Bytes(), nil
}

func (s *Server) calendarQR(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	tok, err := s.st.CalToken(r.Context(), pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	u, _ := s.calURL(r, tok)
	svg, err := qrSVG(webcal(u))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(svg) //nolint:gosec // G705: SVG wird hier aus dem QR-Raster erzeugt, kein Benutzertext
}

func (s *Server) rotateCalendar(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := s.st.RotateCalToken(r.Context(), pid); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#termine", pid)
}

func (s *Server) printAppointments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := s.st.Patient(ctx, pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	as, err := s.st.PatientAppointments(ctx, pid, startOfDay(time.Now()))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tok, err := s.st.CalToken(ctx, pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	u, public := s.calURL(r, tok)
	s.render(w, r, "appointments_print", http.StatusOK, map[string]any{
		"P": p, "Appts": as, "CalURL": u, "Public": public, "Address": s.address(), "Today": time.Now(),
	})
}

// patientAppointmentData für die Patientenseite.
func (s *Server) patientAppointmentData(r *http.Request, pid int64) (map[string]any, error) {
	ctx := r.Context()
	all, err := s.st.PatientAppointments(ctx, pid, time.Time{})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var upcoming, past []store.Appointment
	for _, a := range all {
		if a.End().After(now) {
			upcoming = append(upcoming, a)
		} else {
			past = append(past, a)
		}
	}
	for i, j := 0, len(past)-1; i < j; i, j = i+1, j-1 { // vergangene: neueste zuerst
		past[i], past[j] = past[j], past[i]
	}
	therapists, err := s.st.Therapists(ctx, true)
	if err != nil {
		return nil, err
	}
	af := s.apptForm(therapists)
	tok, err := s.st.CalToken(ctx, pid)
	if err != nil {
		return nil, err
	}
	u, public := s.calURL(r, tok)
	return map[string]any{
		"Upcoming": upcoming, "Past": past, "Therapists": therapists, "AF": af,
		// webcal:// kennt html/template nicht als sicheres Schema; die Adresse bauen wir selbst.
		"CalURL": u, "Webcal": template.URL(webcal(u)), "CalPublic": public, "Address": s.address(), //nolint:gosec // G203
	}, nil
}
