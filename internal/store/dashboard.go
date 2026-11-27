package store

import (
	"context"
	"time"
)

// ---------- Übersicht (Startseite) ----------

// RecentSession ist eine Behandlung aus der jüngsten Vergangenheit, mit Patientennamen.
type RecentSession struct {
	PatientID int64
	Date      string
	Patient   string
	Goal      string
}

// RecentSessions liefert die letzten Behandlungen ab einem Datum (neueste zuerst).
func (s *Store) RecentSessions(ctx context.Context, since time.Time, limit int) ([]RecentSession, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT s.patient_id, s.date, p.name, s.goal
		FROM sessions s JOIN patients p ON p.id = s.patient_id
		WHERE s.date >= ? ORDER BY s.date DESC, s.id DESC LIMIT ?`, since.Format("2006-01-02"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecentSession
	for rows.Next() {
		var r RecentSession
		if err := rows.Scan(&r.PatientID, &r.Date, &r.Patient, &r.Goal); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecentlyUpdatedEntries liefert die zuletzt geänderten Wissenseinträge.
func (s *Store) RecentlyUpdatedEntries(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	return s.entries(ctx, `SELECT `+entryCols+` FROM entries e ORDER BY e.updated_at DESC, e.id DESC LIMIT ?`, limit)
}

// TestRunWithPatient ist eine Einzeldurchführung samt Patient und Testtitel.
type TestRunWithPatient struct {
	TestRun
	Patient string
	Title   string
}

// RecentTestRuns liefert die jüngsten Einzeldurchführungen ab einem Datum.
func (s *Store) RecentTestRuns(ctx context.Context, since time.Time, limit int) ([]TestRunWithPatient, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.patient_id, r.entry_id, r.date, r.data, p.name, e.title
		FROM test_runs r JOIN patients p ON p.id = r.patient_id JOIN entries e ON e.id = r.entry_id
		WHERE r.date >= ? ORDER BY r.date DESC, r.id DESC LIMIT ?`, since.Format("2006-01-02"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestRunWithPatient
	for rows.Next() {
		var r TestRunWithPatient
		if err := rows.Scan(&r.ID, &r.PatientID, &r.EntryID, &r.Date, &r.Data, &r.Patient, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BefundWithPatient ist ein Befund samt Patientennamen (für die Übersicht).
type BefundWithPatient struct {
	Befund
	Patient string
}

// BefundeSince liefert Befunde ab einem Datum (neueste zuerst).
func (s *Store) BefundeSince(ctx context.Context, since time.Time, limit int) ([]BefundWithPatient, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT b.id, b.patient_id, b.date, b.title, b.data, b.created_at, b.updated_at, p.name
		FROM befunde b JOIN patients p ON p.id = b.patient_id
		WHERE b.date >= ? ORDER BY b.date DESC, b.id DESC LIMIT ?`, since.Format("2006-01-02"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BefundWithPatient
	for rows.Next() {
		var b BefundWithPatient
		var c, u int64
		if err := rows.Scan(&b.ID, &b.PatientID, &b.Date, &b.Title, &b.Data, &c, &u, &b.Patient); err != nil {
			return nil, err
		}
		b.Created, b.Updated = time.Unix(c, 0), time.Unix(u, 0)
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpcomingPatientIDs: welche Patienten im Zeitraum einen nicht abgesagten Termin haben.
func (s *Store) UpcomingPatientIDs(ctx context.Context, from, to time.Time) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT patient_id FROM appointments
		WHERE starts_at >= ? AND starts_at < ? AND cancelled = 0`, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// DistinctLocations liefert die in Terminen im Zeitraum verwendeten Orte.
func (s *Store) DistinctLocations(ctx context.Context, from, to time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT location FROM appointments
		WHERE starts_at >= ? AND starts_at < ? AND location <> '' ORDER BY location COLLATE NOCASE`,
		from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err != nil {
			return nil, err
		}
		out = append(out, loc)
	}
	return out, rows.Err()
}
