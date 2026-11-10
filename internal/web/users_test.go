package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/config"
)

func loginAs(t *testing.T, base, user, pw string) *http.Client {
	t.Helper()
	_, c := newClient(t)
	r, err := post(c, base+"/login", url.Values{"username": {user}, "password": {pw}})
	if err != nil || r.StatusCode != http.StatusSeeOther {
		t.Fatalf("login %s: %v %d", user, err, r.StatusCode)
	}
	return c
}

func TestUserManagement(t *testing.T) {
	ts, admin := newServer(t)
	login(t, admin, ts.URL) // admin aus der Konfiguration = erster Admin

	// Verwaltung nur über /admin, kein Link in der App
	if code, html := body(t, admin, ts.URL+"/"); code != 200 || strings.Contains(html, `href="/admin`) {
		t.Error("Verwaltung ist in der App verlinkt")
	}
	if code, _ := body(t, admin, ts.URL+"/admin"); code != 200 {
		t.Fatalf("/admin für Admin: %d", code)
	}
	if code, _ := body(t, admin, ts.URL+"/einstellungen"); code != http.StatusNotFound {
		t.Errorf("alte /einstellungen noch erreichbar: %d", code)
	}

	// Benutzer mit Startpasswort anlegen → kann sich anmelden, sieht /admin nicht (404)
	r, _ := post(admin, ts.URL+"/admin/benutzer", url.Values{"username": {"anna"}, "display_name": {"Anna S."}, "role": {"user"}, "password": {"anna-geheim-1"}})
	if !strings.HasPrefix(r.Header.Get("Location"), "/admin/benutzer/") {
		t.Fatalf("anlegen: %s", r.Header.Get("Location"))
	}
	anna := loginAs(t, ts.URL, "anna", "anna-geheim-1")
	if code, _ := body(t, anna, ts.URL+"/patienten"); code != 200 {
		t.Error("Benutzer kann die App nicht nutzen")
	}
	for _, p := range []string{"/admin", "/admin/benutzer", "/admin/einstellungen"} {
		if code, _ := body(t, anna, ts.URL+p); code != http.StatusNotFound {
			t.Errorf("Benutzer sieht %s: %d", p, code)
		}
	}
	if r, _ := post(anna, ts.URL+"/admin/benutzer", url.Values{"username": {"hacker"}, "role": {"admin"}}); r.StatusCode != http.StatusNotFound {
		t.Errorf("Benutzer kann Benutzer anlegen: %d", r.StatusCode)
	}

	// Deaktivieren beendet die Sitzung sofort
	post(admin, ts.URL+"/admin/benutzer/2", url.Values{"display_name": {"Anna S."}, "role": {"user"}})
	if code, _ := body(t, anna, ts.URL+"/"); code != http.StatusSeeOther {
		t.Error("deaktivierter Benutzer noch angemeldet")
	}
	if r, _ := post(newClientOnly(), ts.URL+"/login", url.Values{"username": {"anna"}, "password": {"anna-geheim-1"}}); r.StatusCode != http.StatusUnauthorized {
		t.Error("deaktivierter Benutzer kann sich anmelden")
	}

	// Letzten Admin schützen
	r, _ = post(admin, ts.URL+"/admin/benutzer/1", url.Values{"role": {"user"}, "active": {"1"}})
	if !strings.Contains(r.Header.Get("Location"), "fehler=") {
		t.Error("letzter Admin konnte herabgestuft werden")
	}

	// Einrichtungs-Link: ohne Passwort angelegt → Link → Passwort setzen → angemeldet; Link danach ungültig
	r, _ = post(admin, ts.URL+"/admin/benutzer", url.Values{"username": {"ben"}, "role": {"user"}})
	loc := r.Header.Get("Location")
	m := regexp.MustCompile(`link=([0-9a-f]{48})`).FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("kein Einrichtungs-Link: %s", loc)
	}
	_, linkPage := body(t, admin, ts.URL+loc)
	if !strings.Contains(linkPage, "/einrichten/"+m[1]) {
		t.Error("Link wird nicht angezeigt")
	}
	_, ben := newClient(t)
	if code, html := body(t, ben, ts.URL+"/einrichten/"+m[1]); code != 200 || !strings.Contains(html, "Willkommen") {
		t.Fatalf("Einrichtungsseite: %d", code)
	}
	r, _ = post(ben, ts.URL+"/einrichten/"+m[1]+"/passwort", url.Values{"password": {"ben-passwort-1"}, "password2": {"ben-passwort-1"}})
	if r.Header.Get("Location") != "/konto?gespeichert=1" {
		t.Fatalf("Passwort über Link: %s", r.Header.Get("Location"))
	}
	if code, _ := body(t, ben, ts.URL+"/konto"); code != 200 {
		t.Error("nach Einrichtung nicht angemeldet")
	}
	if code, _ := body(t, newClientOnly(), ts.URL+"/einrichten/"+m[1]); code != http.StatusNotFound {
		t.Error("Einrichtungs-Link funktioniert zweimal")
	}

	// Abmelden beendet nur die eigenen Sitzungen
	post(ben, ts.URL+"/logout", nil)
	if code, _ := body(t, admin, ts.URL+"/admin"); code != 200 {
		t.Error("Abmelden eines Benutzers hat den Admin abgemeldet")
	}
	// Passwort über Konto ändern: falsches aktuelles Passwort wird abgelehnt
	r, _ = post(admin, ts.URL+"/konto/passwort", url.Values{"current": {"falsch"}, "password": {"neues-passwort-1"}, "password2": {"neues-passwort-1"}})
	if !strings.Contains(r.Header.Get("Location"), "fehler=") {
		t.Error("Passwortänderung ohne aktuelles Passwort")
	}
}

func TestPasskeyEndpoints(t *testing.T) {
	// Ohne HTTPS-Adresse: Passkeys aus, saubere Meldung
	ts, c := newServer(t)
	if r, _ := post(c, ts.URL+"/passkey/login/start", nil); r.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("ohne HTTPS: %d", r.StatusCode)
	}
	// Mit HTTPS-Adresse: Optionen für die Anmeldung (ohne Benutzername) und Registrierung
	ts2, c2 := newServerCfg(t, func(cfg *config.Config) { cfg.BaseURL = "https://physio.example.org" })
	r, _ := post(c2, ts2.URL+"/passkey/login/start", nil)
	b := readBody(r)
	if r.StatusCode != http.StatusOK || !strings.Contains(b, `"challenge"`) || !strings.Contains(b, `"rpId":"physio.example.org"`) {
		t.Fatalf("Login-Optionen: %d %s", r.StatusCode, b[:min(len(b), 200)])
	}
	if r, _ := post(newClientOnly(), ts2.URL+"/passkey/registrieren/start", nil); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("Registrierung ohne Anmeldung: %d", r.StatusCode)
	}
	login(t, c2, ts2.URL)
	r, _ = post(c2, ts2.URL+"/passkey/registrieren/start?name=Laptop", nil)
	b = readBody(r)
	if r.StatusCode != http.StatusOK || !strings.Contains(b, `"residentKey":"required"`) {
		t.Fatalf("Registrierungs-Optionen: %d %s", r.StatusCode, b[:min(len(b), 200)])
	}
	// Abschluss ohne gültige Antwort wird abgelehnt
	if r, _ := post(c2, ts2.URL+"/passkey/registrieren/fertig", nil); r.StatusCode == http.StatusOK {
		t.Error("ungültige Registrierung akzeptiert")
	}
	if r, _ := post(newClientOnly(), ts2.URL+"/passkey/login/fertig", nil); r.StatusCode == http.StatusOK {
		t.Error("ungültige Passkey-Anmeldung akzeptiert")
	}
}
