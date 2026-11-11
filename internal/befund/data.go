package befund

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Row map[string]string

// Stamp ist ein Symbol im Körperschema. X/Y sind relativ (0..1), damit die
// Position unabhängig von Bildschirm- und Druckgröße stimmt.
type Stamp struct {
	V int     `json:"v"` // Ansicht (Index in Views)
	X float64 `json:"x"`
	Y float64 `json:"y"`
	S string  `json:"s"` // Symbol-Key
	C string  `json:"c"` // Farbe #rrggbb
	Z int     `json:"z"` // Größe 1..3
}

type Data struct {
	Fields map[string]string `json:"fields"`
	Tables map[string][]Row  `json:"tables"`
	Stamps []Stamp           `json:"stamps"`
	Draw   []Drawing         `json:"draw,omitempty"` // Freihandzeichnungen
	Q      map[string]QFill  `json:"q,omitempty"`
}

func New() Data {
	return Data{Fields: map[string]string{}, Tables: map[string][]Row{}, Q: map[string]QFill{}, Draw: []Drawing{}}
}

func Parse(raw string) (Data, error) {
	d := New()
	if strings.TrimSpace(raw) == "" {
		return d, nil
	}
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return New(), fmt.Errorf("befund-daten: %w", err)
	}
	if d.Fields == nil {
		d.Fields = map[string]string{}
	}
	if d.Tables == nil {
		d.Tables = map[string][]Row{}
	}
	if d.Q == nil {
		d.Q = map[string]QFill{}
	}
	if d.Draw == nil {
		d.Draw = []Drawing{}
	}
	return d, nil
}

func (d Data) JSON() string {
	b, _ := json.Marshal(d)
	return string(b)
}

func (d Data) Get(k string) string { return d.Fields[k] }

// HasSection: enthält die Sektion irgendeinen Wert? (Fachbereiche aufklappen, Druck filtern)
func (d Data) HasSection(s Section) bool {
	for _, p := range s.Parts {
		if d.HasPart(p) {
			return true
		}
	}
	return false
}

func (d Data) HasPart(p Part) bool {
	if p.Questionnaires && len(d.Q) > 0 {
		return true
	}
	for _, f := range p.Fields {
		if strings.TrimSpace(d.Fields[f.Key]) != "" {
			return true
		}
	}
	if p.Table != nil && len(d.Tables[p.Table.Key]) > 0 {
		return true
	}
	return p.Body && len(d.Stamps) > 0
}

// ---------- Formular → Daten ----------

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// FromForm liest das Befundformular. Tabellenzeilen heißen t.<tabelle>.<i>.<spalte>,
// komplett leere Zeilen werden verworfen.
func FromForm(form url.Values) Data {
	d := New()
	for k, f := range fieldIndex {
		v := strings.TrimSpace(strings.ReplaceAll(form.Get("f."+k), "\r\n", "\n"))
		if f.Type == "check" {
			if v != "" {
				d.Fields[k] = "1"
			}
			continue
		}
		if v != "" {
			d.Fields[k] = v
		}
	}
	for _, t := range Tables {
		n, _ := strconv.Atoi(form.Get("t." + t.Key + ".n"))
		n = min(n, 200)
		for i := range n {
			row := Row{}
			for _, c := range t.Cols {
				if v := strings.TrimSpace(form.Get(fmt.Sprintf("t.%s.%d.%s", t.Key, i, c.Key))); v != "" {
					row[c.Key] = v
				}
			}
			if len(row) > 0 {
				d.Tables[t.Key] = append(d.Tables[t.Key], row)
			}
		}
	}
	var stamps []Stamp
	if json.Unmarshal([]byte(form.Get("stamps")), &stamps) == nil {
		d.Stamps = CleanStamps(stamps)
	}
	// Freihandzeichnungen (wie Stempel, aus einem versteckten Feld)
	if json.Unmarshal([]byte(form.Get("drawings")), &d.Draw) == nil {
		d.Draw = CleanDrawings(d.Draw)
	}
	// Fragebögen: übernommen wird, wer eine Antwort oder eine Anmerkung hat.
	for _, id := range strings.Split(form.Get("q.list"), ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		n, _ := strconv.Atoi(form.Get("f.q." + id + ".n"))
		vals := []string{}
		for i := range min(n, 200) {
			v := strings.TrimSpace(form.Get(fmt.Sprintf("f.q.%s.%d", id, i)))
			if v != "" {
				vals = append(vals, v)
			}
		}
		note := strings.TrimSpace(form.Get("f.q." + id + ".note"))
		if len(vals) > 0 || note != "" {
			d.Q[id] = QFill{Values: vals, Note: note}
		}
	}
	return d
}

func CleanStamps(in []Stamp) []Stamp {
	known := map[string]bool{}
	for _, s := range Symbols {
		known[s.Key] = true
	}
	var out []Stamp
	for _, s := range in {
		if s.V < 0 || s.V >= len(Views) || !known[s.S] || math.IsNaN(s.X) || math.IsNaN(s.Y) ||
			s.X < 0 || s.X > 1 || s.Y < 0 || s.Y > 1 {
			continue
		}
		if !colorRe.MatchString(s.C) {
			s.C = "#d32f2f"
		}
		s.Z = max(1, min(3, s.Z))
		out = append(out, s)
		if len(out) == 500 {
			break
		}
	}
	return out
}

// ---------- Darstellung ----------

// TableRows liefert die Zeilen für das Formular plus leere Zeilen zum Ausfüllen.
func (d Data) TableRows(t *Table, blank int) []Row {
	rows := append([]Row{}, d.Tables[t.Key]...)
	for range blank {
		rows = append(rows, Row{})
	}
	return rows
}

type StampView struct {
	X, Y, Size        float64
	Char, Color, Name string
}

func symbol(key string) Symbol {
	for _, s := range Symbols {
		if s.Key == key {
			return s
		}
	}
	return Symbols[0]
}

// StampSize in viewBox-Einheiten (Bilder sind ca. 80 px breit).
func StampSize(z int) float64 { return []float64{0, 7, 11, 16}[max(1, min(3, z))] }

func (d Data) StampsFor(view int) []StampView {
	v := Views[view]
	var out []StampView
	for _, s := range d.Stamps {
		if s.V != view {
			continue
		}
		sym := symbol(s.S)
		out = append(out, StampView{X: s.X * float64(v.W), Y: s.Y * float64(v.H), Size: StampSize(s.Z), Char: sym.Char, Color: s.C, Name: sym.Name})
	}
	return out
}

// UsedSymbols für die Legende im Druck.
func (d Data) UsedSymbols() []Symbol {
	used := map[string]bool{}
	for _, s := range d.Stamps {
		used[s.S] = true
	}
	var out []Symbol
	for _, s := range Symbols {
		if used[s.Key] {
			out = append(out, s)
		}
	}
	return out
}

// ---------- Import der alten Befundseite ----------

// Legacy ist das Exportformat der früheren Befundseite (Befund_*.json).
type Legacy struct {
	Name, Date, Dob string
	Data            Data
	Sessions        []Row // date, goal, note
}

// ParseLegacy erkennt alte Exporte an den flachen data-bind-Schlüsseln.
func ParseLegacy(b []byte) (Legacy, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return Legacy{}, fmt.Errorf("kein gültiges JSON: %w", err)
	}
	if _, ok := raw["name"]; !ok {
		if _, ok := raw["rom"]; !ok {
			return Legacy{}, fmt.Errorf("keine Befund-Datei der alten Befundseite")
		}
	}
	str := func(k string) string {
		var s string
		if json.Unmarshal(raw[k], &s) == nil {
			return strings.TrimSpace(s)
		}
		var bl bool
		if json.Unmarshal(raw[k], &bl) == nil && bl {
			return "1"
		}
		return ""
	}
	l := Legacy{Name: str("name"), Date: str("date"), Dob: str("dob"), Data: New()}
	for k := range fieldIndex {
		if v := str(k); v != "" {
			l.Data.Fields[k] = v
		}
	}
	// Eigene Zusatzabschnitte hießen custom-xxxx (Überschrift wurde nie gespeichert)
	var extra []string
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasPrefix(k, "custom-") {
			if v := str(k); v != "" {
				extra = append(extra, v)
			}
		}
	}
	if len(extra) > 0 {
		l.Data.Fields["zusatz"] = strings.TrimSpace(l.Data.Fields["zusatz"] + "\n\n" + strings.Join(extra, "\n\n"))
	}
	rowsOf := func(k string) []Row {
		var in []map[string]any
		if err := json.Unmarshal(raw[k], &in); err != nil {
			return nil // Feld fehlt oder hat ein anderes Format
		}
		var out []Row
		for _, m := range in {
			r := Row{}
			for ck, cv := range m {
				if s := strings.TrimSpace(fmt.Sprint(cv)); s != "" && cv != nil {
					r[ck] = s
				}
			}
			if len(r) > 0 {
				out = append(out, r)
			}
		}
		return out
	}
	for _, t := range Tables {
		if rs := rowsOf(t.Key); len(rs) > 0 {
			l.Data.Tables[t.Key] = rs
		}
	}
	l.Sessions = rowsOf("treat")
	return l, nil
}

// ---------- Vergleich zweier Befunde ----------

type FieldDiff struct{ Section, Label, A, B string }

type RowDiff struct {
	Key   string
	Cells []CellDiff
}

type CellDiff struct{ Label, A, B, Delta string }

type TableDiff struct {
	Title string
	Rows  []RowDiff
}

func Compare(a, b Data) ([]FieldDiff, []TableDiff) {
	var fields []FieldDiff
	for _, s := range Sections {
		for _, p := range s.Parts {
			for _, f := range p.Fields {
				va, vb := a.Fields[f.Key], b.Fields[f.Key]
				if va == vb {
					continue
				}
				label := f.Label
				if label == "" {
					label = s.Title
					if p.Title != "" {
						label = p.Title
					}
				}
				if f.Type == "check" {
					va, vb = yesNo(va), yesNo(vb)
				}
				fields = append(fields, FieldDiff{Section: s.Title, Label: label, A: va, B: vb})
			}
		}
	}
	titles := map[string]string{
		"rom": "ROM", "mfp": "MFP", "circ": "Umfang", "tests": "Spezifische Tests",
		"measures": "Assessments & PROMs", "psfs": "PSFS",
	}
	var tables []TableDiff
	for _, t := range Tables {
		if len(t.Match) == 0 {
			continue
		}
		td := TableDiff{Title: titles[t.Key]}
		key := func(r Row) string {
			parts := make([]string, len(t.Match))
			for i, m := range t.Match {
				parts[i] = r[m]
			}
			return strings.Join(parts, " · ")
		}
		ra, rb := map[string]Row{}, map[string]Row{}
		var order []string
		for _, r := range a.Tables[t.Key] {
			k := key(r)
			if _, seen := ra[k]; !seen {
				order = append(order, k)
			}
			ra[k] = r
		}
		for _, r := range b.Tables[t.Key] {
			k := key(r)
			if _, seenA := ra[k]; !seenA {
				if _, seenB := rb[k]; !seenB {
					order = append(order, k)
				}
			}
			rb[k] = r
		}
		for _, k := range order {
			rd := RowDiff{Key: k}
			for _, c := range t.Cols {
				if !contains(t.Diff, c.Key) {
					continue
				}
				va, vb := ra[k][c.Key], rb[k][c.Key]
				if va == "" && vb == "" {
					continue
				}
				rd.Cells = append(rd.Cells, CellDiff{Label: c.Label, A: va, B: vb, Delta: Delta(va, vb)})
			}
			if len(rd.Cells) > 0 {
				td.Rows = append(td.Rows, rd)
			}
		}
		if len(td.Rows) > 0 {
			tables = append(tables, td)
		}
	}
	return fields, tables
}

var numRe = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// Delta vergleicht Zahlen positionsweise: "0-0-120" → "0-0-135" ergibt "+15"
// (nur geänderte Positionen), "3" → "4" ergibt "+1".
func Delta(a, b string) string {
	// Neutral-Null-Notation: Bindestriche sind Trenner, keine Vorzeichen
	norm := func(s string) []float64 {
		s = strings.ReplaceAll(s, "–", "-")
		var out []float64
		for _, m := range numRe.FindAllString(strings.ReplaceAll(s, "-", " "), -1) {
			f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
			if err == nil {
				out = append(out, f)
			}
		}
		return out
	}
	na, nb := norm(a), norm(b)
	if len(na) == 0 || len(na) != len(nb) {
		return ""
	}
	var parts []string
	for i := range na {
		d := nb[i] - na[i]
		if d == 0 {
			continue
		}
		s := strconv.FormatFloat(d, 'f', -1, 64)
		if d > 0 {
			s = "+" + s
		}
		parts = append(parts, strings.ReplaceAll(s, ".", ","))
	}
	if len(parts) == 0 {
		return "±0"
	}
	return strings.Join(parts, " / ")
}

func yesNo(v string) string {
	if v != "" {
		return "ja"
	}
	return "nein"
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// PSFSMean: Mittelwert der PSFS-Aktivitäten (leer, wenn keine Werte).
func PSFSMean(d Data) string {
	var sum float64
	var n int
	for _, r := range d.Tables["psfs"] {
		if f, err := strconv.ParseFloat(strings.ReplaceAll(r["score"], ",", "."), 64); err == nil {
			sum += f
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return strings.ReplaceAll(strconv.FormatFloat(sum/float64(n), 'f', 1, 64), ".", ",")
}

func (d Data) StampsJSON() string {
	if d.Stamps == nil {
		return "[]"
	}
	b, _ := json.Marshal(d.Stamps)
	return string(b)
}
