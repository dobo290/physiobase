// Package dob normalisiert Geburtsdatum und Alter zu einem ISO-Datum mit
// Genauigkeit. Ein als Alter eingetragener Wert wird in ein Geburtsjahr umgerechnet,
// damit er nicht für immer als „45" stehen bleibt.
package dob

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Genauigkeit eines gespeicherten Geburtsdatums.
const (
	Day     = "day"     // volles Datum: 1969-04-27
	Month   = "month"   // Monat: 1969-04
	Year    = "year"    // Jahr: 1969
	Age     = "age"     // aus einer Altersangabe abgeleitetes Geburtsjahr
	Unknown = "unknown" // nicht lesbar; der Originaltext steht in dob_raw
)

var (
	isoFull   = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	isoMonth  = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
	germanDay = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})$`)
	germanTwo = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{2})$`)
	yearOnly  = regexp.MustCompile(`^(\d{4})$`)
	ageOnly   = regexp.MustCompile(`^(\d{1,3})$`)
)

// Parse liest ein freies Geburtsdatum („1969-04-27", „27.04.1969", „1969", „45")
// und liefert ISO-Datum, Genauigkeit und – bei unlesbaren Angaben – den Originaltext.
// now bestimmt, in welches Jahr eine Altersangabe umgerechnet wird.
func Parse(s string, now time.Time) (date, precision, raw string) {
	orig := strings.TrimSpace(s)
	if orig == "" {
		return "", "", ""
	}
	// „geb. 1969" und „geboren am 27.04.1969" kommen in Altdaten vor. Der
	// Originaltext bleibt für raw erhalten, auch wenn er kleingeschrieben wird.
	t := strings.ToLower(orig)
	t = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "geboren am"), "geboren"))
	t = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "geb."), "geb"))
	t = strings.TrimSpace(t)

	if m := isoFull.FindStringSubmatch(t); m != nil {
		if y, mo, d, ok := validDate(m[1], m[2], m[3]); ok {
			return fmt.Sprintf("%04d-%02d-%02d", y, mo, d), Day, ""
		}
	}
	if m := germanDay.FindStringSubmatch(t); m != nil {
		if y, mo, d, ok := validDate(m[3], m[2], m[1]); ok {
			return fmt.Sprintf("%04d-%02d-%02d", y, mo, d), Day, ""
		}
	}
	if m := isoMonth.FindStringSubmatch(t); m != nil {
		if y, mo, ok := validYearMonth(m[1], m[2]); ok {
			return fmt.Sprintf("%04d-%02d", y, mo), Month, ""
		}
	}
	if m := yearOnly.FindStringSubmatch(t); m != nil {
		if y, ok := validYear(m[1]); ok {
			return fmt.Sprintf("%04d", y), Year, ""
		}
	}
	if m := ageOnly.FindStringSubmatch(t); m != nil {
		if age, err := strconv.Atoi(m[1]); err == nil && age >= 0 && age <= 130 {
			return fmt.Sprintf("%04d", now.Year()-age), Age, ""
		}
	}
	// Zweistellige Jahreszahl („11.11.11") ist nicht eindeutig: markieren, nicht raten.
	if germanTwo.MatchString(t) {
		return "", Unknown, orig
	}
	return "", Unknown, orig
}

// Label gibt das gespeicherte Geburtsdatum in deutscher Form zurück. Bei einer
// Altersangabe wird das Alter aus dem Geburtsjahr berechnet, nicht eingefroren.
func Label(date, precision, raw string) string {
	return LabelAt(date, precision, raw, time.Now())
}

// LabelAt ist Label mit einem festen Bezugszeitpunkt (für Tests).
func LabelAt(date, precision, raw string, now time.Time) string {
	switch precision {
	case Day:
		if t, err := time.ParseInLocation("2006-01-02", date, time.Local); err == nil {
			return "geb. " + t.Format("02.01.2006")
		}
		return "geb. " + date
	case Month:
		if t, err := time.ParseInLocation("2006-01", date, time.Local); err == nil {
			return "geb. " + monthName(int(t.Month())) + " " + t.Format("2006")
		}
		return "geb. " + date
	case Year, Age:
		if y, err := strconv.Atoi(date); err == nil && y > 0 {
			if precision == Age {
				return fmt.Sprintf("%d Jahre", ageAt(y, now))
			}
			return "geb. " + date
		}
		return "geb. " + date
	case Unknown:
		if raw != "" {
			return "geb. " + raw
		}
		return ""
	default:
		// Altdaten ohne Genauigkeit: wie bisher anzeigen.
		if date != "" {
			return "geb. " + date
		}
		return ""
	}
}

// monthName liefert den deutschen Monatsnamen.
func monthName(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return [...]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}[m-1]
}

// ageAt liefert das Alter im Bezugsjahr, wenn nur das Geburtsjahr bekannt ist.
func ageAt(birthYear int, now time.Time) int {
	age := now.Year() - birthYear
	if age < 0 {
		age = 0
	}
	return age
}

func validDate(ys, ms, ds string) (year, month, day int, ok bool) {
	y, err1 := strconv.Atoi(ys)
	mo, err2 := strconv.Atoi(ms)
	d, err3 := strconv.Atoi(ds)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	if y < 1900 || y > 2200 || mo < 1 || mo > 12 || d < 1 || d > 31 {
		return 0, 0, 0, false
	}
	// time.Date normalisiert Überläufe; ein Vergleich erkennt ungültige Tage (31.02.).
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != mo || t.Day() != d {
		return 0, 0, 0, false
	}
	return y, mo, d, true
}

func validYearMonth(ys, ms string) (year, month int, ok bool) {
	y, err1 := strconv.Atoi(ys)
	mo, err2 := strconv.Atoi(ms)
	if err1 != nil || err2 != nil || y < 1900 || y > 2200 || mo < 1 || mo > 12 {
		return 0, 0, false
	}
	return y, mo, true
}

func validYear(ys string) (int, bool) {
	y, err := strconv.Atoi(ys)
	if err != nil || y < 1900 || y > 2200 {
		return 0, false
	}
	return y, true
}
