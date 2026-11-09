package web

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/config"
	"forge.gpl-nas.de/physiobase/internal/database"
	"forge.gpl-nas.de/physiobase/internal/store"
)

func newServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close(db) })
	hash, _ := HashPassword("geheim123")
	cfg := &config.Config{Username: "admin", PasswordHash: hash, PracticeName: "Test", MediaDir: dir}
	st := store.New(db)
	if _, err := st.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv, err := New(context.Background(), cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return ts, c
}

func post(c *http.Client, u string, v url.Values) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodPost, u, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return c.Do(req)
}

func body(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	r, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b)
}

func TestAuthFlow(t *testing.T) {
	ts, c := newServer(t)
	if code, _ := body(t, c, ts.URL+"/"); code != http.StatusSeeOther {
		t.Fatalf("ohne Anmeldung: %d, erwartet Redirect", code)
	}
	if code, _ := body(t, c, ts.URL+"/healthz"); code != 200 {
		t.Fatalf("healthz: %d", code)
	}
	r, _ := post(c, ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"falsch"}})
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("falsches Passwort: %d", r.StatusCode)
	}
	r, _ = post(c, ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"geheim123"}})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %d", r.StatusCode)
	}
	code, html := body(t, c, ts.URL+"/?q=knie")
	if code != 200 || !strings.Contains(html, "Patellatendinopathie") {
		t.Fatalf("Suche nach Login: %d", code)
	}
	// Fremde Seite darf nichts posten (CSRF)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/eintrag/1/loeschen", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	r, _ = c.Do(req)
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site POST: %d, erwartet 403", r.StatusCode)
	}
	// Abmelden macht das alte Cookie ungültig
	post(c, ts.URL+"/logout", nil)
	if code, _ := body(t, c, ts.URL+"/"); code != http.StatusSeeOther {
		t.Fatal("nach Abmelden noch angemeldet")
	}
}

func TestPagesRender(t *testing.T) {
	ts, c := newServer(t)
	post(c, ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"geheim123"}})
	for _, p := range []string{"/", "/eintrag/1", "/eintrag/3", "/eintrag/1/bearbeiten", "/eintrag/neu?art=uebung", "/handouts", "/handout/1", "/handout/1/druck"} {
		if code, html := body(t, c, ts.URL+p); code != 200 || !strings.Contains(html, "</html>") {
			t.Errorf("%s: %d", p, code)
		}
	}
	if code, _ := body(t, c, ts.URL+"/eintrag/9999"); code != 404 {
		t.Errorf("unbekannter Eintrag: %d", code)
	}
	if code, _ := body(t, c, ts.URL+"/medien/..%2f..%2fetc%2fpasswd"); code != 404 {
		t.Errorf("Pfad-Trick: %d", code)
	}
	// Neuer Eintrag, verknüpft angelegt
	r, _ := post(c, ts.URL+"/eintrag/neu", url.Values{"kind": {"test"}, "title": {"Lachman-Test"}, "region": {"Knie"}, "link_from": {"1"}})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("anlegen: %d", r.StatusCode)
	}
	if _, html := body(t, c, ts.URL+"/eintrag/1"); !strings.Contains(html, "Lachman-Test") {
		t.Error("neuer Eintrag nicht verknüpft")
	}
	if _, html := body(t, c, ts.URL+"/handout/1/druck"); !strings.Contains(html, "Wandsitz") {
		t.Error("Handout-Druck ohne Übung")
	}
}

func TestPatientBefundFlow(t *testing.T) {
	ts, c := newServer(t)
	post(c, ts.URL+"/login", url.Values{"username": {"admin"}, "password": {"geheim123"}})

	r, _ := post(c, ts.URL+"/patienten", url.Values{"name": {"Max Test"}, "dob": {"1980"}})
	if r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != "/patient/1" {
		t.Fatalf("Patient anlegen: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	r, _ = post(c, ts.URL+"/patient/1/befund", url.Values{})
	if loc := r.Header.Get("Location"); loc != "/befund/1" {
		t.Fatalf("Befund anlegen: %s", loc)
	}
	if code, html := body(t, c, ts.URL+"/befund/1"); code != 200 || !strings.Contains(html, "Red Flags") || !strings.Contains(html, `name="t.rom.n"`) {
		t.Fatalf("Befundformular: %d", code)
	}
	r, _ = post(c, ts.URL+"/befund/1", url.Values{
		"title": {"Erstbefund"}, "date": {"2025-01-10"},
		"f.diag": {"Lumbalgie"}, "f.pain_now": {"7"}, "t.rom.n": {"1"}, "t.rom.0.joint": {"LWS"}, "t.rom.0.move": {"Flex"}, "t.rom.0.act": {"40"},
		"stamps": {`[{"v":1,"x":0.5,"y":0.45,"s":"pain","c":"#d32f2f","z":2}]`},
	})
	if r.StatusCode != http.StatusSeeOther {
		t.Fatalf("Befund speichern: %d", r.StatusCode)
	}
	// Wiederbefund als Kopie, dann Wert ändern und vergleichen
	r, _ = post(c, ts.URL+"/patient/1/befund", url.Values{"kopie": {"1"}})
	if loc := r.Header.Get("Location"); loc != "/befund/2" {
		t.Fatalf("Wiederbefund: %s", loc)
	}
	if _, html := body(t, c, ts.URL+"/befund/2"); !strings.Contains(html, "Lumbalgie") || !strings.Contains(html, "Wiederbefund") {
		t.Fatal("Kopie hat Daten nicht übernommen")
	}
	post(c, ts.URL+"/befund/2", url.Values{
		"title": {"Wiederbefund"}, "date": {"2025-02-10"}, "f.diag": {"Lumbalgie"}, "f.pain_now": {"3"},
		"t.rom.n": {"1"}, "t.rom.0.joint": {"LWS"}, "t.rom.0.move": {"Flex"}, "t.rom.0.act": {"55"},
	})
	code, html := body(t, c, ts.URL+"/patient/1/vergleich")
	if code != 200 || !strings.Contains(html, "&#43;15") || !strings.Contains(html, "NRS aktuell") {
		t.Fatalf("Vergleich: %d\n%s", code, html[strings.Index(html, "<main"):])
	}
	if _, html := body(t, c, ts.URL+"/befund/1/druck"); !strings.Contains(html, "Lumbalgie") || !strings.Contains(html, "<text") || strings.Contains(html, "Fachbereich") {
		t.Error("Druckansicht unvollständig")
	}
	// Verlauf, Handout, Wissen
	post(c, ts.URL+"/patient/1/verlauf", url.Values{"date": {"2025-01-12"}, "goal": {"Bewegungsangst reduzieren"}, "note": {"gut"}})
	post(c, ts.URL+"/patient/1/wissen", url.Values{"entry": {"1"}})
	r, _ = post(c, ts.URL+"/patient/1/handout", url.Values{"title": {"Woche 1"}})
	if !strings.HasPrefix(r.Header.Get("Location"), "/handout/") {
		t.Fatal("Handout für Patient")
	}
	_, html = body(t, c, ts.URL+"/patient/1")
	for _, want := range []string{"Bewegungsangst", "Woche 1", "Patellatendinopathie", "Verlauf der Kernwerte"} {
		if !strings.Contains(html, want) {
			t.Errorf("Patientenseite ohne %q", want)
		}
	}
	// Export → Import ergibt denselben Patienten mit drittem Befund
	resp, _ := c.Get(ts.URL + "/befund/2/export")
	exp, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if code := importFiles(t, c, ts.URL, exp); code != http.StatusSeeOther {
		t.Fatalf("Import Export-Datei: %d", code)
	}
	if _, html := body(t, c, ts.URL+"/patient/1"); strings.Count(html, `href="/befund/`) < 3 {
		t.Error("Import nicht beim selben Patienten")
	}
	// Alte Befundseite
	old := `{"name":"Erika Alt","date":"2024-12-01","diag":"Schulter","rom":[{"joint":"Schulter","move":"Abd","act":"90","pass":"120","end":""}],"treat":[{"date":"2024-12-02","goal":"Mobilisation","note":""}],"_v":2}`
	if code := importFiles(t, c, ts.URL, []byte(old)); code != http.StatusSeeOther {
		t.Fatalf("Import alt: %d", code)
	}
	if _, html := body(t, c, ts.URL+"/patienten"); !strings.Contains(html, "Erika Alt") {
		t.Error("importierter Patient fehlt")
	}
}

func importFiles(t *testing.T, c *http.Client, base string, files ...[]byte) int {
	t.Helper()
	var buf strings.Builder
	w := multipart.NewWriter(&buf)
	for i, f := range files {
		fw, _ := w.CreateFormFile("files", fmt.Sprintf("Befund_%d.json", i))
		fw.Write(f)
	}
	w.Close()
	req, _ := http.NewRequest(http.MethodPost, base+"/patienten/import", strings.NewReader(buf.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	r, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}
