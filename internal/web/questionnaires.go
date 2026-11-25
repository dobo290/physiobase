package web

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// quizEntry ist ein Fragebogen aus der Wissensdatenbank (Test + Schema).
type quizEntry struct {
	ID    int64
	Title string
	Q     befund.Questionnaire
}

func (s *Server) questionnaireEntries(ctx context.Context) ([]quizEntry, error) {
	es, err := s.st.QuestionnaireTests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]quizEntry, 0, len(es))
	for _, e := range es {
		if q, ok := befund.ParseQuestionnaire(e.Schema); ok {
			out = append(out, quizEntry{ID: e.ID, Title: e.Title, Q: q})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// prevScores: für jeden Fragebogen den letzten ausgefüllten Wert aus einem früheren
// Befund des Patienten (für die Anzeige beim Wiederbefund).
func (s *Server) prevScores(ctx context.Context, pid, curID int64) (map[int64]string, error) {
	bs, err := s.st.Befunde(ctx, pid)
	if err != nil {
		return nil, err
	}
	defs := map[int64]quizEntry{}
	qes, err := s.questionnaireEntries(ctx)
	if err != nil {
		return nil, err
	}
	for _, q := range qes {
		defs[q.ID] = q
	}
	out := map[int64]string{}
	// frühere Befunde, neuester zuerst; brutto der erste mit Fragebögen
	for _, b := range bs {
		if b.ID == curID {
			continue
		}
		d, _ := befund.Parse(b.Data)
		if len(d.Q) == 0 {
			continue
		}
		prev := d
		for _, id := range d.QList() {
			var eid int64
			if i, err := strconv.ParseInt(id, 10, 64); err == nil {
				eid = i
			}
			def, ok := defs[eid]
			if !ok {
				continue
			}
			fill := prev.Q[id]
			missing := fill.Missing(def.Q)
			txt := fmt.Sprintf("%d von %d", fill.Sum(), def.Q.MaxTotal())
			if dl := def.Q.DirectionLabel(); dl != "" {
				txt += " (" + dl + ")"
			}
			if missing > 0 {
				txt += " – unvollständig"
			}
			out[eid] = txt + " · " + deDate(b.Date)
		}
		return out, nil // nur der neueste frühere Befund mit Fragebögen
	}
	return out, nil
}

// qHistory für die Patientenseite: pro Befund und Fragebogen eine Zeile.
type qHistRow struct {
	Date, BefundTitle string
	BefundID          int64
	Name              string
	Sum, Max          int
	Dir               string // Anzeigetext Richtung
	Complete          bool
	Note              string
}

func (s *Server) questionnaireHistory(ctx context.Context, pid int64) ([]qHistRow, error) {
	bs, err := s.st.Befunde(ctx, pid)
	if err != nil {
		return nil, err
	}
	qes, err := s.questionnaireEntries(ctx)
	if err != nil {
		return nil, err
	}
	defs := make(map[int64]quizEntry, len(qes))
	for _, q := range qes {
		defs[q.ID] = q
	}
	return buildQuestionnaireHistory(bs, defs), nil
}

// buildQuestionnaireHistory ist die reine Auswertung hinter questionnaireHistory:
// je Befund und Fragebogen eine Zeile, chronologisch aufsteigend, dann nach Name.
// Ohne Datenbank und ohne HTTP, damit es sich einzeln testen lässt.
func buildQuestionnaireHistory(bs []store.Befund, defs map[int64]quizEntry) []qHistRow {
	var out []qHistRow
	for i := len(bs) - 1; i >= 0; i-- { // neueste zuerst geliefert, wir wollen älteste zuerst
		b := bs[i]
		d, err := befund.Parse(b.Data)
		if err != nil {
			continue
		}
		for _, id := range d.QList() {
			eid, err := strconv.ParseInt(id, 10, 64)
			if err != nil {
				continue
			}
			def, ok := defs[eid]
			if !ok {
				continue
			}
			fill := d.Q[id]
			out = append(out, qHistRow{
				Date: b.Date, BefundTitle: b.Title, BefundID: b.ID, Name: def.Title,
				Sum: fill.Sum(), Max: def.Q.MaxTotal(), Dir: def.Q.DirectionLabel(),
				Complete: fill.Complete(def.Q), Note: fill.Note,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// befundQuestionnaires liefert die Fragebögen, die im Befund angezeigt werden:
// die dem Patienten zugeordneten Tests plus die im Befund bereits ausgefüllten.
func (s *Server) befundQuestionnaires(ctx context.Context, linked []store.Entry, d befund.Data) ([]quizEntry, error) {
	all, err := s.questionnaireEntries(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]quizEntry, len(all))
	for _, q := range all {
		byID[q.ID] = q
	}
	seen := map[int64]bool{}
	out := make([]quizEntry, 0, len(all))
	add := func(id int64) {
		if seen[id] {
			return
		}
		if q, ok := byID[id]; ok {
			seen[id] = true
			out = append(out, q)
		}
	}
	for _, e := range linked {
		add(e.ID)
	}
	for _, idStr := range d.QList() {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			add(id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}
