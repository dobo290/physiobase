package befund

import (
	"net/url"
	"strings"
	"testing"
)

func TestDrawingsRoundtrip(t *testing.T) {
	d := New()
	d.Draw = []Drawing{
		{V: 1, Color: "#ff0000", Width: 0.02, Pts: []Pt{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}}},
		{V: 0, Color: "#0000ff", Width: 0.01, Pts: []Pt{{0.0, 0.0}, {1.0, 1.0}}},
	}
	raw := d.JSON()
	back, err := Parse(raw)
	if err != nil || len(back.Draw) != 2 {
		t.Fatalf("roundtrip: %v %d", err, len(back.Draw))
	}
	if back.Draw[0].Path(Views[1]) == "" || !strings.Contains(back.Draw[0].Path(Views[1]), "L") {
		t.Errorf("path leer/fehlerhaft: %q", back.Draw[0].Path(Views[1]))
	}
}

func TestCleanDrawings(t *testing.T) {
	in := []Drawing{
		{V: 0, Color: "#ffffff", Width: 0.02, Pts: []Pt{{0.1, 0.1}, {0.2, 0.2}}}, // ok
		{V: 9, Color: "#fff", Width: 0.02, Pts: []Pt{{0.1, 0.1}, {0.2, 0.2}}},    // falsche Ansicht
		{V: 0, Color: "#00ff00", Width: 0.02, Pts: []Pt{{0.1, 0.1}}},             // zu wenig Punkte
		{V: 0, Color: "kaputt", Width: 0.02, Pts: []Pt{{0.1, 0.1}, {2.0, 0.2}}},  // Farbe + Koordinate außerhalb
	}
	out := CleanDrawings(in)
	if len(out) != 1 || out[0].Color != "#ffffff" {
		t.Fatalf("CleanDrawings: %+v", out)
	}
}

func TestFromFormDrawings(t *testing.T) {
	f := url.Values{"drawings": {`[{"v":0,"c":"#ff0000","w":0.02,"pts":[{"x":0.1,"y":0.1},{"x":0.9,"y":0.9}]}]`}}
	d := FromForm(f)
	if len(d.Draw) != 1 || d.Draw[0].Pts[1].X != 0.9 {
		t.Fatalf("FromForm drawings: %+v", d.Draw)
	}
	if d.DrawingsJSON() == "" || !strings.Contains(d.DrawingsJSON(), "#ff0000") {
		t.Error("DrawingsJSON leer")
	}
}
