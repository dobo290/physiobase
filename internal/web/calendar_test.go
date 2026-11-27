package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/store"
)

func makeTherapist(t *testing.T, c *http.Client, base, name string) {
	t.Helper()
	if r, _ := post(c, base+"/behandler", url.Values{"name": {name}}); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Behandler anlegen: %d", r.StatusCode)
	}
}

func makePatient(t *testing.T, c *http.Client, base, name string) int64 {
	t.Helper()
	r, _ := post(c, base+"/patienten", url.Values{"name": {name}})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Patient anlegen: %d", r.StatusCode)
	}
	return 1
}

func makeAppointment(t *testing.T, c *http.Client, base, date, time, patient, therapist, title string) {
	t.Helper()
	r, _ := post(c, base+"/termine", url.Values{
		"patient": {patient}, "t.appt.n": {"1"},
		"t.appt.0.date": {date}, "t.appt.0.time": {time}, "t.appt.0.duration": {"30"},
		"t.appt.0.therapist": {therapist}, "t.appt.0.title": {title}, "t.appt.0.note": {""},
	})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Termin anlegen: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
}

func TestCalendarPage(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	makeTherapist(t, c, ts.URL, "Anna Schmidt")
	makePatient(t, c, ts.URL, "Hans Müller")
	makeAppointment(t, c, ts.URL, "2026-03-02", "09:00", "1", "1", "KG")

	code, html := body(t, c, ts.URL+"/kalender?tag=2026-03-02")
	if code != http.StatusOK {
		t.Fatalf("Kalender: %d", code)
	}
	for _, want := range []string{"cal-svg", "Anna Schmidt", "Müller H.", "data-cal-grid", "cal-gutter-hl", "08:20"} {
		if !strings.Contains(html, want) {
			t.Errorf("Kalender ohne %q", want)
		}
	}
	// Wochenansicht
	if code, html := body(t, c, ts.URL+"/kalender?ansicht=woche&tag=2026-03-02"); code != http.StatusOK || !strings.Contains(html, "cal-week") {
		t.Errorf("Wochenansicht: %d", code)
	}
	// Druckansicht
	if code, html := body(t, c, ts.URL+"/kalender/druck?tag=2026-03-02"); code != http.StatusOK || !strings.Contains(html, "Tagesplan") {
		t.Errorf("Druckansicht: %d", code)
	}
	// Dashboard zeigt den Tagesplan
	if code, html := body(t, c, ts.URL+"/"); code != http.StatusOK || !strings.Contains(html, "cal-svg") {
		t.Errorf("Dashboard: %d", code)
	}
}

func TestAppointmentConflicts(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	makeTherapist(t, c, ts.URL, "Anna Schmidt")
	makePatient(t, c, ts.URL, "Hans Müller")
	makeAppointment(t, c, ts.URL, "2026-03-03", "09:00", "1", "1", "KG")
	makeAppointment(t, c, ts.URL, "2026-03-03", "09:15", "1", "1", "MT")

	// Überschneidung ist im Kalender markiert
	_, html := body(t, c, ts.URL+"/kalender?tag=2026-03-03")
	if !strings.Contains(html, "cal-conflict") {
		t.Error("Überschneidung nicht markiert")
	}
	// Der Editor zeigt den Konflikt und verlangt eine Bestätigung; das Häkchen
	// muss denselben Feldnamen nutzen wie saveAppointment (bestaetigen).
	if code, html := body(t, c, ts.URL+"/kalender?tag=2026-03-03&termin=2"); code != http.StatusOK || !strings.Contains(html, "Überschneidung") || !strings.Contains(html, `name="bestaetigen"`) {
		t.Errorf("Editor ohne Konflikthinweis/Bestätigung: %d", code)
	}
}

func TestMoveAppointment(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	makeTherapist(t, c, ts.URL, "Anna Schmidt")
	makePatient(t, c, ts.URL, "Hans Müller")
	makeAppointment(t, c, ts.URL, "2026-03-04", "09:00", "1", "1", "KG")
	makeAppointment(t, c, ts.URL, "2026-03-04", "10:00", "1", "1", "MT")

	// Auf einen freien Platz verschieben
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/termin/1/verschieben", strings.NewReader("date=2026-03-04&time=11:00&duration=45&therapist=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Verschieben: %v %d", err, res.StatusCode)
	}
	res.Body.Close()

	// Auf einen belegten Platz: Konflikt, dann mit Bestätigung
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/termin/1/verschieben", strings.NewReader("date=2026-03-04&time=10:15&duration=30&therapist=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("Konflikt nicht erkannt: %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("Antwort kein JSON: %v", err)
	}
	res.Body.Close()
	if cs, _ := body["conflicts"].([]any); len(cs) == 0 {
		t.Error("keine Konflikte in der Antwort")
	}

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/termin/1/verschieben", strings.NewReader("date=2026-03-04&time=10:15&duration=30&therapist=1&bestaetigt=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err = c.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Bestätigtes Verschieben: %v %d", err, res.StatusCode)
	}
	res.Body.Close()

	// Formularweg (ohne JSON): Weiterleitung zurück in den Kalender
	r, _ := post(c, ts.URL+"/termin/2/verschieben", url.Values{"date": {"2026-03-04"}, "time": {"12:00"}, "duration": {"30"}, "therapist": {"1"}})
	if r.StatusCode != http.StatusSeeOther || !strings.Contains(r.Header.Get("Location"), "/kalender") {
		t.Errorf("Formularweg: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
}

func TestSaveAppointmentConflict(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	makeTherapist(t, c, ts.URL, "Anna Schmidt")
	makePatient(t, c, ts.URL, "Hans Müller")
	makeAppointment(t, c, ts.URL, "2026-03-05", "09:00", "1", "1", "KG")
	makeAppointment(t, c, ts.URL, "2026-03-05", "10:00", "1", "1", "MT")

	// Termin 2 auf 09:15 ziehen: Konflikt, Speichern verlangt Bestätigung
	r, _ := post(c, ts.URL+"/termin/2", url.Values{
		"date": {"2026-03-05"}, "time": {"09:15"}, "duration": {"30"}, "therapist": {"1"}, "title": {"MT"},
	})
	if r.StatusCode != http.StatusConflict {
		t.Fatalf("Konflikt beim Speichern: %d", r.StatusCode)
	}
	r, _ = post(c, ts.URL+"/termin/2", url.Values{
		"date": {"2026-03-05"}, "time": {"09:15"}, "duration": {"30"}, "therapist": {"1"}, "title": {"MT"}, "bestaetigen": {"1"},
	})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("bestätigtes Speichern: %d", r.StatusCode)
	}
}

func TestSidebarToggle(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	r, _ := post(c, ts.URL+"/sidebar/umschalten", url.Values{"back": {"/"}})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Seitenleiste umschalten: %d", r.StatusCode)
	}
	_, html := body(t, c, ts.URL+"/")
	if !strings.Contains(html, "sidebar-collapsed") {
		t.Error("Seitenleiste nicht eingeklappt")
	}
}

func TestDashboard(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	makeTherapist(t, c, ts.URL, "Anna Schmidt")
	makePatient(t, c, ts.URL, "Hans Müller")
	makeAppointment(t, c, ts.URL, "2026-03-02", "09:00", "1", "1", "KG")

	// Startseite ist die Übersicht mit Tagesplan, nicht mehr die Wissensliste.
	code, html := body(t, c, ts.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("Übersicht: %d", code)
	}
	for _, want := range []string{"Tagesplan", "Zuletzt gesehen", "Wissen zuletzt geändert", "cal-svg"} {
		if !strings.Contains(html, want) {
			t.Errorf("Übersicht ohne %q", want)
		}
	}
	// Alte Links auf die Wissensliste werden weitergeleitet.
	if code, _ := body(t, c, ts.URL+"/?q=knie"); code != http.StatusSeeOther {
		t.Errorf("alter Suchlink: %d", code)
	}
	if code, html := body(t, c, ts.URL+"/wissen"); code != http.StatusOK || !strings.Contains(html, "Wissen") {
		t.Errorf("Wissensliste: %d", code)
	}
}

// shortName kürzt auf Nachname und Initiale, blockLines lässt die Uhrzeit weg.
func TestShortNameAndBlockLines(t *testing.T) {
	t.Parallel()
	names := map[string]string{
		"":                   "",
		"Hans":               "Hans",
		"Hans Müller":        "Müller H.",
		"Anna Maria Schmidt": "Schmidt A.",
		"  hans   müller  ":  "Müller H.",
	}
	for in, want := range names {
		if got := shortName(in); got != want {
			t.Errorf("shortName(%q) = %q, erwartet %q", in, got, want)
		}
	}

	a := store.Appointment{PatientName: "Hans Müller", Title: "KG", TherapistName: "Anna Schmidt"}
	if got := blockLines(a, 20); len(got) != 1 || got[0] != "Müller H." {
		t.Errorf("20 min: %v, erwartet nur den Namen", got)
	}
	if got := blockLines(a, 40); len(got) != 2 || got[1] != "KG" {
		t.Errorf("40 min: %v, erwartet Name und Art", got)
	}
	if got := blockLines(a, 60); len(got) != 3 || got[2] != "Anna Schmidt" {
		t.Errorf("60 min: %v, erwartet Name, Art und Behandler", got)
	}
	if got := blockLines(store.Appointment{}, 20); len(got) != 1 || got[0] != "Termin" {
		t.Errorf("leerer Termin: %v", got)
	}
}
