package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// PWA: die App installierbar machen (Startseite ohne Adressleiste, eigenes Icon).
// Stage 1 – kein Service Worker, also noch kein Offline-Verhalten.
const (
	pwaThemeColor   = "#1f3b57"
	pwaBackground   = "#ffffff"
	pwaIconSizes    = "192x192"
	pwaIconLarge    = "512x512"
	pwaAppleIcon    = 180
	pwaManifestPath = "/manifest.webmanifest"
)

// pwaIconVersion zählt Logo-Änderungen mit, damit Browser das Icon neu laden.
func (s *Server) pwaIconVersion() string {
	name := s.rawLogo()
	if name == "" {
		return "default"
	}
	if fi, err := os.Stat(filepath.Join(s.brandingDir(), name)); err == nil {
		return strconv.FormatInt(fi.ModTime().Unix(), 10)
	}
	return "default"
}

// handleManifest liefert das Web-App-Manifest mit Praxisnamen und Icons.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	short := s.practice()
	if runes := []rune(short); len(runes) > 12 {
		short = string(runes[:12])
	}
	v := s.pwaIconVersion()
	manifest := map[string]any{
		"name":             s.practice(),
		"short_name":       short,
		"start_url":        "/",
		"scope":            "/",
		"display":          "standalone",
		"orientation":      "any",
		"background_color": pwaBackground,
		"theme_color":      pwaThemeColor,
		"lang":             "de",
		"icons": []map[string]string{
			{"src": "/pwa/icon/192.png?v=" + v, "sizes": pwaIconSizes, "type": "image/png", "purpose": "any"},
			{"src": "/pwa/icon/512.png?v=" + v, "sizes": pwaIconLarge, "type": "image/png", "purpose": "any"},
			{"src": "/pwa/icon/192.png?v=" + v, "sizes": pwaIconSizes, "type": "image/png", "purpose": "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		s.log.Warn("manifest schreiben", "err", err)
	}
}

// iconCache hält die erzeugten Icons im Speicher (Schlüssel: Größe + Logo-Version).
var iconCache sync.Map

// handleIcon liefert /pwa/icon/{size}.png – aus dem eigenen Logo, sonst ein
// neutrales Ersatzsymbol. Kein SVG-Rasterizer: bei SVG-Logos wird das Ersatzsymbol
// genommen, damit das Projekt ohne zusätzliche Abhängigkeit auskommt.
func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	sizeStr := strings.TrimSuffix(r.PathValue("file"), ".png")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || (size != pwaAppleIcon && size != 192 && size != 512) {
		http.NotFound(w, r)
		return
	}
	key := sizeStr + "-" + s.pwaIconVersion()
	if cached, ok := iconCache.Load(key); ok {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(cached.([]byte))
		return
	}
	img := s.iconImage(size)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		s.fail(w, r, err)
		return
	}
	iconCache.Store(key, buf.Bytes())
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(buf.Bytes())
}

// iconImage baut das Icon: eigenes Raster-Logo (PNG/JPEG/WebP) mittig beschnitten und
// skaliert, sonst ein neutrales Symbol mit Kreuz auf Markenfarbe.
func (s *Server) iconImage(size int) image.Image {
	if img, ok := s.logoRaster(); ok {
		return squareIcon(img, size)
	}
	return fallbackIcon(size)
}

// logoRaster lädt das eigene Logo, sofern es ein Rasterbild ist.
func (s *Server) logoRaster() (image.Image, bool) {
	name := s.rawLogo()
	if name == "" {
		return nil, false
	}
	switch extOf(name) {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return nil, false
	}
	f, err := os.Open(filepath.Join(s.brandingDir(), name)) //nolint:gosec // G304: Name aus der Datenbank, kein Nutzereingabepfad
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, false
	}
	return img, true
}

// squareIcon beschneidet das Logo mittig auf ein Quadrat und skaliert es auf size.
func squareIcon(src image.Image, size int) image.Image {
	b := src.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	offX := b.Min.X + (b.Dx()-side)/2
	offY := b.Min.Y + (b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	// Nächster-Nachbar-Skalierung reicht für Icons und spart eine Abhängigkeit.
	for y := range size {
		for x := range size {
			sx := offX + x*side/size
			sy := offY + y*side/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// fallbackIcon zeichnet ein schlichtes Symbol: Markenfarbe, weißes Kreuz.
func fallbackIcon(size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	brand := color.RGBA{R: 0x1f, G: 0x3b, B: 0x57, A: 0xff}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: brand}, image.Point{}, draw.Src)

	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	arm := size / 4 // halbe Kreuzstärke
	length := size / 2
	center := size / 2
	// waagerechter Balken
	draw.Draw(dst, image.Rect(center-length/2, center-arm/2, center+length/2, center+arm/2),
		&image.Uniform{C: white}, image.Point{}, draw.Src)
	// senkrechter Balken
	draw.Draw(dst, image.Rect(center-arm/2, center-length/2, center+arm/2, center+length/2),
		&image.Uniform{C: white}, image.Point{}, draw.Src)
	return dst
}
