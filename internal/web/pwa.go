package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
)

// PWA: die App installierbar machen (Startseite ohne Adressleiste, eigenes Icon).
// Stage 1 – kein Service Worker, also noch kein Offline-Verhalten.
const (
	pwaThemeColor   = "#1f3b57"
	pwaBackground   = "#ffffff"
	pwaManifestPath = "/manifest.webmanifest"
)

// pwaIcon ist das Icon im Manifest: das Logo, das die Praxis ohnehin eingerichtet hat.
// SVG wird als "any" angeboten (Chrome/Android), Rasterbilder mit ihrer echten Größe.
func (s *Server) pwaIcon() (src, sizes, typ string) {
	src = s.logoURL()
	name := s.rawLogo()
	if name == "" {
		name = "logo.svg" // mitgelieferter Platzhalter
	}
	typ = logoExt[extOf(filepath.Base(name))]
	if typ == "" {
		typ = "image/svg+xml"
	}
	sizes = "any"
	if typ == "image/png" || typ == "image/jpeg" || typ == "image/webp" {
		sizes = "512x512" // Rasterlogos werden vom Browser passend skaliert
	}
	return src, sizes, typ
}

// handleManifest liefert das Web-App-Manifest mit Praxisnamen und dem eingerichteten Logo.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	// Der Browser lädt das Manifest ohne Sitzung (Installationsangebot schon auf der
	// Anmeldeseite), deshalb steht es in publicPath().
	name := s.practice()
	short := name
	if runes := []rune(short); len(runes) > 12 {
		short = string(runes[:12])
	}
	src, sizes, typ := s.pwaIcon()
	manifest := map[string]any{
		"name":             name,
		"short_name":       short,
		"start_url":        "/",
		"scope":            "/",
		"display":          "standalone",
		"orientation":      "any",
		"background_color": pwaBackground,
		"theme_color":      pwaThemeColor,
		"lang":             "de",
		"icons": []map[string]string{
			{"src": src, "sizes": sizes, "type": typ, "purpose": "any"},
			{"src": src, "sizes": sizes, "type": typ, "purpose": "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		s.log.Warn("manifest schreiben", "err", err)
	}
}
