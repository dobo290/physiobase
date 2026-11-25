package web

import (
	"testing"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// Die reinen Auswertungsfunktionen (ohne Datenbank, ohne HTTP) werden hier
// einzeln geprüft: Sie entscheiden, welches Ergebnis in der Übersicht steht.

func testDef(id int64, title string) quizEntry {
	return quizEntry{ID: id, Title: title, Q: befund.Questionnaire{
		Direction: "lower",
		Items:     []befund.QItem{{Q: "Schmerz", Max: 10}, {Q: "Funktion", Max: 10}},
	}}
}

func befundeWithQ(t *testing.T, dates map[string]string) []store.Befund {
	t.Helper()
	// Neueste zuerst, wie es der Store liefert.
	var out []store.Befund
	for date, data := range dates {
		out = append(out, store.Befund{ID: int64(len(out) + 1), Date: date, Title: "Befund " + date, Data: data})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Date > out[j-1].Date; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func TestMergeTestResults(t *testing.T) {
	t.Parallel()
	defs := map[int64]quizEntry{7: testDef(7, "Fragebogen Knie"), 9: testDef(9, "Fragebogen Schulter")}
	linked := []store.Entry{{ID: 7, Title: "Fragebogen Knie"}, {ID: 9, Title: "Fragebogen Schulter"}}

	t.Run("ohne Ergebnisse bleibt der Test offen", func(t *testing.T) {
		t.Parallel()
		got := mergeTestResults(linked, defs, nil, nil)
		if len(got) != 2 {
			t.Fatalf("%d Zeilen, erwartet 2", len(got))
		}
		for _, row := range got {
			if row.Date != "" || row.Source != "" {
				t.Errorf("%s: unerwartetes Ergebnis %+v", row.Title, row)
			}
			if !row.HasQ || row.Max != 20 {
				t.Errorf("%s: HasQ=%v Max=%d", row.Title, row.HasQ, row.Max)
			}
		}
	})

	t.Run("Einzeldurchführung schlägt nichts, aber füllt die Zeile", func(t *testing.T) {
		t.Parallel()
		runs := map[int64]store.TestRun{7: {EntryID: 7, Date: "2026-03-01", Data: `{"values":["3","4"],"note":"mit Stütze"}`}}
		got := mergeTestResults(linked, defs, runs, nil)
		row := findRow(t, got, "Fragebogen Knie")
		if row.Date != "2026-03-01" || row.Sum != 7 || row.Max != 20 || row.Source != "Test" || !row.Complete {
			t.Errorf("Zeile = %+v", row)
		}
		if row.Note != "mit Stütze" {
			t.Errorf("Notiz = %q", row.Note)
		}
	})

	t.Run("neuerer Befund sticht die Einzeldurchführung", func(t *testing.T) {
		t.Parallel()
		runs := map[int64]store.TestRun{7: {Date: "2026-03-01", Data: `{"values":["3","4"]}`}}
		bs := befundeWithQ(t, map[string]string{
			"2026-04-01": `{"q":{"7":{"values":["2","2"]}}}`,
		})
		row := findRow(t, mergeTestResults(linked, defs, runs, bs), "Fragebogen Knie")
		if row.Date != "2026-04-01" || row.Sum != 4 || row.Source != "Befund" || row.BefundID == 0 {
			t.Errorf("Zeile = %+v", row)
		}
	})

	t.Run("älterer Befund überschreibt den Einzellauf nicht", func(t *testing.T) {
		t.Parallel()
		runs := map[int64]store.TestRun{7: {Date: "2026-03-01", Data: `{"values":["3","4"]}`}}
		bs := befundeWithQ(t, map[string]string{
			"2026-02-01": `{"q":{"7":{"values":["9","9"]}}}`,
		})
		row := findRow(t, mergeTestResults(linked, defs, runs, bs), "Fragebogen Knie")
		if row.Date != "2026-03-01" || row.Sum != 7 || row.Source != "Test" {
			t.Errorf("Zeile = %+v", row)
		}
	})

	t.Run("gleiches Datum: Einzeldurchführung gewinnt", func(t *testing.T) {
		t.Parallel()
		runs := map[int64]store.TestRun{7: {Date: "2026-03-01", Data: `{"values":["1","1"]}`}}
		bs := befundeWithQ(t, map[string]string{
			"2026-03-01": `{"q":{"7":{"values":["9","9"]}}}`,
		})
		row := findRow(t, mergeTestResults(linked, defs, runs, bs), "Fragebogen Knie")
		if row.Sum != 2 || row.Source != "Test" {
			t.Errorf("Zeile = %+v", row)
		}
	})

	t.Run("nur verknüpfte Tests erscheinen", func(t *testing.T) {
		t.Parallel()
		bs := befundeWithQ(t, map[string]string{
			"2026-04-01": `{"q":{"9":{"values":["2","2"]},"99":{"values":["1"]}}}`,
		})
		got := mergeTestResults(linked, defs, nil, bs)
		if len(got) != 2 {
			t.Fatalf("%d Zeilen, erwartet 2", len(got))
		}
		if row := findRow(t, got, "Fragebogen Schulter"); row.Date != "2026-04-01" {
			t.Errorf("Schulter-Befund nicht übernommen: %+v", row)
		}
	})

	t.Run("kaputte Daten werden übersprungen", func(t *testing.T) {
		t.Parallel()
		runs := map[int64]store.TestRun{7: {Date: "2026-03-01", Data: `{kein json`}}
		bs := befundeWithQ(t, map[string]string{"2026-04-01": `kein json`})
		got := mergeTestResults(linked, defs, runs, bs)
		for _, row := range got {
			if row.Date != "" {
				t.Errorf("%s: Ergebnis aus kaputten Daten: %+v", row.Title, row)
			}
		}
	})

	t.Run("alphabetisch sortiert", func(t *testing.T) {
		t.Parallel()
		got := mergeTestResults(linked, defs, nil, nil)
		if got[0].Title >= got[1].Title {
			t.Errorf("nicht sortiert: %q, %q", got[0].Title, got[1].Title)
		}
	})

	t.Run("Test ohne Fragebogen-Definition", func(t *testing.T) {
		t.Parallel()
		plain := []store.Entry{{ID: 42, Title: "Lachman-Test"}}
		runs := map[int64]store.TestRun{42: {Date: "2026-03-01", Data: `{"note":"positiv"}`}}
		got := mergeTestResults(plain, defs, runs, nil)
		if len(got) != 1 {
			t.Fatalf("%d Zeilen", len(got))
		}
		row := got[0]
		if row.HasQ || row.Max != 0 || row.Sum != 0 || row.Date != "2026-03-01" || row.Note != "positiv" {
			t.Errorf("Zeile = %+v", row)
		}
	})
}

func TestBuildQuestionnaireHistory(t *testing.T) {
	t.Parallel()
	defs := map[int64]quizEntry{7: testDef(7, "Fragebogen Knie"), 9: testDef(9, "Fragebogen Schulter")}
	bs := befundeWithQ(t, map[string]string{
		"2026-01-01": `{"q":{"7":{"values":["5","5"],"note":"erster"}}}`,
		"2026-02-01": `{"q":{"7":{"values":["3"]},"9":{"values":["2","2"]}}}`,
		"2026-03-01": `kaputt`,
	})

	got := buildQuestionnaireHistory(bs, defs)
	if len(got) != 3 {
		t.Fatalf("%d Zeilen, erwartet 3: %+v", len(got), got)
	}
	// Nach Name gruppiert, innerhalb eines Fragebogens chronologisch.
	if got[0].Name != "Fragebogen Knie" || got[1].Name != "Fragebogen Knie" || got[2].Name != "Fragebogen Schulter" {
		t.Fatalf("Reihenfolge = %v", []string{got[0].Name, got[1].Name, got[2].Name})
	}
	if got[0].Date != "2026-01-01" || got[1].Date != "2026-02-01" {
		t.Errorf("nicht chronologisch: %q, %q", got[0].Date, got[1].Date)
	}
	if got[0].Sum != 10 || got[0].Max != 20 || !got[0].Complete || got[0].Note != "erster" {
		t.Errorf("erste Zeile = %+v", got[0])
	}
	if got[1].Complete {
		t.Error("unvollständiger Fragebogen als vollständig gewertet")
	}
	if got[2].Dir != "niedriger ist besser" {
		t.Errorf("Richtungstext = %q", got[2].Dir)
	}
}

func TestBuildQuestionnaireHistoryEmpty(t *testing.T) {
	t.Parallel()
	if got := buildQuestionnaireHistory(nil, nil); len(got) != 0 {
		t.Errorf("%d Zeilen bei leerer Eingabe", len(got))
	}
	// Befund mit Fragebogen, der nicht (mehr) als Test existiert.
	bs := befundeWithQ(t, map[string]string{"2026-01-01": `{"q":{"123":{"values":["1"]}}}`})
	if got := buildQuestionnaireHistory(bs, map[int64]quizEntry{}); len(got) != 0 {
		t.Errorf("%d Zeilen für unbekannten Fragebogen", len(got))
	}
}

// TestNewerThan prüft die Datumsregel der Übersicht isoliert.
func TestNewerThan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		have, date string
		want       bool
	}{
		{"", "2026-01-01", false},          // noch kein Ergebnis
		{"2026-01-01", "2026-01-01", true}, // gleiches Datum: bestehendes bleibt
		{"2026-01-02", "2026-01-01", true},
		{"2026-01-01", "2026-01-02", false},
	}
	for _, c := range cases {
		tp := &patientTest{Date: c.have}
		if got := newerThan(tp, c.date); got != c.want {
			t.Errorf("newerThan(%q, %q) = %v, erwartet %v", c.have, c.date, got, c.want)
		}
	}
}

// TestApplyResult: ein Ergebnis setzt Summe, Max, Vollständigkeit, Notiz, Quelle.
func TestApplyResult(t *testing.T) {
	t.Parallel()
	def := testDef(7, "Knie")
	tp := &patientTest{EntryID: 7, Title: "Knie", Max: 20}
	applyResult(tp, "2026-05-05", befund.QFill{Values: []string{"4", "6"}, Note: "n"}, def, "Befund", 12)
	if tp.Date != "2026-05-05" || tp.Sum != 10 || tp.Max != 20 || !tp.Complete || tp.Source != "Befund" || tp.BefundID != 12 {
		t.Errorf("Zeile = %+v", tp)
	}
	// Ohne Definition wird nur Datum/Summe/Quelle gesetzt.
	tp2 := &patientTest{EntryID: 42, Title: "Lachman"}
	applyResult(tp2, "2026-05-05", befund.QFill{Note: "positiv"}, quizEntry{}, "Test", 0)
	if tp2.Sum != 0 || tp2.Complete || tp2.Max != 0 || tp2.Note != "positiv" {
		t.Errorf("Zeile ohne Definition = %+v", tp2)
	}
}

func findRow(t *testing.T, rows []patientTest, title string) patientTest {
	t.Helper()
	for _, row := range rows {
		if row.Title == title {
			return row
		}
	}
	t.Fatalf("Zeile %q nicht gefunden", title)
	return patientTest{}
}
