package web

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"forge.gpl-nas.de/physiobase/internal/mailer"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// E-Mail-Clients ignorieren externe CSS-Dateien → schlichte Inline-Stile.
var mailHTML = template.Must(template.New("mail").Parse(`<!doctype html>
<html lang="de"><head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:#f4f6f8;">
<div style="max-width:640px;margin:0 auto;padding:24px;font-family:-apple-system,'Segoe UI',Roboto,Arial,sans-serif;font-size:16px;line-height:1.55;color:#1d2a33;background:#ffffff;">
<h1 style="font-size:22px;margin:0 0 4px;color:#16a085;">{{.Title}}</h1>
<p style="margin:0 0 18px;color:#5c6b75;font-size:14px;">{{.Date}}</p>
<p>{{.Greeting}}</p>
<p style="white-space:pre-line;">{{.Intro}}</p>
<div>{{.Body}}</div>
<p style="margin-top:24px;white-space:pre-line;">{{.Closing}}</p>
<p style="margin-top:24px;font-size:12px;color:#8a979f;white-space:pre-line;">{{.Privacy}}</p>
</div></body></html>`))

type mailTexts struct{ Greeting, Intro, Closing, Privacy string }

func (s *Server) mailTexts(name string) mailTexts {
	return mailTexts{
		s.text("mail_greeting", "name", name), s.text("mail_intro", "name", name),
		s.text("mail_closing", "name", name), s.text("mail_privacy", "name", name),
	}
}

func mailText(t mailTexts, title, date, body string) string {
	var b strings.Builder
	b.WriteString(t.Greeting + "\n\n" + t.Intro + "\n\n")
	b.WriteString(title + " (" + date + ")\n" + strings.Repeat("=", len([]rune(title))) + "\n\n")
	b.WriteString(body + "\n\n" + t.Closing + "\n\n" + t.Privacy + "\n")
	return b.String()
}

// mailBlockReason: warum (noch) nicht gesendet werden kann; leer = alles bereit.
func (s *Server) mailBlockReason(p store.Patient) string {
	switch {
	case !s.cfg.SMTP.Enabled():
		return "Der E-Mail-Versand ist nicht eingerichtet (PHYSIOBASE_SMTP_* in .env)."
	case p.Email == "":
		return "Für diesen Patienten ist keine E-Mail-Adresse hinterlegt."
	case !mailer.ValidAddress(p.Email):
		return "Die hinterlegte E-Mail-Adresse ist ungültig."
	case !p.EmailConsent:
		return "Es fehlt die Einwilligung in den Versand per E-Mail (Stammdaten des Patienten)."
	}
	return ""
}

func (s *Server) sendSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	x, p, ok := s.loadSummary(w, r)
	if !ok {
		return
	}
	if reason := s.mailBlockReason(p); reason != "" {
		redirect(w, r, "/zusammenfassung/%d?fehler=%s#senden", x.ID, url.QueryEscape(reason))
		return
	}
	if s.cfg.Demo {
		redirect(w, r, "/zusammenfassung/%d?fehler=%s#senden", x.ID, urlPathEscape("Im Demo ist der E-Mail-Versand deaktiviert."))
		return
	}
	body := Printable(x.Body)
	mt := s.mailTexts(p.Name)
	var html bytes.Buffer
	if err := mailHTML.Execute(&html, map[string]any{
		"Title": x.Title, "Date": deDate(x.Date),
		"Greeting": mt.Greeting, "Intro": mt.Intro, "Closing": mt.Closing, "Privacy": mt.Privacy,
		"Body": funcs["md"].(func(string) template.HTML)(body),
	}); err != nil {
		s.fail(w, r, err)
		return
	}
	msg := mailer.Message{
		To: p.Email, Subject: x.Title, FromName: s.practice(),
		Text: mailText(mt, x.Title, deDate(x.Date), body), HTML: html.String(),
	}
	err := mailer.Send(ctx, s.cfg.SMTP, msg)
	entry := store.MailLog{Recipient: p.Email, Subject: x.Title, OK: err == nil}
	if err != nil {
		entry.Error = err.Error()
		s.log.Error("e-mail fehlgeschlagen", "zusammenfassung", x.ID, "err", err)
	} else {
		s.log.Info("e-mail gesendet", "zusammenfassung", x.ID, "patient", p.ID)
	}
	if lerr := s.st.LogMail(ctx, x.ID, p.ID, entry); lerr != nil {
		s.log.Error("versandprotokoll", "err", lerr)
	}
	if err != nil {
		redirect(w, r, "/zusammenfassung/%d?fehler=%s#senden", x.ID, url.QueryEscape("Senden fehlgeschlagen: "+err.Error()))
		return
	}
	redirect(w, r, "/zusammenfassung/%d?gesendet=1#senden", x.ID)
}
