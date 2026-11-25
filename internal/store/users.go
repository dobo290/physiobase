package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrLastAdmin = errors.New("der letzte aktive Admin kann nicht entfernt, deaktiviert oder herabgestuft werden")
	ErrNameTaken = errors.New("dieser Benutzername ist schon vergeben")
)

type User struct {
	ID                                  int64
	Username, DisplayName, PasswordHash string
	Role                                string // "admin" oder "user"
	Active                              bool
	SessionVersion                      int64
	WebAuthnID                          []byte
	Created                             time.Time
	LastLogin                           time.Time // Nullzeit = nie
	Passkeys                            int       // nur in Users()
	TOTPEnabled                         bool      // Zwei-Faktor-Anmeldung aktiv
}

func (u User) IsAdmin() bool { return u.Role == "admin" }

// Name für die Anzeige: Anzeigename, sonst Benutzername.
func (u User) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

type Passkey struct {
	ID, UserID   int64
	CredentialID []byte
	Data, Name   string
	Created      time.Time
	LastUsed     time.Time
}

const userCols = `id, username, display_name, password_hash, role, active, session_version, webauthn_id, created_at,
	COALESCE(last_login_at, 0), totp_enabled`

func scanUser(sc scanner) (User, error) {
	var u User
	var c, l int64
	err := sc.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.Active, &u.SessionVersion, &u.WebAuthnID, &c, &l, &u.TOTPEnabled)
	u.Created = time.Unix(c, 0)
	if l > 0 {
		u.LastLogin = time.Unix(l, 0)
	}
	return u, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+`, (SELECT count(*) FROM passkeys p WHERE p.user_id = users.id)
		FROM users ORDER BY active DESC, role, username COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var c, l int64
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.Active, &u.SessionVersion,
			&u.WebAuthnID, &c, &l, &u.TOTPEnabled, &u.Passkeys); err != nil {
			return nil, err
		}
		u.Created = time.Unix(c, 0)
		if l > 0 {
			u.LastLogin = time.Unix(l, 0)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) User(ctx context.Context, id int64) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
	return u, notFound(err)
}

func (s *Store) UserByName(ctx context.Context, name string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, strings.TrimSpace(name)))
	return u, notFound(err)
}

func (s *Store) UserByWebAuthnID(ctx context.Context, wid []byte) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE webauthn_id = ?`, wid))
	return u, notFound(err)
}

func (s *Store) CreateUser(ctx context.Context, u User) (int64, error) {
	wid := make([]byte, 32)
	if _, err := rand.Read(wid); err != nil {
		return 0, err
	}
	if u.Role != "admin" {
		u.Role = "user"
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (username, display_name, password_hash, role, active, webauthn_id)
		VALUES (?, ?, ?, ?, 1, ?)`, strings.TrimSpace(u.Username), strings.TrimSpace(u.DisplayName), u.PasswordHash, u.Role, wid)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrNameTaken
		}
		return 0, err
	}
	return res.LastInsertId()
}

// activeAdminsExcept: Anzahl aktiver Admins ohne den angegebenen Benutzer.
func (s *Store) activeAdminsExcept(ctx context.Context, id int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role = 'admin' AND active = 1 AND id <> ?`, id).Scan(&n)
	return n, err
}

// UpdateUser ändert Anzeigename, Rolle und Aktiv-Status. Schützt den letzten Admin.
// Deaktivieren oder Herabstufen beendet die Sitzungen des Benutzers.
func (s *Store) UpdateUser(ctx context.Context, id int64, displayName, role string, active bool) error {
	old, err := s.User(ctx, id)
	if err != nil {
		return err
	}
	if role != "admin" {
		role = "user"
	}
	if old.IsAdmin() && old.Active && (role != "admin" || !active) {
		n, err := s.activeAdminsExcept(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrLastAdmin
		}
	}
	bump := 0
	if (old.Active && !active) || (old.IsAdmin() && role != "admin") {
		bump = 1
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET display_name = ?, role = ?, active = ?, session_version = session_version + ?
		WHERE id = ?`, strings.TrimSpace(displayName), role, active, bump, id)
	return err
}

// SetPassword setzt ein neues Passwort (Hash) und beendet alle Sitzungen des Benutzers.
func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, session_version = session_version + 1 WHERE id = ?`, hash, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// EndSessions beendet alle Sitzungen des Benutzers (z. B. beim Abmelden).
func (s *Store) EndSessions(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET session_version = session_version + 1 WHERE id = ?`, id)
	return err
}

func (s *Store) TouchLogin(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = unixepoch() WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	u, err := s.User(ctx, id)
	if err != nil {
		return err
	}
	if u.IsAdmin() && u.Active {
		n, err := s.activeAdminsExcept(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrLastAdmin
		}
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// ---------- Passkeys ----------

func (s *Store) Passkeys(ctx context.Context, userID int64) ([]Passkey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, credential_id, data, name, created_at, COALESCE(last_used_at, 0)
		FROM passkeys WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Passkey
	for rows.Next() {
		var p Passkey
		var c, l int64
		if err := rows.Scan(&p.ID, &p.UserID, &p.CredentialID, &p.Data, &p.Name, &c, &l); err != nil {
			return nil, err
		}
		p.Created = time.Unix(c, 0)
		if l > 0 {
			p.LastUsed = time.Unix(l, 0)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) AddPasskey(ctx context.Context, userID int64, credID []byte, data, name string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO passkeys (user_id, credential_id, data, name) VALUES (?, ?, ?, ?)`,
		userID, credID, data, strings.TrimSpace(name))
	return err
}

// UpdatePasskeyUse speichert den neuen Zustand (Signaturzähler …) nach einer Anmeldung.
func (s *Store) UpdatePasskeyUse(ctx context.Context, credID []byte, data string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE passkeys SET data = ?, last_used_at = unixepoch() WHERE credential_id = ?`, data, credID)
	return err
}

// DeletePasskey löscht einen Passkey; userID > 0 beschränkt auf diesen Benutzer (eigenes Konto).
func (s *Store) DeletePasskey(ctx context.Context, id, userID int64) (int64, error) {
	var owner int64
	err := s.db.QueryRowContext(ctx, `DELETE FROM passkeys WHERE id = ? AND (? = 0 OR user_id = ?) RETURNING user_id`,
		id, userID, userID).Scan(&owner)
	return owner, notFound(err)
}

// ---------- Einrichtungs-Links ----------

func (s *Store) CreateSetupToken(ctx context.Context, hash string, userID int64, valid time.Duration) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM setup_tokens WHERE expires_at < unixepoch() OR user_id = ?`, userID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO setup_tokens (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hash, userID, time.Now().Add(valid).Unix())
	return err
}

// SetupTokenUser prüft einen Link (ohne ihn zu verbrauchen).
func (s *Store) SetupTokenUser(ctx context.Context, hash string) (User, error) {
	var uid int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM setup_tokens WHERE token_hash = ? AND expires_at > unixepoch()`, hash).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u, err := s.User(ctx, uid)
	if err != nil || !u.Active {
		return User{}, ErrNotFound
	}
	return u, nil
}

// ConsumeSetupToken entwertet einen Link (nach erfolgreicher Einrichtung).
func (s *Store) ConsumeSetupToken(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM setup_tokens WHERE token_hash = ?`, hash)
	return err
}

// HasSetupToken: gibt es für den Benutzer einen gültigen Link? (Anzeige in der Verwaltung)
func (s *Store) SetupTokenExpiry(ctx context.Context, userID int64) (time.Time, bool, error) {
	var exp int64
	err := s.db.QueryRowContext(ctx, `SELECT max(expires_at) FROM setup_tokens WHERE user_id = ? AND expires_at > unixepoch()`, userID).Scan(&exp)
	if err != nil || exp == 0 {
		return time.Time{}, false, nil //nolint:nilerr // keine Zeile/NULL = kein Link
	}
	return time.Unix(exp, 0), true, nil
}
