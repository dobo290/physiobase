package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

func lowerDef() quizEntry {
	return quizEntry{ID: 5, Title: "Schmerzskala", Q: befund.Questionnaire{
		Direction: "lower",
		Items:     []befund.QItem{{Q: "Schmerz", Max: 10}},
	}}
}

func TestBuildTestSeries(t *testing.T) {
	t.Parallel()
	def := lowerDef()
	runs := []store.TestRun{ // Store liefert neueste zuerst
		{EntryID: 5, Date: "2026-03-01", Data: `{"values":["4"]}`},
		{EntryID: 5, Date: "2026-01-15", Data: `{"values":["8"]}`},
	}
	bs := []store.Befund{ // neueste zuerst
		{ID: 2, Date: "2026-02-01", Data: `{"q":{"5":{"values":["6"]}}}`},
	}
	s := buildTestSeries(5, "Schmerzskala", def, runs, bs)
	if !s.Has || len(s.Points) != 3 {
		t.Fatalf("erwartet 3 Punkte, bekommen %d (%+v)", len(s.Points), s.Points)
	}
	if s.Points[0].Date != "2026-01-15" || s.Points[2].Date != "2026-03-01" {
		t.Fatalf("nicht chronologisch sortiert: %+v", s.Points)
	}
	if s.Last != 4 || s.Prev != 6 || s.Delta != -2 {
		t.Fatalf("Trend falsch: last=%d prev=%d delta=%d", s.Last, s.Prev, s.Delta)
	}
	if s.Trend != "down" || !s.Better || !s.Relevant {
		t.Fatalf("Richtung/Relevanz falsch: %+v", s)
	}
	if s.Points[1].Source != "Befund" || s.Points[1].BefundID != 2 {
		t.Fatalf("Quelle aus dem Befund fehlt: %+v", s.Points[1])
	}
	if got := trendLabel(s); !strings.Contains(got, "-2") || !strings.Contains(got, "besser") {
		t.Fatalf("Trendtext unerwartet: %q", got)
	}
	if trendClass(s) != "trend-good" {
		t.Fatalf("Trendklasse unerwartet: %q", trendClass(s))
	}
}

func TestBuildTestSeriesDedupesSameDay(t *testing.T) {
	t.Parallel()
	def := lowerDef()
	s := buildTestSeries(5, "Schmerzskala", def,
		[]store.TestRun{{EntryID: 5, Date: "2026-01-15", Data: `{"values":["7"]}`}},
		[]store.Befund{{ID: 9, Date: "2026-01-15", Data: `{"q":{"5":{"values":["7"]}}}`}})
	if len(s.Points) != 1 {
		t.Fatalf("gleicher Tag und Wert doppelt gezählt: %+v", s.Points)
	}
	if s.Prev != 0 || s.Delta != 0 {
		t.Fatalf("einzelne Messung darf keinen Trend haben: %+v", s)
	}
}

func TestChartSVG(t *testing.T) {
	t.Parallel()
	def := lowerDef()
	s := buildTestSeries(5, "Schmerzskala", def, []store.TestRun{
		{EntryID: 5, Date: "2026-01-15", Data: `{"values":["8"]}`},
		{EntryID: 5, Date: "2026-03-01", Data: `{"values":["4"]}`},
	}, nil)
	svg := string(chartSVG(s, 200, 60, false))
	for _, want := range []string{"<svg", "<path", "<circle", "15.01.26", "01.03.26"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG ohne %q: %s", want, svg)
		}
	}
	// Kein Benutzertext (Titel) im Diagramm – nur Zahlen und feste Beschriftungen.
	if strings.Contains(svg, "Schmerzskala") {
		t.Error("Testtitel darf nicht im SVG landen")
	}
	// Leerer Verlauf ergibt kein SVG.
	if got := chartSVG(testSeries{}, 100, 40, true); got != "" {
		t.Errorf("leerer Verlauf sollte kein SVG liefern, bekam %q", got)
	}
}

func TestTestHistoryPage(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", urlValuesName("Verlauf Test"))
	// Punkt-Fragebogen anlegen (Test mit Schema)
	r, _ := post(c, ts.URL+"/eintrag/neu", urlValues(
		"kind", "test", "title", "Schmerzskala Knie", "fragebogen_an", "1",
		"fragebogen_richtung", "lower", "d.n", "1", "d.0.q", "Schmerz beim Gehen", "d.0.max", "10",
	))
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Test anlegen: %d", r.StatusCode)
	}
	loc := r.Header.Get("Location")
	eid := strings.TrimPrefix(loc, "/eintrag/")
	if eid == "" {
		t.Fatalf("unerwarteter Ort: %s", loc)
	}
	// Dem Patienten zuordnen und zwei Messungen speichern
	if r, _ := post(c, ts.URL+"/patient/1/test", urlValues("test_id", eid)); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Test zuordnen: %d", r.StatusCode)
	}
	for _, m := range []struct{ date, v string }{{"2026-01-10", "8"}, {"2026-02-10", "5"}} {
		if r, _ := post(c, ts.URL+"/patient/1/test/"+eid, urlValues("date", m.date, "v.0", m.v)); r.StatusCode != http.StatusSeeOther {
			t.Fatalf("Messung speichern: %d", r.StatusCode)
		}
	}
	code, html := body(t, c, ts.URL+"/patient/1")
	if code != http.StatusOK || !strings.Contains(html, "Schmerzskala Knie") || !strings.Contains(html, "/verlauf") {
		t.Fatalf("Patientenseite ohne Verlaufshinweis (%d)", code)
	}
	code, html = body(t, c, ts.URL+"/patient/1/test/"+eid+"/verlauf")
	if code != http.StatusOK {
		t.Fatalf("Verlaufsseite: %d", code)
	}
	for _, want := range []string{"Schmerzskala Knie im Verlauf", "<svg", "10.01.2026", "10.02.2026", "besser"} {
		if !strings.Contains(html, want) {
			t.Errorf("Verlaufsseite ohne %q", want)
		}
	}
}

// Kleine Helfer für die Tests oben (bewusst lokal, damit die Testdatei allein lesbar bleibt).
func urlValues(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

func urlValuesName(name string) url.Values { return url.Values{"name": {name}} }
