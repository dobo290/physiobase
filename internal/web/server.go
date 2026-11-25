// Package web enthält Routen, Handler und Templates-Rendering.
package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yuin/goldmark"

	"forge.gpl-nas.de/physiobase/internal/version"
	"github.com/yuin/goldmark/extension"

	physiobase "forge.gpl-nas.de/physiobase"
	"forge.gpl-nas.de/physiobase/internal/befund"
	"forge.gpl-nas.de/physiobase/internal/config"
	"forge.gpl-nas.de/physiobase/internal/store"
)

type Server struct {
	cfg   *config.Config
	st    *store.Store
	log   *slog.Logger
	pages map[string]*template.Template

	sessionKey []byte
	loginSem   chan struct{}
	wa         *webauthn.WebAuthn // nil = Passkeys aus (keine HTTPS-Adresse)
	pk         passkeyState
	demo       *demoStats // nur im Demo-Modus genutzt

	textMu sync.RWMutex
	texts  map[string]string // eigene Texte (nur Abweichungen vom Standard)
	logo   string            // Dateiname des eigenen Logos (leer = mitgeliefertes)
}

func New(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) (*Server, error) {
	s := &Server{cfg: cfg, st: st, log: log, pages: map[string]*template.Template{}, loginSem: make(chan struct{}, maxConcurrentLogins)}
	if err := s.loadSessionSecret(ctx); err != nil {
		return nil, err
	}
	if err := s.loadBranding(ctx); err != nil {
		return nil, err
	}
	if err := s.bootstrapAdmin(ctx); err != nil {
		return nil, err
	}
	s.pk.open = map[string]ceremony{}
	s.demo = newDemoStats(cfg.DemoStatsFile)
	wa, err := newWebAuthn(cfg.BaseURL, s.practice())
	if err != nil {
		return nil, fmt.Errorf("passkeys: %w", err)
	}
	s.wa = wa
	for _, p := range []string{
		"index", "entry", "edit", "handouts", "handout", "login", "login_2fa", "patients", "patient", "befund", "compare", "summary", "appointments", "appointment", "settings",
		"admin", "admin_users", "admin_user", "admin_symbols", "account", "setup", "patient_test",
		"admin", "admin_users", "admin_user", "account", "setup",
	} {
		t, err := template.New("").Funcs(funcs).ParseFS(physiobase.TemplatesFS, "web/templates/layout.html", "web/templates/partials.html", "web/templates/"+p+".html")
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", p, err)
		}
		s.pages[p] = t
	}
	for _, p := range []string{"print", "befund_print", "summary_print", "appointments_print"} {
		t, err := template.New("").Funcs(funcs).ParseFS(physiobase.TemplatesFS, "web/templates/"+p+".html")
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", p, err)
		}
		s.pages[p] = t
	}
	return s, nil
}

// Handler: Auth gilt global, damit neue Routen automatisch geschützt sind.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(physiobase.StaticFS, "web/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheForever(http.FileServerFS(static))))
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /login/2fa", s.loginTwoFactorForm)
	mux.HandleFunc("POST /login/2fa", s.loginTwoFactor)
	mux.HandleFunc("POST /login/2fa/abbrechen", s.loginTwoFactorCancel)
	mux.HandleFunc("POST /logout", s.logout)

	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /eintrag/neu", s.newEntry)
	mux.HandleFunc("POST /eintrag/neu", s.saveEntry)
	mux.HandleFunc("GET /eintrag/{id}", s.showEntry)
	mux.HandleFunc("GET /eintrag/{id}/bearbeiten", s.editEntry)
	mux.HandleFunc("POST /eintrag/{id}/bearbeiten", s.saveEntry)
	mux.HandleFunc("POST /eintrag/{id}/loeschen", s.deleteEntry)
	mux.HandleFunc("POST /eintrag/{id}/verknuepfen", s.linkEntry)
	mux.HandleFunc("POST /eintrag/{id}/entknuepfen/{other}", s.unlinkEntry)
	mux.HandleFunc("POST /eintrag/{id}/quelle", s.addSource)
	mux.HandleFunc("POST /quelle/{id}/loeschen", s.deleteSource)
	mux.HandleFunc("POST /eintrag/{id}/medien", s.uploadMedia)
	mux.HandleFunc("POST /medium/{id}/loeschen", s.deleteMedia)
	mux.HandleFunc("GET /medien/{file}", s.serveMedia)

	mux.HandleFunc("GET /handouts", s.listHandouts)
	mux.HandleFunc("POST /handouts", s.createHandout)
	mux.HandleFunc("POST /handouts/uebung", s.addToHandout)
	mux.HandleFunc("GET /handout/{id}", s.showHandout)
	mux.HandleFunc("POST /handout/{id}", s.updateHandout)
	mux.HandleFunc("POST /handout/{id}/loeschen", s.deleteHandout)
	mux.HandleFunc("POST /handout/{id}/kopieren", s.copyHandout)
	mux.HandleFunc("POST /handout/{id}/uebung", s.addHandoutItem)
	mux.HandleFunc("POST /handout/{id}/uebung/{eid}", s.updateHandoutItem)
	mux.HandleFunc("POST /handout/{id}/uebung/{eid}/hoch", s.moveHandoutItem)
	mux.HandleFunc("POST /handout/{id}/uebung/{eid}/loeschen", s.removeHandoutItem)
	mux.HandleFunc("GET /handout/{id}/druck", s.printHandout)

	mux.HandleFunc("GET /api/tests", s.apiSearchTests)
	mux.HandleFunc("GET /patienten", s.listPatients)
	mux.HandleFunc("POST /patienten", s.createPatient)
	mux.HandleFunc("POST /patienten/import", s.importBefunde)
	mux.HandleFunc("GET /patient/{id}", s.showPatient)
	mux.HandleFunc("POST /patient/{id}", s.updatePatient)
	mux.HandleFunc("POST /patient/{id}/archiv", s.archivePatient)
	mux.HandleFunc("POST /patient/{id}/loeschen", s.deletePatient)
	mux.HandleFunc("POST /patient/{id}/befund", s.newBefund)
	mux.HandleFunc("POST /patient/{id}/test", s.linkPatientTest)
	mux.HandleFunc("GET /patient/{id}/test/{eid}", s.runTestForm)
	mux.HandleFunc("POST /patient/{id}/test/{eid}", s.saveTestRun)
	mux.HandleFunc("POST /patient/{id}/verlauf", s.addSession)
	mux.HandleFunc("POST /verlauf/{id}/loeschen", s.deleteSession)
	mux.HandleFunc("POST /patient/{id}/wissen", s.linkPatientEntry)
	mux.HandleFunc("POST /patient/{id}/wissen/{eid}/loeschen", s.unlinkPatientEntry)
	mux.HandleFunc("POST /patient/{id}/handout", s.createPatientHandout)
	mux.HandleFunc("GET /patient/{id}/vergleich", s.compareBefunde)
	mux.HandleFunc("POST /patient/{id}/dokumente", s.uploadDocs)
	mux.HandleFunc("POST /dokument/{id}/loeschen", s.deleteDoc)
	mux.HandleFunc("GET /dokumente/{file}", s.serveDoc)
	mux.HandleFunc("POST /patient/{id}/zusammenfassung", s.createGeneralSummary)
	mux.HandleFunc("POST /verlauf/{id}/zusammenfassung", s.createSessionSummary)
	mux.HandleFunc("GET /zusammenfassung/{id}", s.showSummary)
	mux.HandleFunc("POST /zusammenfassung/{id}", s.saveSummary)
	mux.HandleFunc("POST /zusammenfassung/{id}/loeschen", s.deleteSummary)
	mux.HandleFunc("GET /zusammenfassung/{id}/druck", s.printSummary)
	mux.HandleFunc("POST /zusammenfassung/{id}/senden", s.sendSummary)

	mux.HandleFunc("GET /termine", s.listAppointments)
	mux.HandleFunc("POST /termine", s.createAppointment)
	mux.HandleFunc("GET /termin/{id}", s.showAppointment)
	mux.HandleFunc("POST /termin/{id}", s.saveAppointment)
	mux.HandleFunc("POST /termin/{id}/absagen", s.toggleCancel)
	mux.HandleFunc("POST /termin/{id}/loeschen", s.deleteAppointment)
	mux.HandleFunc("POST /behandler", s.createTherapist)
	mux.HandleFunc("POST /behandler/{id}", s.updateTherapist)
	mux.HandleFunc("GET /patient/{id}/kalender-qr.svg", s.calendarQR)
	mux.HandleFunc("POST /patient/{id}/kalender-link", s.rotateCalendar)
	mux.HandleFunc("GET /patient/{id}/termine/druck", s.printAppointments)
	mux.HandleFunc("GET /kalender/{file}", s.calendarFeed)

	// Verwaltung: nur über /admin erreichbar (kein Knopf in der App), nur für Admins – sonst 404
	mux.HandleFunc("GET /admin", s.adminOnly(s.adminHome))
	mux.HandleFunc("GET /admin/einstellungen", s.adminOnly(s.settingsPage))
	mux.HandleFunc("POST /admin/einstellungen", s.adminOnly(s.saveSettings))
	mux.HandleFunc("POST /admin/einstellungen/logo", s.adminOnly(s.uploadLogo))
	mux.HandleFunc("POST /admin/einstellungen/logo/loeschen", s.adminOnly(s.deleteLogo))
	mux.HandleFunc("GET /admin/symbole", s.adminOnly(s.symbolsPage))
	mux.HandleFunc("POST /admin/symbole", s.adminOnly(s.addSymbol))
	mux.HandleFunc("POST /admin/symbole/{key}/loeschen", s.adminOnly(s.deleteSymbol))
	mux.HandleFunc("GET /admin/benutzer", s.adminOnly(s.adminUsers))
	mux.HandleFunc("POST /admin/benutzer", s.adminOnly(s.adminCreateUser))
	mux.HandleFunc("GET /admin/benutzer/{id}", s.adminOnly(s.adminUser))
	mux.HandleFunc("POST /admin/benutzer/{id}", s.adminOnly(s.adminUpdateUser))
	mux.HandleFunc("POST /admin/benutzer/{id}/passwort", s.adminOnly(s.adminSetPassword))
	mux.HandleFunc("POST /admin/benutzer/{id}/link", s.adminOnly(s.adminSetupLink))
	mux.HandleFunc("POST /admin/benutzer/{id}/abmelden", s.adminOnly(s.adminEndSessions))
	mux.HandleFunc("POST /admin/benutzer/{id}/loeschen", s.adminOnly(s.adminDeleteUser))
	mux.HandleFunc("POST /admin/passkey/{pid}/loeschen", s.adminOnly(s.adminDeletePasskey))
	mux.HandleFunc("POST /admin/benutzer/{id}/2fa", s.adminOnly(s.adminResetTOTP))

	mux.HandleFunc("GET /konto", s.account)
	mux.HandleFunc("POST /konto/passwort", s.accountPassword)
	mux.HandleFunc("POST /konto/passkey/{pid}/loeschen", s.accountDeletePasskey)
	mux.HandleFunc("GET /konto/2fa/qr.svg", s.totpQRCode)
	mux.HandleFunc("POST /konto/2fa/start", s.accountTOTPStart)
	mux.HandleFunc("POST /konto/2fa/bestaetigen", s.accountTOTPConfirm)
	mux.HandleFunc("POST /konto/2fa/abbrechen", s.accountTOTPCancel)
	mux.HandleFunc("POST /konto/2fa/loeschen", s.accountTOTPDisable)
	mux.HandleFunc("GET /einrichten/{token}", s.setupPage)
	mux.HandleFunc("POST /einrichten/{token}/passwort", s.setupPassword)
	mux.HandleFunc("POST /passkey/registrieren/start", s.passkeyRegisterBegin)
	mux.HandleFunc("POST /passkey/registrieren/fertig", s.passkeyRegisterFinish)
	mux.HandleFunc("POST /passkey/login/start", s.passkeyLoginBegin)
	mux.HandleFunc("POST /passkey/login/fertig", s.passkeyLoginFinish)
	if s.cfg.Demo {
		mux.HandleFunc("POST /demo/reset", s.demoResetHandler)
	}
	mux.HandleFunc("GET /branding/{file}", s.serveBranding)
	mux.HandleFunc("GET /befund/{id}", s.showBefund)
	mux.HandleFunc("POST /befund/{id}", s.saveBefund)
	mux.HandleFunc("POST /befund/{id}/test", s.linkBefundTest)
	mux.HandleFunc("POST /befund/{id}/test/{eid}/entfernen", s.unlinkBefundTest)
	mux.HandleFunc("POST /befund/{id}/loeschen", s.deleteBefund)
	mux.HandleFunc("GET /befund/{id}/druck", s.printBefund)
	mux.HandleFunc("GET /befund/{id}/export", s.exportBefund)

	// Demo: Consent-/Statistik-Spur nur auf Seiten-Render (kein Tracking vor Einwilligung)
	if s.cfg.Demo {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				s.trackDemo(r)
			}
			mux.ServeHTTP(w, r)
		})
		return s.securityHeaders(http.NewCrossOriginProtection().Handler(s.requireAuth(s.logRequests(handler))))
	}

	// CrossOriginProtection (Go 1.25+) blockt fremde POSTs anhand von Sec-Fetch-Site/Origin – CSRF-Schutz ohne Tokens.
	return s.securityHeaders(http.NewCrossOriginProtection().Handler(s.requireAuth(s.logRequests(mux))))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.st.Ping(ctx); err != nil {
		http.Error(w, "false", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("true"))
}

// ---------- Middleware ----------

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self'; style-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		if s.cfg.IsHTTPS() {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(c int) { w.status = c; w.ResponseWriter.WriteHeader(c) }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if r.URL.Path != "/healthz" {
			s.log.Debug("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "dauer", time.Since(start))
		}
	})
}

func cacheForever(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

// ---------- Rendering ----------

var md = goldmark.New(goldmark.WithExtensions(extension.GFM)) // ohne WithUnsafe: kein rohes HTML

var funcs = template.FuncMap{
	"kindLabel": store.KindLabel,
	"md": func(src string) template.HTML {
		var b bytes.Buffer
		if err := md.Convert([]byte(src), &b); err != nil {
			return template.HTML(template.HTMLEscapeString(src)) //nolint:gosec // G203: bereits escaped
		}
		// goldmark ohne WithUnsafe: rohes HTML im Markdown wird verworfen.
		return template.HTML(b.String()) //nolint:gosec // G203
	},
	"date":      func(t time.Time) string { return t.Local().Format("02.01.2006") },
	"add1":      func(i int) int { return i + 1 },
	"day":       dayLabel,
	"hasPrefix": strings.HasPrefix,
	"hm":        func(t time.Time) string { return t.Local().Format("15:04") },
	"ymd":       func(t time.Time) string { return t.Local().Format("2006-01-02") },
	// Diagnose eines Befunds für Listen
	"diag":    func(raw string) string { d, _ := befund.Parse(raw); return d.Get("diag") },
	"cell":    func(t, c string, i int) string { return fmt.Sprintf("t.%s.%d.%s", t, i, c) },
	"fmtNum":  func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) },
	"isoDate": func(t time.Time) string { return t.Local().Format("2006-01-02") },
	"de": func(iso string) string { // 2025-03-01 → 01.03.2025
		if t, err := time.Parse("2006-01-02", iso); err == nil {
			return t.Format("02.01.2006")
		}
		return iso
	},
	"isImage": isImage,
	"isVideo": isVideo,
	"isPDF":   func(f string) bool { return strings.EqualFold(extOf(f), ".pdf") },
	// Punkt-Fragebögen
	"qval": func(f befund.QFill, q befund.Questionnaire, i int) string {
		if i < len(f.Values) {
			return f.Values[i]
		}
		return ""
	},
	"qcomplete": func(f befund.QFill, q befund.Questionnaire) bool { return f.Complete(q) },
	"maxTotal":  func(q befund.Questionnaire) int { return q.MaxTotal() },
	"dirLbl":    func(q befund.Questionnaire) string { return q.DirectionLabel() },
	// Freihandzeichnungen
	"drawPath":  func(dr befund.Drawing, v befund.View) string { return dr.Path(v) },
	"drawWidth": func(dr befund.Drawing, v befund.View) float64 { return dr.StrokeWidth(v) },
	"size": func(n int64) string {
		switch {
		case n >= 1<<20:
			return strings.ReplaceAll(strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64), ".", ",") + " MB"
		case n >= 1<<10:
			return strconv.FormatInt(n>>10, 10) + " KB"
		}
		return strconv.FormatInt(n, 10) + " B"
	},
	"tags": func(s string) []string {
		var out []string
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	},
	// Für Handouts: höchstens zwei Bilder pro Übung
	"images": func(m []store.Media) []store.Media {
		var out []store.Media
		for _, x := range m {
			if isImage(x.Filename) {
				if out = append(out, x); len(out) == 2 {
					break
				}
			}
		}
		return out
	},
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, status int, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Practice"] = s.practice()
	if u, ok := currentUser(r); ok {
		data["Me"] = u
	}
	data["Version"] = version.Version
	data["PasskeyLogin"] = s.wa != nil
	if s.cfg.Demo {
		today, week, total := s.demoInfo()
		data["Demo"] = true
		data["DemoHome"] = s.cfg.DemoHome
		data["DemoInfo"] = map[string]int{"Today": today, "Week": week, "Total": total}
	}
	data["Logo"] = s.logoURL()
	data["Contact"] = s.text("practice_contact")
	data["T"] = s.textMap()
	data["Kinds"] = store.Kinds
	data["Path"] = r.URL.Path
	name := "layout"
	if strings.HasSuffix(page, "print") {
		name = page
	}
	var buf bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&buf, name, data); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Nicht gefunden", http.StatusNotFound)
		return
	}
	s.log.Error("anfrage fehlgeschlagen", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "Interner Fehler", http.StatusInternalServerError)
}

func redirect(w http.ResponseWriter, r *http.Request, format string, args ...any) {
	// Ziele sind immer feste, interne Pfade mit numerischen IDs.
	http.Redirect(w, r, fmt.Sprintf(format, args...), http.StatusSeeOther) //nolint:gosec // G710
}

// urlPathEscape für Dateinamen in Content-Disposition (RFC 5987).
func urlPathEscape(s string) string { return url.PathEscape(s) }

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

func formID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.FormValue(name), 10, 64)
	return id, err == nil && id > 0
}

// formText: getrimmt, Zeilenenden vereinheitlicht.
func formText(r *http.Request, name string) string {
	return strings.TrimSpace(strings.ReplaceAll(r.FormValue(name), "\r\n", "\n"))
}

// limitForm begrenzt normale Formulare (nicht Uploads) auf 2 MB.
func limitForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formular ungültig oder zu groß", http.StatusBadRequest)
		return false
	}
	return true
}
