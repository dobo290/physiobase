package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Demo-Modus: öffentliche Vorschau mit bekanntem Zugang. Beim Abmelden (oder über
// /demo/reset) werden alle Daten gelöscht und neu befüllt. Uploads und E-Mail sind aus.
//
// Statistik nur nach Einwilligung: ohne das Consent-Cookie wird kein Besucher gezählt.

const consentCookie = "physiobase_demo_consent"

type demoStats struct {
	mu    sync.Mutex
	days  map[string]map[string]bool // "2026-11-11" -> gehashte IPs
	total int
}

func newDemoStats() *demoStats {
	return &demoStats{days: map[string]map[string]bool{}}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip := strings.TrimSpace(strings.Split(fwd, ",")[0])
		if ip != "" {
			return ip
		}
	}
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	return host
}

func hashIP(ip string) string {
	sum := sha256.Sum256([]byte("physiobase-demo:" + ip))
	return hex.EncodeToString(sum[:])[:16]
}

// track zählt einen Besuch nur, wenn der Consent-Cookie gesetzt ist.
func (s *Server) trackDemo(r *http.Request) {
	if !s.cfg.Demo {
		return
	}
	if _, err := r.Cookie(consentCookie); err != nil {
		return // ohne Einwilligung kein Tracking
	}
	day := time.Now().Format("2006-01-02")
	h := hashIP(clientIP(r))
	s.demo.mu.Lock()
	defer s.demo.mu.Unlock()
	if s.demo.days[day] == nil {
		s.demo.days[day] = map[string]bool{}
	}
	if !s.demo.days[day][h] {
		s.demo.days[day][h] = true
		s.demo.total++
	}
}

// demoInfo für die Anzeige auf der Anmeldeseite.
func (s *Server) demoInfo() (today, week, total int) {
	s.demo.mu.Lock()
	defer s.demo.mu.Unlock()
	now := time.Now()
	today = len(s.demo.days[now.Format("2006-01-02")])
	total = s.demo.total
	// letzte 7 Tage
	seen := map[string]bool{}
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		for h := range s.demo.days[d] {
			seen[h] = true
		}
	}
	week = len(seen)
	return today, week, total
}

func (s *Server) resetDemo(ctx context.Context, w http.ResponseWriter) {
	if err := s.st.ResetDemo(ctx); err != nil {
		s.log.Error("demo-reset", "err", err)
	}
	if _, err := s.st.Seed(ctx); err != nil {
		s.log.Error("demo-seed", "err", err)
	}
	s.setCookie(w, "", -1)
	s.log.Info("demo zurückgesetzt")
}

func (s *Server) demoResetHandler(w http.ResponseWriter, r *http.Request) {
	s.resetDemo(r.Context(), w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
