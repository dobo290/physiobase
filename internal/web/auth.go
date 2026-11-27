package web

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"forge.gpl-nas.de/physiobase/internal/store"
)

// Sitzung = signiertes Cookie „uid.version.ablauf.mac“. Bei jeder Anfrage wird der
// Benutzer geladen: deaktiviert oder andere Version (Abmelden, Passwortwechsel,
// Deaktivieren) → Sitzung ungültig, und zwar auf allen Geräten.
const (
	sessionCookie       = "physiobase_session"
	sessionTTL          = 30 * 24 * time.Hour
	maxConcurrentLogins = 2               // bcrypt ist absichtlich langsam
	loginWait           = 3 * time.Second // so lange wird auf einen Slot gewartet
	maxLoginBody        = 4 << 10
)

type ctxKey struct{}

// currentUser liefert den angemeldeten Benutzer (nur hinter requireAuth gesetzt).
func currentUser(r *http.Request) (store.User, bool) {
	u, ok := r.Context().Value(ctxKey{}).(store.User)
	return u, ok
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
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
	sum := sha256.Sum256([]byte("physiobase-session:" + secret))
	s.sessionKey = sum[:]
	return nil
}

// bootstrapAdmin: Gibt es noch keinen Benutzer, wird der Login aus der .env zum ersten Admin.
func (s *Server) bootstrapAdmin(ctx context.Context) error {
	n, err := s.st.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	if s.cfg.Username == "" || s.cfg.PasswordHash == "" {
		return errors.New("keine Benutzer vorhanden: PHYSIOBASE_USERNAME und PHYSIOBASE_PASSWORD_HASH für den ersten Admin setzen")
	}
	if _, err := s.st.CreateUser(ctx, store.User{Username: s.cfg.Username, PasswordHash: s.cfg.PasswordHash, Role: "admin"}); err != nil {
		return err
	}
	s.log.Info("erster admin angelegt", "benutzer", s.cfg.Username)
	return nil
}

func (s *Server) mac(uid, ver, exp int64) string {
	m := hmac.New(sha256.New, s.sessionKey)
	_, _ = fmt.Fprintf(m, "%d|%d|%d", uid, ver, exp) // Schreiben in einen Hash schlägt nie fehl
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) setCookie(w http.ResponseWriter, value string, maxAge int) {
	// Secure nur hinter HTTPS (PHYSIOBASE_BASE_URL=https://…); im LAN läuft die App auch per HTTP.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure hängt bewusst von der Konfiguration ab
		Name: sessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.IsHTTPS(), SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u store.User) {
	exp := time.Now().Add(sessionTTL).Unix()
	s.setCookie(w, fmt.Sprintf("%d.%d.%d.%s", u.ID, u.SessionVersion, exp, s.mac(u.ID, u.SessionVersion, exp)), int(sessionTTL.Seconds()))
	if err := s.st.TouchLogin(r.Context(), u.ID); err != nil {
		s.log.Warn("letzte anmeldung", "err", err)
	}
	s.log.Info("anmeldung", "benutzer", u.Username)
}

// sessionUser prüft das Cookie und lädt den Benutzer.
func (s *Server) sessionUser(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return store.User{}, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 4 {
		return store.User{}, false
	}
	var n [3]int64
	for i := range 3 {
		if n[i], err = strconv.ParseInt(parts[i], 10, 64); err != nil {
			return store.User{}, false
		}
	}
	uid, ver, exp := n[0], n[1], n[2]
	if time.Now().Unix() >= exp || !hmac.Equal([]byte(parts[3]), []byte(s.mac(uid, ver, exp))) {
		return store.User{}, false
	}
	u, err := s.st.User(r.Context(), uid)
	if err != nil || !u.Active || u.SessionVersion != ver {
		return store.User{}, false
	}
	return u, true
}

func publicPath(p string) bool {
	// /kalender/ ist der Abo-Feed für Kalender-Apps: geschützt durch den geheimen Token im Pfad.
	// /einrichten/ sind Einmal-Links für neue Benutzer; /passkey/login/ die Passkey-Anmeldung.
	// /passkey/registrieren/ prüft selbst: angemeldet ODER gültiger Einrichtungs-Link (neue Benutzer).
	// /login/2fa ist der zweite Schritt der Anmeldung (Einmalcode).
	// Manifest und Icons braucht der Browser schon auf der Anmeldeseite (Installation).
	return p == "/login" || p == "/healthz" || p == "/manifest.webmanifest" ||
		strings.HasPrefix(p, "/pwa/icon/") || strings.HasPrefix(p, "/login/2fa") ||
		strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/kalender/") ||
		strings.HasPrefix(p, "/branding/") || strings.HasPrefix(p, "/einrichten/") || strings.HasPrefix(p, "/passkey/login/") ||
		strings.HasPrefix(p, "/passkey/registrieren/")
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := s.sessionUser(r); ok {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
			return
		}
		if publicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

// adminOnly: Nicht-Admins sehen 404 – die Verwaltung soll nicht auffallen.
func (s *Server) adminOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if u, ok := currentUser(r); !ok || !u.IsAdmin() {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := currentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, "login", http.StatusOK, nil)
}

// Vergleichs-Hash für unbekannte Benutzernamen (gleiche Laufzeit wie ein echter Vergleich).
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte(randomHex(16)), bcryptCost)
	return h
})

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
	u, err := s.st.UserByName(r.Context(), r.PostFormValue("username"))
	hash := dummyHash()
	if err == nil && u.PasswordHash != "" {
		hash = []byte(u.PasswordHash)
	}
	passOK := bcrypt.CompareHashAndPassword(hash, []byte(r.PostFormValue("password"))) == nil
	if err != nil || !u.Active || u.PasswordHash == "" || !passOK {
		s.log.Warn("fehlgeschlagene anmeldung", "remote", r.RemoteAddr)
		s.pause(time.Second)
		s.render(w, r, "login", http.StatusUnauthorized, map[string]any{"Error": "Benutzername oder Passwort falsch."})
		return
	}
	// Zwei-Faktor-Anmeldung: erst der Einmalcode, dann die Sitzung.
	if u.TOTPEnabled {
		s.startTwoFactor(w, r, u)
		http.Redirect(w, r, "/login/2fa", http.StatusSeeOther)
		return
	}
	s.startSession(w, r, u)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// logout beendet alle Sitzungen dieses Benutzers (z. B. auch auf einem verlorenen Tablet).
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Demo {
		s.resetDemo(r.Context(), w) // löscht Daten und beendet die Sitzung
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if u, ok := currentUser(r); ok {
		if err := s.st.EndSessions(r.Context(), u.ID); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.setCookie(w, "", -1)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// bcryptCost ist der Aufwand für Passwort-Hashes. 12 kostet absichtlich Zeit
// (Brute-Force-Bremse). Tests setzen PHYSIOBASE_BCRYPT_COST niedriger, weil sie
// in einem Lauf hunderte Hashes erzeugen.
var bcryptCost = func() int {
	if v, err := strconv.Atoi(os.Getenv("PHYSIOBASE_BCRYPT_COST")); err == nil && v >= bcrypt.MinCost && v <= bcrypt.MaxCost {
		return v
	}
	return 12
}()

// HashPassword wird vom Unterbefehl "hash-password" und von der Benutzerverwaltung benutzt.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	return string(h), err
}

func bcryptOK(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

const minPasswordLen = 10

func checkPassword(pw string) error {
	if len([]rune(pw)) < minPasswordLen {
		return fmt.Errorf("das Passwort muss mindestens %d Zeichen haben", minPasswordLen)
	}
	return nil
}
