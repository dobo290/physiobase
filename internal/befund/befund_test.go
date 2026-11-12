package befund

import (
	"net/url"
	"testing"
)

func TestDelta(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"0-0-120", "0-0-135", "+15"},
		{"5-0-110", "0-0-130", "-5 / +20"},
		{"3", "4", "+1"},
		{"4", "4", "±0"},
		{"12,5 s", "10 s", "-2,5"},
		{"4+", "5", "+1"},
		{"", "3", ""},
		{"elastisch", "fest", ""},
	}
	for _, c := range cases {
		if got := Delta(c.a, c.b); got != c.want {
			t.Errorf("Delta(%q,%q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestFromForm(t *testing.T) {
	f := url.Values{
		"f.diag": {"  Gonarthrose re  "}, "f.rf_cancer": {"1"}, "f.pain_now": {"6"}, "f.unbekannt": {"x"},
		"t.rom.n": {"3"}, "t.rom.0.joint": {"Knie re"}, "t.rom.0.act": {"0-5-110"}, "t.rom.2.move": {"Ext"},
		"stamps": {`[{"v":0,"x":0.5,"y":0.3,"s":"pain","c":"#ff0000","z":2},{"v":9,"x":0.5,"y":0.5,"s":"pain"},{"v":1,"x":2,"y":0,"s":"pain"},{"v":1,"x":0.1,"y":0.1,"s":"bogus"}]`},
	}
	d := FromForm(f)
	if d.Get("diag") != "Gonarthrose re" || d.Get("rf_cancer") != "1" || d.Get("pain_now") != "6" {
		t.Errorf("Felder: %+v", d.Fields)
	}
	if _, ok := d.Fields["unbekannt"]; ok {
		t.Error("unbekanntes Feld übernommen")
	}
	if n := len(d.Tables["rom"]); n != 2 {
		t.Errorf("ROM-Zeilen %d, erwartet 2 (leere Zeile verworfen)", n)
	}
	if len(d.Stamps) != 1 {
		t.Errorf("Stempel %d, erwartet 1 gültigen", len(d.Stamps))
	}
	if !d.HasSection(Sections[1]) { // Screening
		t.Error("Screening-Sektion sollte Daten haben")
	}
}

// Datei im Format der alten Befundseite
const legacy = `{
  "name": "Erika Muster", "diag": "Z. n. VKB-Plastik re", "date": "2025-10-01", "dob": "1990",
  "S/I": "mittel", "24h": "morgens steif", "im_payr": true, "im_homann": false,
  "custom-ab12cd": "Eigener Abschnitt",
  "rom": [{"joint":"Knie re","move":"Flex/Ext","act":"0-5-100","pass":"0-0-110","end":"fest"}, {"joint":"","move":"","act":"","pass":"","end":""}],
  "mfp": [{"muscle":"Quadrizeps","right":"4","left":"5"}],
  "circ": [], "timeline": [{"date":"2025-08-01","title":"OP","note":"VKB"}],
  "treat": [{"date":"2025-10-02","goal":"Extension","note":"gut"}, {"date":"","goal":"","note":""}],
  "blocks": {}, "fields": {}, "order": [], "_v": 2
}`

func TestParseLegacy(t *testing.T) {
	l, err := ParseLegacy([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "Erika Muster" || l.Date != "2025-10-01" || l.Dob != "1990" {
		t.Errorf("Kopf: %+v", l)
	}
	d := l.Data
	if d.Get("diag") == "" || d.Get("S/I") != "mittel" || d.Get("24h") == "" || d.Get("im_payr") != "1" || d.Get("im_homann") != "" {
		t.Errorf("Felder: %v", d.Fields)
	}
	if d.Get("zusatz") != "Eigener Abschnitt" {
		t.Errorf("Zusatzabschnitt: %q", d.Get("zusatz"))
	}
	if len(d.Tables["rom"]) != 1 || len(d.Tables["mfp"]) != 1 || len(d.Tables["timeline"]) != 1 {
		t.Errorf("Tabellen: %v", d.Tables)
	}
	if len(l.Sessions) != 1 {
		t.Errorf("Behandlungen: %d", len(l.Sessions))
	}
	if _, err := ParseLegacy([]byte(`{"foo":1}`)); err == nil {
		t.Error("fremde JSON-Datei akzeptiert")
	}
}

func TestCompare(t *testing.T) {
	a, b := New(), New()
	a.Fields["pain_now"], b.Fields["pain_now"] = "7", "3"
	a.Fields["diag"], b.Fields["diag"] = "X", "X"
	a.Tables["rom"] = []Row{{"joint": "Knie", "move": "Flex", "act": "0-0-100"}}
	b.Tables["rom"] = []Row{{"joint": "Knie", "move": "Flex", "act": "0-0-125"}, {"joint": "Hüfte", "move": "Flex", "act": "0-0-120"}}
	b.Tables["psfs"] = []Row{{"activity": "Treppe", "score": "6"}}
	fields, tables := Compare(a, b)
	if len(fields) != 1 || fields[0].A != "7" || fields[0].B != "3" {
		t.Errorf("Felder: %+v", fields)
	}
	if len(tables) != 2 || tables[0].Rows[0].Cells[0].Delta != "+25" || len(tables[0].Rows) != 2 {
		t.Errorf("Tabellen: %+v", tables)
	}
	if PSFSMean(b) != "6,0" {
		t.Errorf("PSFS: %q", PSFSMean(b))
	}
}

func TestPainSummary(t *testing.T) {
	d := New()
	d.Tables["pain"] = []Row{{"lok": "Knie", "nrs": "6"}, {"lok": "Schulter", "nrs": "3"}, {"lok": "ohne Wert"}}
	if got := PainSummary(d); got != "P1 6 · P2 3" {
		t.Errorf("PainSummary: %q", got)
	}
	d2 := New()
	if PainSummary(d2) != "" {
		t.Error("leere Schmerzquellen")
	}
}
