package store

import (
	"context"
	"time"
)

// Doc ist eine eingescannte oder fotografierte Unterlage eines Patienten.
type Doc struct {
	ID, PatientID                       int64
	Filename, Original, Title, Category string
	Date                                string // YYYY-MM-DD
	Size                                int64
	Created                             time.Time
}

// DocCategories für die Auswahlliste; freie Eingabe bleibt möglich.
var DocCategories = []string{
	"Ärztliche Verordnung", "Arztbrief / Befund", "OP-Bericht", "Bildgebung (Röntgen, MRT …)",
	"Entlassungsbericht", "Einwilligung / Formular", "Foto (Verlauf, Wunde, Haltung)", "Sonstiges",
}

func (s *Store) Docs(ctx context.Context, pid int64) ([]Doc, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, patient_id, filename, original, title, category, date, size, created_at
		FROM patient_docs WHERE patient_id = ? ORDER BY date DESC, id DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Doc
	for rows.Next() {
		var d Doc
		var c int64
		if err := rows.Scan(&d.ID, &d.PatientID, &d.Filename, &d.Original, &d.Title, &d.Category, &d.Date, &d.Size, &c); err != nil {
			return nil, err
		}
		d.Created = time.Unix(c, 0)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) AddDoc(ctx context.Context, d Doc) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO patient_docs (patient_id, filename, original, title, category, date, size)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, d.PatientID, d.Filename, d.Original, d.Title, d.Category, d.Date, d.Size)
	if err == nil {
		s.touchPatient(ctx, d.PatientID)
	}
	return err
}

func (s *Store) DeleteDoc(ctx context.Context, id int64) (Doc, error) {
	var d Doc
	err := s.db.QueryRowContext(ctx, `DELETE FROM patient_docs WHERE id = ? RETURNING id, patient_id, filename`, id).
		Scan(&d.ID, &d.PatientID, &d.Filename)
	return d, notFound(err)
}

// DocByFile liefert die Unterlage zu einem gespeicherten Dateinamen (nur bekannte Dateien werden ausgeliefert).
func (s *Store) DocByFile(ctx context.Context, filename string) (Doc, error) {
	var d Doc
	err := s.db.QueryRowContext(ctx, `SELECT id, patient_id, filename, original, title FROM patient_docs WHERE filename = ?`, filename).
		Scan(&d.ID, &d.PatientID, &d.Filename, &d.Original, &d.Title)
	return d, notFound(err)
}

// DocFiles: alle Dateinamen eines Patienten, damit sie beim Löschen des Patienten mit entfernt werden.
func (s *Store) DocFiles(ctx context.Context, pid int64) ([]string, error) {
	docs, err := s.Docs(ctx, pid)
	if err != nil {
		return nil, err
	}
	files := make([]string, len(docs))
	for i, d := range docs {
		files[i] = d.Filename
	}
	return files, nil
}
