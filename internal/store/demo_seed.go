package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"forge.gpl-nas.de/physiobase/internal/dob"
)

// SeedDemoOnce legt die Demo-Daten nur an, wenn noch keine Patienten
// existieren – so hat eine frisch gestartete Demo sofort Inhalte.
func (s *Store) SeedDemoOnce(ctx context.Context) (bool, error) {
	ps, err := s.Patients(ctx, "", false)
	if err != nil {
		return false, err
	}
	if len(ps) > 0 {
		return false, nil
	}
	if err := s.SeedDemo(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// SeedDemo legt drei Beispiel-Behandler und drei Beispiel-Patienten an.
// Wird nur im Demo-Modus aufgerufen (nach ResetDemo + Seed).
func (s *Store) SeedDemo(ctx context.Context) error {
	// Standardisierte Assessments: Grundlage für Suche, Verknüpfung und Verlauf.
	if _, err := s.SeedAssessments(ctx); err != nil {
		return err
	}
	therapists := []string{"Anna Berger (PT)", "Tobias Kern (PT)", "Miriam Stolz (PT)"}
	for _, name := range therapists {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO therapists (name, active) VALUES (?, 1)`, name); err != nil {
			return err
		}
	}

	patients := []struct {
		name, dob, phone, notes string
	}{
		{
			"Erika Musterfrau", "12.04.1969", "07361 123456",
			"Zustand nach Hüft-TEP rechts (03/2026). Ziele: Gangbild, Treppensteigen, Alltagssicherheit.",
		},
		{
			"Jonas Beispielmann", "03.09.1994", "07361 654321",
			"Chronischer unspezifischer Rückenschmerz, Büroarbeit. Ziel: Kraftaufbau, Bewegungsangst abbauen.",
		},
		{
			"Hilde Mustermann", "27.11.1961", "07361 998877",
			"Parkinson-Syndrom, Sturzangst. Ziel: Gleichgewicht, Ganginitiierung, Sturzprophylaxe.",
		},
	}

	ids := make([]int64, 0, len(patients))
	for _, p := range patients {
		dobDate, dobPrecision, dobRaw := dob.Parse(p.dob, time.Now())
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO patients (name, dob, dob_precision, dob_raw, phone, notes, email_consent) VALUES (?, ?, ?, ?, ?, ?, 0)`,
			p.name, dobDate, dobPrecision, dobRaw, p.phone, p.notes)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}

	// Ein Beispiel-Befund beim ersten Patienten (zeigt Befund + Test)
	if len(ids) > 0 {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO befunde (patient_id, date, title, data) VALUES (?, ?, ?, ?)`,
			ids[0], time.Now().AddDate(0, 0, -14).Format("2006-01-02"), "Erstbefund",
			`{"fields":{"diag":"Zustand nach Hüft-TEP rechts","pain_now":"4","main":"Gangbild verbessern"},`+
				`"tables":{"rom":[{"joint":"Hüfte rechts","move":"Flexion","act":"95"}]},"stamps":[],"q":{}}`); err != nil {
			return err
		}
	}

	// Sicherstellen, dass mindestens zwei Punkt-Fragebögen existieren (Demo)
	tests, err := s.entries(ctx, `SELECT `+entryCols+` FROM entries e WHERE e.kind = 'test' AND e.schema <> '' ORDER BY e.title LIMIT 2`)
	if err != nil {
		return err
	}
	if len(tests) < 2 {
		demo := []struct{ title, schema string }{
			{
				"Punkte-Fragebogen Knie (Demo)",
				`{"direction":"lower","items":[{"q":"Schmerzen beim Treppensteigen","max":10},` +
					`{"q":"Schmerzen beim Aufstehen","max":10},{"q":"Schwellung nach Belastung","max":10}]}`,
			},
			{
				"Punkte-Fragebogen Schulter (Demo)",
				`{"direction":"lower","items":[{"q":"Schmerzen in Ruhe","max":10},` +
					`{"q":"Schmerzen bei Überkopfarbeit","max":10},{"q":"Nachtschmerz","max":10}]}`,
			},
		}
		for _, d := range demo {
			if _, err := s.db.ExecContext(ctx,
				`INSERT INTO entries (kind, title, summary, body, region, tags, schema) VALUES ('test', ?, ?, '', '', 'Fragebogen, Demo', ?)`,
				d.title, "Beispielfragebogen für die Demo", d.schema); err != nil {
				return err
			}
		}
		tests, err = s.entries(ctx, `SELECT `+entryCols+` FROM entries e WHERE e.kind = 'test' AND e.schema <> '' ORDER BY e.title LIMIT 2`)
		if err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		for i, t := range tests {
			if err := s.LinkPatientTest(ctx, ids[0], t.ID); err != nil {
				return err
			}
			vals := `{"values":["2","3","2"],"note":"Demo-Datensatz"}`
			if i > 0 {
				vals = `{"values":["1","1","2","1"],"note":""}`
			}
			if err := s.AddTestRun(ctx, ids[0], t.ID,
				time.Now().AddDate(0, 0, -7).Format("2006-01-02"), vals); err != nil {
				return err
			}
		}
	}
	// Beim dritten Patienten einen Test ohne Lauf (zeigt "noch nicht durchgeführt")
	if len(ids) > 2 && len(tests) > 0 {
		if err := s.LinkPatientTest(ctx, ids[2], tests[0].ID); err != nil {
			return err
		}
	}
	// Zwei Assessments mit Verlauf bzw. offen verknüpfen (Suche + Verknüpfung zeigen)
	if len(ids) > 2 {
		if id, err := s.assessmentID(ctx, "Berg Balance Scale"); err == nil && id > 0 {
			if err := s.LinkPatientTest(ctx, ids[2], id); err != nil {
				return err
			}
		}
	}
	if len(ids) > 0 {
		if id, err := s.assessmentID(ctx, "Functional Ambulation Categories"); err == nil && id > 0 {
			if err := s.LinkPatientTest(ctx, ids[0], id); err != nil {
				return err
			}
			if err := s.AddTestRun(ctx, ids[0], id,
				time.Now().AddDate(0, 0, -3).Format("2006-01-02"),
				`{"note":"mit Unterarmgehstütze links"}`); err != nil {
				return err
			}
		}
	}
	return nil
}

// assessmentID findet die ID eines mitgelieferten Assessments über den
// Titel-Anfang. 0 = nicht vorhanden.
func (s *Store) assessmentID(ctx context.Context, prefix string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM entries WHERE kind = 'test' AND title LIKE ? ORDER BY id LIMIT 1`,
		prefix+"%").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
