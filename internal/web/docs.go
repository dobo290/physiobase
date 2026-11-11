package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"forge.gpl-nas.de/physiobase/internal/store"
)

const maxDocUpload = 300 << 20 // 300 MB pro Upload (mehrere Scans auf einmal)

// Scans und Fotos. HEIC/TIFF werden gespeichert, aber vom Browser meist nicht angezeigt (→ Download).
var docExt = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
	".heic": true, ".heif": true, ".tif": true, ".tiff": true,
}

func isPreviewable(f string) bool { return isImage(f) }

func (s *Server) uploadDocs(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Demo {
		http.Error(w, "Im Demo sind Uploads deaktiviert.", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := s.st.Patient(ctx, pid); err != nil {
		s.fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDocUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil { //nolint:gosec // G120: Größe oben per MaxBytesReader begrenzt
		http.Error(w, "Upload fehlgeschlagen (max. 300 MB auf einmal)", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "Keine Datei ausgewählt", http.StatusBadRequest)
		return
	}
	title, category, date := formText(r, "title"), formText(r, "category"), validDate(formText(r, "date"))
	for i, fh := range files {
		ext := extOf(fh.Filename)
		if !docExt[ext] {
			http.Error(w, "Dateityp nicht erlaubt: "+fh.Filename+" (PDF oder Foto)", http.StatusBadRequest)
			return
		}
		name := fmt.Sprintf("%d-%s%s", pid, randomHex(10), ext)
		if err := saveUpload(s.cfg.DocsDir, fh.Open, name); err != nil {
			s.fail(w, r, err)
			return
		}
		t := title
		switch {
		case t == "":
			t = strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
		case len(files) > 1:
			t = fmt.Sprintf("%s (%d/%d)", title, i+1, len(files))
		}
		d := store.Doc{PatientID: pid, Filename: name, Original: filepath.Base(fh.Filename), Title: t, Category: category, Date: date, Size: fh.Size}
		if err := s.st.AddDoc(ctx, d); err != nil {
			os.Remove(filepath.Join(s.cfg.DocsDir, name)) //nolint:gosec // G703: Name vom Server erzeugt
			s.fail(w, r, err)
			return
		}
	}
	redirect(w, r, "/patient/%d#dokumente", pid)
}

func (s *Server) deleteDoc(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	d, err := s.st.DeleteDoc(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.removeDocFile(d.Filename)
	redirect(w, r, "/patient/%d#dokumente", d.PatientID)
}

func (s *Server) removeDocFile(name string) {
	err := os.Remove(filepath.Join(s.cfg.DocsDir, name)) //nolint:gosec // G703: Name stammt aus der Datenbank
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Warn("dokument nicht gelöscht", "datei", name, "err", err)
	}
}

func (s *Server) serveDoc(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	d, err := s.st.DocByFile(r.Context(), name)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Gesundheitsdaten: nicht in geteilten Caches ablegen
	w.Header().Set("Cache-Control", "private, no-store")
	disposition := "inline"
	if !isPreviewable(name) && extOf(name) != ".pdf" {
		disposition = "attachment" // HEIC/TIFF kann der Browser meist nicht anzeigen
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename*=UTF-8''%s`, disposition, urlPathEscape(d.Original)))
	http.ServeFile(w, r, filepath.Join(s.cfg.DocsDir, name)) //nolint:gosec // G703: Name geprüft und in der DB bekannt
}
