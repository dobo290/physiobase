package web

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Sitzung = signiertes Cookie (Ablaufzeit + HMAC), kein Serverzustand.
// Der Schlüssel enthält ein gespeichertes Geheimnis; Abmelden erneuert es und
// macht damit alle bisherigen Cookies ungültig (z. B. verlorenes Tablet).
const (
	sessionCookie       = "physiobase_session"
	sessionTTL          = 30 * 24 * time.Hour
	maxConcurrentLogins = 2               // bcrypt ist absichtlich langsam
	loginWait           = 3 * time.Second // so lange wird auf einen Slot gewartet
	maxLoginBody        = 4 << 10
)

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) loadSessionSecret(ctx context.Context) error {
	secret, err := s.st.Setting(ctx, "session_secret")
	if err != nil {
		return err
	}
	if secret == "" {
		secret = randomHex(32)
		if err := s.st.SetSetting(ctx, "session_secret", secret); err != nil {
			return err
		}
	}
	s.sessionSecret = secret
	return nil
}

func (s *Server) rotateSessionSecret(ctx context.Context) error {
	secret := randomHex(32)
	if err := s.st.SetSetting(ctx, "session_secret", secret); err != nil {
		return err
	}
	s.sessionMu.Lock()
	s.sessionSecret = secret
	s.sessionMu.Unlock()
	return nil
}

func (s *Server) sessionKey() []byte {
	s.sessionMu.RLock()
	secret := s.sessionSecret
	s.sessionMu.RUnlock()
	// Benutzername/Hash mit drin: Passwortwechsel beendet alle Sitzungen
	sum := sha256.Sum256([]byte(s.cfg.Username + ":" + s.cfg.PasswordHash + ":" + secret))
	return sum[:]
}

func (s *Server) sign(exp int64) string {
	m := hmac.New(sha256.New, s.sessionKey())
	m.Write([]byte(strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) setCookie(w http.ResponseWriter, value string, maxAge int) {
	// Secure nur hinter HTTPS (PHYSIOBASE_BASE_URL=https://…); im LAN läuft die App auch per HTTP.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure hängt bewusst von der Konfiguration ab
		Name: sessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.IsHTTPS(), SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) hasSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	raw, mac, ok := strings.Cut(c.Value, "|")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || !hmac.Equal([]byte(mac), []byte(s.sign(exp))) {
		return false
	}
	return time.Now().Unix() < exp
}

func publicPath(p string) bool {
	return p == "/login" || p == "/healthz" || strings.HasPrefix(p, "/static/")
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path) || s.hasSession(r) {
			next.ServeHTTP(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if s.hasSession(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, "login", http.StatusOK, nil)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formular ungültig", http.StatusBadRequest)
		return
	}
	// Erst einen Slot holen, dann hashen – begrenzt die CPU-Last bei vielen Versuchen.
	select {
	case s.loginSem <- struct{}{}:
		defer func() { <-s.loginSem }()
	case <-time.After(loginWait):
		s.render(w, r, "login", http.StatusTooManyRequests, map[string]any{"Error": "Zu viele Anmeldeversuche, bitte gleich nochmal."})
		return
	}
	// Passwort immer prüfen, auch bei falschem Benutzernamen (gleiche Laufzeit).
	userOK := subtle.ConstantTimeCompare([]byte(r.PostFormValue("username")), []byte(s.cfg.Username)) == 1
	passOK := bcrypt.CompareHashAndPassword([]byte(s.cfg.PasswordHash), []byte(r.PostFormValue("password"))) == nil
	if !userOK || !passOK {
		s.log.Warn("fehlgeschlagene anmeldung", "remote", r.RemoteAddr)
		time.Sleep(time.Second)
		s.render(w, r, "login", http.StatusUnauthorized, map[string]any{"Error": "Benutzername oder Passwort falsch."})
		return
	}
	exp := time.Now().Add(sessionTTL).Unix()
	s.setCookie(w, fmt.Sprintf("%d|%s", exp, s.sign(exp)), int(sessionTTL.Seconds()))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.hasSession(r) {
		if err := s.rotateSessionSecret(r.Context()); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.setCookie(w, "", -1)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HashPassword wird vom Unterbefehl "hash-password" benutzt.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	return string(h), err
}
