package otp

import (
	"testing"
	"time"
)

// FuzzVerify: Codes und Geheimnisse kommen aus Formularen und aus dem
// Zwischenspeicher. Verify darf nie panisch werden und nie etwas akzeptieren,
// was nicht exakt sechs Ziffern sind.
func FuzzVerify(f *testing.F) {
	f.Add("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "123456")
	f.Add("", "")
	f.Add("kein-base32", "123456")
	f.Add("GEZDGNBVGY3TQOJQ", "000000")
	f.Add("======", "123456")
	now := time.Unix(1111111111, 0)
	f.Fuzz(func(t *testing.T, secret, code string) {
		ok := Verify(secret, code, now)
		if !ok {
			return
		}
		if len(code) < 6 {
			t.Fatalf("Code akzeptiert, der zu kurz ist: %q", code)
		}
		// Akzeptierte Codes müssen aus sechs Ziffern bestehen.
		digits := make([]rune, 0, 6)
		for _, r := range code {
			if r >= '0' && r <= '9' {
				digits = append(digits, r)
			}
		}
		if len(digits) != 6 {
			t.Fatalf("akzeptierter Code hat nicht sechs Ziffern: %q", code)
		}
	})
}

// FuzzNormalize prüft, dass Normalisieren das Geheimnis nicht "repariert",
// sondern nur Leerraum entfernt und Groß-/Kleinschreibung vereinheitlicht.
func FuzzNormalize(f *testing.F) {
	f.Add("gezd gnbv-gy3t qojq")
	f.Add("")
	f.Add("-- --")
	f.Fuzz(func(t *testing.T, s string) {
		n := Normalize(s)
		for _, r := range n {
			if r == ' ' || r == '-' || r == '\t' {
				t.Fatalf("Leerraum im normalisierten Geheimnis: %q", n)
			}
			if r >= 'a' && r <= 'z' {
				t.Fatalf("Kleinbuchstabe im normalisierten Geheimnis: %q", n)
			}
		}
	})
}
