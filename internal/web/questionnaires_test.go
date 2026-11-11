package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func entryID(t *testing.T, loc string) int64 {
	t.Helper()
	m := regexp.MustCompile(`/eintrag/([0-9]+)`).FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("keine Eintrags-ID in %q", loc)
	}
	id, _ := strconv.ParseInt(m[1], 10, 64)
	return id
}

func TestQuestionnaireFlow(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)

	// 1) Fragebogen-Test anlegen
	r, _ := post(c, ts.URL+"/eintrag/neu", url.Values{
		"kind": {"test"}, "title": {"KNEE-Q Beispiel"}, "fragebogen_an": {"1"},
		"fragebogen_richtung": {"lower"}, "d.n": {"3"},
		"d.0.q": {"Schmerzen beim Treppensteigen"}, "d.0.max": {"10"},
		"d.1.q": {"Schmerzen beim Gehen"}, "d.1.max": {"10"},
		"d.2.q": {"Schmerzen beim Hocken"}, "d.2.max": {"10"},
	})
	qid := entryID(t, r.Header.Get("Location"))
	qidS := strconv.FormatInt(qid, 10)
	if _, html := body(t, c, ts.URL+"/eintrag/"+qidS); !strings.Contains(html, "Fragebogen (Punkteskala)") {
		t.Error("Fragebogen-Vorschau fehlt")
	}

	// 2) Patient + Befund, Fragebogen ausfüllen
	post(c, ts.URL+"/patienten", url.Values{"name": {"Lena Frage"}})
	post(c, ts.URL+"/patient/1/befund", url.Values{})
	post(c, ts.URL+"/befund/1", url.Values{
		"title": {"Erstbefund"}, "date": {"2026-11-01"}, "f.diag": {"Patellabeschwerden"},
		"q.list": {qidS + ","}, "f.q." + qidS + ".n": {"3"},
		"f.q." + qidS + ".0": {"2"}, "f.q." + qidS + ".1": {"4"}, "f.q." + qidS + ".2": {"3"},
		"f.q." + qidS + ".note": {"mit Gehstütze"},
	})
	_, html := body(t, c, ts.URL+"/befund/1")
	if !strings.Contains(html, "9 von 30") || !strings.Contains(html, "niedriger ist besser") {
		t.Error("Befund-Formular ohne Summe")
	}
	// Patient-Übersicht zeigt Verlauf
	_, html = body(t, c, ts.URL+"/patient/1")
	if !strings.Contains(html, "KNEE-Q Beispiel") || !strings.Contains(html, "9 von 30") || !strings.Contains(html, "mit Gehstütze") {
		t.Error("Patientenübersicht ohne Fragebogen-Verlauf")
	}
	// Druck: Zusammenfassung + Link, nicht die Fragenliste
	if _, pr := body(t, c, ts.URL+"/befund/1/druck"); !strings.Contains(pr, "Fragebögen (Zusammenfassung)") ||
		!strings.Contains(pr, "9 von 30") || !strings.Contains(pr, "Fragebogen ansehen") || strings.Contains(pr, "Schmerzen beim Treppensteigen") {
		t.Error("Druckansicht Fragebogen falsch")
	}

	// 3) Wiederbefund als Kopie: zeigt vorherigen Wert
	post(c, ts.URL+"/patient/1/befund", url.Values{"kopie": {"1"}})
	if _, html := body(t, c, ts.URL+"/befund/2"); !strings.Contains(html, "Letzter Wert: 9 von 30") {
		t.Error("Wiederbefund ohne vorherigen Wert")
	}
	// Antworten ändern (unvollständig → Flag)
	post(c, ts.URL+"/befund/2", url.Values{
		"title": {"Wiederbefund"}, "date": {"2026-12-01"}, "f.diag": {"Patellabeschwerden"},
		"q.list": {qidS + ","}, "f.q." + qidS + ".n": {"3"},
		"f.q." + qidS + ".0": {"1"}, "f.q." + qidS + ".1": {"2"}, "f.q." + qidS + ".2": {""},
	})
	if _, html := body(t, c, ts.URL+"/patient/1"); !strings.Contains(html, "unvollständig") {
		t.Error("unvollständiger Fragebogen nicht markiert")
	}
	// Vergleich enthält Fragebogen
	if _, html := body(t, c, ts.URL+"/patient/1/vergleich"); !strings.Contains(html, "KNEE-Q Beispiel") || !strings.Contains(html, "9 von 30") {
		t.Error("Vergleich ohne Fragebogen")
	}
	// Nicht-Fragebogen-Test (ohne Häkchen) bekommt kein Schema
	r, _ = post(c, ts.URL+"/eintrag/neu", url.Values{"kind": {"test"}, "title": {"Lachman (ohne Fragebogen)"}})
	if _, html := body(t, c, ts.URL+"/eintrag/"+strconv.FormatInt(entryID(t, r.Header.Get("Location")), 10)); strings.Contains(html, "Fragebogen (Punkteskala)") {
		t.Error("Test ohne Fragebogen zeigt Vorschau")
	}
	// Fehlerhafte Definition (0 Punkte) wird abgelehnt (Seite rendert mit Fehler, kein Redirect)
	r, _ = post(c, ts.URL+"/eintrag/neu", url.Values{
		"kind": {"test"}, "title": {"Kaputt"}, "fragebogen_an": {"1"},
		"fragebogen_richtung": {"lower"}, "d.n": {"1"}, "d.0.q": {"Frage"}, "d.0.max": {"0"},
	})
	if r.StatusCode != http.StatusBadRequest || r.Header.Get("Location") != "" {
		t.Errorf("Fragebogen mit max 0: rc=%d", r.StatusCode)
	}
}
