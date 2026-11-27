package web

import (
	"encoding/json"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

// Das Manifest nennt Praxisnamen, Start-URL und Icons; die Icons kommen als PNG.
func TestPWAManifestAndIcons(t *testing.T) {
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
	if m.Name == "" || m.StartURL != "/" || m.Display != "standalone" {
		t.Errorf("manifest unvollständig: %+v", m)
	}
	if len(m.Icons) < 2 {
		t.Fatalf("%d Icons", len(m.Icons))
	}
	sizes := map[string]bool{}
	for _, i := range m.Icons {
		sizes[i.Sizes] = true
	}
	if !sizes["192x192"] || !sizes["512x512"] {
		t.Errorf("Icon-Größen = %v", sizes)
	}

	for _, tc := range []struct {
		path string
		want int
	}{{"/pwa/icon/192.png", 192}, {"/pwa/icon/512.png", 512}, {"/pwa/icon/180.png", 180}} {
		r, err := c.Get(ts.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != http.StatusOK {
			t.Errorf("%s: %d", tc.path, r.StatusCode)
			r.Body.Close()
			continue
		}
		img, err := png.Decode(r.Body)
		r.Body.Close()
		if err != nil {
			t.Errorf("%s: kein PNG: %v", tc.path, err)
			continue
		}
		if b := img.Bounds(); b.Dx() != tc.want || b.Dy() != tc.want {
			t.Errorf("%s: %dx%d, erwartet %dx%d", tc.path, b.Dx(), b.Dy(), tc.want, tc.want)
		}
	}

	// Unbekannte Größe ist ein 404, kein generiertes Bild.
	if r, err := c.Get(ts.URL + "/pwa/icon/99.png"); err != nil || r.StatusCode != http.StatusNotFound {
		t.Errorf("icon-99: %v %v", r, err)
	}
}

// Layout und CSP müssen die Installation erlauben.
func TestPWAWiring(t *testing.T) {
	ts, c := newServer(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/login", nil)
	res, err := c.Do(req)
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
