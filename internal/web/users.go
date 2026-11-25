package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"forge.gpl-nas.de/physiobase/internal/store"
)

const setupValid = 24 * time.Hour

// ---------- /admin ----------

func (s *Server) adminHome(w http.ResponseWriter, r *http.Request) {
	us, err := s.st.Users(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "admin", http.StatusOK, map[string]any{"Users": us, "Passkeys": s.wa != nil})
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	us, err := s.st.Users(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	s.render(w, r, "admin_users", http.StatusOK, map[string]any{"Users": us, "Error": q.Get("fehler"), "Saved": q.Get("gespeichert") == "1"})
}

func (s *Server) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	u := store.User{Username: formText(r, "username"), DisplayName: formText(r, "display_name"), Role: formText(r, "role")}
	if !validUsername(u.Username) {
		redirect(w, r, "/admin/benutzer?fehler=%s", urlPathEscape("Benutzername: 2–40 Zeichen, nur Buchstaben, Ziffern, Punkt, Minus, Unterstrich."))
		return
	}
	if pw := r.FormValue("password"); pw != "" {
		if err := checkPassword(pw); err != nil {
			redirect(w, r, "/admin/benutzer?fehler=%s", urlPathEscape(err.Error()))
			return
		}
		h, err := HashPassword(pw)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		u.PasswordHash = h
	}
	id, err := s.st.CreateUser(r.Context(), u)
	if errors.Is(err, store.ErrNameTaken) {
		redirect(w, r, "/admin/benutzer?fehler=%s", urlPathEscape(err.Error()))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("benutzer angelegt", "benutzer", u.Username, "rolle", u.Role, "von", s.who(r))
	// ohne Passwort: direkt einen Einrichtungs-Link erzeugen
	if u.PasswordHash == "" {
		s.newSetupLink(w, r, id)
		return
	}
	redirect(w, r, "/admin/benutzer/%d?gespeichert=1", id)
}

func validUsername(n string) bool {
	if l := len(n); l < 2 || l > 40 {
		return false
	}
	for _, c := range n {
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		digit := c >= '0' && c <= '9'
		if !letter && !digit && c != '.' && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func (s *Server) who(r *http.Request) string {
	if u, ok := currentUser(r); ok {
		return u.Username
	}
	return "-"
}

func (s *Server) adminUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	u, err := s.st.User(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pks, err := s.st.Passkeys(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	exp, hasLink, _ := s.st.SetupTokenExpiry(ctx, id)
	me, _ := currentUser(r)
	q := r.URL.Query()
	link := ""
	if t := q.Get("link"); t != "" { // nur direkt nach dem Erzeugen sichtbar (Klartext steht nirgends gespeichert)
		link = s.publicBase(r) + "/einrichten/" + t
	}
	s.render(w, r, "admin_user", http.StatusOK, map[string]any{
		"U": u, "PK": pks, "Me": me.ID == u.ID, "HasLink": hasLink, "LinkExpires": exp, "Link": link,
		"Error": q.Get("fehler"), "Saved": q.Get("gespeichert") == "1", "PasskeysOn": s.wa != nil,
	})
}

func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := s.st.UpdateUser(r.Context(), id, formText(r, "display_name"), formText(r, "role"), r.FormValue("active") == "1")
	if errors.Is(err, store.ErrLastAdmin) {
		redirect(w, r, "/admin/benutzer/%d?fehler=%s", id, urlPathEscape(err.Error()))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("benutzer geändert", "id", id, "von", s.who(r))
	redirect(w, r, "/admin/benutzer/%d?gespeichert=1", id)
}

func (s *Server) adminSetPassword(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pw := r.FormValue("password")
	if err := checkPassword(pw); err != nil {
		redirect(w, r, "/admin/benutzer/%d?fehler=%s", id, urlPathEscape(err.Error()))
		return
	}
	h, err := HashPassword(pw)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.SetPassword(r.Context(), id, h); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("passwort zurückgesetzt", "id", id, "von", s.who(r))
	redirect(w, r, "/admin/benutzer/%d?gespeichert=1", id)
}

func (s *Server) adminSetupLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.newSetupLink(w, r, id)
}

func (s *Server) newSetupLink(w http.ResponseWriter, r *http.Request, id int64) {
	tok := randomHex(24)
	if err := s.st.CreateSetupToken(r.Context(), tokenHash(tok), id, setupValid); err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("einrichtungs-link erzeugt", "id", id, "von", s.who(r))
	redirect(w, r, "/admin/benutzer/%d?link=%s#einrichten", id, tok)
}

func (s *Server) adminDeletePasskey(w http.ResponseWriter, r *http.Request) {
	pid, ok := pathID(r, "pid")
	if !ok {
		http.NotFound(w, r)
		return
	}
	uid, err := s.st.DeletePasskey(r.Context(), pid, 0)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("passkey gelöscht", "benutzer-id", uid, "von", s.who(r))
	redirect(w, r, "/admin/benutzer/%d?gespeichert=1#passkeys", uid)
}

func (s *Server) adminEndSessions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := s.st.EndSessions(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/admin/benutzer/%d?gespeichert=1", id)
}

func (s *Server) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if me, _ := currentUser(r); me.ID == id {
		redirect(w, r, "/admin/benutzer/%d?fehler=%s", id, urlPathEscape("Das eigene Konto kann man nicht selbst löschen."))
		return
	}
	err := s.st.DeleteUser(r.Context(), id)
	if errors.Is(err, store.ErrLastAdmin) {
		redirect(w, r, "/admin/benutzer/%d?fehler=%s", id, urlPathEscape(err.Error()))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.log.Info("benutzer gelöscht", "id", id, "von", s.who(r))
	redirect(w, r, "/admin/benutzer?gespeichert=1")
}

// publicBase: Adresse für Links (HTTPS-Adresse der Praxis, sonst wie aufgerufen).
func (s *Server) publicBase(r *http.Request) string {
	if s.cfg.BaseURL != "" {
		return strings.TrimRight(s.cfg.BaseURL, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// ---------- /konto (eigenes Konto) ----------

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.showAccount(w, r, http.StatusOK, map[string]any{
		"Error": q.Get("fehler"), "Saved": q.Get("gespeichert") == "1",
	})
}

// showAccount rendert die Kontoseite samt Passkeys und Zwei-Faktor-Status.
func (s *Server) showAccount(w http.ResponseWriter, r *http.Request, status int, data map[string]any) {
	me, _ := currentUser(r)
	pks, err := s.st.Passkeys(r.Context(), me.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	secret, enabled, err := s.st.TOTPState(r.Context(), me.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	left, err := s.st.RecoveryCodesLeft(r.Context(), me.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	data["U"], data["PK"], data["PasskeysOn"] = me, pks, s.wa != nil
	data["TOTPEnabled"], data["CodesLeft"] = enabled, left
	if !enabled {
		data["TOTPSecret"] = secret // laufende Einrichtung (noch nicht scharf)
	}
	s.render(w, r, "account", status, data)
}

func (s *Server) accountPassword(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	me, _ := currentUser(r)
	if me.PasswordHash != "" {
		if ok := s.passwordMatches(me, r.FormValue("current")); !ok {
			s.pause(time.Second)
			redirect(w, r, "/konto?fehler=%s", urlPathEscape("Das aktuelle Passwort stimmt nicht."))
			return
		}
	}
	if r.FormValue("password") != r.FormValue("password2") {
		redirect(w, r, "/konto?fehler=%s", urlPathEscape("Die beiden neuen Passwörter stimmen nicht überein."))
		return
	}
	if err := checkPassword(r.FormValue("password")); err != nil {
		redirect(w, r, "/konto?fehler=%s", urlPathEscape(err.Error()))
		return
	}
	h, err := HashPassword(r.FormValue("password"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.SetPassword(r.Context(), me.ID, h); err != nil {
		s.fail(w, r, err)
		return
	}
	// Alle Sitzungen sind jetzt beendet – auf diesem Gerät gleich neu anmelden
	u, err := s.st.User(r.Context(), me.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.startSession(w, r, u)
	redirect(w, r, "/konto?gespeichert=1")
}

func (s *Server) accountDeletePasskey(w http.ResponseWriter, r *http.Request) {
	me, _ := currentUser(r)
	pid, ok := pathID(r, "pid")
	if !ok {
		http.NotFound(w, r)
		return
	}
	pks, err := s.st.Passkeys(r.Context(), me.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if me.PasswordHash == "" && len(pks) <= 1 {
		redirect(w, r, "/konto?fehler=%s", urlPathEscape("Das ist dein einziger Zugang – erst ein Passwort setzen oder einen weiteren Passkey anlegen."))
		return
	}
	if _, err := s.st.DeletePasskey(r.Context(), pid, me.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/konto?gespeichert=1#passkeys")
}

// ---------- /einrichten/{token} (Einmal-Link für neue Benutzer) ----------

func (s *Server) setupPage(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	u, err := s.st.SetupTokenUser(r.Context(), tokenHash(tok))
	if err != nil {
		s.render(w, r, "setup", http.StatusNotFound, map[string]any{"Invalid": true})
		return
	}
	s.render(w, r, "setup", http.StatusOK, map[string]any{"U": u, "Token": tok, "PasskeysOn": s.wa != nil, "Error": r.URL.Query().Get("fehler")})
}

func (s *Server) setupPassword(w http.ResponseWriter, r *http.Request) {
	if !limitForm(w, r) {
		return
	}
	tok := r.PathValue("token")
	th := tokenHash(tok)
	u, err := s.st.SetupTokenUser(r.Context(), th)
	if err != nil {
		s.render(w, r, "setup", http.StatusNotFound, map[string]any{"Invalid": true})
		return
	}
	if r.FormValue("password") != r.FormValue("password2") {
		redirect(w, r, "/einrichten/%s?fehler=%s", tok, urlPathEscape("Die beiden Passwörter stimmen nicht überein."))
		return
	}
	if err := checkPassword(r.FormValue("password")); err != nil {
		redirect(w, r, "/einrichten/%s?fehler=%s", tok, urlPathEscape(err.Error()))
		return
	}
	h, err := HashPassword(r.FormValue("password"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.st.SetPassword(r.Context(), u.ID, h); err != nil {
		s.fail(w, r, err)
		return
	}
	_ = s.st.ConsumeSetupToken(r.Context(), th)
	u, _ = s.st.User(r.Context(), u.ID)
	s.startSession(w, r, u)
	s.log.Info("konto eingerichtet (passwort)", "benutzer", u.Username)
	redirect(w, r, "/konto?gespeichert=1")
}

func (s *Server) passwordMatches(u store.User, pw string) bool {
	return u.PasswordHash != "" && bcryptOK(u.PasswordHash, pw)
}
