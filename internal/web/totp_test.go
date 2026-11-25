package web

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"forge.gpl-nas.de/physiobase/internal/otp"
)

var (
	secretRE = regexp.MustCompile(`class="mono secret">([A-Z2-7]+)<`)
	rcodeRE  = regexp.MustCompile(`<li class="mono">([a-z0-9]{4}-[a-z0-9]{4})</li>`)
)

// logout meldet ab (Fehler werden im Test nicht gebraucht).
func logout(t *testing.T, c *http.Client, base string) {
	t.Helper()
	resp := doPost(t, c, base+"/logout", url.Values{})
	resp.Body.Close()
}

// doPost schickt ein Formular und schließt nichts – der Aufrufer entscheidet.
func doPost(t *testing.T, c *http.Client, u string, v url.Values) *http.Response {
	t.Helper()
	r, err := post(c, u, v)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// codeAt liefert den aktuellen Code zum Geheimnis aus dem Konto-Formular.
func codeAt(t *testing.T, secret string, steps int64) string {
	t.Helper()
	code, err := otp.Code(secret, otp.Step(time.Now())+steps)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// startSessionLogin meldet mit Passwort an und liefert den Status der Antwort.
func loginStatus(t *testing.T, c *http.Client, base, user, pw string) *http.Response {
	t.Helper()
	r, err := post(c, base+"/login", url.Values{"username": {user}, "password": {pw}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// submit2FA schickt den Code und weicht dem Sekundenwechsel aus: passt der Code
// der aktuellen Periode nicht, wird der der nächsten probiert.
func submit2FA(t *testing.T, c *http.Client, base string, form url.Values, secret string) *http.Response {
	t.Helper()
	for _, steps := range []int64{0, 1} {
		v := url.Values{}
		for k, vs := range form {
			v[k] = vs
		}
		v.Set("code", codeAt(t, secret, steps))
		r, err := post(c, base+"/login/2fa", v)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode == http.StatusSeeOther {
			return r
		}
	}
	t.Fatal("Code wurde in keiner Periode akzeptiert")
	return nil
}

func TestTwoFactorFlow(t *testing.T) {
	ts, c := newServer(t)
	base := ts.URL
	loginStatus(t, c, base, "admin", "geheim123").Body.Close()

	// ── Einrichtung ────────────────────────────────────────────────────────

	code, page := body(t, c, base+"/konto")
	if code != http.StatusOK || !strings.Contains(page, "Zwei-Faktor-Anmeldung") {
		t.Fatalf("Konto-Seite: %d", code)
	}
	if strings.Contains(page, "/konto/2fa/qr.svg") {
		t.Error("QR-Code vor dem Start sichtbar")
	}

	r, _ := post(c, base+"/konto/2fa/start", url.Values{})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Einrichtung starten: %d", r.StatusCode)
	}
	_, page = body(t, c, base+"/konto")
	m := secretRE.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("Geheimnis fehlt auf der Kontoseite")
	}
	secret := m[1]
	if _, err := otp.SecretBytes(secret); err != nil {
		t.Fatalf("Geheimnis nicht dekodierbar: %v", err)
	}
	if !strings.Contains(page, "/konto/2fa/qr.svg") {
		t.Error("QR-Code fehlt")
	}
	if code, svg := body(t, c, base+"/konto/2fa/qr.svg"); code != http.StatusOK || !strings.HasPrefix(svg, "<svg") {
		t.Errorf("QR-SVG: %d", code)
	}

	// Falscher Code schaltet nicht scharf.
	r, _ = post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {"000000"}})
	if r.StatusCode != http.StatusSeeOther || !strings.Contains(r.Header.Get("Location"), "fehler") {
		t.Errorf("falscher Code: %d %s", r.StatusCode, r.Header.Get("Location"))
	}

	// Richtiger Code schaltet scharf und zeigt die Wiederherstellungscodes.
	r, _ = post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {codeAt(t, secret, 0)}})
	if r.StatusCode == http.StatusSeeOther && strings.Contains(r.Header.Get("Location"), "fehler") {
		// Sekundenwechsel: mit dem Code der nächsten Periode erneut versuchen.
		r, _ = post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {codeAt(t, secret, 1)}})
	}
	final := readBody(r)
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("Aktivieren: %d", r.StatusCode)
	}
	codes := rcodeRE.FindAllStringSubmatch(final, -1)
	if len(codes) != recoveryCodeNum {
		t.Fatalf("Wiederherstellungscodes: %d, erwartet %d", len(codes), recoveryCodeNum)
	}
	recovery := codes[0][1]
	if !strings.Contains(final, "Aktiv") {
		t.Error("Status „Aktiv“ fehlt")
	}

	// ── Anmeldung mit zweitem Faktor ───────────────────────────────────────

	logout(t, c, base)
	if r := loginStatus(t, c, base, "admin", "geheim123"); r.StatusCode != http.StatusSeeOther ||
		r.Header.Get("Location") != "/login/2fa" {
		t.Fatalf("Login mit 2FA: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	// Vor dem Code gibt es keine Sitzung.
	if code, _ := body(t, c, base+"/"); code != http.StatusSeeOther {
		t.Errorf("Zugriff ohne Code: %d", code)
	}
	if code, form := body(t, c, base+"/login/2fa"); code != http.StatusOK || !strings.Contains(form, `name="code"`) {
		t.Errorf("2FA-Formular: %d", code)
	}
	if r, _ := post(c, base+"/login/2fa", url.Values{"code": {"000000"}}); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("falscher Code: %d, erwartet 401", r.StatusCode)
	}
	submit2FA(t, c, base, url.Values{}, secret).Body.Close()
	if code, _ := body(t, c, base+"/"); code != http.StatusOK {
		t.Errorf("nach Code kein Zugriff: %d", code)
	}

	// ── Wiederherstellungscode ─────────────────────────────────────────────

	logout(t, c, base)
	loginStatus(t, c, base, "admin", "geheim123").Body.Close()
	r, _ = post(c, base+"/login/2fa", url.Values{"recovery": {recovery}})
	if r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != "/" {
		t.Fatalf("Wiederherstellungscode: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	logout(t, c, base)
	loginStatus(t, c, base, "admin", "geheim123").Body.Close()
	if r, _ := post(c, base+"/login/2fa", url.Values{"recovery": {recovery}}); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("verbrauchter Code wurde akzeptiert: %d", r.StatusCode)
	}

	// ── Abschalten durch den Admin ─────────────────────────────────────────

	submit2FA(t, c, base, url.Values{}, secret).Body.Close()
	r, _ = post(c, base+"/admin/benutzer/1/2fa", url.Values{})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Admin-Reset: %d", r.StatusCode)
	}
	logout(t, c, base)
	if r := loginStatus(t, c, base, "admin", "geheim123"); r.Header.Get("Location") != "/" {
		t.Errorf("nach Admin-Reset: %s", r.Header.Get("Location"))
	}
}

func TestTwoFactorDisableNeedsPassword(t *testing.T) {
	ts, c := newServer(t)
	base := ts.URL
	loginStatus(t, c, base, "admin", "geheim123").Body.Close()

	doPost(t, c, base+"/konto/2fa/start", url.Values{}).Body.Close()
	_, page := body(t, c, base+"/konto")
	m := secretRE.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("Geheimnis fehlt")
	}
	r, _ := post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {codeAt(t, m[1], 0)}})
	if r.StatusCode == http.StatusSeeOther {
		r, _ = post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {codeAt(t, m[1], 1)}})
	}
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("Aktivieren: %d", r.StatusCode)
	}

	// Ohne richtiges Passwort bleibt die Anmeldung aktiv.
	r, _ = post(c, base+"/konto/2fa/loeschen", url.Values{"current": {"falsch"}})
	if !strings.Contains(r.Header.Get("Location"), "fehler") {
		t.Errorf("falsches Passwort akzeptiert: %s", r.Header.Get("Location"))
	}
	_, page = body(t, c, base+"/konto")
	if !strings.Contains(page, "Aktiv") {
		t.Error("2FA wurde ohne richtiges Passwort abgeschaltet")
	}

	// Mit Passwort abschalten.
	r, _ = post(c, base+"/konto/2fa/loeschen", url.Values{"current": {"geheim123"}})
	if r.StatusCode != http.StatusSeeOther || strings.Contains(r.Header.Get("Location"), "fehler") {
		t.Fatalf("Abschalten: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	_, page = body(t, c, base+"/konto")
	if strings.Contains(page, "Aktiv.") {
		t.Error("2FA ist noch aktiv")
	}
	logout(t, c, base)
	if r := loginStatus(t, c, base, "admin", "geheim123"); r.Header.Get("Location") != "/" {
		t.Errorf("Login ohne 2FA: %s", r.Header.Get("Location"))
	}
}

// Das Zwischen-Cookie darf ohne gültige Signatur keine Sitzung erzeugen.
func TestTwoFactorPendingCookieIsSigned(t *testing.T) {
	ts, c := newServer(t)
	base := ts.URL
	loginStatus(t, c, base, "admin", "geheim123").Body.Close()
	doPost(t, c, base+"/konto/2fa/start", url.Values{}).Body.Close()
	_, page := body(t, c, base+"/konto")
	m := secretRE.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("Geheimnis fehlt")
	}
	r, _ := post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {codeAt(t, m[1], 0)}})
	if r.StatusCode == http.StatusSeeOther {
		r, _ = post(c, base+"/konto/2fa/bestaetigen", url.Values{"code": {codeAt(t, m[1], 1)}})
	}
	r.Body.Close()
	logout(t, c, base)
	loginStatus(t, c, base, "admin", "geheim123").Body.Close()

	// Ein gefälschtes Zwischen-Cookie führt zurück zum Login.
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	c.Jar.SetCookies(u, []*http.Cookie{
		{Name: "physiobase_2fa", Value: "1.9999999999.deadbeef", Path: "/"},
	})
	res, err := c.Get(base + "/login/2fa")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("gefälschtes Cookie: %d, erwartet Redirect", res.StatusCode)
	}
}
