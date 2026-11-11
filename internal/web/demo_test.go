package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/config"
)

func TestDemoMode(t *testing.T) {
	ts, c := newServerCfg(t, func(cfg *config.Config) {
		cfg.Demo = true
		cfg.BaseURL = "https://physiobase.example.org"
	})
	login(t, c, ts.URL) // admin aus der Konfiguration

	// Daten anlegen, dann Abmelden → alles weg
	post(c, ts.URL+"/patienten", url.Values{"name": {"Demo Patient"}})
	post(c, ts.URL+"/eintrag/neu", url.Values{"kind": {"notiz"}, "title": {"Demo Notiz"}})
	if _, html := body(t, c, ts.URL+"/patienten"); !strings.Contains(html, "Demo Patient") {
		t.Fatal("Patient nicht angelegt")
	}
	post(c, ts.URL+"/logout", nil)
	// nach Abmelden zurückgesetzt: Patient weg, Seed wieder da
	if _, html := body(t, c, ts.URL+"/patienten"); strings.Contains(html, "Demo Patient") {
		t.Error("Demo-Daten nach Abmelden nicht gelöscht")
	}
	// "Demo Notiz" ist gelöscht; die Wissensdatenbank ist wieder gefüllt (Seed)
	if _, html := body(t, c, ts.URL+"/?art=notiz"); strings.Contains(html, "Demo Notiz") {
		t.Error("Demo-Eintrag nach Abmelden noch da")
	}
	// Upload im Demo gesperrt
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"P2"}})
	post(c, ts.URL+"/patient/1/befund", url.Values{})
	r, _ := post(c, ts.URL+"/eintrag/1/medien", url.Values{})
	if r.StatusCode != http.StatusForbidden {
		t.Errorf("Upload im Demo: %d", r.StatusCode)
	}
	// /demo/reset setzt zurück (öffentlich)
	r, _ = post(c, ts.URL+"/demo/reset", nil)
	if r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != "/login" {
		t.Errorf("demo/reset: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	// Nach Reset: alte Sitzung ungültig (session_version erhöht), Login-Seite zeigt Demo-Infos
	if code, html := body(t, c, ts.URL+"/login"); code != 200 || !strings.Contains(html, "demo123") {
		t.Error("Login-Seite ohne Demo-Zugangsdaten")
	}
	// Statistik wird ohne Consent nicht gezählt, mit Consent gezählt
	if _, html := body(t, c, ts.URL+"/login"); !strings.Contains(html, "gesamt 0") {
		t.Error("Statistik ohne Einwilligung gezählt")
	}
}
