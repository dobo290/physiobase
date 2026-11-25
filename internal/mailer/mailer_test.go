package mailer

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"forge.gpl-nas.de/physiobase/internal/config"
)

func TestValidAddress(t *testing.T) {
	ok := []string{"a@example.org", "vorname.nachname@praxis.gpl-nas.de", "x+y@sub.example.com"}
	bad := []string{
		"", "keine-mail", "a@b", "a@example.org\r\nBcc: x@y.z", "a@example.org, b@example.org",
		"a@example.org;b@example.org", "Name <a@example.org>", "a b@example.org",
	}
	for _, s := range ok {
		if !ValidAddress(s) {
			t.Errorf("gültige Adresse abgelehnt: %q", s)
		}
	}
	for _, s := range bad {
		if ValidAddress(s) {
			t.Errorf("ungültige Adresse akzeptiert: %q", s)
		}
	}
}

func TestBuildHasBothParts(t *testing.T) {
	cfg := config.SMTP{From: "praxis@example.org"}
	raw, err := Build(cfg, Message{
		To: "patient@example.org", Subject: "Zusammenfassung",
		Text: "Hallo", HTML: "<p>Hallo</p>", FromName: "Praxis",
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"To: patient@example.org", "text/plain", "text/html", "Hallo"} {
		if !strings.Contains(s, want) {
			t.Errorf("%q fehlt in der Nachricht", want)
		}
	}
}

// TestSendToFakeServer prüft den Versand gegen einen echten SMTP-Server
// (ohne TLS), damit Fehler in der Zustellung nicht erst in der Praxis auffallen.
func TestSendToFakeServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		say := func(s string) { _, _ = rw.WriteString(s + "\r\n"); _ = rw.Flush() }
		say("220 fake ESMTP")
		var body strings.Builder
		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250-fake")
				say("250 HELP")
			case strings.HasPrefix(cmd, "MAIL FROM"), strings.HasPrefix(cmd, "RCPT TO"):
				say("250 OK")
			case cmd == "DATA":
				say("354 los")
				for {
					l, err := rw.ReadString('\n')
					if err != nil {
						return
					}
					if strings.TrimRight(l, "\r\n") == "." {
						break
					}
					body.WriteString(l)
				}
				say("250 angenommen")
			case cmd == "QUIT":
				say("221 tschüss")
				got <- body.String()
				return
			default:
				say("250 OK")
			}
		}
	}()

	cfg := config.SMTP{Host: host, Port: port, From: "praxis@example.org", Security: "none"}
	err = Send(context.Background(), cfg, Message{To: "patient@example.org", Subject: "Test", Text: "Hallo Welt"})
	if err != nil {
		t.Fatalf("Versand: %v", err)
	}
	select {
	case body := <-got:
		if !strings.Contains(body, "Hallo Welt") || !strings.Contains(body, "Test") {
			t.Errorf("Nachricht unvollständig: %q", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server hat keine Nachricht erhalten")
	}
}

func TestSendWithoutConfigFails(t *testing.T) {
	if err := Send(context.Background(), config.SMTP{}, Message{To: "a@example.org"}); err == nil {
		t.Fatal("Versand ohne Konfiguration muss fehlschlagen")
	}
}
