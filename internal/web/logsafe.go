package web

import "strings"

// maxLogValue begrenzt, wie viel von einer Benutzereingabe im Log landet.
const maxLogValue = 200

// logValue macht einen benutzergesteuerten Wert protokolltauglich: Steuerzeichen
// (Zeilenumbrüche, Tabs, ESC-Sequenzen) werden entfernt, die Länge wird begrenzt.
//
// Hintergrund: Werte wie der angefragte Pfad, ein Benutzername oder ein Symbolname
// kommen vom Client. Ohne Bereinigung lassen sich damit Log-Zeilen fälschen
// ("log injection"), etwa ein vorgetäuschter Anmeldeerfolg in der nächsten Zeile.
// slog maskiert Steuerzeichen zwar selbst, aber die Bereinigung hier macht die
// Absicht sichtbar und schützt auch Ausgaben, die später anders formatiert werden.
func logValue(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1 // Steuerzeichen verwerfen
		}
		return r
	}, s)
	if len([]rune(s)) > maxLogValue {
		runes := []rune(s)
		return string(runes[:maxLogValue]) + "…"
	}
	return s
}

// logValues bereinigt eine Liste, etwa die gesammelten Importfehler.
func logValues(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = logValue(s)
	}
	return out
}
