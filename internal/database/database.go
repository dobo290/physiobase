// Package database öffnet die SQLite-Datei und spielt die Migrationen ein.
package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // reiner Go-Treiber, kein CGO
)

// Open öffnet (oder erzeugt) die Datenbank. Die Pragmas stehen im DSN, damit sie
// für jede Verbindung gelten – foreign_keys ist in SQLite pro Verbindung.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("db-ordner anlegen: %w", err)
	}
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db öffnen: %w", err)
	}
	// Eine Verbindung: keine SQLITE_BUSY-Konflikte zwischen Schreibern. Für eine
	// Ein-Personen-Anwendung mehr als schnell genug.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration: %w", err)
	}
	return db, nil
}

// Close schreibt das WAL zurück in die Hauptdatei und schließt die Datenbank.
func Close(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		db.Close()
		return fmt.Errorf("wal checkpoint: %w", err)
	}
	return db.Close()
}

// Migrationen werden nur angehängt, nie geändert. Jede läuft in einer eigenen
// Transaktion zusammen mit dem Eintrag in schema_migrations.
var migrations = []struct {
	version int
	sql     string
}{
	{1, `
CREATE TABLE entries (
    id           INTEGER PRIMARY KEY,
    kind         TEXT NOT NULL,
    title        TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    body         TEXT NOT NULL DEFAULT '',
    region       TEXT NOT NULL DEFAULT '',
    tags         TEXT NOT NULL DEFAULT '',
    dosage       TEXT NOT NULL DEFAULT '',
    patient_text TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at   INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_entries_kind ON entries(kind);

-- Volltextsuche; remove_diacritics: "Ubung" findet "Übung"
CREATE VIRTUAL TABLE entries_fts USING fts5(
    title, summary, tags, region, body, patient_text,
    content='entries', content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);
CREATE TRIGGER entries_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, title, summary, tags, region, body, patient_text)
    VALUES (new.id, new.title, new.summary, new.tags, new.region, new.body, new.patient_text);
END;
CREATE TRIGGER entries_ad AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, title, summary, tags, region, body, patient_text)
    VALUES ('delete', old.id, old.title, old.summary, old.tags, old.region, old.body, old.patient_text);
END;
CREATE TRIGGER entries_au AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, title, summary, tags, region, body, patient_text)
    VALUES ('delete', old.id, old.title, old.summary, old.tags, old.region, old.body, old.patient_text);
    INSERT INTO entries_fts(rowid, title, summary, tags, region, body, patient_text)
    VALUES (new.id, new.title, new.summary, new.tags, new.region, new.body, new.patient_text);
END;

-- ungerichtete Verknüpfung, immer mit a < b gespeichert
CREATE TABLE entry_links (
    a INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    b INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    PRIMARY KEY (a, b),
    CHECK (a < b)
) WITHOUT ROWID;
CREATE INDEX idx_links_b ON entry_links(b);

CREATE TABLE sources (
    id       INTEGER PRIMARY KEY,
    entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    ref      TEXT NOT NULL,
    page     TEXT NOT NULL DEFAULT '',
    url      TEXT NOT NULL DEFAULT '',
    note     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_sources_entry ON sources(entry_id);

CREATE TABLE media (
    id         INTEGER PRIMARY KEY,
    entry_id   INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL UNIQUE,
    caption    TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_media_entry ON media(entry_id);

CREATE TABLE handouts (
    id         INTEGER PRIMARY KEY,
    title      TEXT NOT NULL,
    recipient  TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE handout_items (
    handout_id INTEGER NOT NULL REFERENCES handouts(id) ON DELETE CASCADE,
    entry_id   INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    pos        INTEGER NOT NULL DEFAULT 0,
    dosage     TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (handout_id, entry_id)
) WITHOUT ROWID;

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) WITHOUT ROWID;
`},
	{2, `
CREATE TABLE patients (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    dob        TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    archived   INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_patients_name ON patients(name COLLATE NOCASE);

-- Befund: feste Spalten für Liste/Sortierung, Inhalt als JSON (Felder, Tabellen, Körperschema).
-- Die Feldschlüssel entsprechen der alten Befundseite, alte JSON-Exporte lassen sich importieren.
CREATE TABLE befunde (
    id         INTEGER PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    date       TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT 'Befund',
    data       TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_befunde_patient ON befunde(patient_id, date);

-- Behandlungsverlauf: gehört zum Patienten, nicht zu einem einzelnen Befund
CREATE TABLE sessions (
    id         INTEGER PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    date       TEXT NOT NULL,
    goal       TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_sessions_patient ON sessions(patient_id, date);

-- Wissen ↔ Patient (z. B. Krankheitsbild, Protokoll)
CREATE TABLE patient_entries (
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    entry_id   INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    PRIMARY KEY (patient_id, entry_id)
) WITHOUT ROWID;

ALTER TABLE handouts ADD COLUMN patient_id INTEGER REFERENCES patients(id) ON DELETE SET NULL;
CREATE INDEX idx_handouts_patient ON handouts(patient_id);
`},
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}
	var current int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("version %d: %w", m.version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
