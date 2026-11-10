package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"forge.gpl-nas.de/physiobase/internal/store"
)

// Passkeys (WebAuthn). Nur mit HTTPS-Adresse (PHYSIOBASE_BASE_URL=https://…) – so verlangen es die Browser.

const ceremonyCookie = "physiobase_passkey"

type ceremony struct {
	data    webauthn.SessionData
	userID  int64  // bei Registrierung: für wen
	token   string // bei Registrierung über Einrichtungs-Link: Hash des Links
	name    string // Name des Passkeys
	expires time.Time
}

type passkeyState struct {
	mu   sync.Mutex
	open map[string]ceremony
}

func newWebAuthn(baseURL, practice string) (*webauthn.WebAuthn, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, nil //nolint:nilnil,nilerr // ohne HTTPS-Adresse keine Passkeys (kein Fehler)
	}
	return webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: practice,
		RPOrigins:     []string{strings.TrimRight(baseURL, "/")},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired, // „Mit Passkey anmelden“ ohne Namen
			UserVerification: protocol.VerificationPreferred,
		},
	})
}

// waUser verbindet einen Benutzer mit seinen gespeicherten Passkeys (Schnittstelle von go-webauthn).
type waUser struct {
	u     store.User
	creds []webauthn.Credential
}

func (w waUser) WebAuthnID() []byte                         { return w.u.WebAuthnID }
func (w waUser) WebAuthnName() string                       { return w.u.Username }
func (w waUser) WebAuthnDisplayName() string                { return w.u.Name() }
func (w waUser) WebAuthnCredentials() []webauthn.Credential { return w.creds }

func (s *Server) loadWAUser(ctx context.Context, u store.User) (waUser, error) {
	pks, err := s.st.Passkeys(ctx, u.ID)
	if err != nil {
		return waUser{}, err
	}
	w := waUser{u: u}
	for _, p := range pks {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(p.Data), &c); err == nil {
			w.creds = append(w.creds, c)
		}
	}
	return w, nil
}

func (s *Server) putCeremony(w http.ResponseWriter, c ceremony) {
	id := randomHex(24)
	c.expires = time.Now().Add(5 * time.Minute)
	s.pk.mu.Lock()
	for k, v := range s.pk.open { // Aufräumen
		if time.Now().After(v.expires) {
			delete(s.pk.open, k)
		}
	}
	s.pk.open[id] = c
	s.pk.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: ceremonyCookie, Value: id, Path: "/", MaxAge: 300, HttpOnly: true, Secure: s.cfg.IsHTTPS(), SameSite: http.SameSiteStrictMode}) //nolint:gosec // G124: Secure wie Sitzungscookie
}

func (s *Server) takeCeremony(r *http.Request) (ceremony, bool) {
	ck, err := r.Cookie(ceremonyCookie)
	if err != nil {
		return ceremony{}, false
	}
	s.pk.mu.Lock()
	defer s.pk.mu.Unlock()
	c, ok := s.pk.open[ck.Value]
	delete(s.pk.open, ck.Value) // jede Zeremonie nur einmal
	if !ok || time.Now().After(c.expires) {
		return ceremony{}, false
	}
	return c, true
}

func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}

func tokenHash(t string) string {
	sum := sha256.Sum256([]byte("physiobase-setup:" + t))
	return hex.EncodeToString(sum[:])
}

// ---------- Registrieren ----------

// passkeyRegisterBegin: eigenes Konto (angemeldet) oder über Einrichtungs-Link (?token=…).
func (s *Server) passkeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if s.wa == nil {
		jsonErr(w, http.StatusServiceUnavailable, "Passkeys gehen nur über die HTTPS-Adresse der Praxis.")
		return
	}
	ctx := r.Context()
	var u store.User
	var th string
	if tok := r.URL.Query().Get("token"); tok != "" {
		var err error
		th = tokenHash(tok)
		if u, err = s.st.SetupTokenUser(ctx, th); err != nil {
			jsonErr(w, http.StatusForbidden, "Der Einrichtungs-Link ist ungültig oder abgelaufen.")
			return
		}
	} else if cu, ok := currentUser(r); ok {
		u = cu
	} else {
		jsonErr(w, http.StatusUnauthorized, "Nicht angemeldet.")
		return
	}
	wu, err := s.loadWAUser(ctx, u)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	opts, data, err := s.wa.BeginRegistration(wu, webauthn.WithExclusions(webauthn.Credentials(wu.creds).CredentialDescriptors()))
	if err != nil {
		s.log.Error("passkey registrierung start", "err", err)
		jsonErr(w, http.StatusInternalServerError, "Passkey konnte nicht vorbereitet werden.")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = "Passkey " + time.Now().Format("02.01.2006")
	}
	s.putCeremony(w, ceremony{data: *data, userID: u.ID, token: th, name: name})
	jsonOut(w, http.StatusOK, opts)
}

func (s *Server) passkeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if s.wa == nil {
		jsonErr(w, http.StatusServiceUnavailable, "Passkeys sind nicht eingerichtet.")
		return
	}
	ctx := r.Context()
	c, ok := s.takeCeremony(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "Zeitüberschreitung – bitte noch einmal versuchen.")
		return
	}
	// Angemeldet: nur für sich selbst. Über Link: der Link muss noch gelten.
	if c.token != "" {
		if u, err := s.st.SetupTokenUser(ctx, c.token); err != nil || u.ID != c.userID {
			jsonErr(w, http.StatusForbidden, "Der Einrichtungs-Link ist ungültig oder abgelaufen.")
			return
		}
	} else if cu, ok := currentUser(r); !ok || cu.ID != c.userID {
		jsonErr(w, http.StatusForbidden, "Nicht erlaubt.")
		return
	}
	u, err := s.st.User(ctx, c.userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	wu, err := s.loadWAUser(ctx, u)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	cred, err := s.wa.FinishRegistration(wu, c.data, r)
	if err != nil {
		s.log.Warn("passkey registrierung", "benutzer", u.Username, "err", err)
		jsonErr(w, http.StatusBadRequest, "Der Passkey konnte nicht gespeichert werden.")
		return
	}
	data, _ := json.Marshal(cred)
	if err := s.st.AddPasskey(ctx, u.ID, cred.ID, string(data), c.name); err != nil {
		s.fail(w, r, err)
		return
	}
	if c.token != "" {
		_ = s.st.ConsumeSetupToken(ctx, c.token)
	}
	s.log.Info("passkey hinzugefügt", "benutzer", u.Username, "name", c.name)
	jsonOut(w, http.StatusOK, map[string]string{"ok": "Passkey gespeichert."})
}

// ---------- Anmelden ----------

func (s *Server) passkeyLoginBegin(w http.ResponseWriter, _ *http.Request) {
	if s.wa == nil {
		jsonErr(w, http.StatusServiceUnavailable, "Passkeys gehen nur über die HTTPS-Adresse der Praxis.")
		return
	}
	opts, data, err := s.wa.BeginDiscoverableLogin()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "Anmeldung konnte nicht vorbereitet werden.")
		return
	}
	s.putCeremony(w, ceremony{data: *data})
	jsonOut(w, http.StatusOK, opts)
}

func (s *Server) passkeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if s.wa == nil {
		jsonErr(w, http.StatusServiceUnavailable, "Passkeys sind nicht eingerichtet.")
		return
	}
	ctx := r.Context()
	c, ok := s.takeCeremony(r)
	if !ok {
		jsonErr(w, http.StatusBadRequest, "Zeitüberschreitung – bitte noch einmal versuchen.")
		return
	}
	var found store.User
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		u, err := s.st.UserByWebAuthnID(ctx, userHandle)
		if err != nil || !u.Active {
			return nil, errors.New("unbekannter oder deaktivierter Benutzer")
		}
		found = u
		return s.loadWAUser(ctx, u)
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	cred, err := s.wa.FinishDiscoverableLogin(handler, c.data, r)
	if err != nil {
		s.log.Warn("passkey anmeldung fehlgeschlagen", "remote", r.RemoteAddr, "err", err)
		time.Sleep(time.Second)
		jsonErr(w, http.StatusUnauthorized, "Anmeldung mit diesem Passkey nicht möglich.")
		return
	}
	data, _ := json.Marshal(cred)
	if err := s.st.UpdatePasskeyUse(ctx, cred.ID, string(data)); err != nil {
		s.log.Warn("passkey zähler", "err", err)
	}
	s.startSession(w, r, found)
	jsonOut(w, http.StatusOK, map[string]string{"ok": "angemeldet", "redirect": "/"})
}
