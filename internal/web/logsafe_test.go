package web

import (
	"strings"
	"testing"
)

func TestLogValue(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"normaler-pfad", "normaler-pfad"},
		{"/patient/1", "/patient/1"},
		{"zeile1\nzeile2", "zeile1zeile2"},
		{"tab\tund\x00null", "tabundnull"},
		{"\x1b[31mrot\x1b[0m", "[31mrot[0m"},
		{"a\rb", "ab"},
	}
	for _, c := range cases {
		if got := logValue(c.in); got != c.want {
			t.Errorf("logValue(%q) = %q, erwartet %q", c.in, got, c.want)
		}
	}

	// Längenbegrenzung
	long := strings.Repeat("x", 500)
	if got := logValue(long); len([]rune(got)) != maxLogValue+1 {
		t.Errorf("Längenbegrenzung: %d Zeichen", len([]rune(got)))
	}

	// Listen
	list := logValues([]string{"ok", "böse\nzeile"})
	if len(list) != 2 || list[1] != "bösezeile" {
		t.Errorf("logValues = %v", list)
	}
}

// Ein gefälschter Zeilenumbruch im Pfad darf keine zweite Log-Zeile erzeugen.
func TestLogValueBlocksForgedLine(t *testing.T) {
	t.Parallel()
	forged := "/patient/1\nlevel=INFO msg=anmeldung benutzer=admin"
	if strings.ContainsAny(logValue(forged), "\n\r") {
		t.Fatal("Zeilenumbruch überlebt die Bereinigung")
	}
}
