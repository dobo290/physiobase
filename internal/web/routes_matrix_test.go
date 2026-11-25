package web

import (
	"net/url"
	"strings"
	"testing"
)

// TestRouteMatrix prüft die Zugriffskontrolle über alle wichtigen Seiten:
// anonym → Anmeldung, angemeldeter Benutzer → kein /admin, Admin → alles.
// Neue Routen fallen auf, wenn sie hier ergänzt werden müssten.
func TestRouteMatrix(t *testing.T) {
	ts, adminClient := newServer(t)
	login(t, adminClient, ts.URL)
	post(adminClient, ts.URL+"/patienten", url.Values{"name": {"Matrix Test"}})
	post(adminClient, ts.URL+"/patient/1/befund", url.Values{})
	post(adminClient, ts.URL+"/handouts", url.Values{"title": {"Matrix Handout"}})

	// Normalen Benutzer über die Verwaltung anlegen und anmelden.
	post(adminClient, ts.URL+"/admin/benutzer", url.Values{
		"username": {"matrixuser"}, "role": {"user"}, "password": {"matrix-geheim-1"},
	})
	user := loginAs(t, ts.URL, "matrixuser", "matrix-geheim-1")

	admin := []string{
		"/", "/patienten", "/patient/1", "/befund/1", "/termine", "/handouts",
		"/handout/1", "/konto", "/admin", "/admin/benutzer", "/admin/einstellungen", "/admin/symbole",
		"/eintrag/1", "/patient/1/vergleich", "/api/tests?q=Test",
	}
	// Seiten, die auch ein normaler Benutzer sehen darf.
	shared := map[string]bool{
		"/": true, "/patienten": true, "/patient/1": true, "/befund/1": true,
		"/termine": true, "/handouts": true, "/handout/1": true, "/konto": true,
		"/eintrag/1": true, "/patient/1/vergleich": true, "/api/tests?q=Test": true,
	}

	for _, path := range admin {
		// anonym: Weiterleitung zur Anmeldung
		res, err := newClientOnly().Get(ts.URL + path)
		if err != nil {
			t.Fatalf("%s anonym: %v", path, err)
		}
		res.Body.Close()
		if !strings.HasPrefix(path, "/api/") && res.StatusCode != 303 && res.StatusCode != 401 {
			t.Errorf("%s anonym: Status %d, erwartet 303/401", path, res.StatusCode)
		}

		// normaler Benutzer
		res, err = user.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("%s Benutzer: %v", path, err)
		}
		res.Body.Close()
		if shared[path] {
			if res.StatusCode != 200 {
				t.Errorf("%s Benutzer: Status %d, erwartet 200", path, res.StatusCode)
			}
		} else if res.StatusCode != 404 {
			t.Errorf("%s Benutzer: Status %d, erwartet 404 (Verwaltung versteckt)", path, res.StatusCode)
		}

		// Admin
		res, err = adminClient.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("%s Admin: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("%s Admin: Status %d, erwartet 200", path, res.StatusCode)
		}
	}
}

// TestNotFoundAndBadIDs: unbekannte IDs und Pfade dürfen nicht 500 liefern.
func TestNotFoundAndBadIDs(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	for _, path := range []string{
		"/eintrag/999999", "/patient/999999", "/befund/999999",
		"/termin/999999", "/handout/999999", "/zusammenfassung/999999", "/patient/0", "/eintrag/abc",
	} {
		res, err := c.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Errorf("%s: Status %d, erwartet 404", path, res.StatusCode)
		}
	}
}
