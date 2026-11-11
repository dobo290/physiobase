package web

import "testing"

func TestNormalizeSVG(t *testing.T) {
	cases := []struct{ in, want string }{
		{
			`<svg width="2000" height="2000" viewBox="0 0 2000 2000"><circle/></svg>`,
			`<svg viewBox="0 0 2000 2000"><circle/></svg>`,
		},
		{
			`<svg width='1200px' height='900' viewBox="0 0 1200 900"></svg>`,
			`<svg viewBox="0 0 1200 900"></svg>`,
		},
		{`<svg viewBox="0 0 100 100" width="1000"></svg>`, // nur eine Größe zu entfernen
			`<svg viewBox="0 0 100 100"></svg>`},
		// ohne viewBox: keine Normalisierung
		{`<svg width="2000" height="2000"><rect/></svg>`, `<svg width="2000" height="2000"><rect/></svg>`},
		// width/height als letzte Attribute
		{
			`<svg viewBox="0 0 48 48" fill="none" width="48" height="48"><path/></svg>`,
			`<svg viewBox="0 0 48 48" fill="none"><path/></svg>`,
		},
		// kein SVG
		{`<html></html>`, `<html></html>`},
	}
	for _, c := range cases {
		if got := string(normalizeSVG([]byte(c.in))); got != c.want {
			t.Errorf("normalizeSVG: \n in:  %s\n got: %s\nwant: %s", c.in, got, c.want)
		}
	}
}
