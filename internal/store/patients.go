package store

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"
)

type Patient struct {
	ID                      int64
	Name, Dob, Phone, Notes string
	Archived                bool
	Created, Updated        time.Time
	Befunde, Sessions       int
	LastDate                string // letzter Befund oder letzte Behandlung (YYYY-MM-DD)
}

type Befund struct {
	ID, PatientID    int64
	Date, Title      string
	Data             string // JSON, siehe Paket befund
	Created, Updated time.Time
}

type Session struct {
	ID, PatientID    int64
	Date, Goal, Note string
}

// ---------- Patienten ----------

func (s *Store) Patients(ctx context.Context, q string, archived bool) ([]Patient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.name, p.dob, p.phone, p.notes, p.archived, p.created_at, p.updated_at,
		(SELECT count(*) FROM befunde b WHERE b.patient_id = p.id),
		(SELECT count(*) FROM sessions x WHERE x.patient_id = p.id),
		COALESCE(max((SELECT max(date) FROM befunde b WHERE b.patient_id = p.id),
		             (SELECT max(date) FROM sessions x WHERE x.patient_id = p.id)), '')
		FROM patients p
		WHERE p.archived = ? AND (? = '' OR p.name LIKE '%' || ? || '%' OR p.dob LIKE '%' || ? || '%')
		ORDER BY p.updated_at DESC LIMIT 500`, archived, q, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Patient
	for rows.Next() {
		p, err := scanPatient(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPatient(sc scanner, withStats bool) (Patient, error) {
	var p Patient
	var c, u int64
	var last sql.NullString
	dest := []any{&p.ID, &p.Name, &p.Dob, &p.Phone, &p.Notes, &p.Archived, &c, &u}
	if withStats {
		dest = append(dest, &p.Befunde, &p.Sessions, &last)
	}
	err := sc.Scan(dest...)
	p.Created, p.Updated, p.LastDate = time.Unix(c, 0), time.Unix(u, 0), last.String
	return p, err
}

func (s *Store) Patient(ctx context.Context, id int64) (Patient, error) {
	p, err := scanPatient(s.db.QueryRowContext(ctx, `SELECT id, name, dob, phone, notes, archived, created_at, updated_at
		FROM patients WHERE id = ?`, id), false)
	return p, notFound(err)
}

func (s *Store) CreatePatient(ctx context.Context, p Patient) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO patients (name, dob, phone, notes) VALUES (?, ?, ?, ?)`, p.Name, p.Dob, p.Phone, p.Notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdatePatient(ctx context.Context, p Patient) error {
	res, err := s.db.ExecContext(ctx, `UPDATE patients SET name=?, dob=?, phone=?, notes=?, updated_at=unixepoch() WHERE id=?`,
		p.Name, p.Dob, p.Phone, p.Notes, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetArchived(ctx context.Context, id int64, archived bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE patients SET archived=?, updated_at=unixepoch() WHERE id=?`, archived, id)
	return err
}

func (s *Store) DeletePatient(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM patients WHERE id = ?`, id)
	return err
}

// touchPatient sortiert den Patienten in der Liste nach oben; ein Fehler ist nicht kritisch.
func (s *Store) touchPatient(ctx context.Context, id int64) {
	if _, err := s.db.ExecContext(ctx, `UPDATE patients SET updated_at = unixepoch() WHERE id = ?`, id); err != nil {
		slog.Warn("patient updated_at", "id", id, "err", err)
	}
}

// FindPatient sucht exakt nach Name (+ Geburtsdatum, falls angegeben) – für den Import.
func (s *Store) FindPatient(ctx context.Context, name, dob string) (int64, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM patients WHERE lower(trim(name)) = lower(trim(?))
		AND (? = '' OR dob = '' OR dob = ?) ORDER BY id LIMIT 1`, name, dob, dob).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return id, err == nil, err
}

// ---------- Befunde ----------

const befundCols = `id, patient_id, date, title, data, created_at, updated_at`

func scanBefund(sc scanner) (Befund, error) {
	var b Befund
	var c, u int64
	err := sc.Scan(&b.ID, &b.PatientID, &b.Date, &b.Title, &b.Data, &c, &u)
	b.Created, b.Updated = time.Unix(c, 0), time.Unix(u, 0)
	return b, err
}

func (s *Store) Befunde(ctx context.Context, pid int64) ([]Befund, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+befundCols+` FROM befunde WHERE patient_id = ? ORDER BY date DESC, id DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Befund
	for rows.Next() {
		b, err := scanBefund(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) Befund(ctx context.Context, id int64) (Befund, error) {
	b, err := scanBefund(s.db.QueryRowContext(ctx, `SELECT `+befundCols+` FROM befunde WHERE id = ?`, id))
	return b, notFound(err)
}

func (s *Store) CreateBefund(ctx context.Context, b Befund) (int64, error) {
	if b.Date == "" {
		b.Date = time.Now().Format("2006-01-02")
	}
	if strings.TrimSpace(b.Title) == "" {
		b.Title = "Befund"
	}
	if b.Data == "" {
		b.Data = "{}"
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO befunde (patient_id, date, title, data) VALUES (?, ?, ?, ?)`,
		b.PatientID, b.Date, b.Title, b.Data)
	if err != nil {
		return 0, err
	}
	s.touchPatient(ctx, b.PatientID)
	return res.LastInsertId()
}

func (s *Store) UpdateBefund(ctx context.Context, b Befund) error {
	res, err := s.db.ExecContext(ctx, `UPDATE befunde SET date=?, title=?, data=?, updated_at=unixepoch() WHERE id=?`,
		b.Date, b.Title, b.Data, b.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.touchPatient(ctx, b.PatientID)
	return nil
}

func (s *Store) DeleteBefund(ctx context.Context, id int64) (int64, error) {
	var pid int64
	err := s.db.QueryRowContext(ctx, `DELETE FROM befunde WHERE id = ? RETURNING patient_id`, id).Scan(&pid)
	return pid, notFound(err)
}

// ---------- Behandlungsverlauf ----------

func (s *Store) Sessions(ctx context.Context, pid int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, patient_id, date, goal, note FROM sessions
		WHERE patient_id = ? ORDER BY date DESC, id DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var x Session
		if err := rows.Scan(&x.ID, &x.PatientID, &x.Date, &x.Goal, &x.Note); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) AddSession(ctx context.Context, x Session) error {
	if x.Date == "" {
		x.Date = time.Now().Format("2006-01-02")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (patient_id, date, goal, note) VALUES (?, ?, ?, ?)`, x.PatientID, x.Date, x.Goal, x.Note)
	if err == nil {
		s.touchPatient(ctx, x.PatientID)
	}
	return err
}

func (s *Store) DeleteSession(ctx context.Context, id int64) (int64, error) {
	var pid int64
	err := s.db.QueryRowContext(ctx, `DELETE FROM sessions WHERE id = ? RETURNING patient_id`, id).Scan(&pid)
	return pid, notFound(err)
}

// ---------- Wissen ↔ Patient ----------

func (s *Store) PatientEntries(ctx context.Context, pid int64) ([]Entry, error) {
	return s.entries(ctx, `SELECT `+entryCols+` FROM patient_entries pe JOIN entries e ON e.id = pe.entry_id
		WHERE pe.patient_id = ? ORDER BY e.kind, e.title COLLATE NOCASE`, pid)
}

func (s *Store) LinkPatientEntry(ctx context.Context, pid, eid int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO patient_entries (patient_id, entry_id) VALUES (?, ?)`, pid, eid)
	return err
}

func (s *Store) UnlinkPatientEntry(ctx context.Context, pid, eid int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM patient_entries WHERE patient_id = ? AND entry_id = ?`, pid, eid)
	return err
}

// EntriesOfKind z. B. alle Tests als Vorschlagsliste im Befund.
func (s *Store) EntriesOfKind(ctx context.Context, kinds ...string) ([]Entry, error) {
	if len(kinds) == 0 {
		return s.entries(ctx, `SELECT `+entryCols+` FROM entries e ORDER BY e.kind, e.title COLLATE NOCASE`)
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",")
	args := make([]any, len(kinds))
	for i, k := range kinds {
		args[i] = k
	}
	return s.entries(ctx, `SELECT `+entryCols+` FROM entries e WHERE e.kind IN (`+ph+`) ORDER BY e.kind, e.title COLLATE NOCASE`, args...)
}

// ---------- Handouts je Patient ----------

func (s *Store) HandoutsForPatient(ctx context.Context, pid int64) ([]Handout, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT h.id, h.title, h.recipient, h.notes, h.created_at, h.updated_at,
		(SELECT count(*) FROM handout_items i WHERE i.handout_id = h.id)
		FROM handouts h WHERE h.patient_id = ? ORDER BY h.updated_at DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Handout
	for rows.Next() {
		var h Handout
		var c, u int64
		if err := rows.Scan(&h.ID, &h.Title, &h.Recipient, &h.Notes, &c, &u, &h.Count); err != nil {
			return nil, err
		}
		h.Created, h.Updated = time.Unix(c, 0), time.Unix(u, 0)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) CreateHandoutFor(ctx context.Context, title string, p Patient) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO handouts (title, recipient, patient_id) VALUES (?, ?, ?)`, title, p.Name, p.ID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// HandoutPatient liefert den zugeordneten Patienten eines Handouts (0 = keiner).
func (s *Store) HandoutPatient(ctx context.Context, hid int64) (int64, string, error) {
	var pid sql.NullInt64
	var name sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT h.patient_id, p.name FROM handouts h LEFT JOIN patients p ON p.id = h.patient_id
		WHERE h.id = ?`, hid).Scan(&pid, &name)
	return pid.Int64, name.String, notFound(err)
}
