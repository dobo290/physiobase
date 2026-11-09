package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/mailer"
	"forge.gpl-nas.de/physiobase/internal/store"
)

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validDate(s string) string {
	if isoDate.MatchString(s) {
		return s
	}
	return time.Now().Format("2006-01-02")
}

// ---------- Patientenliste ----------

func (s *Server) listPatients(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	archived := r.URL.Query().Get("archiv") == "1"
	ps, err := s.st.Patients(r.Context(), q, archived)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "patients", http.StatusOK, map[string]any{
		"Patients": ps, "Q": q, "Archived": archived, "Imported": r.URL.Query().Get("importiert"),
	})
}

func (s *Server) createPatient(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	p := store.Patient{
		Name: formText(r, "name"), Dob: formText(r, "dob"), Phone: formText(r, "phone"),
		Email: formText(r, "email"), EmailConsent: r.FormValue("email_consent") == "1",
	}
	if p.Name == "" {
		http.Error(w, "Name fehlt", http.StatusBadRequest)
		return
	}
	if p.Email != "" && !mailer.ValidAddress(p.Email) {
		http.Error(w, "Ungültige E-Mail-Adresse", http.StatusBadRequest)
		return
	}
	id, err := s.st.CreatePatient(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d", id)
}

// ---------- Patient ----------

func (s *Server) showPatient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := s.st.Patient(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	bs, err := s.st.Befunde(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	sessions, err := s.st.Sessions(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	linked, err := s.st.PatientEntries(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	linkable, err := s.st.EntriesOfKind(ctx, "krankheitsbild", "protokoll", "notiz")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	handouts, err := s.st.HandoutsForPatient(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	docs, err := s.st.Docs(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	summaries, err := s.st.Summaries(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	appt, err := s.patientAppointmentData(r, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Verlauf der Kernwerte über alle Befunde (älteste zuerst)
	type trendRow struct {
		Date, Title             string
		Now, Worst, PSFS, Tests string
		ID                      int64
	}
	var trend []trendRow
	for i := len(bs) - 1; i >= 0; i-- {
		d, _ := befund.Parse(bs[i].Data)
		trend = append(trend, trendRow{
			ID: bs[i].ID, Date: bs[i].Date, Title: bs[i].Title,
			Now: d.Get("pain_now"), Worst: d.Get("pain_worst"), PSFS: befund.PSFSMean(d),
			Tests: fmt.Sprint(len(d.Tables["measures"])),
		})
	}
	s.render(w, r, "patient", http.StatusOK, map[string]any{
		"P": p, "Befunde": bs, "Sessions": sessions, "Linked": linked, "Linkable": linkable,
		"Handouts": handouts, "Trend": trend, "Today": time.Now().Format("2006-01-02"),
		"Docs": docs, "DocCategories": store.DocCategories, "Summaries": summaries, "Appt": appt,
		"Error": r.URL.Query().Get("fehler"),
	})
}

func (s *Server) updatePatient(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p := store.Patient{
		ID: id, Name: formText(r, "name"), Dob: formText(r, "dob"), Phone: formText(r, "phone"), Notes: formText(r, "notes"),
		Email: formText(r, "email"), EmailConsent: r.FormValue("email_consent") == "1",
	}
	if p.Name == "" {
		http.Error(w, "Name fehlt", http.StatusBadRequest)
		return
	}
	if p.Email != "" && !mailer.ValidAddress(p.Email) {
		http.Error(w, "Ungültige E-Mail-Adresse", http.StatusBadRequest)
		return
	}
	if err := s.st.UpdatePatient(r.Context(), p); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d", id)
}

func (s *Server) archivePatient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.st.SetArchived(r.Context(), id, r.FormValue("archiv") == "1"); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d", id)
}

func (s *Server) deletePatient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	files, err := s.st.DocFiles(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.DeletePatient(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	for _, f := range files {
		s.removeDocFile(f)
	}
	redirect(w, r, "/patienten")
}

func (s *Server) addSession(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	x := store.Session{PatientID: id, Date: validDate(formText(r, "date")), Goal: formText(r, "goal"), Note: formText(r, "note")}
	if x.Goal != "" || x.Note != "" {
		if err := s.st.AddSession(r.Context(), x); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	redirect(w, r, "/patient/%d#verlauf", id)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pid, err := s.st.DeleteSession(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#verlauf", pid)
}

func (s *Server) linkPatientEntry(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok1 := pathID(r, "id")
	eid, ok2 := formID(r, "entry")
	if !ok1 || !ok2 {
		http.Error(w, "Bitte einen Eintrag auswählen", http.StatusBadRequest)
		return
	}
	if err := s.st.LinkPatientEntry(r.Context(), id, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#wissen", id)
}

func (s *Server) unlinkPatientEntry(w http.ResponseWriter, r *http.Request) {
	id, ok1 := pathID(r, "id")
	eid, ok2 := pathID(r, "eid")
	if !ok1 || !ok2 {
		http.NotFound(w, r)
		return
	}
	if err := s.st.UnlinkPatientEntry(r.Context(), id, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d#wissen", id)
}

func (s *Server) createPatientHandout(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := s.st.Patient(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	title := formText(r, "title")
	if title == "" {
		title = "Heimübungsprogramm"
	}
	hid, err := s.st.CreateHandoutFor(r.Context(), title, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handout/%d", hid)
}

// ---------- Befund ----------

// newBefund: leer oder als Kopie des letzten Befunds (Wiederbefund – nur Werte neu messen).
func (s *Server) newBefund(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	ctx := r.Context()
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := s.st.Patient(ctx, pid); err != nil {
		s.fail(w, r, err)
		return
	}
	b := store.Befund{PatientID: pid, Date: time.Now().Format("2006-01-02"), Title: "Erstbefund"}
	if src, ok := formID(r, "kopie"); ok {
		old, err := s.st.Befund(ctx, src)
		if err != nil || old.PatientID != pid {
			http.Error(w, "Vorlage nicht gefunden", http.StatusBadRequest)
			return
		}
		b.Data, b.Title = old.Data, "Wiederbefund"
	}
	if t := formText(r, "title"); t != "" {
		b.Title = t
	}
	id, err := s.st.CreateBefund(ctx, b)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/befund/%d", id)
}

func (s *Server) loadBefund(w http.ResponseWriter, r *http.Request) (store.Befund, store.Patient, befund.Data, bool) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return store.Befund{}, store.Patient{}, befund.Data{}, false
	}
	b, err := s.st.Befund(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return b, store.Patient{}, befund.Data{}, false
	}
	p, err := s.st.Patient(ctx, b.PatientID)
	if err != nil {
		s.fail(w, r, err)
		return b, p, befund.Data{}, false
	}
	d, err := befund.Parse(b.Data)
	if err != nil {
		s.fail(w, r, err)
		return b, p, d, false
	}
	return b, p, d, true
}

func (s *Server) showBefund(w http.ResponseWriter, r *http.Request) {
	b, p, d, ok := s.loadBefund(w, r)
	if !ok {
		return
	}
	tests, err := s.st.EntriesOfKind(r.Context(), "test")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "befund", http.StatusOK, map[string]any{
		"B": b, "P": p, "D": d, "Sections": befund.Sections, "Symbols": befund.Symbols, "Views": befund.Views,
		"Tests": tests, "Measures": befund.Measures, "Saved": r.URL.Query().Get("gespeichert") == "1",
	})
}

func (s *Server) saveBefund(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formular ungültig oder zu groß", http.StatusBadRequest)
		return
	}
	b, _, _, ok := s.loadBefund(w, r)
	if !ok {
		return
	}
	b.Date = validDate(formText(r, "date"))
	b.Title = formText(r, "title")
	if b.Title == "" {
		b.Title = "Befund"
	}
	b.Data = befund.FromForm(r.PostForm).JSON()
	if err := s.st.UpdateBefund(r.Context(), b); err != nil {
		s.fail(w, r, err)
		return
	}
	if r.FormValue("next") == "druck" {
		redirect(w, r, "/befund/%d/druck", b.ID)
		return
	}
	redirect(w, r, "/befund/%d?gespeichert=1", b.ID)
}

func (s *Server) deleteBefund(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pid, err := s.st.DeleteBefund(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/patient/%d", pid)
}

func (s *Server) printBefund(w http.ResponseWriter, r *http.Request) {
	b, p, d, ok := s.loadBefund(w, r)
	if !ok {
		return
	}
	var sessions []store.Session
	if r.URL.Query().Get("verlauf") == "1" {
		var err error
		if sessions, err = s.st.Sessions(r.Context(), p.ID); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.render(w, r, "befund_print", http.StatusOK, map[string]any{
		"B": b, "P": p, "D": d, "Sections": befund.Sections, "Views": befund.Views, "Sessions": sessions, "Today": time.Now(),
	})
}

// Export im eigenen Format (lässt sich wieder importieren).
type exportFile struct {
	Format  string          `json:"format"`
	Patient exportPatient   `json:"patient"`
	Date    string          `json:"date"`
	Title   string          `json:"title"`
	Data    json.RawMessage `json:"data"`
}
type exportPatient struct {
	Name string `json:"name"`
	Dob  string `json:"dob"`
}

const exportFormat = "physiobase-befund-1"

func (s *Server) exportBefund(w http.ResponseWriter, r *http.Request) {
	b, p, _, ok := s.loadBefund(w, r)
	if !ok {
		return
	}
	name := strings.Map(func(c rune) rune {
		if c == ' ' || c == '/' || c == '\\' || c == '"' {
			return '_'
		}
		return c
	}, p.Name)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Befund_%s_%s.json"`, name, b.Date))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(exportFile{Format: exportFormat, Patient: exportPatient{p.Name, p.Dob}, Date: b.Date, Title: b.Title, Data: json.RawMessage(b.Data)}); err != nil {
		s.log.Error("export", "befund", b.ID, "err", err)
	}
}

// ---------- Vergleich ----------

func (s *Server) compareBefunde(w http.ResponseWriter, r *http.Request) {
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
	bs, err := s.st.Befunde(ctx, pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	data := map[string]any{"P": p, "Befunde": bs}
	if len(bs) >= 2 {
		// Standard: ältester (Baseline) gegen neuesten
		aID, bID := bs[len(bs)-1].ID, bs[0].ID
		if v, ok := formID(r, "a"); ok {
			aID = v
		}
		if v, ok := formID(r, "b"); ok {
			bID = v
		}
		var A, B *store.Befund
		for i := range bs {
			if bs[i].ID == aID {
				A = &bs[i]
			}
			if bs[i].ID == bID {
				B = &bs[i]
			}
		}
		if A != nil && B != nil {
			da, _ := befund.Parse(A.Data)
			db, _ := befund.Parse(B.Data)
			fields, tables := befund.Compare(da, db)
			data["A"], data["B"], data["Fields"], data["Tables"] = A, B, fields, tables
		}
	}
	s.render(w, r, "compare", http.StatusOK, data)
}

// ---------- Import alter Befunde ----------

func (s *Server) importBefunde(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil { //nolint:gosec // G120: Größe oben per MaxBytesReader begrenzt
		http.Error(w, "Upload fehlgeschlagen", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	var count int
	var errs []string
	for _, fh := range r.MultipartForm.File["files"] {
		f, err := fh.Open()
		if err != nil {
			errs = append(errs, fh.Filename+": "+err.Error())
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(f, 10<<20))
		_ = f.Close()
		if err != nil {
			errs = append(errs, fh.Filename+": "+err.Error())
			continue
		}
		if err := s.importOne(r, raw); err != nil {
			errs = append(errs, fh.Filename+": "+err.Error())
			continue
		}
		count++
	}
	if len(errs) > 0 {
		s.log.Warn("import teilweise fehlgeschlagen", "fehler", errs)
		ps, _ := s.st.Patients(ctx, "", false)
		s.render(w, r, "patients", http.StatusBadRequest, map[string]any{
			"Patients": ps, "Imported": fmt.Sprint(count), "ImportErrors": errs,
		})
		return
	}
	redirect(w, r, "/patienten?importiert=%d", count)
}

func (s *Server) importOne(r *http.Request, raw []byte) error {
	ctx := r.Context()
	var probe struct {
		Format string `json:"format"`
	}
	_ = json.Unmarshal(raw, &probe) // kein eigenes Format → alte Befundseite

	var name, dob, date, title string
	var data befund.Data
	var sessions []befund.Row
	if probe.Format == exportFormat {
		var ef exportFile
		if err := json.Unmarshal(raw, &ef); err != nil {
			return err
		}
		d, err := befund.Parse(string(ef.Data))
		if err != nil {
			return err
		}
		d.Stamps = befund.CleanStamps(d.Stamps)
		name, dob, date, title, data = ef.Patient.Name, ef.Patient.Dob, ef.Date, ef.Title, d
	} else {
		l, err := befund.ParseLegacy(raw)
		if err != nil {
			return err
		}
		name, dob, date, title, data, sessions = l.Name, l.Dob, l.Date, "Befund (Import)", l.Data, l.Sessions
	}
	if strings.TrimSpace(name) == "" {
		name = "Unbekannt (Import)"
	}
	pid, found, err := s.st.FindPatient(ctx, name, dob)
	if err != nil {
		return err
	}
	if !found {
		if pid, err = s.st.CreatePatient(ctx, store.Patient{Name: name, Dob: dob}); err != nil {
			return err
		}
	}
	if _, err := s.st.CreateBefund(ctx, store.Befund{PatientID: pid, Date: validDate(date), Title: title, Data: data.JSON()}); err != nil {
		return err
	}
	for _, x := range sessions {
		if x["goal"] == "" && x["note"] == "" {
			continue
		}
		if err := s.st.AddSession(ctx, store.Session{PatientID: pid, Date: validDate(x["date"]), Goal: x["goal"], Note: x["note"]}); err != nil {
			return err
		}
	}
	return nil
}
