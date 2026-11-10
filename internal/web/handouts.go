package web

import (
	"net/http"
	"time"

	"forge.gpl-nas.de/physiobase/internal/store"
)

func (s *Server) listHandouts(w http.ResponseWriter, r *http.Request) {
	hs, err := s.st.Handouts(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "handouts", http.StatusOK, map[string]any{"Handouts": hs})
}

func (s *Server) createHandout(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	ctx := r.Context()
	title := formText(r, "title")
	if title == "" {
		title = "Mein Übungsprogramm"
	}
	id, err := s.st.CreateHandoutWithNotes(ctx, title, formText(r, "recipient"), s.text("handout_default_notes"), 0)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Direkt mit einer Übung starten (Knopf auf der Übungsseite)
	if eid, ok := formID(r, "entry"); ok {
		if err := s.st.AddHandoutItem(ctx, id, eid); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	redirect(w, r, "/handout/%d", id)
}

func (s *Server) showHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	h, err := s.st.Handout(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.st.HandoutItems(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	exercises, err := s.st.ExercisesNotIn(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pid, pname, err := s.st.HandoutPatient(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "handout", http.StatusOK, map[string]any{
		"H": h, "Items": items, "Exercises": exercises,
		"PatientID": pid, "PatientName": pname,
	})
}

func (s *Server) updateHandout(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	h := store.Handout{ID: id, Title: formText(r, "title"), Recipient: formText(r, "recipient"), Notes: formText(r, "notes")}
	if h.Title == "" {
		h.Title = "Mein Übungsprogramm"
	}
	if err := s.st.UpdateHandout(r.Context(), h); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handout/%d", id)
}

func (s *Server) deleteHandout(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.st.DeleteHandout(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handouts")
}

func (s *Server) copyHandout(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	nid, err := s.st.CopyHandout(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handout/%d", nid)
}

func (s *Server) addHandoutItem(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok1 := pathID(r, "id")
	eid, ok2 := formID(r, "entry")
	if !ok1 || !ok2 {
		http.Error(w, "Bitte eine Übung auswählen", http.StatusBadRequest)
		return
	}
	if err := s.st.AddHandoutItem(r.Context(), id, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	if r.FormValue("from") == "entry" {
		redirect(w, r, "/eintrag/%d?handout=%d", eid, id)
		return
	}
	redirect(w, r, "/handout/%d", id)
}

func (s *Server) itemIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	id, ok1 := pathID(r, "id")
	eid, ok2 := pathID(r, "eid")
	if !ok1 || !ok2 {
		http.NotFound(w, r)
		return 0, 0, false
	}
	return id, eid, true
}

func (s *Server) updateHandoutItem(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, eid, ok := s.itemIDs(w, r)
	if !ok {
		return
	}
	if err := s.st.UpdateHandoutItem(r.Context(), id, eid, formText(r, "dosage"), formText(r, "note")); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handout/%d#u%d", id, eid)
}

func (s *Server) moveHandoutItem(w http.ResponseWriter, r *http.Request) {
	id, eid, ok := s.itemIDs(w, r)
	if !ok {
		return
	}
	if err := s.st.MoveHandoutItemUp(r.Context(), id, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handout/%d#u%d", id, eid)
}

func (s *Server) removeHandoutItem(w http.ResponseWriter, r *http.Request) {
	id, eid, ok := s.itemIDs(w, r)
	if !ok {
		return
	}
	if err := s.st.RemoveHandoutItem(r.Context(), id, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/handout/%d", id)
}

func (s *Server) printHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	h, err := s.st.Handout(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items, err := s.st.HandoutItems(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "print", http.StatusOK, map[string]any{"H": h, "Items": items, "Today": time.Now()})
}

// addToHandout: von der Übungsseite aus, Handout per Auswahlliste (ohne JS).
func (s *Server) addToHandout(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	hid, ok1 := formID(r, "handout")
	eid, ok2 := formID(r, "entry")
	if !ok1 || !ok2 {
		http.Error(w, "Bitte Handout und Übung auswählen", http.StatusBadRequest)
		return
	}
	if err := s.st.AddHandoutItem(r.Context(), hid, eid); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/eintrag/%d?handout=%d#handout", eid, hid)
}
