package befund

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Questionnaire ist die Definition eines Punkt-Fragebogens, den ein Test (kind "test")
// tragen kann. items: Frage + maximale Punktzahl; direction: "", "lower" oder "higher".
type QItem struct {
	Q   string `json:"q"`
	Max int    `json:"max"`
}

type Questionnaire struct {
	Direction string  `json:"direction"`
	Items     []QItem `json:"items"`
}

// QFill sind die Antworten zu einem Fragebogen in einem spezifischen Befund.
// Values hält die Punktwerte (Zeichenfolgen, leere = nicht beantwortet), Note eine
// freie Anmerkung/Flag (z. B. „mit Unterarmgehstützen“ oder „angepasst“).
type QFill struct {
	Values []string `json:"values,omitempty"`
	Note   string   `json:"note,omitempty"`
}

// Q ist die Zuordnung Fragebogen-ID (Eintrags-ID als Zeichenkette) → Antworten.
var qKey = func(name string) string { return name }

// ParseQuestionnaire liest die Schema-Zeichenkette eines Tests. ok=false = kein Fragebogen.
func ParseQuestionnaire(s string) (Questionnaire, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" || s == "null" {
		return Questionnaire{}, false
	}
	q := Questionnaire{}
	if err := json.Unmarshal([]byte(s), &q); err != nil || len(q.Items) == 0 {
		return Questionnaire{}, false
	}
	// nur sinnvolle Einträge, positives Maximum, non-empty Frage
	clean := q.Items[:0:0]
	for _, it := range q.Items {
		if strings.TrimSpace(it.Q) == "" || it.Max < 1 {
			continue
		}
		clean = append(clean, it)
	}
	q.Items = clean
	if len(clean) == 0 {
		return Questionnaire{}, false
	}
	if q.Direction != "lower" && q.Direction != "higher" {
		q.Direction = ""
	}
	return q, true
}

// MarshalQuestionnaire schreibt die Definition als JSON (für entries.schema).
func MarshalQuestionnaire(q Questionnaire) string { //nolint:unused // wird von Testern benutzt
	b, _ := json.Marshal(q)
	return string(b)
}

// MaxTotal ist die Summe aller Einzelmaxima.
func (q Questionnaire) MaxTotal() int {
	n := 0
	for _, it := range q.Items {
		n += it.Max
	}
	return n
}

// DirectionLabel für die Anzeige hinter der Summe.
func (q Questionnaire) DirectionLabel() string {
	switch q.Direction {
	case "lower":
		return "niedriger ist besser"
	case "higher":
		return "höher ist besser"
	}
	return ""
}

// Sum addiert die beantworteten (numerischen) Werte.
func (f QFill) Sum() int {
	n := 0
	for _, v := range f.Values {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && i > 0 {
			n += i
		}
	}
	return n
}

// Missing zählt Einzelfragen ohne Antwort (nur bis zur Zahl der Fragen).
func (f QFill) Missing(q Questionnaire) int {
	missing := 0
	if len(q.Items) == 0 {
		return 0
	}
	for i := range q.Items {
		if len(f.Values) <= i || strings.TrimSpace(f.Values[i]) == "" {
			missing++
		}
	}
	return missing
}

func (f QFill) Complete(q Questionnaire) bool { return f.Missing(q) == 0 }

// QList liefert die Fragebogen-IDs sortiert (stabile Darstellung).
func (d Data) QList() []string {
	out := make([]string, 0, len(d.Q))
	for id := range d.Q {
		out = append(out, id)
	}
	// stabil numerisch sortieren
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && idNum(out[j]) < idNum(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func idNum(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return 1 << 30
}

var _ = qKey
