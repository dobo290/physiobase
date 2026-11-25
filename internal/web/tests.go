package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// apiSearchTests liefert Testvorschläge für das Suchfeld in Befund und
// Patientenübersicht. Ohne q kommen alle Tests (für die Liste beim Klick ins
// Feld), mit q wird nach dem Titelanfang gefiltert.
// GET /api/tests?q=lahm&limit=15
func (s *Server) apiSearchTests(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 15
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	es, err := s.st.SearchTests(r.Context(), q, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	type item struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	out := make([]item, 0, len(es))
	for _, e := range es {
		out = append(out, item{ID: e.ID, Title: e.Title})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		s.log.Error("api/tests: Schreiben fehlgeschlagen", "err", err)
	}
}
