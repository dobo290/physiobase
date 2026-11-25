package database

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
)

// TestMigrateFromEveryVersion prüft jede Zwischenstufe: eine Datenbank wird auf
// Stand k gebracht, mit Daten gefüllt, dann auf den aktuellen Stand migriert.
// Die Daten müssen erhalten bleiben und die neuen Spalten/Tabellen da sein.
func TestMigrateFromEveryVersion(t *testing.T) {
	head := len(migrations)
	for k := 1; k < head; k++ {
		t.Run(versionName(k), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "old.db")
			db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
			if err != nil {
				t.Fatal(err)
			}
			// Schema auf Stand k bringen (ohne migrate()).
			if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
				t.Fatal(err)
			}
			for _, m := range migrations {
				if m.version > k {
					break
				}
				if _, err := db.Exec(m.sql); err != nil {
					t.Fatalf("version %d: %v", m.version, err)
				}
				if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
					t.Fatal(err)
				}
			}
			// Daten, die es in dieser Version schon gibt (patients kam erst später).
			hasPatients := hasTable(t, db, "patients")
			if _, err := db.Exec(`INSERT INTO entries (kind, title) VALUES ('test', 'Alter Test')`); err != nil {
				t.Fatal(err)
			}
			if hasPatients {
				if _, err := db.Exec(`INSERT INTO patients (name, dob) VALUES ('Altbestand', '01.02.1950')`); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			// Öffnen migriert auf den aktuellen Stand.
			db2, err := Open(path)
			if err != nil {
				t.Fatalf("migration von %d: %v", k, err)
			}
			defer Close(db2)

			var version int
			if err := db2.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
				t.Fatal(err)
			}
			if version != head {
				t.Errorf("Version = %d, erwartet %d", version, head)
			}

			// Daten überlebt?
			var title string
			if err := db2.QueryRow(`SELECT title FROM entries WHERE title = 'Alter Test'`).Scan(&title); err != nil {
				t.Errorf("Eintrag fehlt nach Migration: %v", err)
			}
			if hasPatients {
				var name string
				if err := db2.QueryRow(`SELECT name FROM patients WHERE name = 'Altbestand'`).Scan(&name); err != nil {
					t.Errorf("Patient fehlt nach Migration: %v", err)
				}
			}

			// Kopf-Schema vorhanden (Stichproben über die Versionen hinweg).
			for _, col := range []struct{ table, column string }{
				{"patients", "address"},
				{"patients", "payer_bg"},
				{"users", "totp_secret"},
				{"users", "totp_enabled"},
			} {
				if !hasColumn(t, db2, col.table, col.column) {
					t.Errorf("Spalte %s.%s fehlt", col.table, col.column)
				}
			}
			if !hasTable(t, db2, "totp_recovery") {
				t.Error("Tabelle totp_recovery fehlt")
			}

			// Die migrierte Datenbank ist benutzbar (Foreign Keys aktiv, Schreibzugriff).
			if _, err := db2.Exec(`INSERT INTO patients (name) VALUES ('Nach Migration')`); err != nil {
				t.Errorf("Schreiben nach Migration: %v", err)
			}
			if _, err := db2.Exec(`PRAGMA foreign_keys = ON`); err != nil {
				t.Fatal(err)
			}
			var fk int
			if err := db2.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
				t.Errorf("foreign_keys = %d (%v)", fk, err)
			}
		})
	}
}

func versionName(k int) string {
	return "von Version " + strconv.Itoa(k)
}

func hasColumn(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func hasTable(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
		return false
	}
	return n > 0
}
