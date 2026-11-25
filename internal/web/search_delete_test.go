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

// TestDeleteFromListAndAppointments: Die Wissensliste bleibt ohne Lösch-Kreuz,
// die Terminübersicht bietet das Löschen je Zeile an.
func TestDeleteFromListAndAppointments(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)

	_, index := body(t, c, ts.URL+"/?art=test")
	if strings.Contains(index, "card-delete") {
		t.Error("Lösch-Kreuz ist wieder in der Wissensliste")
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

// TestStaticAssetVersion: Skripte und Styles tragen die Versionsnummer, damit
// ein neuer Stand nicht aus dem Browsercache läuft.
func TestStaticAssetVersion(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Cache Test"}})

	for _, path := range []string{"/", "/patient/1"} {
		_, page := body(t, c, ts.URL+path)
		for _, want := range []string{"/static/style.css?v=", "/static/befund.js?v="} {
			if !strings.Contains(page, want) {
				t.Errorf("%s: %q fehlt", path, want)
			}
		}
	}
	// Die Testsuche im Befund muss wirklich ausgeliefert werden (stand früher
	// zwischen den define-Blöcken und kam im Browser nie an).
	post(c, ts.URL+"/patient/1/befund", url.Values{})
	_, befund := body(t, c, ts.URL+"/befund/1")
	if !strings.Contains(befund, "/static/test-search.js?v=") {
		t.Error("Testsuche-Skript fehlt im Befund")
	}
	if n := strings.Count(befund, "test-search.js"); n != 1 {
		t.Errorf("Testsuche-Skript %d-mal eingebunden, erwartet 1", n)
	}
	if _, patient := body(t, c, ts.URL+"/patient/1"); !strings.Contains(patient, "/static/test-search.js?v=") {
		t.Error("Testsuche-Skript fehlt in der Patientenübersicht")
	}
}

// TestRemovePatientTest: Test samt Durchführungen vom Patienten entfernen,
// Ergebnisse im Befund bleiben.
func TestRemovePatientTest(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Entfern Test"}})

	hits := apiTests(t, c, ts.URL, "q=Fragebogen")
	if len(hits) == 0 {
		t.Fatal("kein Test vorhanden")
	}
	eid := strconv.FormatInt(hits[0].ID, 10)
	if r, _ := post(c, ts.URL+"/patient/1/test", url.Values{"test_id": {eid}}); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("verknüpfen: %d", r.StatusCode)
	}
	if r, _ := post(c, ts.URL+"/patient/1/test/"+eid, url.Values{"date": {"2026-04-01"}, "v.0": {"3"}}); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Durchführung: %d", r.StatusCode)
	}

	// ✕-Link und Bestätigungsblock
	_, page := body(t, c, ts.URL+"/patient/1")
	if !strings.Contains(page, "?entfernen="+eid) {
		t.Fatal("Entfernen-Link fehlt")
	}
	_, confirm := body(t, c, ts.URL+"/patient/1?entfernen="+eid)
	if !strings.Contains(confirm, "/patient/1/test/"+eid+"/entfernen") {
		t.Fatal("Bestätigungsformular fehlt")
	}

	if r, _ := post(c, ts.URL+"/patient/1/test/"+eid+"/entfernen", nil); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("entfernen: %d", r.StatusCode)
	}
	_, after := body(t, c, ts.URL+"/patient/1")
	if strings.Contains(after, "/patient/1/test/"+eid) {
		t.Error("Test steht noch am Patienten")
	}
	if strings.Contains(after, "?entfernen="+eid) {
		t.Error("Entfernen-Link noch vorhanden")
	}
	// Der Test selbst bleibt in der Wissensdatenbank.
	if code, _ := body(t, c, ts.URL+"/eintrag/"+eid); code != http.StatusOK {
		t.Errorf("Eintrag wurde mitgelöscht: %d", code)
	}
}
