package befund

import "testing"

// FilledCols blendet Spalten ohne Werte aus (Ausdruck), ohne die Definition zu ändern.
func TestFilledCols(t *testing.T) {
	d := New()
	d.Tables["pain"] = []Row{
		{"lok": "vorderes Knie links", "nrs": "6", "mech": "nozizeptiv"},
		{"lok": "Kniekehle", "nrs": "4"},
	}
	cols := d.FilledCols(TablePain)
	keys := make([]string, 0, len(cols))
	for _, c := range cols {
		keys = append(keys, c.Key)
	}
	if len(keys) != 3 || keys[0] != "lok" || keys[1] != "nrs" || keys[2] != "mech" {
		t.Fatalf("Spalten = %v, erwartet [lok nrs mech]", keys)
	}
	if len(TablePain.Cols) != 4 {
		t.Error("Schema wurde verändert")
	}

	// Ganz leere Tabelle: keine Spalten.
	if got := New().FilledCols(TablePain); len(got) != 0 {
		t.Errorf("%d Spalten bei leerer Tabelle", len(got))
	}
	// Nur eine Zeile mit einem Wert: genau eine Spalte.
	d2 := New()
	d2.Tables["pain"] = []Row{{"lok": "Knie"}}
	if got := d2.FilledCols(TablePain); len(got) != 1 || got[0].Key != "lok" {
		t.Errorf("Spalten = %v", got)
	}
}
