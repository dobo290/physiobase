package web

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"forge.gpl-nas.de/physiobase/internal/config"
)

func TestAppointmentsAndCalendar(t *testing.T) {
	t.Parallel()
	ts, c := newServerCfg(t, func(cfg *config.Config) {
		cfg.PublicURL = "https://physio.example.org"
		cfg.Address = "Musterstraße 1, 50667 Köln"
	})
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Kai Kalender"}})
	post(c, ts.URL+"/behandler", url.Values{"name": {"Anna Schmidt"}})

	// Serie: 3× wöchentlich, ab morgen
	day := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	r, _ := post(c, ts.URL+"/termine", url.Values{
		"patient": {"1"}, "date": {day}, "time": {"09:30"}, "duration": {"25"},
		"therapist": {"1"}, "title": {"Krankengymnastik"}, "note": {"Handtuch, mitbringen; bitte"}, "repeat": {"3"}, "every": {"7"}, "back": {"patient"},
	})
	if r.Header.Get("Location") != "/patient/1#termine" {
		t.Fatalf("Termin anlegen: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	_, html := body(t, c, ts.URL+"/patient/1")
	if strings.Count(html, "Krankengymnastik · Anna Schmidt") != 3 {
		t.Fatalf("Serie nicht vollständig auf der Patientenseite")
	}
	m := regexp.MustCompile(`https://physio\.example\.org/kalender/([0-9a-f]{48})\.ics`).FindStringSubmatch(html)
	if m == nil || !strings.Contains(html, "webcal://physio.example.org/kalender/") {
		t.Fatal("Kalender-Link fehlt")
	}
	feed := "/kalender/" + m[1] + ".ics"

	// Feed ohne Login abrufbar, korrektes Format, keine Gesundheitsdaten/Namen
	anon := &http.Client{}
	resp, err := anon.Get(ts.URL + feed)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	ics := string(b)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/calendar") {
		t.Fatalf("Feed: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR", "SUMMARY:Krankengymnastik – Test", "Behandlung bei: Anna Schmidt",
		`Hinweis: Handtuch\, mitbringen\; bitte`, `LOCATION:Musterstraße 1\, 50667 Köln`, "STATUS:CONFIRMED",
	} {
		if !strings.Contains(strings.ReplaceAll(ics, "\r\n ", ""), want) {
			t.Errorf("Feed ohne %q", want)
		}
	}
	if strings.Count(ics, "BEGIN:VEVENT") != 3 || strings.Contains(ics, "Kai Kalender") {
		t.Error("Feed: falsche Anzahl Termine oder Patientenname enthalten")
	}

	// Absagen → STATUS:CANCELLED, SEQUENCE steigt
	post(c, ts.URL+"/termin/1/absagen", nil)
	resp, _ = anon.Get(ts.URL + feed)
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "STATUS:CANCELLED") || !strings.Contains(string(b), "SEQUENCE:1") {
		t.Error("Absage nicht im Feed")
	}

	// Falscher Token → 404; Link erneuern macht alten Link ungültig
	if resp, _ := anon.Get(ts.URL + "/kalender/" + strings.Repeat("0", 48) + ".ics"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("falscher Token: %d", resp.StatusCode)
	}
	post(c, ts.URL+"/patient/1/kalender-link", nil)
	if resp, _ := anon.Get(ts.URL + feed); resp.StatusCode != http.StatusNotFound {
		t.Errorf("alter Link nach Erneuern noch gültig: %d", resp.StatusCode)
	}

	// QR-Code, Druckliste, Übersicht
	if code, svg := body(t, c, ts.URL+"/patient/1/kalender-qr.svg"); code != http.StatusOK || !strings.HasPrefix(svg, "<svg") {
		t.Errorf("QR: %d", code)
	}
	if _, pr := body(t, c, ts.URL+"/patient/1/termine/druck"); !strings.Contains(pr, "Ihre Termine") || strings.Count(pr, "Anna Schmidt") < 3 {
		t.Error("Druckliste unvollständig")
	}
	if _, ov := body(t, c, ts.URL+"/termine"); !strings.Contains(ov, "Kai Kalender") || !strings.Contains(ov, "abgesagt") {
		t.Error("Übersicht unvollständig")
	}

	// Behandlungszusammenfassung nennt den nächsten Termin
	post(c, ts.URL+"/patient/1/verlauf", url.Values{"date": {time.Now().Format("2006-01-02")}, "goal": {"Mobilisation"}})
	r, _ = post(c, ts.URL+"/verlauf/1/zusammenfassung", nil)
	if _, sum := body(t, c, ts.URL+r.Header.Get("Location")); !strings.Contains(sum, "um 09:30 Uhr bei Anna Schmidt") {
		t.Error("nächster Termin fehlt in der Zusammenfassung")
	}
}
