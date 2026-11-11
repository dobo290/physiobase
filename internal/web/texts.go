package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"forge.gpl-nas.de/physiobase/internal/store"
)

// TextDef beschreibt einen änderbaren Text. Leer gespeichert (oder gleich dem
// Standard) bedeutet: Standard verwenden – so kommen spätere Verbesserungen an.
type TextDef struct {
	Key, Group, Label, Help, Default string
	Kind                             string // "line" (einzeilig), "area", "number", "time"
}

// Gruppen in der Reihenfolge der Einstellungsseite.
var textGroups = []string{"Praxis", "Ausdrucke", "E-Mail", "Zusammenfassungen", "Termine", "Dokumente und Handouts"}

var textDefs = []TextDef{
	{
		Key: "practice_name", Group: "Praxis", Label: "Praxisname", Kind: "line",
		Help: "Erscheint oben in der App, auf allen Ausdrucken, in E-Mails und im Kalender der Patienten.",
	},
	{
		Key: "practice_address", Group: "Praxis", Label: "Adresse", Kind: "line",
		Help: "Standard-Ort für Termine; erscheint auf der Terminliste.",
	},
	{
		Key: "practice_contact", Group: "Praxis", Label: "Kontakt", Kind: "line",
		Help: "Telefon, E-Mail oder Website – erscheint auf Ausdrucken unter dem Praxisnamen.",
	},

	{
		Key: "print_appointments_footer", Group: "Ausdrucke", Label: "Terminliste – Hinweis unten", Kind: "area",
		Default: "Bitte sagen Sie Termine, die Sie nicht wahrnehmen können, mindestens 24 Stunden vorher ab.",
	},
	{
		Key: "print_appointments_calendar", Group: "Ausdrucke", Label: "Terminliste – Text beim QR-Code", Kind: "area",
		Default: "QR-Code mit der Kamera scannen und den Kalender abonnieren. Neue Termine, Änderungen und Absagen erscheinen dann automatisch.",
	},
	{
		Key: "print_handout_footer", Group: "Ausdrucke", Label: "Übungsblatt – Hinweis unten", Kind: "area",
		Default: "Bei starken oder zunehmenden Beschwerden die Übung pausieren und Rücksprache halten.",
	},
	{
		Key: "print_summary_footer", Group: "Ausdrucke", Label: "Zusammenfassung – Hinweis unten", Kind: "area",
		Default: "Bei Fragen sprechen Sie mich gern beim nächsten Termin an.",
	},
	{
		Key: "print_befund_sig_left", Group: "Ausdrucke", Label: "Befund – Unterschriftszeile links", Kind: "line",
		Default: "Datum, Unterschrift Therapeut*in",
	},
	{
		Key: "print_befund_sig_right", Group: "Ausdrucke", Label: "Befund – Unterschriftszeile rechts", Kind: "line",
		Default: "Datum, Unterschrift Patient*in",
	},

	{Key: "mail_greeting", Group: "E-Mail", Label: "Anrede", Kind: "line", Default: "Guten Tag {name},"},
	{Key: "mail_intro", Group: "E-Mail", Label: "Einleitung", Kind: "area", Default: "hier ist Ihre Zusammenfassung zum Nachlesen."},
	{Key: "mail_closing", Group: "E-Mail", Label: "Gruß", Kind: "area", Default: "Viele Grüße\n{praxis}"},
	{
		Key: "mail_privacy", Group: "E-Mail", Label: "Hinweis unten", Kind: "area",
		Default: "Diese Nachricht enthält persönliche Gesundheitsinformationen und ist nur für {name} bestimmt.",
	},

	{
		Key: "summary_general_title", Group: "Zusammenfassungen", Label: "Titel der Gesamtzusammenfassung", Kind: "line",
		Default: "Ihre Physiotherapie – Zusammenfassung",
	},
	{
		Key: "summary_session_title", Group: "Zusammenfassungen", Label: "Titel der Behandlungszusammenfassung", Kind: "line",
		Default: "Ihre Behandlung am {datum}",
	},
	{
		Key: "summary_safety_net", Group: "Zusammenfassungen", Label: "Abschnitt „Wann Sie sich melden sollten“", Kind: "area",
		Help: "Markdown. Zeilen mit ✎ sind Hinweise für dich und werden nicht gedruckt.",
		Default: "- wenn die Beschwerden deutlich stärker werden oder nach Übungen länger als einen Tag schlechter bleiben\n" +
			"- bei neuem Taubheitsgefühl, Kribbeln oder Kraftverlust\n" +
			"- bei Schwellung, Rötung, Überwärmung oder Fieber\n" +
			"✎ Bei Rückenbeschwerden ggf. ergänzen: Taubheit im Gesäß-/Genitalbereich oder Probleme beim Wasserlassen → sofort ärztlich abklären.",
	},
	{
		Key: "summary_session_watch", Group: "Zusammenfassungen", Label: "Behandlung – Vorschlag „Darauf sollten Sie achten“", Kind: "area",
		Help:    "Markdown. Wird in jede neue Behandlungszusammenfassung übernommen.",
		Default: "- \n✎ z. B. Belastung dosieren, Schmerz bei Übungen bis 3–4 von 10 ist in Ordnung, am Folgetag nicht schlechter.",
	},

	{
		Key: "appt_types", Group: "Termine", Label: "Behandlungsarten (eine pro Zeile)", Kind: "area",
		Default: "Physiotherapie\nKrankengymnastik\nManuelle Therapie\nKlassische Massage\nLymphdrainage\nBefundaufnahme\nWiederbefund\nHausbesuch",
	},
	{Key: "appt_default_type", Group: "Termine", Label: "Vorausgewählte Behandlungsart", Kind: "line", Default: "Physiotherapie"},
	{Key: "appt_default_duration", Group: "Termine", Label: "Standarddauer (Minuten)", Kind: "number", Default: "30"},
	{Key: "appt_default_time", Group: "Termine", Label: "Standard-Uhrzeit", Kind: "time", Default: "08:00"},
	{
		Key: "appt_default_note", Group: "Termine", Label: "Standard-Hinweis für Patienten", Kind: "line",
		Help: "Wird bei neuen Terminen vorausgefüllt, z. B. „Bitte Handtuch mitbringen“.",
	},

	{
		Key: "doc_categories", Group: "Dokumente und Handouts", Label: "Dokument-Kategorien (eine pro Zeile)", Kind: "area",
		Default: strings.Join(store.DocCategories, "\n"),
	},
	{
		Key: "handout_default_notes", Group: "Dokumente und Handouts", Label: "Übungsblatt – Standard-Hinweis oben", Kind: "area",
		Help:    "Wird in jedes neue Übungsblatt übernommen.",
		Default: "Übungen bitte 1× täglich. Ein leichtes Ziehen oder Schmerz bis 3 von 10 ist in Ordnung.",
	},
}

var textIndex = func() map[string]TextDef {
	m := map[string]TextDef{}
	for _, d := range textDefs {
		m[d.Key] = d
	}
	return m
}()

const textPrefix = "text."

// defaultText: Standard aus der Definition, für Praxisname/Adresse aus der Konfiguration.
func (s *Server) defaultText(key string) string {
	switch key {
	case "practice_name":
		return s.cfg.PracticeName
	case "practice_address":
		return s.cfg.Address
	}
	return textIndex[key].Default
}

func (s *Server) loadTexts(ctx context.Context) error {
	m := map[string]string{}
	for _, d := range textDefs {
		v, err := s.st.Setting(ctx, textPrefix+d.Key)
		if err != nil {
			return err
		}
		if v != "" {
			m[d.Key] = v
		}
	}
	s.textMu.Lock()
	s.texts = m
	s.textMu.Unlock()
	return nil
}

// text liefert den wirksamen Text (eigener Text oder Standard), ohne Platzhalter.
func (s *Server) rawText(key string) string {
	s.textMu.RLock()
	v, ok := s.texts[key]
	s.textMu.RUnlock()
	if ok {
		return v
	}
	return s.defaultText(key)
}

// text ersetzt {praxis} und {adresse}; weitere Platzhalter über vars ("name", "Max" …).
func (s *Server) text(key string, vars ...string) string {
	repl := []string{"{praxis}", s.practice(), "{adresse}", s.rawText("practice_address")}
	for i := 0; i+1 < len(vars); i += 2 {
		repl = append(repl, "{"+vars[i]+"}", vars[i+1])
	}
	return strings.NewReplacer(repl...).Replace(s.rawText(key))
}

func (s *Server) practice() string {
	if v := strings.TrimSpace(s.rawText("practice_name")); v != "" {
		return v
	}
	return "PhysioBase"
}

func (s *Server) address() string { return s.rawText("practice_address") }

// lines: Liste aus einem mehrzeiligen Text (leere Zeilen weg).
func (s *Server) lines(key string) []string {
	var out []string
	for _, l := range strings.Split(s.rawText(key), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// textMap für Templates: alle wirksamen Texte mit {praxis}/{adresse} ersetzt.
func (s *Server) textMap() map[string]string {
	m := make(map[string]string, len(textDefs))
	for _, d := range textDefs {
		m[d.Key] = s.text(d.Key)
	}
	return m
}

// ---------- Logo ----------

var logoExt = map[string]string{".svg": "image/svg+xml", ".png": "image/png", ".webp": "image/webp", ".jpg": "image/jpeg", ".jpeg": "image/jpeg"}

func (s *Server) brandingDir() string { return filepath.Join(filepath.Dir(s.cfg.MediaDir), "branding") }

// logoURL: eigenes Logo (mit Versions-Parameter gegen Browser-Cache) oder das mitgelieferte.
func (s *Server) logoURL() string {
	name := s.rawLogo()
	if name == "" {
		return "/static/logo.svg" // neutraler Platzhalter
	}
	if fi, err := os.Stat(filepath.Join(s.brandingDir(), name)); err == nil {
		return fmt.Sprintf("/branding/%s?v=%d", name, fi.ModTime().Unix())
	}
	return "/static/logo.svg" // neutraler Platzhalter
}

func (s *Server) rawLogo() string {
	s.textMu.RLock()
	defer s.textMu.RUnlock()
	return s.logo
}

// SVGs können Skripte enthalten – solche Dateien werden abgelehnt.
var svgDanger = regexp.MustCompile(`(?i)<\s*script|<\s*foreignobject|\son[a-z]+\s*=|javascript:|<\s*iframe|<\s*embed|<\s*object|href\s*=\s*["']\s*(https?:|data:text)`)

// svgStart findet das öffnende <svg>-Tag (Attribute stehen in Gruppe 1).
var svgStart = regexp.MustCompile(`(?i)<\s*svg\b([^>]*)>`)

// svgDim entfernt width=… und height=… jedes Attribut-Paars im öffnenden <svg>-Tag.
var svgDim = regexp.MustCompile(`(?i)\s*(?:width|height)\s*=\s*("[^"\\]*(?:\\.[^"\\]*)*"|'[^'\\]*(?:\\.[^'\\]*)*')`)

// normalizeSVG entfernt absichtlich width/height aus dem <svg>-Tag, solange eine
// viewBox vorhanden ist. Das SVG wird dann nur durch viewBox skaliert und passt sich
// automatisch der Höhe an, die das CSS vorgibt (wie beim Standard-Logo). Ohne viewBox
// bleibt es unverändert, weil es sonst keine einheitliche Skalierung gäbe.
func normalizeSVG(b []byte) []byte {
	m := svgStart.FindSubmatchIndex(b)
	if m == nil {
		return b
	}
	attrs := b[m[2]:m[3]]
	if !regexp.MustCompile(`(?i)viewBox\s*=`).Match(attrs) {
		return b
	}
	cleaned := svgDim.ReplaceAll(attrs, nil) // entfernt auch die Leerzeichen vor width/height
	cleaned = bytes.TrimSpace(cleaned)
	if bytes.Equal(cleaned, attrs) {
		return b
	}
	out := make([]byte, 0, len(b))
	out = append(out, b[:m[2]]...)
	if len(cleaned) > 0 { // `<svg` und erste(n) Restattribut(e) mit einem Leerzeichen trennen
		out = append(out, ' ')
	}
	out = append(out, cleaned...)
	out = append(out, b[m[3]:]...)
	return out
}

func checkSVG(b []byte) error {
	if !bytes.Contains(bytes.ToLower(b[:min(len(b), 4096)]), []byte("<svg")) {
		return fmt.Errorf("keine SVG-Datei")
	}
	if svgDanger.Match(b) {
		return fmt.Errorf("die SVG-Datei enthält Skripte oder externe Verweise und wird aus Sicherheitsgründen nicht übernommen")
	}
	return nil
}

func (s *Server) serveBranding(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if name != s.rawLogo() || name == "" || name != filepath.Base(name) {
		http.NotFound(w, r)
		return
	}
	ct, ok := logoExt[extOf(name)]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	// SVG zusätzlich abschotten, falls es direkt geöffnet wird
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:")
	http.ServeFile(w, r, filepath.Join(s.brandingDir(), name)) //nolint:gosec // G703: Name geprüft
}
