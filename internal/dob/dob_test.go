package dob

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.Local)
	cases := []struct {
		in, date, precision, raw string
	}{
		{"", "", "", ""},
		{"1969-04-27", "1969-04-27", Day, ""},
		{"27.04.1969", "1969-04-27", Day, ""},
		{"geb. 27.04.1969", "1969-04-27", Day, ""},
		{"geboren am 27.04.1969", "1969-04-27", Day, ""},
		{"1969-04", "1969-04", Month, ""},
		{"1969", "1969", Year, ""},
		{"45", "1981", Age, ""},
		{" 7 ", "2019", Age, ""},
		{"130", "1896", Age, ""},
		{"131", "", Unknown, "131"},
		{"45 J.", "", Unknown, "45 J."},
		{"11.11.11", "", Unknown, "11.11.11"},
		{"31.02.1969", "", Unknown, "31.02.1969"},
		{"1899", "", Unknown, "1899"},
	}
	for _, c := range cases {
		date, precision, raw := Parse(c.in, now)
		if date != c.date || precision != c.precision || raw != c.raw {
			t.Errorf("Parse(%q) = (%q, %q, %q), erwartet (%q, %q, %q)",
				c.in, date, precision, raw, c.date, c.precision, c.raw)
		}
	}
}

func TestLabelAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.Local)
	cases := []struct {
		date, precision, raw, want string
	}{
		{"", "", "", ""},
		{"1969-04-27", Day, "", "geb. 27.04.1969"},
		{"1969-04", Month, "", "geb. April 1969"},
		{"1969", Year, "", "geb. 1969"},
		{"1981", Age, "", "45 Jahre"},
		{"", Unknown, "45 J.", "geb. 45 J."},
		{"", Unknown, "", ""},
		{"1969-04-27", "", "", "geb. 1969-04-27"},
	}
	for _, c := range cases {
		if got := LabelAt(c.date, c.precision, c.raw, now); got != c.want {
			t.Errorf("LabelAt(%q, %q, %q) = %q, erwartet %q", c.date, c.precision, c.raw, got, c.want)
		}
	}
}
