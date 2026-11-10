package web

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func uploadLogo(t *testing.T, c *http.Client, base, name, content string) string {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("logo", name)
	io.WriteString(fw, content)
	w.Close()
	req, _ := http.NewRequest(http.MethodPost, base+"/einstellungen/logo", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	r, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	return r.Header.Get("Location")
}

func TestSettingsTextsAndLogo(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)

	if code, html := body(t, c, ts.URL+"/einstellungen"); code != http.StatusOK || !strings.Contains(html, "Terminliste – Hinweis unten") {
		t.Fatalf("Einstellungsseite: %d", code)
	}
	// Praxisname und eigener Hinweistext mit Platzhalter
	post(c, ts.URL+"/einstellungen", url.Values{
		"t.practice_name":             {"Praxis Sonnenschein"},
		"t.print_appointments_footer": {"Absagen bitte 48 h vorher – {praxis}"},
		"t.appt_default_time":         {"kaputt"},
	})
	_, html := body(t, c, ts.URL+"/")
	if !strings.Contains(html, "Praxis Sonnenschein") || strings.Contains(html, "<strong>PhysioBase</strong>") {
		t.Error("Praxisname nicht übernommen")
	}
	post(c, ts.URL+"/patienten", url.Values{"name": {"Lea Liste"}})
	_, pr := body(t, c, ts.URL+"/patient/1/termine/druck")
	if !strings.Contains(pr, "Absagen bitte 48 h vorher – Praxis Sonnenschein") {
		t.Error("eigener Hinweis nicht auf der Terminliste")
	}
	// ungültige Uhrzeit → Standard bleibt
	if _, set := body(t, c, ts.URL+"/einstellungen"); !strings.Contains(set, `value="08:00"`) {
		t.Error("ungültige Standard-Uhrzeit übernommen")
	}
	// Standard wiederherstellen
	post(c, ts.URL+"/einstellungen", url.Values{"t.print_appointments_footer": {"egal"}, "reset.print_appointments_footer": {"1"}})
	if _, pr := body(t, c, ts.URL+"/patient/1/termine/druck"); !strings.Contains(pr, "mindestens 24 Stunden vorher") {
		t.Error("Standardtext nicht wiederhergestellt")
	}

	// SVG mit Skript wird abgelehnt, sauberes SVG übernommen und ausgeliefert
	if loc := uploadLogo(t, c, ts.URL, "boese.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`); !strings.Contains(loc, "fehler=") {
		t.Errorf("SVG mit Skript angenommen: %s", loc)
	}
	if loc := uploadLogo(t, c, ts.URL, "boese2.svg", `<svg xmlns="http://www.w3.org/2000/svg"><rect onload="x()"/></svg>`); !strings.Contains(loc, "fehler=") {
		t.Errorf("SVG mit Event-Handler angenommen: %s", loc)
	}
	if loc := uploadLogo(t, c, ts.URL, "logo.svg", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" fill="#16a085"/></svg>`); !strings.Contains(loc, "gespeichert=1") {
		t.Fatalf("sauberes SVG abgelehnt: %s", loc)
	}
	_, html = body(t, c, ts.URL+"/")
	i := strings.Index(html, `src="/branding/`)
	if i < 0 {
		t.Fatal("eigenes Logo nicht in der Kopfzeile")
	}
	src := html[i+5 : i+5+strings.IndexByte(html[i+5:], '"')]
	anon := &http.Client{}
	r, err := anon.Get(ts.URL + strings.ReplaceAll(src, "&amp;", "&"))
	if err != nil || r.StatusCode != http.StatusOK || r.Header.Get("Content-Type") != "image/svg+xml" ||
		!strings.Contains(r.Header.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("Logo-Auslieferung: %v %d %s", err, r.StatusCode, r.Header.Get("Content-Type"))
	}
	if r != nil {
		r.Body.Close()
	}
	if code, _ := body(t, c, ts.URL+"/branding/irgendwas.svg"); code != http.StatusNotFound {
		t.Errorf("fremde Datei unter /branding: %d", code)
	}
}

func TestBulkAppointments(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Ben Bulk"}})
	post(c, ts.URL+"/behandler", url.Values{"name": {"Tom"}})
	d := func(days int) string { return time.Now().AddDate(0, 0, days).Format("2006-01-02") }
	v := url.Values{
		"patient": {"1"}, "back": {"patient"}, "location": {"Hausbesuch"}, "t.appt.n": {"4"},
		"t.appt.0.date": {d(1)}, "t.appt.0.time": {"09:00"}, "t.appt.0.duration": {"25"}, "t.appt.0.therapist": {"1"}, "t.appt.0.title": {"KG"},
		"t.appt.1.date": {d(8)}, "t.appt.1.time": {"09:00"}, "t.appt.1.duration": {"25"}, "t.appt.1.therapist": {"1"}, "t.appt.1.title": {"KG"},
		"t.appt.2.date": {""}, "t.appt.2.time": {"09:00"}, // leer → ignoriert
		"t.appt.3.date": {d(15)}, "t.appt.3.time": {"10:30"}, "t.appt.3.title": {"MT"},
	}
	r, _ := post(c, ts.URL+"/termine", v)
	if r.Header.Get("Location") != "/patient/1#termine" {
		t.Fatalf("Sammel-Eingabe: %s", r.Header.Get("Location"))
	}
	_, html := body(t, c, ts.URL+"/patient/1")
	if n := strings.Count(html, "KG · Tom") + strings.Count(html, ">MT<") + strings.Count(html, "MT</span>"); strings.Count(html, "KG · Tom") != 2 || !strings.Contains(html, "10:30") || n < 2 {
		t.Errorf("Sammel-Eingabe unvollständig (KG·Tom=%d)", strings.Count(html, "KG · Tom"))
	}
	if !strings.Contains(html, `name="t.appt.2.date"`) || !strings.Contains(html, "+ Zeile") {
		t.Error("Sammel-Formular fehlt auf der Patientenseite")
	}
	// fehlerhafte Zeile → klare Meldung, nichts gespeichert
	v2 := url.Values{
		"patient": {"1"}, "back": {"patient"}, "t.appt.n": {"2"},
		"t.appt.0.date": {d(20)}, "t.appt.0.time": {"16:45"}, "t.appt.1.date": {d(27)}, "t.appt.1.time": {""},
	}
	r, _ = post(c, ts.URL+"/termine", v2)
	if !strings.Contains(r.Header.Get("Location"), "fehler=") {
		t.Fatal("fehlerhafte Zeile nicht gemeldet")
	}
	if _, html := body(t, c, ts.URL+"/patient/1"); strings.Contains(html, "16:45") {
		t.Error("bei Fehler wurde trotzdem gespeichert")
	}
}
