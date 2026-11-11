package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kind := r.URL.Query().Get("art")
	if !store.ValidKind(kind) {
		kind = ""
	}
	region := strings.TrimSpace(r.URL.Query().Get("region"))

	entries, err := s.st.Search(ctx, q, kind, region)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	counts, total, err := s.st.KindCounts(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	regions, err := s.st.Regions(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "index", http.StatusOK, map[string]any{
		"Entries": entries, "Q": q, "Art": kind, "Region": region,
		"Counts": counts, "Total": total, "Regions": regions,
	})
}

// questionnaireEditor liefert die Zeilen und Richtung für den Editor (Standard: 3 leere Zeilen).
func questionnaireEditor(schema string) (items []befund.QItem, direktion string) {
	if q, ok := befund.ParseQuestionnaire(schema); ok {
		return append([]befund.QItem(nil), q.Items...), q.Direction
	}
	return make([]befund.QItem, 3), ""
}

type group struct {
	Label   string
	Entries []store.Entry
}

func (s *Server) showEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	e, err := s.st.Entry(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	related, err := s.st.Related(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	linkable, err := s.st.Linkable(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	sources, err := s.st.Sources(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	media, err := s.st.Media(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var handouts []store.Handout
	if e.Kind == "uebung" {
		if handouts, err = s.st.Handouts(ctx); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	// Verknüpfte Einträge nach Art gruppieren, in fester Reihenfolge
	var groups []group
	for _, k := range store.Kinds {
		g := group{Label: k.Plural}
		for _, x := range related {
			if x.Kind == k.Key {
				g.Entries = append(g.Entries, x)
			}
		}
		if len(g.Entries) > 0 {
			groups = append(groups, g)
		}
	}
	data := map[string]any{
		"E": e, "Groups": groups, "Linkable": linkable, "Sources": sources,
		"Media": media, "Handouts": handouts, "Added": r.URL.Query().Get("handout"),
	}
	if e.Kind == "test" {
		q, ok := befund.ParseQuestionnaire(e.Schema)
		data["FB"], data["FBOn"] = q, ok
	}
	s.render(w, r, "entry", http.StatusOK, data)
}

func (s *Server) newEntry(w http.ResponseWriter, r *http.Request) {
	regions, err := s.st.Regions(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kind := r.URL.Query().Get("art")
	if !store.ValidKind(kind) {
		kind = "krankheitsbild"
	}
	items, dir := questionnaireEditor("")
	s.render(w, r, "edit", http.StatusOK, map[string]any{
		"E": store.Entry{Kind: kind, Region: r.URL.Query().Get("region")}, "New": true,
		"Regions": regions, "LinkFrom": r.URL.Query().Get("von"), "FItems": items, "FRichtung": dir,
	})
}

func (s *Server) editEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	e, err := s.st.Entry(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	regions, err := s.st.Regions(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, dir := questionnaireEditor(e.Schema)
	s.render(w, r, "edit", http.StatusOK, map[string]any{"E": e, "Regions": regions, "FItems": items, "FRichtung": dir})
}

func (s *Server) saveEntry(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	ctx := r.Context()
	e := store.Entry{
		Kind: formText(r, "kind"), Title: formText(r, "title"), Summary: formText(r, "summary"),
		Body: formText(r, "body"), Region: formText(r, "region"), Tags: formText(r, "tags"),
		Dosage: formText(r, "dosage"), PatientText: formText(r, "patient_text"),
	}
	// Fragebogen-Definition bei Tests: aktiv über Häkchen, Fragen als Tabelle (d.<i>.q / d.<i>.max)
	if e.Kind == "test" && r.FormValue("fragebogen_an") == "1" {
		n, _ := strconv.Atoi(formText(r, "d.n"))
		q := befund.Questionnaire{Direction: formText(r, "fragebogen_richtung")}
		for i := range min(n, 200) {
			frage := formText(r, fmt.Sprintf("d.%d.q", i))
			max, _ := strconv.Atoi(formText(r, fmt.Sprintf("d.%d.max", i)))
			if frage != "" && max >= 1 {
				q.Items = append(q.Items, befund.QItem{Q: frage, Max: max})
			}
		}
		if len(q.Items) == 0 {
			regions, _ := s.st.Regions(ctx)
			id, _ := pathID(r, "id")
			e.ID = id
			s.render(w, r, "edit", http.StatusBadRequest, map[string]any{
				"E": e, "New": id == 0, "Regions": regions, "LinkFrom": r.FormValue("link_from"),
				"Error":  "Bitte mindestens eine Frage mit positiver Maximalpunktzahl eingeben.",
				"FItems": q.Items, "FRichtung": q.Direction,
			})
			return
		}
		b, _ := json.Marshal(q)
		e.Schema = string(b)
	} else {
		e.Schema = ""
	}
	if !store.ValidKind(e.Kind) || e.Title == "" {
		regions, _ := s.st.Regions(ctx)
		id, _ := pathID(r, "id")
		e.ID = id
		s.render(w, r, "edit", http.StatusBadRequest, map[string]any{
			"E": e, "New": id == 0, "Regions": regions, "LinkFrom": r.FormValue("link_from"),
			"Error": "Bitte einen Titel angeben.",
		})
		return
	}
	if r.PathValue("id") == "" {
		id, err := s.st.CreateEntry(ctx, e)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		// "Neuen Eintrag verknüpft anlegen" vom Ausgangseintrag aus
		if from, ok := formID(r, "link_from"); ok {
			if err := s.st.Link(ctx, from, id); err != nil {
				s.fail(w, r, err)
				return
			}
		}
		redirect(w, r, "/eintrag/%d", id)
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	e.ID = id
	if err := s.st.UpdateEntry(ctx, e); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/eintrag/%d", id)
}

func (s *Server) deleteEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	files, err := s.st.DeleteEntry(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, f := range files {
		// Dateinamen stammen aus der Datenbank (vom Server erzeugt), nicht vom Benutzer.
		if err := os.Remove(filepath.Join(s.cfg.MediaDir, f)); err != nil && !os.IsNotExist(err) { //nolint:gosec // G703
			s.log.Warn("medium nicht gelöscht", "datei", f, "err", err)
		}
	}
	redirect(w, r, "/")
}

func (s *Server) linkEntry(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok1 := pathID(r, "id")
	other, ok2 := formID(r, "other")
	if !ok1 || !ok2 {
		http.Error(w, "Bitte einen Eintrag auswählen", http.StatusBadRequest)
		return
	}
	if err := s.st.Link(r.Context(), id, other); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/eintrag/%d#verknuepft", id)
}

func (s *Server) unlinkEntry(w http.ResponseWriter, r *http.Request) {
	id, ok1 := pathID(r, "id")
	other, ok2 := pathID(r, "other")
	if !ok1 || !ok2 {
		http.NotFound(w, r)
		return
	}
	if err := s.st.Unlink(r.Context(), id, other); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/eintrag/%d#verknuepft", id)
}

func (s *Server) addSource(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	src := store.Source{EntryID: id, Ref: formText(r, "ref"), Page: formText(r, "page"), URL: formText(r, "url"), Note: formText(r, "note")}
	// Nur http(s)-Links zulassen
	if src.URL != "" && !strings.HasPrefix(src.URL, "http://") && !strings.HasPrefix(src.URL, "https://") {
		src.URL = "https://" + src.URL
	}
	if src.Ref != "" {
		if err := s.st.AddSource(r.Context(), src); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	redirect(w, r, "/eintrag/%d#quellen", id)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	eid, err := s.st.DeleteSource(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/eintrag/%d#quellen", eid)
}
