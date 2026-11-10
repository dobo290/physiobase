package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// Zeilen, die mit diesem Zeichen beginnen, sind Hinweise für die Therapeutin
// und erscheinen nicht im Ausdruck.
const noteMark = "✎"

// Printable entfernt Hinweiszeilen und daraus entstandene leere Abschnitte.
func Printable(body string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), noteMark) {
			continue
		}
		out = append(out, line)
	}
	// Überschriften ohne Inhalt bis zur nächsten Überschrift entfernen
	var cleaned []string
	for i := 0; i < len(out); i++ {
		if strings.HasPrefix(out[i], "## ") {
			j := i + 1
			for j < len(out) && strings.TrimSpace(out[j]) == "" {
				j++
			}
			if j == len(out) || strings.HasPrefix(out[j], "## ") {
				i = j - 1
				continue
			}
		}
		cleaned = append(cleaned, out[i])
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

type draft struct{ b strings.Builder }

func (d *draft) section(title string) { fmt.Fprintf(&d.b, "\n## %s\n\n", title) }
func (d *draft) line(format string, a ...any) {
	fmt.Fprintf(&d.b, format+"\n", a...)
}
func (d *draft) note(text string) { d.line("%s %s", noteMark, text) }
func (d *draft) String() string   { return strings.TrimSpace(d.b.String()) + "\n" }

// latestHandoutItems: Übungen des zuletzt bearbeiteten Heimprogramms des Patienten.
func (s *Server) latestHandoutItems(ctx context.Context, pid int64) (store.Handout, []store.HandoutItem, error) {
	hs, err := s.st.HandoutsForPatient(ctx, pid)
	if err != nil || len(hs) == 0 {
		return store.Handout{}, nil, err
	}
	items, err := s.st.HandoutItems(ctx, hs[0].ID)
	return hs[0], items, err
}

func (d *draft) exercises(h store.Handout, items []store.HandoutItem) {
	if len(items) == 0 {
		d.line("- ")
		d.note("Noch kein Heimprogramm – auf der Patientenseite unter „Heimprogramme“ anlegen.")
		return
	}
	for _, it := range items {
		if it.Dosage != "" {
			d.line("- **%s** – %s", it.Entry.Title, it.Dosage)
		} else {
			d.line("- **%s**", it.Entry.Title)
		}
	}
	d.line("")
	d.line("Die genaue Anleitung mit Bildern finden Sie im Übungsblatt „%s“.", h.Title)
}

func (s *Server) safetyNet(d *draft) {
	d.section("Wann Sie sich melden sollten")
	d.line("%s", s.text("summary_safety_net"))
}

// generalDraft baut die Gesamtzusammenfassung aus dem neuesten Befund, dem
// Heimprogramm und dem Verlauf seit dem ersten Befund.
func (s *Server) generalDraft(ctx context.Context, p store.Patient) (string, error) {
	bs, err := s.st.Befunde(ctx, p.ID)
	if err != nil {
		return "", err
	}
	h, items, err := s.latestHandoutItems(ctx, p.ID)
	if err != nil {
		return "", err
	}
	d := &draft{}
	d.note("Entwurf aus den Befunddaten. In einfache Worte bringen; ✎-Zeilen werden nicht gedruckt.")
	var last, first befund.Data
	if len(bs) > 0 {
		last, _ = befund.Parse(bs[0].Data)
		first, _ = befund.Parse(bs[len(bs)-1].Data)
	}

	d.section("Ihre Beschwerden")
	if diag := last.Get("diag"); diag != "" {
		d.line("Ärztliche Diagnose: %s", diag)
		d.line("")
	}
	d.line("")
	d.note("In 2–3 Sätzen erklären, was los ist und warum. Fachbegriffe übersetzen.")
	if h := last.Get("hypAfterExam"); h != "" {
		d.note("Ihre Arbeitshypothese: " + oneLine(h))
	}

	if now, worst := last.Get("pain_now"), last.Get("pain_worst"); now != "" || worst != "" {
		d.section("Was wir festgestellt haben")
		if now != "" {
			d.line("- Schmerz heute: %s von 10", now)
		}
		if worst != "" {
			d.line("- Stärkster Schmerz in den letzten 24 Stunden: %s von 10", worst)
		}
		d.note("Wichtigste Befunde in Alltagssprache ergänzen (Beweglichkeit, Kraft …).")
	}

	d.section("Ihre Ziele")
	for _, k := range []string{"goal_patient", "goal_agreement"} {
		if v := last.Get(k); v != "" {
			for _, l := range strings.Split(v, "\n") {
				if l = strings.TrimSpace(strings.TrimLeft(l, "-•* ")); l != "" {
					d.line("- %s", l)
				}
			}
		}
	}
	for _, r := range last.Tables["psfs"] {
		if r["activity"] != "" {
			if r["score"] != "" {
				d.line("- %s (heute %s von 10 möglich)", r["activity"], r["score"])
			} else {
				d.line("- %s", r["activity"])
			}
		}
	}
	d.note("Ziele so formulieren, wie der Patient sie selbst sagen würde.")

	d.section("So gehen wir vor")
	if plan := last.Get("plan"); plan != "" {
		d.note("Therapieplan aus dem Befund: " + oneLine(plan))
	}
	d.line("- ")
	d.note("Behandlungsschwerpunkte, Anzahl/Häufigkeit der Termine, wann wir neu messen.")

	d.section("Ihr Übungsprogramm für zu Hause")
	d.exercises(h, items)

	if len(bs) > 1 {
		d.section("Ihr Fortschritt")
		if a, b := first.Get("pain_now"), last.Get("pain_now"); a != "" && b != "" {
			d.line("- Schmerz: von %s auf %s (von 10)", a, b)
		}
		if a, b := befund.PSFSMean(first), befund.PSFSMean(last); a != "" && b != "" {
			d.line("- Ihre Alltagsaktivitäten: von durchschnittlich %s auf %s (von 10)", a, b)
		}
		d.note("Was hat sich im Alltag verbessert? Konkrete Beispiele des Patienten aufgreifen.")
	}

	s.safetyNet(d)
	return d.String(), nil
}

// sessionDraft baut die Zusammenfassung einer Behandlung.
func (s *Server) sessionDraft(ctx context.Context, x store.Session) (string, error) {
	h, items, err := s.latestHandoutItems(ctx, x.PatientID)
	if err != nil {
		return "", err
	}
	d := &draft{}
	d.note("Entwurf aus dem Verlaufseintrag. ✎-Zeilen werden nicht gedruckt.")
	d.section("Das haben wir heute gemacht")
	if x.Goal != "" {
		d.line("- %s", x.Goal)
	} else {
		d.line("- ")
	}
	if x.Note != "" {
		d.note("Ihre Notiz: " + oneLine(x.Note))
	}
	d.section("Ihre Übungen bis zum nächsten Termin")
	d.exercises(h, items)
	d.section("Darauf sollten Sie achten")
	d.line("%s", s.text("summary_session_watch"))
	d.section("Nächster Termin")
	day, _ := time.ParseInLocation("2006-01-02", x.Date, time.Local)
	next, found, err := s.st.NextAppointment(ctx, x.PatientID, day.Add(24*time.Hour-time.Second))
	if err != nil {
		return "", err
	}
	if found {
		with := ""
		if next.TherapistName != "" {
			with = " bei " + next.TherapistName
		}
		d.line("- %s um %s Uhr%s", dayLabel(next.Start), next.Start.Format("15:04"), with)
	} else {
		d.line("- ")
		d.note("Noch kein Folgetermin eingetragen.")
	}
	return d.String(), nil
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// ---------- Handler ----------

func (s *Server) createGeneralSummary(w http.ResponseWriter, r *http.Request) {
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
	body, err := s.generalDraft(ctx, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	id, err := s.st.CreateSummary(ctx, store.Summary{
		PatientID: pid, Kind: "gesamt",
		Date: time.Now().Format("2006-01-02"), Title: s.text("summary_general_title", "name", p.Name), Body: body,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/zusammenfassung/%d", id)
}

func (s *Server) createSessionSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	x, err := s.st.Session(ctx, sid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	body, err := s.sessionDraft(ctx, x)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	id, err := s.st.CreateSummary(ctx, store.Summary{
		PatientID: x.PatientID, SessionID: x.ID, Kind: "sitzung",
		Date: x.Date, Title: s.text("summary_session_title", "datum", deDate(x.Date)), Body: body,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/zusammenfassung/%d", id)
}

func (s *Server) loadSummary(w http.ResponseWriter, r *http.Request) (store.Summary, store.Patient, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return store.Summary{}, store.Patient{}, false
	}
	x, err := s.st.Summary(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return x, store.Patient{}, false
	}
	p, err := s.st.Patient(r.Context(), x.PatientID)
	if err != nil {
		s.fail(w, r, err)
		return x, p, false
	}
	return x, p, true
}

func (s *Server) showSummary(w http.ResponseWriter, r *http.Request) {
	x, p, ok := s.loadSummary(w, r)
	if !ok {
		return
	}
	log, err := s.st.MailLogFor(r.Context(), x.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	s.render(w, r, "summary", http.StatusOK, map[string]any{
		"S": x, "P": p, "Preview": Printable(x.Body), "Saved": q.Get("gespeichert") == "1",
		"MailBlock": s.mailBlockReason(p), "MailLog": log, "Sent": q.Get("gesendet") == "1", "MailError": q.Get("fehler"),
	})
}

func (s *Server) saveSummary(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	x, _, ok := s.loadSummary(w, r)
	if !ok {
		return
	}
	x.Title, x.Date, x.Body = formText(r, "title"), validDate(formText(r, "date")), formText(r, "body")
	if x.Title == "" {
		x.Title = "Zusammenfassung"
	}
	if err := s.st.UpdateSummary(r.Context(), x); err != nil {
		s.fail(w, r, err)
		return
	}
	if r.FormValue("next") == "druck" {
		redirect(w, r, "/zusammenfassung/%d/druck", x.ID)
		return
	}
	redirect(w, r, "/zusammenfassung/%d?gespeichert=1", x.ID)
}

func (s *Server) deleteSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pid, err := s.st.DeleteSummary(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#zusammenfassungen", pid)
}

func (s *Server) printSummary(w http.ResponseWriter, r *http.Request) {
	x, p, ok := s.loadSummary(w, r)
	if !ok {
		return
	}
	s.render(w, r, "summary_print", http.StatusOK, map[string]any{"S": x, "P": p, "Body": Printable(x.Body)})
}

func deDate(iso string) string {
	if t, err := time.Parse("2006-01-02", iso); err == nil {
		return t.Format("02.01.2006")
	}
	return iso
}
