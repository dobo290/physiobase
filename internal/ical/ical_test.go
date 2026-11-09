package ical

import (
	"strings"
	"testing"
	"time"
)

func TestEscape(t *testing.T) {
	if got := Escape("a,b;c\\d\ne"); got != `a\,b\;c\\d\ne` {
		t.Errorf("Escape: %q", got)
	}
}

func TestFoldKeepsUTF8AndLength(t *testing.T) {
	line := "DESCRIPTION:" + strings.Repeat("Übungsprogramm für die Kniestreckung – ", 6)
	out := fold(line)
	for _, l := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if len(l) > 75 {
			t.Errorf("Zeile länger als 75 Bytes: %d", len(l))
		}
		if !strings.HasPrefix(l, "DESCRIPTION") && !strings.HasPrefix(l, " ") {
			t.Errorf("Folgezeile ohne Leerzeichen: %q", l)
		}
	}
	// Entfalten ergibt wieder das Original
	if unfolded := strings.ReplaceAll(strings.TrimSuffix(out, "\r\n"), "\r\n ", ""); unfolded != line {
		t.Error("Faltung nicht verlustfrei")
	}
}

func TestCalendar(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	start := time.Date(2026, 11, 12, 9, 30, 0, 0, berlin)
	c := Calendar{Name: "Physiotherapie, Praxis", ProdID: "-//PhysioBase//DE", Events: []Event{
		{UID: "abc@x", Start: start, End: start.Add(30 * time.Minute), Stamp: start, Summary: "Physiotherapie", Location: "Musterstr. 1, Köln", Sequence: 2},
		{UID: "def@x", Start: start, End: start, Stamp: start, Summary: "Abgesagt", Cancelled: true},
	}}
	s := string(c.Bytes())
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "X-WR-CALNAME:Physiotherapie\\, Praxis\r\n", "DTSTART:20261112T083000Z\r\n",
		"DTEND:20261112T090000Z\r\n", "SEQUENCE:2\r\n", "LOCATION:Musterstr. 1\\, Köln\r\n", "STATUS:CANCELLED\r\n", "END:VCALENDAR\r\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Error("nackte LF-Zeilenenden")
	}
}
