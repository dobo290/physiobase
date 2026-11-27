package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Lange Listen auf der Patientenseite werden gekuerzt, der Rest ist verlinkt.
func TestPatientPageLimitsLongLists(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Viele Sitzungen"}})
	for i := range 25 {
		date := fmt.Sprintf("2026-01-%02d", i%28+1)
		r, _ := post(c, ts.URL+"/patient/1/verlauf", url.Values{"date": {date}, "goal": {"KG"}, "note": {fmt.Sprint(i)}})
		if r.StatusCode != http.StatusSeeOther {
			t.Fatalf("Sitzung %d: %d", i, r.StatusCode)
		}
	}
	_, page := body(t, c, ts.URL+"/patient/1")
	if n := strings.Count(page, `class="session"`); n != 20 {
		t.Errorf("%d Sitzungen gerendert, erwartet 20", n)
	}
	if !strings.Contains(page, "25 insgesamt anzeigen") {
		t.Error("Hinweis auf die Gesamtliste fehlt")
	}
	_, all := body(t, c, ts.URL+"/patient/1?alle=1")
	if n := strings.Count(all, `class="session"`); n != 25 {
		t.Errorf("mit ?alle=1: %d Sitzungen, erwartet 25", n)
	}
}
