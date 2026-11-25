package store

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestSeedAssessments: die mitgelieferten Assessments landen vollständig in der
// Datenbank, ein zweiter Lauf legt nichts doppelt an, und die Titel sind über
// die Testsuche erreichbar (Grundlage für Verknüpfen im Befund).
func TestSeedAssessments(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	n, err := s.SeedAssessments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(assessmentEntries) {
		t.Fatalf("angelegt = %d, erwartet %d", n, len(assessmentEntries))
	}
	if again, err := s.SeedAssessments(ctx); err != nil || again != 0 {
		t.Fatalf("zweiter Lauf: %d, %v", again, err)
	}

	for _, e := range assessmentEntries {
		hits, err := s.SearchTests(ctx, e.Title, 20)
		if err != nil {
			t.Fatalf("%s: %v", e.Title, err)
		}
		entry, found := Entry{}, false
		for _, h := range hits {
			if h.Title == e.Title {
				entry, found = h, true
			}
		}
		if !found {
			t.Fatalf("%s: nicht gefunden", e.Title)
		}
		if entry.Kind != "test" {
			t.Errorf("%s: kind = %q", e.Title, entry.Kind)
		}
		if strings.TrimSpace(entry.Summary) == "" || entry.Body == "" {
			t.Errorf("%s: Summary oder Body leer", e.Title)
		}
		// Urheberrecht: keine Fragebogen-Definitionen, nur Beschreibung.
		if entry.Schema != "" {
			t.Errorf("%s: Schema sollte leer bleiben", e.Title)
		}
		// Jeder Eintrag nennt eine Quelle.
		if !strings.Contains(entry.Body, "## Quelle") {
			t.Errorf("%s: Quellenangabe fehlt", e.Title)
		}
		// Ein Datensatz darf kein JSON sein, sondern ist Markdown.
		if json.Valid([]byte(entry.Body)) {
			t.Errorf("%s: Body ist JSON", e.Title)
		}
	}
}

func TestAssessmentSearchTerms(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.SeedAssessments(ctx); err != nil {
		t.Fatal(err)
	}
	// So, wie es in Praxis und Demo getippt wird: Abkürzung oder Wortanfang.
	cases := map[string]string{
		"Berg":      "Berg Balance Scale (BBS)",
		"BBS":       "Berg Balance Scale (BBS)",
		"berg bal":  "Berg Balance Scale (BBS)",
		"DEMMI":     "de Morton Mobility Index (DEMMI)",
		"DGI":       "DGI F (Fassung F)", // erstes Ergebnis alphabetisch
		"FGA":       "Functional Gait Assessment (FGA)",
		"FAC":       "Functional Ambulation Categories (FAC)",
		"Mini-BEST": "Mini-BESTest",
		"TIS":       "TIS F (Fassung F)",
		"Trunk":     "Trunk Control Test (TCT)",
		"Gait":      "Manual Gait Index (MGI)",
	}
	for q, want := range cases {
		hits, err := s.SearchTests(ctx, q, 20)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if len(hits) == 0 {
			t.Errorf("%q: kein Treffer", q)
			continue
		}
		found := false
		for _, h := range hits {
			if h.Title == want {
				found = true
			}
		}
		if !found {
			titles := make([]string, 0, len(hits))
			for _, h := range hits {
				titles = append(titles, h.Title)
			}
			t.Errorf("%q: %q fehlt in %v", q, want, titles)
		}
	}
}

// Die Volltextsuche der Wissensdatenbank findet die Assessments ebenfalls.
func TestAssessmentFullText(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.SeedAssessments(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := s.Search(ctx, "Sturzrisiko", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("keine Treffer für Sturzrisiko")
	}
	for _, e := range res {
		if e.Kind != "test" {
			t.Errorf("%s: kind = %q", e.Title, e.Kind)
		}
	}
}

// TestSeedDemoAssessments: das Demo verknüpft Assessments mit Patienten
// (einmal offen, einmal mit Durchführung) – Grundlage für die Demo-Seite.
func TestSeedDemoAssessments(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	tests, err := s.PatientTests(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	titles := make([]string, 0, len(tests))
	for _, e := range tests {
		titles = append(titles, e.Title)
	}
	if !slices.Contains(titles, "Functional Ambulation Categories (FAC)") {
		t.Errorf("FAC nicht mit Patient 1 verknüpft: %v", titles)
	}
	runs, err := s.LatestTestRuns(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 {
		t.Error("keine Testdurchführungen im Demo")
	}
}
