package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/database"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close(db) })
	return New(db)
}

func TestFTSQuery(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"knie":             `"knie"*`,
		`knie "OR" sehne*`: `"knie"* "OR"* "sehne"*`,
		"Lachman-Test":     `"Lachman"* "Test"*`,
		`"); DROP`:         `"DROP"*`,
	}
	for in, want := range cases {
		if got := FTSQuery(in); got != want {
			t.Errorf("FTSQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeedSearchLinks(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seeded, err := s.Seed(ctx)
	if err != nil || !seeded {
		t.Fatalf("seed: %v %v", seeded, err)
	}
	// zweites Mal: nichts passiert
	if again, _ := s.Seed(ctx); again {
		t.Fatal("seed lief zweimal")
	}
	// Präfix + Umlaut-unabhängig
	for _, q := range []string{"patella", "Ubung", "kniebeuge schräg", "jumper"} {
		res, err := s.Search(ctx, q, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(res) == 0 {
			t.Errorf("Suche %q: keine Treffer", q)
		}
	}
	res, _ := s.Search(ctx, "", "uebung", "Knie")
	if len(res) != 3 {
		t.Errorf("Filter Übung+Knie: %d Treffer, erwartet 3", len(res))
	}

	all, _ := s.Search(ctx, "Patellatendinopathie", "krankheitsbild", "")
	if len(all) != 1 {
		t.Fatalf("Krankheitsbild nicht gefunden")
	}
	rel, err := s.Related(ctx, all[0].ID)
	if err != nil || len(rel) != 5 {
		t.Fatalf("Related: %d %v", len(rel), err)
	}
	// Update hält den Suchindex aktuell
	e := all[0]
	e.Title = "Springerknie"
	if err := s.UpdateEntry(ctx, e); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Search(ctx, "springerknie", "", ""); len(r) != 1 {
		t.Error("geänderter Titel nicht im Index")
	}
	if r, _ := s.Search(ctx, "Patellatendinopathie", "krankheitsbild", ""); len(r) != 0 {
		t.Error("alter Titel noch im Index")
	}
	// Löschen entfernt Verknüpfungen (Fremdschlüssel aktiv)
	if _, err := s.DeleteEntry(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM entry_links WHERE a = ? OR b = ?`, e.ID, e.ID).Scan(&n)
	if n != 0 {
		t.Errorf("%d Verknüpfungen übrig", n)
	}
	if _, err := s.Entry(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("gelöschter Eintrag: %v", err)
	}
}

func TestHandoutOrderAndCopy(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	var ids []int64
	for _, title := range []string{"A", "B", "C"} {
		id, _ := s.CreateEntry(ctx, Entry{Kind: "uebung", Title: title, Dosage: "3x10"})
		ids = append(ids, id)
	}
	notEx, _ := s.CreateEntry(ctx, Entry{Kind: "test", Title: "kein Übung"})
	hid, _ := s.CreateHandout(ctx, "H", "")
	for _, id := range append(ids, notEx) {
		if err := s.AddHandoutItem(ctx, hid, id); err != nil {
			t.Fatal(err)
		}
	}
	items, _ := s.HandoutItems(ctx, hid)
	if len(items) != 3 {
		t.Fatalf("%d Items, erwartet 3 (Tests dürfen nicht rein)", len(items))
	}
	if items[0].Dosage != "3x10" {
		t.Error("Standard-Dosierung nicht übernommen")
	}
	if err := s.MoveHandoutItemUp(ctx, hid, ids[2]); err != nil {
		t.Fatal(err)
	}
	items, _ = s.HandoutItems(ctx, hid)
	got := items[0].Entry.Title + items[1].Entry.Title + items[2].Entry.Title
	if got != "ACB" {
		t.Errorf("Reihenfolge %s, erwartet ACB", got)
	}
	nid, err := s.CopyHandout(ctx, hid)
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := s.HandoutItems(ctx, nid)
	if len(cp) != 3 || cp[1].Entry.Title != "C" {
		t.Error("Kopie unvollständig")
	}
}
