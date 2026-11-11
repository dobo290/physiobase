package befund

import (
	"encoding/json"
	"math"
	"strings"
)

// Pt ist ein Stützpunkt einer Freihandzeichnung, normalisiert auf 0..1
// (unabhängig von Bildschirm- und Druckgröße).
type Pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Drawing ist ein zusammenhängender Freihand-Strich auf einer Körperansicht.
type Drawing struct {
	V     int     `json:"v"`   // Ansicht (Index in Views)
	Color string  `json:"c"`   // #rrggbb
	Width float64 `json:"w"`   // relative Strichbreite (0..1, bezogen auf die Höhe)
	Pts   []Pt    `json:"pts"` // mindestens 2 Punkte
}

// Drawings sind alle Freihand-Striche eines Befunds.
func (d Data) Drawings() []Drawing { return d.Draw }

// DrawingsFor liefert die Striche einer Ansicht (stabil, in Reihenfolge).
func (d Data) DrawingsFor(view int) []Drawing {
	var out []Drawing
	for _, dr := range d.Draw {
		if dr.V == view {
			out = append(out, dr)
		}
	}
	return out
}

// Path baut das SVG-d-Attribut im viewBox-Koordinatensystem der Ansicht.
func (dr Drawing) Path(v View) string {
	var b strings.Builder
	for i, p := range dr.Pts {
		x := p.X * float64(v.W)
		y := p.Y * float64(v.H)
		if i == 0 {
			b.WriteString("M ")
		} else {
			b.WriteString("L ")
		}
		b.WriteString(trimF(x))
		b.WriteString(" ")
		b.WriteString(trimF(y))
		b.WriteString(" ")
	}
	return b.String()
}

// StrokeWidth in viewBox-Einheiten (bezogen auf die Höhe der Ansicht).
func (dr Drawing) StrokeWidth(v View) float64 {
	return math.Max(0.5, dr.Width*float64(v.H))
}

func trimF(f float64) string {
	s := strings.TrimRight(strings.TrimRight(jsonNumber(f), "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func jsonNumber(f float64) string {
	b, _ := json.Marshal(math.Round(f*100) / 100)
	return string(b)
}

func (d Data) DrawingsJSON() string {
	if d.Draw == nil {
		return "[]"
	}
	b, _ := json.Marshal(d.Draw)
	return string(b)
}

// CleanDrawings filtert ungültige Striche (falsche Ansicht, zu wenige Punkte,
// Koordinaten außerhalb 0..1).
func CleanDrawings(in []Drawing) []Drawing {
	var out []Drawing
	for _, dr := range in {
		if dr.V < 0 || dr.V >= len(Views) || len(dr.Pts) < 2 {
			continue
		}
		if !colorOK(dr.Color) {
			dr.Color = "#000000"
		}
		if dr.Width <= 0 || math.IsNaN(dr.Width) || dr.Width > 0.1 {
			dr.Width = 0.02
		}
		pts := dr.Pts[:0:0]
		ok := true
		for _, p := range dr.Pts {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) || p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
				ok = false
				break
			}
			pts = append(pts, p)
		}
		if !ok || len(pts) < 2 {
			continue
		}
		dr.Pts = pts
		out = append(out, dr)
		if len(out) == 500 {
			break
		}
	}
	return out
}

func colorOK(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	hex := func(r rune) bool {
		return r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
	}
	for _, r := range c[1:] {
		if !hex(r) {
			return false
		}
	}
	return true
}
