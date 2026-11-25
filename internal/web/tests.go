package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// apiSearchTests liefert Testvorschläge für das Suchfeld im Befund.
// GET /api/tests?q=lahm
func (s *Server) apiSearchTests(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	es, err := s.st.SearchTests(r.Context(), q, 15)
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
	if err := json.NewEncoder(w).Encode(out); err != nil {
		s.log.Error("api/tests: Schreiben fehlgeschlagen", "err", err)
	}
}
