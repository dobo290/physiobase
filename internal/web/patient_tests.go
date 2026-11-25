package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// patientTest ist ein dem Patienten zugeordneter Test samt letztem Ergebnis.
type patientTest struct {
	EntryID  int64
	Title    string
	HasQ     bool
	Date     string
	Sum, Max int
	Complete bool
	Note     string
	Source   string // "Befund" oder "Test"
	BefundID int64
}

// patientTests baut die Liste für die Patientenübersicht: verknüpfte Tests mit
// dem jeweils neuesten Ergebnis (aus Befunden oder Einzeldurchführungen).
func (s *Server) patientTests(ctx context.Context, pid int64, bs []store.Befund) ([]patientTest, error) {
	linked, err := s.st.PatientTests(ctx, pid)
	if err != nil {
		return nil, err
	}
	qes, err := s.questionnaireEntries(ctx)
	if err != nil {
		return nil, err
	}
	defs := make(map[int64]quizEntry, len(qes))
	for _, q := range qes {
		defs[q.ID] = q
	}
	runs, err := s.st.LatestTestRuns(ctx, pid)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]*patientTest, len(linked))
	for _, e := range linked {
		out[e.ID] = &patientTest{EntryID: e.ID, Title: e.Title}
		if _, ok := defs[e.ID]; ok {
			out[e.ID].HasQ = true
		}
	}

	// Ergebnisse aus Einzeldurchführungen
	for id, run := range runs {
		t, ok := out[id]
		if !ok {
			continue
		}
		var f befund.QFill
		if err := json.Unmarshal([]byte(run.Data), &f); err != nil {
			continue
		}
		def := defs[id]
		t.Date, t.Sum, t.Max = run.Date, f.Sum(), def.Q.MaxTotal()
		t.Complete = f.Missing(def.Q) == 0 && len(f.Values) > 0
		t.Note = f.Note
		t.Source = "Test"
	}

	// Ergebnisse aus Befunden (neuester Befund gewinnt, wenn jünger)
	for _, b := range bs { // neueste zuerst
		d, err := befund.Parse(b.Data)
		if err != nil || len(d.Q) == 0 {
			continue
		}
		for _, idStr := range d.QList() {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				continue
			}
			t, ok := out[id]
			if !ok {
				continue
			}
			if t.Date >= b.Date && t.Date != "" {
				continue // Einzellauf ist neuer
			}
			f := d.Q[idStr]
			def := defs[id]
			t.Date, t.Sum, t.Max = b.Date, f.Sum(), def.Q.MaxTotal()
			t.Complete = f.Missing(def.Q) == 0 && len(f.Values) > 0
			t.Note = f.Note
			t.Source = "Befund"
			t.BefundID = b.ID
		}
	}

	list := make([]patientTest, 0, len(out))
	for _, t := range out {
		list = append(list, *t)
	}
	sortTests(list)
	return list, nil
}

func sortTests(l []patientTest) {
	for i := 1; i < len(l); i++ {
		for j := i; j > 0 && strings.ToLower(l[j].Title) < strings.ToLower(l[j-1].Title); j-- {
			l[j], l[j-1] = l[j-1], l[j]
		}
	}
}

// runTestForm zeigt das Formular für eine Testdurchführung ohne Befund.
func (s *Server) runTestForm(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	eid, ok := pathID(r, "eid")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := s.st.Patient(r.Context(), pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	e, err := s.st.Entry(r.Context(), eid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q, hasQ := befund.ParseQuestionnaire(e.Schema)
	runs, _ := s.st.TestRuns(r.Context(), pid, eid)
	var last befund.QFill
	if len(runs) > 0 {
		_ = json.Unmarshal([]byte(runs[0].Data), &last)
	}
	s.render(w, r, "patient_test", http.StatusOK, map[string]any{
		"P": p, "E": e, "Q": q, "HasQ": hasQ, "Last": last, "Runs": len(runs),
		"Today": time.Now().Format("2006-01-02"),
		"Error": r.URL.Query().Get("fehler"),
	})
}

// saveTestRun speichert eine Testdurchführung ohne Befund.
func (s *Server) saveTestRun(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	eid, ok := pathID(r, "eid")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formular ungültig", http.StatusBadRequest)
		return
	}
	e, err := s.st.Entry(r.Context(), eid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q, hasQ := befund.ParseQuestionnaire(e.Schema)
	fill := befund.QFill{Note: strings.TrimSpace(r.FormValue("note"))}
	if hasQ {
		for i, it := range q.Items {
			v := strings.TrimSpace(r.FormValue("v." + strconv.Itoa(i)))
			if v == "" {
				fill.Values = append(fill.Values, "")
				continue
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				n = 0
			}
			if n > it.Max {
				n = it.Max
			}
			fill.Values = append(fill.Values, strconv.Itoa(n))
		}
	}
	raw, _ := json.Marshal(fill)
	date := strings.TrimSpace(r.FormValue("date"))
	if !isoDate.MatchString(date) {
		date = time.Now().Format("2006-01-02")
	}
	if err := s.st.AddTestRun(r.Context(), pid, eid, date, string(raw)); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/patient/"+strconv.FormatInt(pid, 10)+"#tests", http.StatusSeeOther)
}

// removePatientTest entfernt einen Test samt Durchführungen von einem Patienten.
// Der Befund bleibt unverändert, dort dokumentierte Ergebnisse bleiben erhalten.
func (s *Server) removePatientTest(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	eid, ok := pathID(r, "eid")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.st.DeleteTestRuns(r.Context(), pid, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.UnlinkPatientTest(r.Context(), pid, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("test vom patienten entfernt", "patient", pid, "test", eid, "von", s.who(r))
	redirect(w, r, "/patient/%d#tests", pid)
}

// linkPatientTest verknüpft einen Test direkt aus der Patientenübersicht.
func (s *Server) linkPatientTest(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	eid, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("test_id")), 10, 64)
	if err != nil || eid <= 0 {
		http.Redirect(w, r, "/patient/"+strconv.FormatInt(pid, 10)+"#tests", http.StatusSeeOther)
		return
	}
	if err := s.st.LinkPatientTest(r.Context(), pid, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/patient/"+strconv.FormatInt(pid, 10)+"#tests", http.StatusSeeOther)
}
