package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type apiTest struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// apiTests ruft die Testvorschläge ab.
func apiTests(t *testing.T, c *http.Client, base, query string) []apiTest {
	t.Helper()
	res, err := c.Get(base + "/api/tests?" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s: Status %d", query, res.StatusCode)
	}
	var out []apiTest
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestAPITestsListAndFilter: ohne Suchbegriff kommen Tests (Liste beim Klick ins
// Feld), mit Suchbegriff wird gefiltert, limit wird beachtet.
func TestAPITestsListAndFilter(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)

	all := apiTests(t, c, ts.URL, "limit=30")
	if len(all) == 0 {
		t.Fatal("leere Liste ohne Suchbegriff")
	}
	// alphabetisch sortiert
	for i := 1; i < len(all); i++ {
		if strings.ToLower(all[i].Title) < strings.ToLower(all[i-1].Title) {
			t.Errorf("nicht sortiert: %q vor %q", all[i-1].Title, all[i].Title)
			break
		}
	}

	filtered := apiTests(t, c, ts.URL, "q=Fragebogen")
	if len(filtered) == 0 || len(filtered) > len(all) {
		t.Fatalf("gefiltert = %d von %d", len(filtered), len(all))
	}
	for _, it := range filtered {
		if !strings.Contains(strings.ToLower(it.Title), "fragebogen") {
			t.Errorf("%q passt nicht zum Suchbegriff", it.Title)
		}
	}

	small := apiTests(t, c, ts.URL, "limit=2")
	if len(small) > 2 {
		t.Errorf("limit ignoriert: %d Treffer", len(small))
	}

	// Unsinniges limit fällt auf den Standard zurück (kein Fehler, kein Absturz).
	if got := apiTests(t, c, ts.URL, "limit=abc&q=Frage"); len(got) == 0 {
		t.Error("ungültiges limit liefert keine Treffer")
	}
}

// TestDeleteEntryCascade: ein Test am Patienten lässt sich löschen; Verknüpfung,
// Durchführungen und Antworten im Befund verschwinden mit.
func TestDeleteEntryCascade(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Lösch Test"}})

	hits := apiTests(t, c, ts.URL, "q=Fragebogen")
	if len(hits) == 0 {
		t.Fatal("kein Fragebogen-Test vorhanden")
	}
	eid := hits[0].ID
	eidStr := strconv.FormatInt(eid, 10)

	// verknüpfen und eine Durchführung speichern
	if r, _ := post(c, ts.URL+"/patient/1/test", url.Values{"test_id": {eidStr}}); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("verknüpfen: %d", r.StatusCode)
	}
	if r, _ := post(c, ts.URL+"/patient/1/test/"+eidStr, url.Values{"date": {"2026-03-01"}, "v.0": {"4"}}); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Durchführung: %d", r.StatusCode)
	}

	// Löschen wird auf der Eintragsseite angeboten, mit ?loeschen=1 aufgeklappt
	_, page := body(t, c, ts.URL+"/eintrag/"+eidStr+"?loeschen=1")
	if !strings.Contains(page, `action="/eintrag/`+eidStr+`/loeschen"`) {
		t.Error("Löschformular fehlt")
	}
	if !strings.Contains(page, "danger\" id=\"loeschen\" open") {
		t.Error("Löschblock nicht aufgeklappt")
	}

	if r, _ := post(c, ts.URL+"/eintrag/"+eidStr+"/loeschen", nil); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("löschen: %d", r.StatusCode)
	}
	if code, _ := body(t, c, ts.URL+"/eintrag/"+eidStr); code != http.StatusNotFound {
		t.Errorf("Eintrag nach dem Löschen noch da: %d", code)
	}
	_, pat := body(t, c, ts.URL+"/patient/1")
	if strings.Contains(pat, hits[0].Title) {
		t.Error("gelöschter Test steht noch am Patienten")
	}
	if code, _ := body(t, c, ts.URL+"/patient/1/test/"+eidStr); code != http.StatusNotFound {
		t.Errorf("Testformular nach dem Löschen: %d", code)
	}
}

// TestDeleteFromListAndAppointments: Liste verlinkt das Löschen, Terminübersicht
// bietet es je Zeile an und der Löschblock ist geöffnet.
func TestDeleteFromListAndAppointments(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)

	_, index := body(t, c, ts.URL+"/?art=test")
	if !strings.Contains(index, "?loeschen=1#loeschen") {
		t.Error("Lösch-Link an den Karten fehlt")
	}

	// Patient und Termin anlegen (Sammel-Eingabe: t.appt.<i>.*)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Termin Test"}})
	if r, _ := post(c, ts.URL+"/termine", url.Values{
		"patient": {"1"}, "t.appt.n": {"1"},
		"t.appt.0.date": {"2030-05-01"}, "t.appt.0.time": {"09:00"}, "t.appt.0.duration": {"45"},
	}); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Termin anlegen: %d", r.StatusCode)
	}
	_, list := body(t, c, ts.URL+"/termine?von=2030-05-01")
	if !strings.Contains(list, "?loeschen=1#loeschen") {
		t.Fatal("Lösch-Link in der Terminübersicht fehlt")
	}
	m := regexp.MustCompile(`/termin/([0-9]+)\?loeschen=1`).FindStringSubmatch(list)
	if m == nil {
		t.Fatal("Termin-ID nicht gefunden")
	}
	id := m[1]
	_, page := body(t, c, ts.URL+"/termin/"+id+"?loeschen=1")
	if !strings.Contains(page, "danger\" id=\"loeschen\" open") {
		t.Error("Löschblock am Termin nicht aufgeklappt")
	}
	if r, _ := post(c, ts.URL+"/termin/"+id+"/loeschen", nil); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Termin löschen: %d", r.StatusCode)
	}
	// Der Patient steht weiter im Formular "Neue Termine", der Termin selbst nicht mehr.
	if _, after := body(t, c, ts.URL+"/termine?von=2030-05-01"); strings.Contains(after, "/termin/"+id) {
		t.Error("gelöschter Termin steht noch in der Übersicht")
	}
}
