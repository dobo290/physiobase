package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// testPoint ist eine einzelne Messung eines Tests (Summe eines Punkt-Fragebogens).
type testPoint struct {
	Date     string
	Sum      int
	Max      int
	Complete bool
	Note     string
	Source   string // "Befund" oder "Test"
	BefundID int64
}

// testSeries ist der Verlauf eines Tests: Punkte chronologisch aufsteigend,
// Kennzahlen für Trend und Vergleich zur vorigen Messung.
type testSeries struct {
	EntryID int64
	Title   string
	Max     int
	Dir     string // "niedriger ist besser" / "höher ist besser" / ""
	Points  []testPoint

	Has       bool   // mindestens ein Punkt
	Prev      int    // vorletzter Wert
	Last      int    // letzter Wert
	Delta     int    // Last - Prev
	Threshold int    // ab dieser Änderung gilt sie als klinisch relevant
	Trend     string // "up", "down", "flat"
	Better    bool   // die letzte Änderung geht in die bessere Richtung
	Relevant  bool   // |Delta| >= Threshold
}

// buildTestSeries sammelt alle Messungen eines Tests aus Einzeldurchführungen und
// Befunden. bs kommt neueste zuerst (so liefert es der Store).
func buildTestSeries(eid int64, title string, def quizEntry, runs []store.TestRun, bs []store.Befund) testSeries {
	s := testSeries{EntryID: eid, Title: title, Max: def.Q.MaxTotal(), Dir: def.Q.DirectionLabel()}
	for _, r := range runs {
		var f befund.QFill
		if err := json.Unmarshal([]byte(r.Data), &f); err != nil {
			continue
		}
		s.Points = append(s.Points, testPoint{
			Date: r.Date, Sum: f.Sum(), Max: s.Max, Complete: f.Complete(def.Q),
			Note: f.Note, Source: "Test",
		})
	}
	for _, b := range bs {
		d, err := befund.Parse(b.Data)
		if err != nil {
			continue
		}
		f, ok := d.Q[strconv.FormatInt(eid, 10)]
		if !ok {
			continue
		}
		s.Points = append(s.Points, testPoint{
			Date: b.Date, Sum: f.Sum(), Max: s.Max, Complete: f.Complete(def.Q),
			Note: f.Note, Source: "Befund", BefundID: b.ID,
		})
	}
	finishSeries(&s)
	return s
}

// finishSeries sortiert die Punkte, entfernt Doppel (gleicher Tag, gleicher Wert)
// und berechnet Trend und Vergleich zur vorigen Messung. Reine Logik, testbar.
func finishSeries(s *testSeries) {
	sort.SliceStable(s.Points, func(i, j int) bool {
		if s.Points[i].Date != s.Points[j].Date {
			return s.Points[i].Date < s.Points[j].Date
		}
		return s.Points[i].Sum < s.Points[j].Sum
	})
	// Gleicher Tag und gleicher Wert aus beiden Quellen: nur einmal zählen.
	uniq := s.Points[:0]
	for _, p := range s.Points {
		if n := len(uniq); n > 0 && uniq[n-1].Date == p.Date && uniq[n-1].Sum == p.Sum {
			continue
		}
		uniq = append(uniq, p)
	}
	s.Points = uniq
	s.Has = len(s.Points) > 0
	if !s.Has {
		return
	}
	s.Last = s.Points[len(s.Points)-1].Sum
	if s.Max <= 0 {
		s.Max = s.Last
	}
	s.Threshold = max(2, s.Max/10)
	if len(s.Points) < 2 {
		return
	}
	s.Prev = s.Points[len(s.Points)-2].Sum
	s.Delta = s.Last - s.Prev
	switch {
	case s.Delta > 0:
		s.Trend = "up"
	case s.Delta < 0:
		s.Trend = "down"
	default:
		s.Trend = "flat"
	}
	// Direction "lower" heißt: niedriger ist besser. "higher": höher ist besser.
	switch s.Dir {
	case "niedriger ist besser":
		s.Better = s.Delta < 0
	case "höher ist besser":
		s.Better = s.Delta > 0
	}
	s.Relevant = abs(s.Delta) >= s.Threshold
}

// testHistories baut für alle einem Patienten zugeordneten Tests den Verlauf.
func (s *Server) testHistories(ctx context.Context, pid int64, bs []store.Befund) ([]testSeries, error) {
	linked, err := s.st.PatientTests(ctx, pid)
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
	out := make([]testSeries, 0, len(linked))
	for _, e := range linked {
		def, ok := defs[e.ID]
		if !ok {
			continue // kein Punkt-Fragebogen: kein Verlauf
		}
		runs, err := s.st.TestRuns(ctx, pid, e.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, buildTestSeries(e.ID, e.Title, def, runs, bs))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out, nil
}

// seriesByEntry für Templates: Verlauf je Test-ID.
func seriesByEntry(ss []testSeries) map[int64]testSeries {
	m := make(map[int64]testSeries, len(ss))
	for _, s := range ss {
		m[s.EntryID] = s
	}
	return m
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ---------- SVG-Diagramm ----------

// chartSVG zeichnet den Verlauf als SVG. Server-gerendert, ohne JavaScript und
// ohne Inline-Styles (CSP): Farben als SVG-Attribute. spark=true → kleine Kurve
// ohne Achsen für die Tabelle, sonst ein Diagramm mit Achsen und Beschriftung.
func chartSVG(s testSeries, width, height int, spark bool) template.HTML {
	if len(s.Points) == 0 {
		return ""
	}
	const stroke = "#2c6fbb"
	good, bad := "#16a085", "#c0392b"
	lineColor := stroke
	if s.Has && s.Dir != "" && s.Relevant {
		if s.Better {
			lineColor = good
		} else {
			lineColor = bad
		}
	}
	padL, padR, padT, padB := 2, 2, 3, 3
	if !spark {
		padL, padR, padT, padB = 34, 6, 6, 18
	}
	plotW := float64(width - padL - padR)
	plotH := float64(height - padT - padB)
	maxV := s.Max
	for _, p := range s.Points {
		if p.Sum > maxV {
			maxV = p.Sum
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	x := func(i int) float64 {
		if len(s.Points) == 1 {
			return float64(padL) + plotW/2
		}
		return float64(padL) + plotW*float64(i)/float64(len(s.Points)-1)
	}
	y := func(v int) float64 {
		return float64(padT) + plotH*(1-float64(v)/float64(maxV))
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="%d" height="%d" class="chart" role="img" aria-hidden="true">`,
		width, height, width, height)
	if !spark {
		// Gitterlinien für 0, Mitte, Maximum
		for _, v := range []int{0, maxV / 2, maxV} {
			yy := y(v)
			fmt.Fprintf(&b, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#d5dbe1" stroke-width="1"/>`,
				padL, yy, width-padR, yy)
			fmt.Fprintf(&b, `<text x="%d" y="%.1f" text-anchor="end" font-size="9" fill="#5b6873">%d</text>`,
				padL-4, yy+3, v)
		}
		// Erste und letzte Datumsangabe
		first, last := s.Points[0].Date, s.Points[len(s.Points)-1].Date
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="start" font-size="9" fill="#5b6873">%s</text>`,
			padL, height-4, deISO(first))
		if last != first {
			fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="end" font-size="9" fill="#5b6873">%s</text>`,
				width-padR, height-4, deISO(last))
		}
	}
	// Kurve
	var path strings.Builder
	for i, p := range s.Points {
		if i == 0 {
			fmt.Fprintf(&path, "M%.1f %.1f", x(i), y(p.Sum))
		} else {
			fmt.Fprintf(&path, " L%.1f %.1f", x(i), y(p.Sum))
		}
	}
	fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="%d" stroke-linejoin="round" stroke-linecap="round"/>`,
		path.String(), lineColor, map[bool]int{true: 1, false: 2}[spark])
	// Punkte
	r := 2.4
	if spark {
		r = 1.6
	}
	for i, p := range s.Points {
		// Unvollständige Fragebögen hohl, vollständige gefüllt
		fill := "#ffffff"
		if p.Complete {
			fill = lineColor
		}
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" stroke="%s" stroke-width="1.2"/>`,
			x(i), y(p.Sum), r, fill, lineColor)
	}
	// Letzter Punkt hervorgehoben
	i := len(s.Points) - 1
	fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>`,
		x(i), y(s.Points[i].Sum), r+1.2, lineColor)
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) //nolint:gosec // G203: SVG wird ausschließlich aus Zahlen und festen Farben gebaut
}

// deISO formatiert 2026-03-01 als 01.03.2026 (kurz, ohne Jahr bei Diagrammplatz).
func deISO(iso string) string {
	if t, err := time.Parse("2006-01-02", iso); err == nil {
		return t.Format("02.01.06")
	}
	return iso
}

// trendLabel liefert den Text für die Trendanzeige in der Tabelle.
func trendLabel(s testSeries) string {
	if !s.Has || len(s.Points) < 2 {
		return ""
	}
	arrow := "→"
	switch s.Trend {
	case "up":
		arrow = "↑"
	case "down":
		arrow = "↓"
	}
	sign := ""
	if s.Delta > 0 {
		sign = "+"
	}
	txt := fmt.Sprintf("%s %s%d", arrow, sign, s.Delta)
	if s.Relevant {
		if s.Better {
			txt += " besser"
		} else {
			txt += " schlechter"
		}
	}
	return txt
}

// trendClass für die farbliche Kennzeichnung (CSS-Klasse).
func trendClass(s testSeries) string {
	if !s.Has || len(s.Points) < 2 {
		return "trend-flat"
	}
	if s.Relevant {
		if s.Better {
			return "trend-good"
		}
		return "trend-bad"
	}
	if s.Trend == "flat" {
		return "trend-flat"
	}
	return "trend-neutral"
}

// ---------- Seite: Verlauf eines Tests ----------

// testHistoryPage zeigt den Verlauf eines Tests mit Diagramm und Tabelle.
func (s *Server) testHistoryPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	eid, ok := pathID(r, "eid")
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, err := s.st.Patient(ctx, pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	e, err := s.st.Entry(ctx, eid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	bs, err := s.st.Befunde(ctx, pid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	runs, err := s.st.TestRuns(ctx, pid, eid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q, hasQ := befund.ParseQuestionnaire(e.Schema)
	def := quizEntry{ID: eid, Title: e.Title, Q: q}
	series := buildTestSeries(eid, e.Title, def, runs, bs)
	s.render(w, r, "test_history", http.StatusOK, map[string]any{
		"P": p, "E": e, "S": series, "HasQ": hasQ,
	})
}

// deltaOf liefert die Änderung des Punktes i gegenüber dem vorigen Punkt als Text.
func deltaOf(points []testPoint, i int) string {
	if i <= 0 || i >= len(points) {
		return ""
	}
	d := points[i].Sum - points[i-1].Sum
	if d == 0 {
		return "±0"
	}
	sign := "+"
	if d < 0 {
		sign = ""
	}
	return sign + strconv.Itoa(d)
}
