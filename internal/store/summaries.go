package store

import (
	"context"
	"database/sql"
	"time"
)

// Summary ist eine Zusammenfassung zum Mitgeben: Kind "gesamt" oder "sitzung".
type Summary struct {
	ID, PatientID           int64
	SessionID               int64 // 0 = keine
	Kind, Date, Title, Body string
	Created, Updated        time.Time
}

const summaryCols = `id, patient_id, session_id, kind, date, title, body, created_at, updated_at`

func scanSummary(sc scanner) (Summary, error) {
	var x Summary
	var sid sql.NullInt64
	var c, u int64
	err := sc.Scan(&x.ID, &x.PatientID, &sid, &x.Kind, &x.Date, &x.Title, &x.Body, &c, &u)
	x.SessionID, x.Created, x.Updated = sid.Int64, time.Unix(c, 0), time.Unix(u, 0)
	return x, err
}

func (s *Store) Summaries(ctx context.Context, pid int64) ([]Summary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+summaryCols+` FROM summaries WHERE patient_id = ? ORDER BY date DESC, id DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Summary
	for rows.Next() {
		x, err := scanSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) Summary(ctx context.Context, id int64) (Summary, error) {
	x, err := scanSummary(s.db.QueryRowContext(ctx, `SELECT `+summaryCols+` FROM summaries WHERE id = ?`, id))
	return x, notFound(err)
}

func (s *Store) CreateSummary(ctx context.Context, x Summary) (int64, error) {
	var sid any
	if x.SessionID > 0 {
		sid = x.SessionID
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO summaries (patient_id, session_id, kind, date, title, body) VALUES (?, ?, ?, ?, ?, ?)`,
		x.PatientID, sid, x.Kind, x.Date, x.Title, x.Body)
	if err != nil {
		return 0, err
	}
	s.touchPatient(ctx, x.PatientID)
	return res.LastInsertId()
}

func (s *Store) UpdateSummary(ctx context.Context, x Summary) error {
	res, err := s.db.ExecContext(ctx, `UPDATE summaries SET date=?, title=?, body=?, updated_at=unixepoch() WHERE id=?`,
		x.Date, x.Title, x.Body, x.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteSummary(ctx context.Context, id int64) (int64, error) {
	var pid int64
	err := s.db.QueryRowContext(ctx, `DELETE FROM summaries WHERE id = ? RETURNING patient_id`, id).Scan(&pid)
	return pid, notFound(err)
}

func (s *Store) Session(ctx context.Context, id int64) (Session, error) {
	var x Session
	err := s.db.QueryRowContext(ctx, `SELECT id, patient_id, date, goal, note FROM sessions WHERE id = ?`, id).
		Scan(&x.ID, &x.PatientID, &x.Date, &x.Goal, &x.Note)
	return x, notFound(err)
}

// MailLog ist ein Eintrag im Versandprotokoll.
type MailLog struct {
	Recipient, Subject, Error string
	OK                        bool
	Sent                      time.Time
}

func (s *Store) LogMail(ctx context.Context, summaryID, patientID int64, l MailLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mail_log (summary_id, patient_id, recipient, subject, ok, error) VALUES (?, ?, ?, ?, ?, ?)`,
		summaryID, patientID, l.Recipient, l.Subject, l.OK, l.Error)
	return err
}

func (s *Store) MailLogFor(ctx context.Context, summaryID int64) ([]MailLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT recipient, subject, ok, error, sent_at FROM mail_log WHERE summary_id = ? ORDER BY id DESC`, summaryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MailLog
	for rows.Next() {
		var l MailLog
		var t int64
		if err := rows.Scan(&l.Recipient, &l.Subject, &l.OK, &l.Error, &t); err != nil {
			return nil, err
		}
		l.Sent = time.Unix(t, 0)
		out = append(out, l)
	}
	return out, rows.Err()
}
