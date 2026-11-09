// Package mailer verschickt E-Mails (Text + HTML) über SMTP, nur mit der Standardbibliothek.
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/config"
)

type Message struct {
	To, Subject, Text, HTML string
	FromName                string
}

// ValidAddress prüft eine einzelne Empfängeradresse (keine Listen, keine Header-Injektion).
func ValidAddress(s string) bool {
	if strings.ContainsAny(s, "\r\n,;") {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s
}

// Build erzeugt die komplette MIME-Nachricht (multipart/alternative).
func Build(cfg config.SMTP, m Message, now time.Time) ([]byte, error) {
	if !ValidAddress(m.To) {
		return nil, fmt.Errorf("ungültige Empfängeradresse")
	}
	from := (&mail.Address{Name: m.FromName, Address: cfg.From}).String()
	domain := cfg.From[strings.LastIndex(cfg.From, "@")+1:]
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ ctype, content string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		if part.content == "" {
			continue
		}
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(pw)
		if _, err := qp.Write([]byte(part.content)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	var msg bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&msg, "%s: %s\r\n", k, v) }
	h("From", from)
	h("To", m.To)
	h("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	h("Date", now.Format(time.RFC1123Z))
	h("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	h("MIME-Version", "1.0")
	h("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())
	return msg.Bytes(), nil
}

// Send verbindet sich mit dem SMTP-Server und liefert die Nachricht aus.
func Send(ctx context.Context, cfg config.SMTP, m Message) error {
	if !cfg.Enabled() {
		return errors.New("E-Mail-Versand ist nicht eingerichtet (PHYSIOBASE_SMTP_*)")
	}
	raw, err := Build(cfg, m, time.Now())
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	d := &net.Dialer{}
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	if cfg.Security == "ssl" {
		conn, err = (&tls.Dialer{NetDialer: d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("verbindung zu %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()
	if cfg.Security == "starttls" {
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if cfg.User != "" {
		// PlainAuth verweigert unverschlüsselte Verbindungen außer zu localhost.
		if err := c.Auth(smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("anmeldung: %w", err)
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return fmt.Errorf("absender: %w", err)
	}
	if err := c.Rcpt(m.To); err != nil {
		return fmt.Errorf("empfänger: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("versand: %w", err)
	}
	return c.Quit()
}
