package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/otp"
	"forge.gpl-nas.de/physiobase/internal/store"
)

// Zwei-Faktor-Anmeldung (TOTP), optional pro Benutzer:
//
//	Passwort richtig + 2FA aktiv  → Zwischenschritt mit Einmalcode (5 Minuten gültig)
//	Passkey                        → genügt für sich, kein zweiter Schritt
//
// Das Geheimnis liegt im Klartext in der Datenbank (wie die Passkey-Daten) –
// Verschlüsselung der Datenbank steht auf der Roadmap.
const (
	twoFactorCookie = "physiobase_2fa"
	twoFactorTTL    = 5 * time.Minute
	recoveryCodeNum = 8
)

// twoFactorMAC signiert das Zwischen-Cookie der zweiten Stufe.
func (s *Server) twoFactorMAC(uid, exp int64) string {
	m := hmac.New(sha256.New, s.sessionKey)
	_, _ = fmt.Fprintf(m, "2fa|%d|%d", uid, exp) // Hash-Schreibfehler gibt es nicht
	return hex.EncodeToString(m.Sum(nil))
}

// startTwoFactor verlangt nach gültigem Passwort den Einmalcode.
func (s *Server) startTwoFactor(w http.ResponseWriter, r *http.Request, u store.User) {
	exp := time.Now().Add(twoFactorTTL).Unix()
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure hängt bewusst von der Konfiguration ab
		Name: twoFactorCookie, Value: fmt.Sprintf("%d.%d.%s", u.ID, exp, s.twoFactorMAC(u.ID, exp)),
		Path: "/", MaxAge: int(twoFactorTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.IsHTTPS(), SameSite: http.SameSiteLaxMode,
	})
	s.log.Info("zwei-faktor-anmeldung angefordert", "benutzer", u.Username)
}

// clearTwoFactor löscht das Zwischen-Cookie.
func (s *Server) clearTwoFactor(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Lösch-Cookie, Attribute wie im setCookie-Helfer
		Name: twoFactorCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.IsHTTPS(), SameSite: http.SameSiteLaxMode,
	})
}

// pendingTwoFactor liefert den Benutzer aus dem Zwischen-Cookie.
func (s *Server) pendingTwoFactor(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(twoFactorCookie)
	if err != nil {
		return store.User{}, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 3 {
		return store.User{}, false
	}
	uid, err1 := strconv.ParseInt(parts[0], 10, 64)
	exp, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || time.Now().Unix() >= exp {
		return store.User{}, false
	}
	if !hmac.Equal([]byte(parts[2]), []byte(s.twoFactorMAC(uid, exp))) {
		return store.User{}, false
	}
	u, err := s.st.User(r.Context(), uid)
	if err != nil || !u.Active || !u.TOTPEnabled {
		return store.User{}, false
	}
	return u, true
}

func (s *Server) loginTwoFactorForm(w http.ResponseWriter, r *http.Request) {
	u, ok := s.pendingTwoFactor(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, r, "login_2fa", http.StatusOK, map[string]any{"Username": u.Username})
}

// loginTwoFactor prüft Code oder Wiederherstellungscode und startet die Sitzung.
func (s *Server) loginTwoFactor(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	u, ok := s.pendingTwoFactor(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	select {
	case s.loginSem <- struct{}{}:
		defer func() { <-s.loginSem }()
	case <-time.After(loginWait):
		s.render(w, r, "login_2fa", http.StatusTooManyRequests,
			map[string]any{"Username": u.Username, "Error": "Zu viele Anmeldeversuche, bitte gleich nochmal."})
		return
	}
	secret, enabled, err := s.st.TOTPState(r.Context(), u.ID)
	if err != nil || !enabled {
		s.clearTwoFactor(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if recovery := formText(r, "recovery"); recovery != "" {
		ok, err := s.st.UseRecoveryCode(r.Context(), u.ID, store.RecoveryCodeHash(recovery))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if !ok {
			time.Sleep(time.Second)
			s.render(w, r, "login_2fa", http.StatusUnauthorized,
				map[string]any{"Username": u.Username, "Error": "Dieser Wiederherstellungscode gilt nicht (oder wurde schon benutzt)."})
			return
		}
		s.log.Warn("anmeldung mit wiederherstellungscode", "benutzer", u.Username)
		s.finishTwoFactor(w, r, u)
		return
	}

	if !otp.Verify(secret, formText(r, "code"), time.Now()) {
		time.Sleep(time.Second)
		s.render(w, r, "login_2fa", http.StatusUnauthorized,
			map[string]any{"Username": u.Username, "Error": "Der Code stimmt nicht oder ist abgelaufen."})
		return
	}
	s.finishTwoFactor(w, r, u)
}

// finishTwoFactor beendet den Zwischenschritt und meldet an.
func (s *Server) finishTwoFactor(w http.ResponseWriter, r *http.Request, u store.User) {
	s.clearTwoFactor(w)
	s.startSession(w, r, u)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) loginTwoFactorCancel(w http.ResponseWriter, r *http.Request) {
	s.clearTwoFactor(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ---------- Einrichtung im eigenen Konto ----------

// otpIssuer ist der Name, den die Authenticator-App anzeigt.
func (s *Server) otpIssuer() string { return s.practice() }

// totpQRCode zeichnet den otpauth-Link des angemeldeten Benutzers als SVG.
func (s *Server) totpQRCode(w http.ResponseWriter, r *http.Request) {
	me, _ := currentUser(r)
	secret, enabled, err := s.st.TOTPState(r.Context(), me.ID)
	if err != nil || secret == "" || enabled {
		http.NotFound(w, r)
		return
	}
	svg, err := qrSVG(otp.URL(s.otpIssuer(), me.Username, secret))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(svg)
}

// accountTOTPStart legt ein Geheimnis an und zeigt QR-Code und Bestätigungsfeld.
func (s *Server) accountTOTPStart(w http.ResponseWriter, r *http.Request) {
	me, _ := currentUser(r)
	if _, enabled, err := s.st.TOTPState(r.Context(), me.ID); err != nil {
		s.fail(w, r, err)
		return
	} else if enabled {
		redirect(w, r, "/konto?fehler=%s", urlPathEscape("Die Zwei-Faktor-Anmeldung ist schon aktiv."))
		return
	}
	secret, err := otp.NewSecret()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.SetTOTPSecret(r.Context(), me.ID, secret); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("zwei-faktor-einrichtung gestartet", "benutzer", me.Username)
	redirect(w, r, "/konto#2fa")
}

// accountTOTPConfirm schaltet die Anmeldung scharf, nachdem ein Code stimmte,
// und zeigt die Wiederherstellungscodes genau einmal an.
func (s *Server) accountTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	me, _ := currentUser(r)
	secret, enabled, err := s.st.TOTPState(r.Context(), me.ID)
	if err != nil || secret == "" {
		redirect(w, r, "/konto?fehler=%s", urlPathEscape("Es läuft keine Einrichtung. Bitte neu beginnen."))
		return
	}
	if enabled {
		redirect(w, r, "/konto")
		return
	}
	if !otp.Verify(secret, formText(r, "code"), time.Now()) {
		time.Sleep(time.Second)
		redirect(w, r, "/konto?fehler=%s", urlPathEscape("Der Code stimmt nicht. Uhrzeit des Geräts prüfen und den aktuellen Code eingeben."))
		return
	}
	codes := make([]string, 0, recoveryCodeNum)
	hashes := make([]string, 0, recoveryCodeNum)
	for range recoveryCodeNum {
		c, err := otp.NewRecoveryCode()
		if err != nil {
			s.fail(w, r, err)
			return
		}
		codes = append(codes, c)
		hashes = append(hashes, store.RecoveryCodeHash(c))
	}
	if err := s.st.ConfirmTOTP(r.Context(), me.ID, hashes); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("zwei-faktor-anmeldung aktiviert", "benutzer", me.Username)
	// Die Klartext-Codes gibt es nur in dieser Antwort.
	s.showAccount(w, r, http.StatusOK, map[string]any{"Codes": codes, "Saved": true})
}

// accountTOTPCancel verwirft eine laufende Einrichtung.
func (s *Server) accountTOTPCancel(w http.ResponseWriter, r *http.Request) {
	me, _ := currentUser(r)
	if err := s.st.DisableTOTP(r.Context(), me.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/konto")
}

// accountTOTPDisable schaltet die Zwei-Faktor-Anmeldung ab.
func (s *Server) accountTOTPDisable(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	me, _ := currentUser(r)
	if me.PasswordHash != "" && !s.passwordMatches(me, r.FormValue("current")) {
		time.Sleep(time.Second)
		redirect(w, r, "/konto?fehler=%s", urlPathEscape("Das aktuelle Passwort stimmt nicht."))
		return
	}
	if err := s.st.DisableTOTP(r.Context(), me.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	// Wer ohne Passwort und ohne Passkey nur den Code hatte, soll neu anmelden.
	if me.PasswordHash == "" {
		if err := s.st.EndSessions(r.Context(), me.ID); err != nil {
			s.fail(w, r, err)
			return
		}
		redirect(w, r, "/login")
		return
	}
	s.log.Warn("zwei-faktor-anmeldung abgeschaltet", "benutzer", me.Username)
	redirect(w, r, "/konto?gespeichert=1")
}

// adminResetTOTP schaltet die Zwei-Faktor-Anmeldung eines Benutzers ab, wenn
// Gerät oder Wiederherstellungscodes verloren sind. Die Aktion wird protokolliert.
func (s *Server) adminResetTOTP(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	u, err := s.st.User(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.DisableTOTP(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Warn("zwei-faktor-anmeldung zurückgesetzt", "benutzer", u.Username, "von", s.who(r))
	redirect(w, r, "/admin/benutzer/%d?gespeichert=1#2fa", id)
}
