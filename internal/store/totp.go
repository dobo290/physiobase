package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// TOTP: optionale Zwei-Faktor-Anmeldung über zeitbasierte Einmalkennwörter.
//
// Ablauf: SetTOTPSecret legt ein noch nicht aktives Geheimnis ab. Erst wenn
// ConfirmTOTP einen gültigen Code gesehen hat, wird die Anmeldung mit
// Einmalkennwort verlangt – so kann sich niemand versehentlich aussperren.
// Die Wiederherstellungscodes werden nur als SHA-256-Hash gespeichert.

// TOTPState liefert Geheimnis und Aktiv-Status eines Benutzers.
func (s *Store) TOTPState(ctx context.Context, userID int64) (secret string, enabled bool, err error) {
	err = notFound(s.db.QueryRowContext(ctx,
		`SELECT totp_secret, totp_enabled FROM users WHERE id = ?`, userID).Scan(&secret, &enabled))
	return secret, enabled, err
}

// SetTOTPSecret legt das Geheimnis einer laufenden Einrichtung ab (noch inaktiv).
func (s *Store) SetTOTPSecret(ctx context.Context, userID int64, secret string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = ?, totp_enabled = 0 WHERE id = ?`, secret, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConfirmTOTP aktiviert die Zwei-Faktor-Anmeldung und speichert die
// Wiederherstellungscodes (Klartextwerte, werden hier gehasht).
func (s *Store) ConfirmTOTP(ctx context.Context, userID int64, codeHashes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Commit unten entscheidet

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET totp_enabled = 1 WHERE id = ? AND totp_secret <> ''`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM totp_recovery WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, h := range codeHashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO totp_recovery (user_id, code_hash) VALUES (?, ?)`, userID, h); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DisableTOTP schaltet die Zwei-Faktor-Anmeldung ab und löscht Geheimnis und
// Wiederherstellungscodes.
func (s *Store) DisableTOTP(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET totp_secret = '', totp_enabled = 0 WHERE id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM totp_recovery WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// UseRecoveryCode verbraucht einen Wiederherstellungscode. ok=false, wenn der
// Hash unbekannt oder schon benutzt ist.
func (s *Store) UseRecoveryCode(ctx context.Context, userID int64, codeHash string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE totp_recovery SET used_at = unixepoch()
		WHERE user_id = ? AND code_hash = ? AND used_at IS NULL`, userID, codeHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// RecoveryCodesLeft zählt die noch nutzbaren Wiederherstellungscodes.
func (s *Store) RecoveryCodesLeft(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM totp_recovery WHERE user_id = ? AND used_at IS NULL`, userID).Scan(&n)
	return n, err
}

// RecoveryCodeHash bildet den Hash, unter dem ein Wiederherstellungscode liegt.
func RecoveryCodeHash(code string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))))
	return hex.EncodeToString(sum[:])
}
