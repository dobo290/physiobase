// Package ical erzeugt iCalendar-Feeds (RFC 5545) zum Abonnieren in Kalender-Apps.
package ical

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"
)

type Event struct {
	UID                            string
	Start, End, Stamp              time.Time
	Summary, Description, Location string
	Sequence                       int
	Cancelled                      bool
}

type Calendar struct {
	Name, ProdID string
	Events       []Event
}

// Escape für TEXT-Werte (RFC 5545 3.3.11).
func Escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`)
	return r.Replace(s)
}

// fold bricht Zeilen nach höchstens 75 Bytes um, ohne UTF-8-Zeichen zu zerteilen.
func fold(line string) string {
	if len(line) <= 75 {
		return line + "\r\n"
	}
	var b strings.Builder
	limit := 75
	for len(line) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
		limit = 74 // Folgezeilen beginnen mit einem Leerzeichen
	}
	b.WriteString(line)
	b.WriteString("\r\n")
	return b.String()
}

func utc(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

func (c Calendar) Bytes() []byte {
	var b bytes.Buffer
	w := func(s string) { b.WriteString(fold(s)) }
	w("BEGIN:VCALENDAR")
	w("VERSION:2.0")
	w("PRODID:" + c.ProdID)
	w("CALSCALE:GREGORIAN")
	w("METHOD:PUBLISH")
	w("X-WR-CALNAME:" + Escape(c.Name))
	w("REFRESH-INTERVAL;VALUE=DURATION:PT1H")
	w("X-PUBLISHED-TTL:PT1H")
	for _, e := range c.Events {
		w("BEGIN:VEVENT")
		w("UID:" + e.UID)
		w("DTSTAMP:" + utc(e.Stamp))
		w("DTSTART:" + utc(e.Start))
		w("DTEND:" + utc(e.End))
		w("SEQUENCE:" + itoa(e.Sequence))
		w("SUMMARY:" + Escape(e.Summary))
		if e.Location != "" {
			w("LOCATION:" + Escape(e.Location))
		}
		if e.Description != "" {
			w("DESCRIPTION:" + Escape(e.Description))
		}
		if e.Cancelled {
			w("STATUS:CANCELLED")
		} else {
			w("STATUS:CONFIRMED")
		}
		w("TRANSP:OPAQUE")
		w("END:VEVENT")
	}
	w("END:VCALENDAR")
	return b.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		d = append([]byte{'-'}, d...)
	}
	return string(d)
}
