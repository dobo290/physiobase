package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type Therapist struct {
	ID     int64
	Name   string
	Active bool
}

type Appointment struct {
	ID, PatientID, TherapistID int64
	UID                        string
	Start                      time.Time
	Duration                   int // Minuten
	Title, Location, Note      string
	Cancelled                  bool
	Sequence                   int
	Updated                    time.Time
	TherapistName, PatientName string
}

func (a Appointment) End() time.Time { return a.Start.Add(time.Duration(a.Duration) * time.Minute) }

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand schlägt praktisch nie fehl
	}
	return hex.EncodeToString(b)
}

// ---------- Behandler ----------

func (s *Store) Therapists(ctx context.Context, activeOnly bool) ([]Therapist, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, active FROM therapists WHERE (? = 0 OR active = 1)
		ORDER BY active DESC, name COLLATE NOCASE`, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Therapist
	for rows.Next() {
		var t Therapist
		if err := rows.Scan(&t.ID, &t.Name, &t.Active); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTherapist(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO therapists (name) VALUES (?)`, name)
	return err
}

func (s *Store) UpdateTherapist(ctx context.Context, t Therapist) error {
	_, err := s.db.ExecContext(ctx, `UPDATE therapists SET name = ?, active = ? WHERE id = ?`, t.Name, t.Active, t.ID)
	return err
}

// ---------- Termine ----------

const apptSelect = `SELECT a.id, a.uid, a.patient_id, COALESCE(a.therapist_id, 0), a.starts_at, a.duration_min, a.title,
	a.location, a.note, a.cancelled, a.sequence, a.updated_at, COALESCE(t.name, ''), p.name
	FROM appointments a JOIN patients p ON p.id = a.patient_id LEFT JOIN therapists t ON t.id = a.therapist_id`

func scanAppt(sc scanner) (Appointment, error) {
	var a Appointment
	var start, upd int64
	err := sc.Scan(&a.ID, &a.UID, &a.PatientID, &a.TherapistID, &start, &a.Duration, &a.Title, &a.Location, &a.Note,
		&a.Cancelled, &a.Sequence, &upd, &a.TherapistName, &a.PatientName)
	a.Start, a.Updated = time.Unix(start, 0), time.Unix(upd, 0)
	return a, err
}

func (s *Store) appts(ctx context.Context, where string, args ...any) ([]Appointment, error) {
	rows, err := s.db.QueryContext(ctx, apptSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Appointment
	for rows.Next() {
		a, err := scanAppt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PatientAppointments ab einem Zeitpunkt (Null-Zeit = alle), aufsteigend.
func (s *Store) PatientAppointments(ctx context.Context, pid int64, from time.Time) ([]Appointment, error) {
	var since int64
	if !from.IsZero() {
		since = from.Unix()
	}
	return s.appts(ctx, `WHERE a.patient_id = ? AND a.starts_at >= ? ORDER BY a.starts_at`, pid, since)
}

// AppointmentsBetween für die Praxisübersicht, optional nach Behandler gefiltert.
func (s *Store) AppointmentsBetween(ctx context.Context, from, to time.Time, therapistID int64) ([]Appointment, error) {
	return s.appts(ctx, `WHERE a.starts_at >= ? AND a.starts_at < ? AND (? = 0 OR a.therapist_id = ?) ORDER BY a.starts_at`,
		from.Unix(), to.Unix(), therapistID, therapistID)
}

// NextAppointment nach einem Zeitpunkt (nicht abgesagt).
func (s *Store) NextAppointment(ctx context.Context, pid int64, after time.Time) (Appointment, bool, error) {
	as, err := s.appts(ctx, `WHERE a.patient_id = ? AND a.starts_at > ? AND a.cancelled = 0 ORDER BY a.starts_at LIMIT 1`, pid, after.Unix())
	if err != nil || len(as) == 0 {
		return Appointment{}, false, err
	}
	return as[0], true, nil
}

func (s *Store) Appointment(ctx context.Context, id int64) (Appointment, error) {
	as, err := s.appts(ctx, `WHERE a.id = ?`, id)
	if err != nil {
		return Appointment{}, err
	}
	if len(as) == 0 {
		return Appointment{}, ErrNotFound
	}
	return as[0], nil
}

func nullID(id int64) any {
	if id > 0 {
		return id
	}
	return nil
}

func (s *Store) CreateAppointment(ctx context.Context, a Appointment) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO appointments (uid, patient_id, therapist_id, starts_at, duration_min, title, location, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, randomToken(16), a.PatientID, nullID(a.TherapistID), a.Start.Unix(), a.Duration, a.Title, a.Location, a.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAppointment erhöht sequence, damit Kalender-Apps die Änderung übernehmen.
func (s *Store) UpdateAppointment(ctx context.Context, a Appointment) error {
	res, err := s.db.ExecContext(ctx, `UPDATE appointments SET therapist_id=?, starts_at=?, duration_min=?, title=?, location=?, note=?,
		cancelled=?, sequence=sequence+1, updated_at=unixepoch() WHERE id=?`,
		nullID(a.TherapistID), a.Start.Unix(), a.Duration, a.Title, a.Location, a.Note, a.Cancelled, a.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAppointment(ctx context.Context, id int64) (int64, error) {
	var pid int64
	err := s.db.QueryRowContext(ctx, `DELETE FROM appointments WHERE id = ? RETURNING patient_id`, id).Scan(&pid)
	return pid, notFound(err)
}

// ---------- Kalender-Link ----------

// CalToken liefert den Abo-Token des Patienten und legt ihn bei Bedarf an.
func (s *Store) CalToken(ctx context.Context, pid int64) (string, error) {
	var tok sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT cal_token FROM patients WHERE id = ?`, pid).Scan(&tok); err != nil {
		return "", notFound(err)
	}
	if tok.Valid && tok.String != "" {
		return tok.String, nil
	}
	return s.RotateCalToken(ctx, pid)
}

// RotateCalToken erzeugt einen neuen Link; der alte funktioniert danach nicht mehr.
func (s *Store) RotateCalToken(ctx context.Context, pid int64) (string, error) {
	tok := randomToken(24)
	res, err := s.db.ExecContext(ctx, `UPDATE patients SET cal_token = ? WHERE id = ?`, tok, pid)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrNotFound
	}
	return tok, nil
}

// PatientByCalToken für den öffentlichen Feed.
func (s *Store) PatientByCalToken(ctx context.Context, token string) (int64, error) {
	if len(token) < 32 {
		return 0, ErrNotFound
	}
	var pid int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM patients WHERE cal_token = ?`, token).Scan(&pid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return pid, err
}
