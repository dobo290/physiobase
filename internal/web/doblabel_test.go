package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"forge.gpl-nas.de/physiobase/internal/dob"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// dobLabel zeigt Datum, Jahr, Monat und abgeleitetes Alter passend zur
// gespeicherten Genauigkeit – ein eingetragenes Alter darf nicht als
// Geburtsdatum erscheinen.
func TestDobLabel(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		p    store.Patient
		want string
	}{
		{"leer", store.Patient{}, ""},
		{"Alter", store.Patient{Dob: "1981", DobPrecision: dob.Age}, "45 Jahre"},
		{"Alter einstellig", store.Patient{Dob: "2019", DobPrecision: dob.Age}, "7 Jahre"},
		{"Jahr", store.Patient{Dob: "1969", DobPrecision: dob.Year}, "geb. 1969"},
		{"Tag", store.Patient{Dob: "1969-04-12", DobPrecision: dob.Day}, "geb. 12.04.1969"},
		{"Monat", store.Patient{Dob: "1969-04", DobPrecision: dob.Month}, "geb. April 1969"},
		{"unbekannt", store.Patient{DobPrecision: dob.Unknown, DobRaw: "45 J."}, "geb. 45 J."},
		{"Altdatum ohne Genauigkeit", store.Patient{Dob: "1969-04-12"}, "geb. 1969-04-12"},
	}
	for _, c := range cases {
		if got := dob.LabelAt(c.p.Dob, c.p.DobPrecision, c.p.DobRaw, now); got != c.want {
			t.Errorf("%s: LabelAt(%q, %q, %q) = %q, erwartet %q",
				c.name, c.p.Dob, c.p.DobPrecision, c.p.DobRaw, got, c.want)
		}
	}
	// dobLabel reicht nur an dob.Label weiter.
	if got := dobLabel(store.Patient{Dob: "1969-04-12", DobPrecision: dob.Day}); got != "geb. 12.04.1969" {
		t.Errorf("dobLabel = %q, erwartet %q", got, "geb. 12.04.1969")
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
