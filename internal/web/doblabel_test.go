package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// dobLabel unterscheidet Datum, Jahr und Alter – ein eingetragenes Alter darf
// nicht als Geburtsdatum erscheinen.
func TestDobLabel(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"", ""},
		{"45", "45 Jahre"},
		{" 7 ", "7 Jahre"},
		{"100", "100 Jahre"},
		{"1969", "geb. 1969"},
		{"12.04.1969", "geb. 12.04.1969"},
		{"1969-04-12", "geb. 1969-04-12"},
		{"45 J.", "geb. 45 J."}, // nicht als Alter erkannt, bleibt beim Datums-Label
	}
	for _, c := range cases {
		if got := dobLabel(c.in); got != c.want {
			t.Errorf("dobLabel(%q) = %q, erwartet %q", c.in, got, c.want)
		}
	}
}

// Der Ausdruck zeigt das Alter als Alter und lässt leere Tabellenspalten weg.
func TestPrintHeaderAgeAndEmptyColumns(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Druck Test"}, "dob": {"45"}})
	r, _ := post(c, ts.URL+"/patient/1/befund", url.Values{})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Befund anlegen: %d", r.StatusCode)
	}
	id := strings.TrimPrefix(r.Header.Get("Location"), "/befund/")

	// Schmerzzeile mit Lokalisation und NRS, aber ohne Subtyp
	form := url.Values{
		"f.pain_now": {"5"}, "t.pain.n": {"1"},
		"t.pain.0.lok": {"vorderes Knie links"}, "t.pain.0.nrs": {"6"}, "t.pain.0.mech": {"nozizeptiv"},
	}
	if r, _ := post(c, ts.URL+"/befund/"+id, form); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Befund speichern: %d", r.StatusCode)
	}

	code, page := body(t, c, ts.URL+"/befund/"+id+"/druck")
	if code != http.StatusOK {
		t.Fatalf("Druckansicht: %d", code)
	}
	if !strings.Contains(page, "45 Jahre") {
		t.Error("Alter wird im Druck nicht als Alter gezeigt")
	}
	if strings.Contains(page, "geb. 45") {
		t.Error("Alter erscheint weiterhin als Geburtsdatum")
	}
	if strings.Contains(page, "Subtyp (nur nozizeptiv)") {
		t.Error("leere Subtyp-Spalte wird gedruckt")
	}
	if !strings.Contains(page, "vorderes Knie links") {
		t.Error("gefüllte Spalte fehlt")
	}
}
