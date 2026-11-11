package web

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const logoSetting = "branding.logo"

func (s *Server) loadBranding(ctx context.Context) error {
	if err := s.loadTexts(ctx); err != nil {
		return err
	}
	logo, err := s.st.Setting(ctx, logoSetting)
	if err != nil {
		return err
	}
	if logo != "" && strings.EqualFold(extOf(logo), ".svg") {
		path := filepath.Join(s.brandingDir(), logo)
		if b, err := os.ReadFile(path); err == nil {
			if nb := normalizeSVG(b); len(nb) != len(b) {
				if err := os.WriteFile(path, nb, 0o640); err != nil { //nolint:gosec // G703: Name aus der DB
					s.log.Warn("logo-normalisierung", "err", err)
				}
			}
		}
	}
	s.textMu.Lock()
	s.logo = logo
	s.textMu.Unlock()
	return nil
}

type settingField struct {
	TextDef
	Value     string
	IsDefault bool
	DefText   string
}

type settingGroup struct {
	Name   string
	Fields []settingField
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	var groups []settingGroup
	for _, g := range textGroups {
		sg := settingGroup{Name: g}
		for _, d := range textDefs {
			if d.Group != g {
				continue
			}
			s.textMu.RLock()
			_, custom := s.texts[d.Key]
			s.textMu.RUnlock()
			sg.Fields = append(sg.Fields, settingField{TextDef: d, Value: s.rawText(d.Key), IsDefault: !custom, DefText: s.defaultText(d.Key)})
		}
		groups = append(groups, sg)
	}
	q := r.URL.Query()
	s.render(w, r, "settings", http.StatusOK, map[string]any{
		"Groups": groups, "LogoURL": s.logoURL(), "CustomLogo": s.rawLogo() != "",
		"Saved": q.Get("gespeichert") == "1", "Error": q.Get("fehler"),
	})
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formular ungültig oder zu groß", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	for _, d := range textDefs {
		if _, sent := r.PostForm["t."+d.Key]; !sent {
			continue
		}
		v := strings.TrimSpace(strings.ReplaceAll(r.PostForm.Get("t."+d.Key), "\r\n", "\n"))
		switch d.Kind {
		case "number":
			if n, err := strconv.Atoi(v); err != nil || n < 5 || n > 480 {
				v = ""
			}
		case "time":
			if _, err := time.Parse("15:04", v); err != nil {
				v = ""
			}
		}
		// Leer oder gleich dem Standard → Standard verwenden (eigene Fassung löschen)
		if v == "" || v == s.defaultText(d.Key) || r.PostForm.Get("reset."+d.Key) == "1" {
			v = ""
		}
		if err := s.st.SetSetting(ctx, textPrefix+d.Key, v); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	if err := s.loadTexts(ctx); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/admin/einstellungen?gespeichert=1")
}

func (s *Server) uploadLogo(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Demo {
		http.Error(w, "Im Demo sind Uploads deaktiviert.", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	if err := r.ParseMultipartForm(3 << 20); err != nil { //nolint:gosec // G120: per MaxBytesReader begrenzt
		redirect(w, r, "/admin/einstellungen?fehler=%s#logo", urlPathEscape("Datei zu groß (max. 2 MB)"))
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	f, fh, err := r.FormFile("logo")
	if err != nil {
		redirect(w, r, "/admin/einstellungen?fehler=%s#logo", urlPathEscape("Keine Datei ausgewählt"))
		return
	}
	defer func() { _ = f.Close() }()
	ext := extOf(fh.Filename)
	if _, ok := logoExt[ext]; !ok {
		redirect(w, r, "/admin/einstellungen?fehler=%s#logo", urlPathEscape("Erlaubt sind SVG, PNG, WebP und JPG"))
		return
	}
	b, err := io.ReadAll(io.LimitReader(f, 2<<20+1))
	if err != nil || len(b) > 2<<20 {
		redirect(w, r, "/admin/einstellungen?fehler=%s#logo", urlPathEscape("Datei zu groß (max. 2 MB)"))
		return
	}
	if ext == ".svg" {
		if err := checkSVG(b); err != nil {
			redirect(w, r, "/admin/einstellungen?fehler=%s#logo", urlPathEscape(err.Error()))
			return
		}
		// Fremde SVG-Größen (z. B. 2000×2000 px) auf viewBox-Normalisierung zurückführen,
		// damit das Logo automatisch dieselbe Größe wie das Standard-Logo hat.
		b = normalizeSVG(b)
	} else if ct := http.DetectContentType(b); !strings.HasPrefix(ct, "image/") {
		redirect(w, r, "/admin/einstellungen?fehler=%s#logo", urlPathEscape("Die Datei ist kein Bild"))
		return
	}
	if err := os.MkdirAll(s.brandingDir(), 0o750); err != nil {
		s.fail(w, r, err)
		return
	}
	name := "logo-" + randomHex(4) + ext
	if err := os.WriteFile(filepath.Join(s.brandingDir(), name), b, 0o640); err != nil { //nolint:gosec // G703: Name vom Server erzeugt
		s.fail(w, r, err)
		return
	}
	old := s.rawLogo()
	if err := s.st.SetSetting(ctx, logoSetting, name); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.loadBranding(ctx); err != nil {
		s.fail(w, r, err)
		return
	}
	if old != "" {
		os.Remove(filepath.Join(s.brandingDir(), old)) //nolint:gosec // G703: Name aus der Datenbank
	}
	redirect(w, r, "/admin/einstellungen?gespeichert=1#logo")
}

func (s *Server) deleteLogo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	old := s.rawLogo()
	if err := s.st.SetSetting(ctx, logoSetting, ""); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.loadBranding(ctx); err != nil {
		s.fail(w, r, err)
		return
	}
	if old != "" {
		os.Remove(filepath.Join(s.brandingDir(), old)) //nolint:gosec // G703: Name aus der Datenbank
	}
	redirect(w, r, "/admin/einstellungen?gespeichert=1#logo")
}
