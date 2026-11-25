package web

import (
	"bufio"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"forge.gpl-nas.de/physiobase/internal/config"
)

func login(t *testing.T, c *http.Client, base string) {
	t.Helper()
	r, err := post(c, base+"/login", url.Values{"username": {"admin"}, "password": {"geheim123"}})
	if err != nil || r.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %v", err)
	}
}

func TestPrintable(t *testing.T) {
	t.Parallel()
	in := "✎ Hinweis oben\n\n## Ihre Beschwerden\n\nKnie tut weh.\n✎ nur für mich\n\n## Leer\n\n✎ nichts\n\n## Übungen\n\n- Wandsitz\n"
	got := Printable(in)
	if strings.Contains(got, "✎") || strings.Contains(got, "## Leer") {
		t.Errorf("Hinweise oder leerer Abschnitt im Druck:\n%s", got)
	}
	if !strings.Contains(got, "## Ihre Beschwerden") || !strings.Contains(got, "- Wandsitz") {
		t.Errorf("Inhalt fehlt:\n%s", got)
	}
}

func TestPatientDocs(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Doku Test"}})

	upload := func(name, content string) int {
		var buf strings.Builder
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile("files", name)
		io.WriteString(fw, content)
		w.WriteField("category", "Arztbrief / Befund")
		w.WriteField("date", "2026-10-05")
		w.Close()
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/patient/1/dokumente", strings.NewReader(buf.String()))
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		r, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if code := upload("Arztbrief Müller.pdf", "%PDF-1.4 test"); code != http.StatusSeeOther {
		t.Fatalf("PDF-Upload: %d", code)
	}
	if code := upload("boese.exe", "MZ"); code != http.StatusBadRequest {
		t.Fatalf("exe akzeptiert: %d", code)
	}
	_, html := body(t, c, ts.URL+"/patient/1")
	i := strings.Index(html, "/dokumente/1-")
	if i < 0 || !strings.Contains(html, "Arztbrief Müller") || !strings.Contains(html, "05.10.2026") {
		t.Fatal("Dokument nicht auf der Patientenseite")
	}
	link := html[i : i+strings.IndexByte(html[i:], '"')]
	r, _ := c.Get(ts.URL + link)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK || string(b) != "%PDF-1.4 test" || !strings.Contains(r.Header.Get("Cache-Control"), "no-store") {
		t.Fatalf("Dokument abrufen: %d %q", r.StatusCode, r.Header.Get("Cache-Control"))
	}
	// Abgemeldet kein Zugriff
	anon := &http.Client{CheckRedirect: c.CheckRedirect}
	if r, _ := anon.Get(ts.URL + link); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("ohne Login: %d", r.StatusCode)
	}
	// Patient löschen entfernt auch die Datei
	post(c, ts.URL+"/patient/1/loeschen", nil)
	if code, _ := body(t, c, ts.URL+link); code != http.StatusNotFound {
		t.Errorf("Dokument nach Löschen noch da: %d", code)
	}
}

func TestSummaries(t *testing.T) {
	t.Parallel()
	ts, c := newServer(t)
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Sam Beispiel"}})
	post(c, ts.URL+"/patient/1/befund", url.Values{})
	post(c, ts.URL+"/befund/1", url.Values{
		"title": {"Erstbefund"}, "date": {"2026-10-01"}, "f.diag": {"Patellatendinopathie"},
		"f.pain_now": {"6"}, "f.goal_patient": {"Wieder Basketball spielen"}, "t.psfs.n": {"1"}, "t.psfs.0.activity": {"Treppe"}, "t.psfs.0.score": {"4"},
	})
	r, _ := post(c, ts.URL+"/patient/1/handout", url.Values{"title": {"Woche 1"}})
	hid := strings.TrimPrefix(r.Header.Get("Location"), "/handout/")
	post(c, ts.URL+"/handout/"+hid+"/uebung", url.Values{"entry": {"3"}}) // Wandsitz aus den Beispieldaten

	r, _ = post(c, ts.URL+"/patient/1/zusammenfassung", nil)
	loc := r.Header.Get("Location")
	if !strings.HasPrefix(loc, "/zusammenfassung/") {
		t.Fatalf("Gesamtzusammenfassung: %s", loc)
	}
	_, html := body(t, c, ts.URL+loc)
	for _, want := range []string{"Patellatendinopathie", "Schmerz heute: 6 von 10", "Wieder Basketball spielen", "Treppe (heute 4 von 10", "Wandsitz", "Wann Sie sich melden"} {
		if !strings.Contains(html, want) {
			t.Errorf("Entwurf ohne %q", want)
		}
	}
	_, pr := body(t, c, ts.URL+loc+"/druck")
	if strings.Contains(pr, "✎") || !strings.Contains(pr, "Wandsitz") {
		t.Error("Druck enthält Hinweise oder keine Übungen")
	}
	// Behandlungszusammenfassung
	post(c, ts.URL+"/patient/1/verlauf", url.Values{"date": {"2026-10-08"}, "goal": {"Kniestreckung kräftigen, Hüpfen dosiert"}})
	r, _ = post(c, ts.URL+"/verlauf/1/zusammenfassung", nil)
	_, html = body(t, c, ts.URL+r.Header.Get("Location"))
	if !strings.Contains(html, "Ihre Behandlung am 08.10.2026") || !strings.Contains(html, "Kniestreckung kräftigen") {
		t.Error("Behandlungszusammenfassung unvollständig")
	}
	// Ohne SMTP: Senden wird verhindert, mit Begründung
	_, html = body(t, c, ts.URL+loc)
	if !strings.Contains(html, "nicht eingerichtet") {
		t.Error("fehlender Hinweis auf SMTP")
	}
}

// fakeSMTP nimmt genau eine Nachricht an (ohne TLS/Anmeldung) und gibt sie zurück.
func fakeSMTP(t *testing.T) (string, func() string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var got strings.Builder
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		rd := bufio.NewReader(conn)
		say := func(s string) { fmt.Fprintf(conn, "%s\r\n", s) }
		say("220 fake")
		inData := false
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					say("250 ok")
					continue
				}
				mu.Lock()
				got.WriteString(line)
				mu.Unlock()
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
				mu.Lock()
				got.WriteString(line)
				mu.Unlock()
				say("250 ok")
			case cmd == "DATA":
				inData = true
				say("354 go")
			case cmd == "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().String(), func() string { mu.Lock(); defer mu.Unlock(); return got.String() }
}

func TestSendSummary(t *testing.T) {
	t.Parallel()
	addr, received := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	var p int
	fmt.Sscan(port, &p)
	ts, c := newServerCfg(t, func(cfg *config.Config) {
		cfg.SMTP = config.SMTP{Host: host, Port: p, Security: "none", From: "praxis@example.org"}
	})
	login(t, c, ts.URL)
	post(c, ts.URL+"/patienten", url.Values{"name": {"Mia Mail"}, "email": {"mia@example.org"}})
	r, _ := post(c, ts.URL+"/patient/1/zusammenfassung", nil)
	loc := r.Header.Get("Location")

	// ohne Einwilligung: kein Versand
	r, _ = post(c, ts.URL+loc+"/senden", nil)
	if !strings.Contains(r.Header.Get("Location"), "fehler=") {
		t.Fatal("Versand ohne Einwilligung nicht verhindert")
	}
	post(c, ts.URL+"/patient/1", url.Values{"name": {"Mia Mail"}, "email": {"mia@example.org"}, "email_consent": {"1"}})
	post(c, ts.URL+loc, url.Values{"title": {"Ihre Physiotherapie"}, "date": {"2026-11-01"}, "body": {"## Ihre Übungen\n\n- Wandsitz\n✎ geheim"}})
	r, _ = post(c, ts.URL+loc+"/senden", nil)
	if !strings.Contains(r.Header.Get("Location"), "gesendet=1") {
		t.Fatalf("Versand: %s", r.Header.Get("Location"))
	}
	msg := received()
	for _, want := range []string{"RCPT TO:<mia@example.org>", "Subject: Ihre Physiotherapie", "Guten Tag Mia Mail", "Wandsitz", "multipart/alternative"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Mail ohne %q", want)
		}
	}
	if strings.Contains(msg, "geheim") {
		t.Error("✎-Hinweis wurde mitgeschickt")
	}
	if _, html := body(t, c, ts.URL+loc); !strings.Contains(html, "✓") {
		t.Error("Versandprotokoll fehlt")
	}
	// Header-Injektion in der Adresse wird abgelehnt
	r, _ = post(c, ts.URL+"/patient/1", url.Values{"name": {"X"}, "email": {"a@b.de\r\nBcc: evil@x.de"}})
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("Injektion akzeptiert: %d", r.StatusCode)
	}
}
