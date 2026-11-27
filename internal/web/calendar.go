package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"forge.gpl-nas.de/physiobase/internal/store"
)

// Der Kalender ist der Tagesplan der Praxis: eine Spalte pro Behandler, Zeitachse,
// Blöcke zum Verschieben. Er wird als SVG gerendert (kein Inline-CSS wegen der CSP,
// keine Positionierung per style-Attribut). Die Woche ist eine Übersicht mit einer
// Karte pro Tag.

const (
	calWidth    = 1000.0 // viewBox-Breite; skaliert per CSS auf die Seitenbreite
	calGutter   = 62.0   // Platz für die Uhrzeiten
	calHeader   = 42.0   // Kopfzeile mit den Behandlern
	calBottom   = 12.0
	calPxPerMin = 1.0
)

// calParams sind die Ansichtsparameter aus der Adresse.
type calParams struct {
	View      string // "tag" oder "woche"
	Day       time.Time
	Therapist int64
	Location  string
	Mine      bool
	Selected  int64
}

// calBlock ist ein Termin mit seiner Geometrie im Tagesplan.
type calBlock struct {
	A          store.Appointment
	X, Y, W, H float64
	Lane       int
	Lanes      int
	Conflict   bool
}

// calColumn ist eine Spalte im Tagesplan (ein Behandler).
type calColumn struct {
	TherapistID int64
	Name        string
	X, W        float64
	Blocks      []calBlock
	Count       int
}

// calDay ist ein Tag in der Wochenansicht.
type calDay struct {
	Day     time.Time
	Label   string
	Today   bool
	Appts   []store.Appointment
	Columns []calColumn // nur in der Tagesansicht gefüllt
}

// calView ist alles, was die Vorlage zum Zeichnen braucht.
type calView struct {
	View       string
	Day        time.Time
	From, To   time.Time
	Grid       int
	StartMin   int
	EndMin     int
	Width      float64
	Height     float64
	Columns    []calColumn
	Days       []calDay
	Appts      []store.Appointment
	Therapists []store.Therapist
	Filter     calParams
	Locations  []string
	MineOK     bool
	Selected   *store.Appointment
	Conflicts  []string
	Empty      bool
	// Vorlage: Navigation, Schnell-Anlage, Druck
	PrevDay, NextDay string
	WeekLabel        string
	Patients         []store.Patient
	AF               apptFormData
}

func parseClock(v string, def int) int {
	p := strings.SplitN(strings.TrimSpace(v), ":", 2)
	if len(p) != 2 {
		return def
	}
	h, err1 := strconv.Atoi(p[0])
	m, err2 := strconv.Atoi(p[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return def
	}
	return h*60 + m
}

func (s *Server) calendarGrid() int {
	n, err := strconv.Atoi(strings.TrimSpace(s.rawText("calendar_grid")))
	if err != nil || (n != 5 && n != 10) {
		return 10
	}
	return n
}

func (s *Server) calendarStartMin() int { return parseClock(s.rawText("calendar_start"), 8*60) }
func (s *Server) calendarEndMin() int   { return parseClock(s.rawText("calendar_end"), 20*60) }

// calParamsFrom liest die Ansicht aus der Adresse (tag, ansicht, behandler, ort, meine, termin).
func calParamsFrom(r *http.Request) calParams {
	q := r.URL.Query()
	p := calParams{View: "tag", Day: startOfDay(time.Now())}
	if q.Get("ansicht") == "woche" {
		p.View = "woche"
	}
	if v := q.Get("tag"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			p.Day = t
		}
	}
	if p.View == "woche" {
		p.Day = startOfWeek(p.Day)
	}
	p.Therapist, _ = strconv.ParseInt(q.Get("behandler"), 10, 64)
	p.Location = strings.TrimSpace(q.Get("ort"))
	p.Mine = q.Get("meine") == "1"
	p.Selected, _ = strconv.ParseInt(q.Get("termin"), 10, 64)
	return p
}

func startOfWeek(t time.Time) time.Time {
	d := startOfDay(t)
	off := (int(d.Weekday()) + 6) % 7 // Montag = 0
	return d.AddDate(0, 0, -off)
}

// buildCalendar sammelt Termine, Behandler und Spalten für die Ansicht.
func (s *Server) buildCalendar(ctx context.Context, r *http.Request, p calParams) (*calView, error) {
	v := &calView{
		View: p.View, Day: p.Day, Grid: s.calendarGrid(),
		StartMin: s.calendarStartMin(), EndMin: s.calendarEndMin(),
		Width: calWidth, Filter: p,
	}
	if v.EndMin <= v.StartMin {
		v.StartMin, v.EndMin = 8*60, 20*60
	}
	if u, ok := currentUser(r); ok && u.TherapistID > 0 {
		v.MineOK = true
	}
	// Behandler-Zuordnung: "nur meine Spalte" wirkt nur, wenn ein Behandler hinterlegt ist.
	if p.Mine && v.MineOK {
		if u, ok := currentUser(r); ok {
			p.Therapist = u.TherapistID
		}
	}
	v.Filter = p

	if v.View == "woche" {
		v.From, v.To = p.Day, p.Day.AddDate(0, 0, 7)
	} else {
		v.From, v.To = p.Day, p.Day.AddDate(0, 0, 1)
	}
	as, err := s.st.AppointmentsBetween(ctx, v.From, v.To, 0)
	if err != nil {
		return nil, err
	}
	// Ort-Filter: leerer Ort gilt als Praxisadresse.
	locs, err := s.st.DistinctLocations(ctx, v.From, v.To)
	if err != nil {
		return nil, err
	}
	v.Locations = locs
	var filtered []store.Appointment
	for _, a := range as {
		if p.Therapist > 0 && a.TherapistID != p.Therapist {
			continue
		}
		if p.Location != "" && effectiveLocation(a.Location, s.address()) != p.Location {
			continue
		}
		filtered = append(filtered, a)
	}
	v.Appts = filtered
	v.Empty = len(filtered) == 0

	therapists, err := s.st.Therapists(ctx, false)
	if err != nil {
		return nil, err
	}
	v.Therapists = therapists

	if v.View == "woche" {
		v.buildWeek()
	} else {
		v.buildDay(as, filtered, p)
	}
	// Editor: ausgewählter Termin samt Konflikten
	if p.Selected > 0 {
		if a, err := s.st.Appointment(ctx, p.Selected); err == nil {
			v.Selected = &a
			conflicts, err := s.appointmentConflicts(ctx, a)
			if err != nil {
				return nil, err
			}
			v.Conflicts = conflicts
		}
	}
	// Überschneidungen sichtbar machen (rote Umrandung), nicht nur beim Speichern.
	v.markConflicts()

	// Navigation und Vorlagen
	step := 1
	if v.View == "woche" {
		step = 7
		v.WeekLabel = v.Day.Format("02.01.") + " – " + v.Day.AddDate(0, 0, 6).Format("02.01.2006")
	}
	v.PrevDay = v.Day.AddDate(0, 0, -step).Format("2006-01-02")
	v.NextDay = v.Day.AddDate(0, 0, step).Format("2006-01-02")
	patients, err := s.st.Patients(ctx, "", false)
	if err != nil {
		return nil, err
	}
	v.Patients = patients
	v.AF = s.apptForm(therapists)
	return v, nil
}

func effectiveLocation(loc, fallback string) string {
	if strings.TrimSpace(loc) == "" {
		return fallback
	}
	return loc
}

// buildDay verteilt die Termine auf Behandler-Spalten und berechnet die Lanes.
func (v *calView) buildDay(all, filtered []store.Appointment, p calParams) {
	// Zeitraum an die Termine anpassen, damit nichts abgeschnitten wird.
	for _, a := range all {
		if m := a.Start.Hour()*60 + a.Start.Minute(); m < v.StartMin {
			v.StartMin = m
		}
		if m := a.End().Hour()*60 + a.End().Minute(); m > v.EndMin {
			v.EndMin = m
		}
	}
	v.StartMin = max(0, v.StartMin/5*5)
	v.EndMin = min(24*60, (v.EndMin+4)/5*5)
	v.Height = calHeader + float64(v.EndMin-v.StartMin)*calPxPerMin + calBottom

	// Spalten: alle aktiven Behandler plus Behandler, die an diesem Tag Termine haben.
	type col struct {
		id   int64
		name string
	}
	cols := []col{{0, "Ohne Behandler"}}
	seen := map[int64]bool{}
	for _, t := range v.Therapists {
		if t.Active {
			cols = append(cols, col{t.ID, t.Name})
			seen[t.ID] = true
		}
	}
	for _, a := range all {
		if a.TherapistID > 0 && !seen[a.TherapistID] {
			cols = append(cols, col{a.TherapistID, a.TherapistName})
			seen[a.TherapistID] = true
		}
	}
	// Ohne-Behandler-Spalte nur zeigen, wenn es solche Termine gibt.
	hasUnassigned := false
	for _, a := range all {
		if a.TherapistID == 0 {
			hasUnassigned = true
		}
	}
	if !hasUnassigned && len(cols) > 1 {
		cols = cols[1:]
	}
	if p.Therapist > 0 {
		var only []col
		for _, c := range cols {
			if c.id == p.Therapist {
				only = append(only, c)
			}
		}
		if len(only) > 0 {
			cols = only
		}
	}
	if len(cols) == 0 {
		cols = []col{{0, "Termine"}}
	}

	usable := v.Width - calGutter - 4
	colW := usable / float64(len(cols))
	v.Columns = make([]calColumn, len(cols))
	for i, c := range cols {
		v.Columns[i] = calColumn{TherapistID: c.id, Name: c.name, X: calGutter + float64(i)*colW, W: colW}
	}
	idx := map[int64]int{}
	for i, c := range v.Columns {
		idx[c.TherapistID] = i
	}
	byCol := map[int][]store.Appointment{}
	for _, a := range filtered {
		i, ok := idx[a.TherapistID]
		if !ok {
			i = 0
		}
		byCol[i] = append(byCol[i], a)
	}
	for i := range v.Columns {
		blocks := v.layoutColumn(byCol[i], v.Columns[i].X, v.Columns[i].W)
		v.Columns[i].Blocks = blocks
		v.Columns[i].Count = len(blocks)
	}
}

// layoutColumn ordnet die Blöcke einer Spalte und verteilt Überschneidungen auf Lanes.
func (v *calView) layoutColumn(as []store.Appointment, x, w float64) []calBlock {
	sort.SliceStable(as, func(i, j int) bool { return as[i].Start.Before(as[j].Start) })
	blocks := make([]calBlock, 0, len(as))
	for _, a := range as {
		startMin := a.Start.Hour()*60 + a.Start.Minute()
		dur := max(a.Duration, 5)
		y := calHeader + float64(startMin-v.StartMin)*calPxPerMin
		h := float64(dur) * calPxPerMin
		blocks = append(blocks, calBlock{A: a, X: x + 2, Y: y, W: w - 4, H: max(h, 10)})
	}
	// Lanes: für jeden Block zählen, wie viele ihn zeitlich überlappen.
	for i := range blocks {
		lane := 0
		for j := range blocks {
			if i == j {
				continue
			}
			if blocksOverlap(blocks[i], blocks[j]) && blocks[j].A.Start.Before(blocks[i].A.Start) {
				lane++
			}
		}
		blocks[i].Lane = lane
	}
	// Lanes-Anzahl je Überlappungsgruppe (zusammenhängend).
	groupStart := 0
	for i := 0; i <= len(blocks); i++ {
		if i == 0 || i == len(blocks) || !blocksOverlap(blocks[i-1], blocks[i]) {
			n := 1
			for k := groupStart; k < i; k++ {
				n = max(n, blocks[k].Lane+1)
			}
			for k := groupStart; k < i; k++ {
				blocks[k].Lanes = n
			}
			groupStart = i
		}
	}
	for i := range blocks {
		if blocks[i].Lanes < 1 {
			blocks[i].Lanes = 1
		}
		lw := blocks[i].W / float64(blocks[i].Lanes)
		blocks[i].W = lw - 2
		blocks[i].X += float64(blocks[i].Lane) * lw
	}
	return blocks
}

func blocksOverlap(a, b calBlock) bool {
	return a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// buildWeek füllt die Tage der Wochenansicht.
func (v *calView) buildWeek() {
	for i := range 7 {
		d := v.Day.AddDate(0, 0, i)
		cd := calDay{Day: d, Label: dayLabel(d), Today: startOfDay(d).Equal(startOfDay(time.Now()))}
		for _, a := range v.Appts {
			if startOfDay(a.Start).Equal(d) {
				cd.Appts = append(cd.Appts, a)
			}
		}
		v.Days = append(v.Days, cd)
	}
}

// markConflicts kennzeichnet Blöcke, die sich im selben Behandler überschneiden,
// sowie die Konflikte des gerade geöffneten Termins.
func (v *calView) markConflicts() {
	for i := range v.Columns {
		bs := v.Columns[i].Blocks
		for j := range bs {
			for k := range bs {
				if j == k || bs[j].A.Cancelled || bs[k].A.Cancelled {
					continue
				}
				if blocksOverlap(bs[j], bs[k]) {
					v.Columns[i].Blocks[j].Conflict = true
				}
			}
			if v.Selected != nil && bs[j].A.ID != v.Selected.ID && !bs[j].A.Cancelled &&
				overlaps(bs[j].A, *v.Selected) &&
				(bs[j].A.TherapistID == v.Selected.TherapistID || bs[j].A.PatientID == v.Selected.PatientID) {
				v.Columns[i].Blocks[j].Conflict = true
			}
		}
	}
}

// ---------- Konflikte ----------

// appointmentConflicts prüft einen Termin gegen alle anderen am selben Tag:
// Überschneidung im selben Behandler und Doppelbelegung eines Patienten.
func (s *Server) appointmentConflicts(ctx context.Context, a store.Appointment) ([]string, error) {
	day := startOfDay(a.Start)
	as, err := s.st.AppointmentsBetween(ctx, day, day.AddDate(0, 0, 1), 0)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	add := func(msg string) {
		if !seen[msg] {
			seen[msg] = true
			out = append(out, msg)
		}
	}
	for _, b := range as {
		if b.ID == a.ID || b.Cancelled || a.Cancelled {
			continue
		}
		if !overlaps(a, b) {
			continue
		}
		if a.TherapistID > 0 && a.TherapistID == b.TherapistID {
			add(fmt.Sprintf("Überschneidung mit %s, %s–%s (%s)", b.PatientName, hmOf(b.Start), hmOf(b.End()), b.Title))
		}
		if a.PatientID == b.PatientID {
			add(fmt.Sprintf("%s hat zur gleichen Zeit schon einen Termin: %s–%s%s",
				b.PatientName, hmOf(b.Start), hmOf(b.End()), withTherapist(b.TherapistName)))
		}
	}
	return out, nil
}

func overlaps(a, b store.Appointment) bool {
	return a.Start.Before(b.End()) && b.Start.Before(a.End())
}

func hmOf(t time.Time) string { return t.Local().Format("15:04") }

func withTherapist(name string) string {
	if name == "" {
		return ""
	}
	return " bei " + name
}

// ---------- Handler ----------

func (s *Server) calendarPage(w http.ResponseWriter, r *http.Request) {
	p := calParamsFrom(r)
	v, err := s.buildCalendar(r.Context(), r, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "calendar", http.StatusOK, map[string]any{
		"Cal": v, "Error": r.URL.Query().Get("fehler"), "Saved": r.URL.Query().Get("gespeichert") == "1",
	})
}

// moveAppointment verschiebt einen Termin oder ändert seine Dauer. Derselbe Endpunkt
// bedient das Ziehen im Kalender (JSON) und die Formular-Knöpfe ohne JavaScript
// (Weiterleitung zurück in den Kalender).
func (s *Server) moveAppointment(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
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
	back := func(msg string) {
		if msg == "" {
			redirect(w, r, "/kalender?tag=%s&termin=%d&gespeichert=1", startOfDay(a.Start).Format("2006-01-02"), id)
			return
		}
		redirect(w, r, "/kalender?tag=%s&termin=%d&fehler=%s", startOfDay(a.Start).Format("2006-01-02"), id, urlPathEscape(msg))
	}
	// Schnellknöpfe (ohne JavaScript): Verschiebung oder Daueränderung als Delta.
	if d, err := strconv.Atoi(formText(r, "delta_min")); err == nil && d != 0 {
		a.Start = a.Start.Add(time.Duration(d) * time.Minute)
	} else {
		start, err := time.ParseInLocation("2006-01-02 15:04", formText(r, "date")+" "+formText(r, "time"), time.Local)
		if err != nil {
			if wantsJSON(r) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Datum oder Uhrzeit ungültig"})
				return
			}
			back("Datum oder Uhrzeit ist ungültig.")
			return
		}
		a.Start = start
	}
	if d, err := strconv.Atoi(formText(r, "delta_dur")); err == nil && d != 0 {
		a.Duration = max(5, min(480, a.Duration+d))
	} else if dur, err := strconv.Atoi(formText(r, "duration")); err == nil && dur >= 5 && dur <= 480 {
		a.Duration = dur
	}
	if formText(r, "therapist") != "" {
		if tid, ok := formID(r, "therapist"); ok {
			a.TherapistID = tid
		} else {
			a.TherapistID = 0
		}
	}
	conflicts, err := s.appointmentConflicts(r.Context(), a)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(conflicts) > 0 && r.FormValue("bestaetigt") != "1" {
		if wantsJSON(r) {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "conflicts": conflicts})
			return
		}
		back("Überschneidung: " + strings.Join(conflicts, "; ") + " – zum Übernehmen erneut mit Bestätigung speichern.")
		return
	}
	if err := s.st.UpdateAppointment(r.Context(), a); err != nil {
		s.fail(w, r, err)
		return
	}
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tag": startOfDay(a.Start).Format("2006-01-02")})
		return
	}
	back("")
}

// wantsJSON: das Kalender-Skript fragt mit Accept: application/json an.
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// printCalendar druckt den Tagesplan (pro Behandler oder für die ganze Praxis).
func (s *Server) printCalendar(w http.ResponseWriter, r *http.Request) {
	p := calParamsFrom(r)
	v, err := s.buildCalendar(r.Context(), r, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Druck: immer der Tagesplan, auch wenn die Woche gewählt war.
	if v.View == "woche" {
		p.View = "tag"
		v, err = s.buildCalendar(r.Context(), r, p)
		if err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.render(w, r, "calendar_print", http.StatusOK, map[string]any{
		"Cal": v, "Address": s.address(), "Today": time.Now(),
	})
}

// ---------- SVG ----------

// calendarSVG zeichnet den Tagesplan als SVG. Koordinaten sind fest in einer
// viewBox; CSS skaliert das Bild auf die Seitenbreite. Links öffnen den Editor.
func calendarSVG(v *calView) template.HTML {
	if v == nil || v.View != "tag" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="cal-svg" viewBox="0 0 %.0f %.0f" role="img" aria-label="Tagesplan" data-cal-day="%s" data-cal-grid="%d" data-cal-startmin="%d" data-cal-pxmin="%.2f" data-cal-gutter="%.0f" data-cal-header="%.0f" data-cal-width="%.0f" data-cal-therapists="%s">`,
		v.Width, v.Height, v.Day.Format("2006-01-02"), v.Grid, v.StartMin, calPxPerMin, calGutter, calHeader, v.Width, therapistIDs(v))

	// Zeitachse: Linien im eingestellten Raster, Beschriftung im 20-Minuten-Takt.
	for m := v.StartMin; m <= v.EndMin; m += v.Grid {
		y := calHeader + float64(m-v.StartMin)*calPxPerMin
		hour := m%60 == 0
		cls := "cal-line"
		if hour {
			cls = "cal-line cal-hour"
		}
		fmt.Fprintf(&b, `<line class="%s" x1="%.0f" y1="%.1f" x2="%.0f" y2="%.1f"/>`, cls, calGutter, y, v.Width, y)
		if m%20 == 0 {
			fmt.Fprintf(&b, `<text class="cal-time" x="%.0f" y="%.1f" text-anchor="end">%02d:%02d</text>`, calGutter-6, y+4, m/60, m%60)
		}
	}
	// Spaltenköpfe und Trennlinien
	for _, c := range v.Columns {
		fmt.Fprintf(&b, `<text class="cal-colhead" x="%.1f" y="%.0f" text-anchor="middle">%s</text>`,
			c.X+c.W/2, calHeader-14, html.EscapeString(c.Name))
		fmt.Fprintf(&b, `<line class="cal-colsep" x1="%.1f" y1="%.0f" x2="%.1f" y2="%.0f"/>`, c.X, calHeader, c.X, v.Height-calBottom)
	}
	fmt.Fprintf(&b, `<line class="cal-colsep" x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f"/>`, v.Width-2, calHeader, v.Width-2, v.Height-calBottom)

	// Jetzt-Linie
	if now := time.Now(); startOfDay(now).Equal(v.Day) {
		m := now.Hour()*60 + now.Minute()
		if m >= v.StartMin && m <= v.EndMin {
			y := calHeader + float64(m-v.StartMin)*calPxPerMin
			fmt.Fprintf(&b, `<line class="cal-now" x1="%.0f" y1="%.1f" x2="%.0f" y2="%.1f"/>`, calGutter, y, v.Width, y)
		}
	}

	// Blöcke
	for _, c := range v.Columns {
		for _, blk := range c.Blocks {
			a := blk.A
			cls := "cal-block"
			if a.Cancelled {
				cls += " cal-cancelled"
			}
			if blk.Conflict {
				cls += " cal-conflict"
			}
			if blk.Lanes > 1 {
				cls += " cal-split"
			}
			href := fmt.Sprintf("/kalender?tag=%s&amp;ansicht=tag&amp;termin=%d", v.Day.Format("2006-01-02"), a.ID)
			if v.Filter.Therapist > 0 {
				href += "&behandler=" + strconv.FormatInt(v.Filter.Therapist, 10)
			}
			fmt.Fprintf(&b, `<a class="cal-link" href="%s" data-appt="%d" data-startmin="%d" data-duration="%d" data-therapist="%d" data-col="%d">`,
				href, a.ID, a.Start.Hour()*60+a.Start.Minute(), a.Duration, a.TherapistID, c.TherapistID)
			fmt.Fprintf(&b, `<g class="%s">`, cls)
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="4"/>`, blk.X, blk.Y, blk.W, blk.H)
			// Text: Name (Nachname + Initial), Art, Behandler – je nach Höhe.
			lines := blockLines(a, blk.H)
			tx := blk.X + 5
			ty := blk.Y + 12
			for i, ln := range lines {
				lcls := "cal-btext"
				if i == 0 {
					lcls = "cal-bname"
				}
				fmt.Fprintf(&b, `<text class="%s" x="%.1f" y="%.1f">%s</text>`, lcls, tx, ty+float64(i)*11, html.EscapeString(ln))
			}
			if a.Cancelled {
				fmt.Fprintf(&b, `<line class="cal-strike" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`,
					blk.X+2, blk.Y+2, blk.X+blk.W-2, blk.Y+blk.H-2)
			}
			b.WriteString(`</g>`)
			// Beim Überfahren leuchtet der Zeitstreifen in der linken Spalte auf.
			fmt.Fprintf(&b, `<rect class="cal-gutter-hl" x="0" y="%.1f" width="%.1f" height="%.1f" aria-hidden="true"/>`,
				blk.Y, calGutter-4, blk.H)
			b.WriteString(`</a>`)
			// Schnellzugriffe oben rechts, sichtbar beim Überfahren: Patientenakte und Löschen.
			if blk.W >= 40 && blk.H >= 20 {
				b.WriteString(`<g class="cal-actions">`)
				fmt.Fprintf(&b, `<a class="cal-patient" href="/patient/%d#termine" target="_blank" rel="noopener">`, a.PatientID)
				fmt.Fprintf(&b, `<title>Patientenakte von %s</title>`, html.EscapeString(a.PatientName))
				fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="7"/>`, blk.X+blk.W-10, blk.Y+10)
				fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle">↗</text>`, blk.X+blk.W-10, blk.Y+13)
				b.WriteString(`</a>`)
				if blk.W >= 58 {
					fmt.Fprintf(&b, `<a class="cal-delete" href="/kalender?tag=%s&amp;termin=%d#loeschen" data-appt="%d" data-tag="%s">`,
						v.Day.Format("2006-01-02"), a.ID, a.ID, v.Day.Format("2006-01-02"))
					fmt.Fprintf(&b, `<title>Termin löschen: %s, %s</title>`, html.EscapeString(a.PatientName), hmOf(a.Start))
					fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="7"/>`, blk.X+blk.W-27, blk.Y+10)
					fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle">×</text>`, blk.X+blk.W-27, blk.Y+14)
					b.WriteString(`</a>`)
				}
				b.WriteString(`</g>`)
			}
		}
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) //nolint:gosec // G203: nur aus geprüften Feldern und html.EscapeString gebaut
}

// blockLines wählt die Zeilen im Block passend zur Höhe. Die Uhrzeit steht
// nicht mehr im Block – sie leuchtet beim Überfahren in der Zeitachse auf.
func blockLines(a store.Appointment, h float64) []string {
	name := shortName(a.PatientName)
	if name == "" {
		name = a.Title
	}
	if name == "" {
		name = "Termin"
	}
	lines := []string{name}
	if h >= 34 && a.Title != "" && a.Title != name {
		lines = append(lines, a.Title)
	}
	if h >= 48 && a.TherapistName != "" {
		lines = append(lines, a.TherapistName)
	}
	return lines
}

// shortName kürzt "Vorname Nachname" auf "Nachname V.".
func shortName(name string) string {
	f := strings.Fields(name)
	if len(f) == 0 {
		return ""
	}
	if len(f) == 1 {
		return f[0]
	}
	last := []rune(f[len(f)-1])
	first := []rune(f[0])
	if len(first) == 0 || len(last) == 0 {
		return strings.Join(f, " ")
	}
	last[0] = unicode.ToUpper(last[0])
	return string(last) + " " + string(unicode.ToUpper(first[0])) + "."
}

// therapistIDs liefert die Spaltenzuordnung als data-Attribut (JS braucht sie beim Ziehen).
func therapistIDs(v *calView) string {
	ids := make([]string, len(v.Columns))
	for i, c := range v.Columns {
		ids[i] = strconv.FormatInt(c.TherapistID, 10)
	}
	return strings.Join(ids, ",")
}
