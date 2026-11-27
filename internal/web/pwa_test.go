package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Das Manifest nennt Praxisnamen, Start-URL und als Icon das eingerichtete Logo.
func TestPWAManifest(t *testing.T) {
	ts, c := newServer(t)

	res, err := c.Get(ts.URL + "/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("manifest: %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/manifest+json; charset=utf-8" {
		t.Errorf("content type = %q", ct)
	}
	var m struct {
		Name            string `json:"name"`
		ShortName       string `json:"short_name"`
		StartURL        string `json:"start_url"`
		Display         string `json:"display"`
		ThemeColor      string `json:"theme_color"`
		BackgroundColor string `json:"background_color"`
		Lang            string `json:"lang"`
		Icons           []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.Name == "" || m.StartURL != "/" || m.Display != "standalone" || m.Lang != "de" {
		t.Errorf("manifest unvollständig: %+v", m)
	}
	if len(m.Icons) < 2 || m.Icons[0].Src == "" {
		t.Fatalf("Icons = %+v", m.Icons)
	}
	if m.Icons[0].Src != "/static/logo.svg" {
		t.Errorf("Icon-Quelle = %q, erwartet das eingerichtete Logo", m.Icons[0].Src)
	}
	if m.Icons[0].Type != "image/svg+xml" {
		t.Errorf("Icon-Typ = %q", m.Icons[0].Type)
	}
	if m.Icons[1].Purpose != "maskable" {
		t.Errorf("zweites Icon = %+v", m.Icons[1])
	}

	// Das Logo muss ohne Anmeldung abrufbar sein, sonst kann der Browser nicht installieren.
	if r, err := c.Get(ts.URL + m.Icons[0].Src); err != nil || r.StatusCode != http.StatusOK {
		t.Errorf("Logo %s: %v %v", m.Icons[0].Src, r, err)
	} else {
		r.Body.Close()
	}
}

// Ein eigenes Rasterlogo erscheint mit seiner echten Größe im Manifest.
func TestPWAManifestUsesUploadedLogo(t *testing.T) {
	ts, c := newServer(t)
	login(t, c, ts.URL)
	// 1x1-PNG als Logo; der Upload-Helfer stammt aus den Einstellungs-Tests.
	pngBytes := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89" +
		"\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82"
	if loc := uploadLogo(t, c, ts.URL, "logo.png", pngBytes); loc == "" {
		t.Fatal("Logo-Upload ohne Redirect")
	}

	res, err := c.Get(ts.URL + "/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var m struct {
		Icons []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		} `json:"icons"`
	}
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if len(m.Icons) == 0 || !strings.Contains(m.Icons[0].Src, "/branding/logo") ||
		!strings.HasSuffix(strings.Split(m.Icons[0].Src, "?")[0], ".png") {
		t.Fatalf("Icon = %+v", m.Icons)
	}
	if m.Icons[0].Type != "image/png" || m.Icons[0].Sizes != "512x512" {
		t.Errorf("Icon-Typ/Größe = %q/%q", m.Icons[0].Type, m.Icons[0].Sizes)
	}
}

// Layout und CSP müssen die Installation erlauben.
func TestPWAWiring(t *testing.T) {
	ts, c := newServer(t)
	res, err := c.Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := make([]byte, 64<<10)
	n, _ := res.Body.Read(buf)
	page := string(buf[:n])
	for _, want := range []string{`rel="manifest"`, `rel="apple-touch-icon"`, "theme-color", "Zum Home-Bildschirm"} {
		if !strings.Contains(page, want) {
			t.Errorf("%q fehlt auf der Login-Seite", want)
		}
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "worker-src 'self'") {
		t.Errorf("CSP ohne worker-src: %s", csp)
	}
}
