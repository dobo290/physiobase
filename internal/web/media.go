package web

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"forge.gpl-nas.de/physiobase/internal/store"
)

const maxUpload = 1 << 30 // 1 GB pro Upload (Videos)

var (
	imageExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	videoExt = map[string]bool{".mp4": true, ".webm": true, ".mov": true, ".m4v": true}
)

func extOf(f string) string { return strings.ToLower(filepath.Ext(f)) }
func isImage(f string) bool { return imageExt[extOf(f)] }
func isVideo(f string) bool { return videoExt[extOf(f)] }

func allowedUpload(f string) bool {
	e := extOf(f)
	return imageExt[e] || videoExt[e] || e == ".pdf"
}

func (s *Server) uploadMedia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := s.st.Entry(ctx, id); err != nil {
		s.fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	// Nur 32 MB im Speicher, der Rest landet in Temp-Dateien
	if err := r.ParseMultipartForm(32 << 20); err != nil { //nolint:gosec // G120: Größe oben per MaxBytesReader begrenzt
		http.Error(w, "Upload fehlgeschlagen (max. 1 GB)", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	caption := formText(r, "caption")
	for _, fh := range r.MultipartForm.File["files"] {
		if !allowedUpload(fh.Filename) {
			http.Error(w, "Dateityp nicht erlaubt: "+fh.Filename+" (Bilder, Videos, PDF)", http.StatusBadRequest)
			return
		}
		name := fmt.Sprintf("%d-%s%s", id, randomHex(8), extOf(fh.Filename))
		if err := s.saveUpload(fh.Open, name); err != nil {
			s.fail(w, r, err)
			return
		}
		c := caption
		if c == "" {
			c = strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
		}
		if err := s.st.AddMedia(ctx, store.Media{EntryID: id, Filename: name, Caption: c}); err != nil {
			os.Remove(filepath.Join(s.cfg.MediaDir, name)) //nolint:gosec // G703: Name vom Server erzeugt
			s.fail(w, r, err)
			return
		}
	}
	redirect(w, r, "/eintrag/%d#medien", id)
}

// saveUpload schreibt erst in eine Temp-Datei und benennt dann um, damit nie
// halbe Dateien unter dem endgültigen Namen liegen.
func (s *Server) saveUpload(open func() (multipart.File, error), name string) error {
	src, err := open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	tmp, err := os.CreateTemp(s.cfg.MediaDir, ".upload-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	// CreateTemp legt die Datei mit 0600 an; ausgeliefert wird sie nur über die App.
	return os.Rename(tmp.Name(), filepath.Join(s.cfg.MediaDir, name)) //nolint:gosec // G703: Name vom Server erzeugt
}

func (s *Server) deleteMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	m, err := s.st.DeleteMedia(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := os.Remove(filepath.Join(s.cfg.MediaDir, m.Filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Warn("medium nicht gelöscht", "datei", m.Filename, "err", err)
	}
	redirect(w, r, "/eintrag/%d#medien", m.EntryID)
}

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	// Nur Dateien ausliefern, die in der Datenbank stehen – kein Pfad-Trick möglich.
	if name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	known, err := s.st.MediaKnown(r.Context(), name)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !known {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	if extOf(name) == ".pdf" {
		w.Header().Set("Content-Disposition", "inline")
	}
	// ServeFile unterstützt Range-Requests → Videos sind spulbar.
	http.ServeFile(w, r, filepath.Join(s.cfg.MediaDir, name)) //nolint:gosec // G703: Name geprüft und in der DB bekannt
}
