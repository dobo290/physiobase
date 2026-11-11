package store

import "context"

// ResetDemo löscht alle Daten des Demo-Modus (Wissen, Patienten, Befunde, …),
// beendet alle Sitzungen und bereitet die Neubefüllung per Seed vor.
// Benutzer/Passkeys/Einstellungen bleiben erhalten.
func (s *Store) ResetDemo(ctx context.Context) error {
	tables := []string{
		"entry_links", "sources", "media", "handout_items", "handouts", "patient_entries",
		"patient_docs", "appointments", "summaries", "mail_log", "sessions", "befunde",
		"patients", "therapists", "entries",
	}
	for _, t := range tables {
		// t stammt aus einer festen, lokalen Liste – keine Nutzereingabe.
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+t); err != nil { //nolint:gosec // G202
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = 'seeded'`); err != nil {
		return err
	}
	// alle Sitzungen beenden
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET session_version = session_version + 1`); err != nil {
		return err
	}
	return nil
}
