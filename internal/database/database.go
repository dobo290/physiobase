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
	{3, `
-- Eingescannte/fotografierte Unterlagen je Patient (Arztbrief, Verordnung, Bildgebung …)
CREATE TABLE patient_docs (
    id         INTEGER PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL UNIQUE,
    original   TEXT NOT NULL DEFAULT '',
    title      TEXT NOT NULL DEFAULT '',
    category   TEXT NOT NULL DEFAULT '',
    date       TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_patient_docs_patient ON patient_docs(patient_id, date);
`},
	{4, `
-- Zusammenfassungen für Patienten: gesamt oder zu einer Behandlung
CREATE TABLE summaries (
    id         INTEGER PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    session_id INTEGER REFERENCES sessions(id) ON DELETE SET NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('gesamt', 'sitzung')),
    date       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_summaries_patient ON summaries(patient_id, date);
`},
	{5, `
ALTER TABLE patients ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE patients ADD COLUMN email_consent INTEGER NOT NULL DEFAULT 0;
-- Versandprotokoll (auch Fehlversuche)
CREATE TABLE mail_log (
    id         INTEGER PRIMARY KEY,
    summary_id INTEGER REFERENCES summaries(id) ON DELETE SET NULL,
    patient_id INTEGER REFERENCES patients(id) ON DELETE CASCADE,
    recipient  TEXT NOT NULL,
    subject    TEXT NOT NULL,
    ok         INTEGER NOT NULL,
    error      TEXT NOT NULL DEFAULT '',
    sent_at    INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_mail_log_summary ON mail_log(summary_id);
`},
	{6, `
CREATE TABLE therapists (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Termine. uid + sequence für den Kalender-Feed (Änderungen werden von Kalender-Apps erkannt).
-- note ist ein Hinweis FÜR den Patienten und erscheint im Feed und auf dem Ausdruck.
CREATE TABLE appointments (
    id           INTEGER PRIMARY KEY,
    uid          TEXT NOT NULL UNIQUE,
    patient_id   INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    therapist_id INTEGER REFERENCES therapists(id) ON DELETE SET NULL,
    starts_at    INTEGER NOT NULL,
    duration_min INTEGER NOT NULL DEFAULT 30,
    title        TEXT NOT NULL DEFAULT 'Physiotherapie',
    location     TEXT NOT NULL DEFAULT '',
    note         TEXT NOT NULL DEFAULT '',
    cancelled    INTEGER NOT NULL DEFAULT 0,
    sequence     INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at   INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_appointments_patient ON appointments(patient_id, starts_at);
CREATE INDEX idx_appointments_start ON appointments(starts_at);

-- Geheimer Link für das Kalender-Abo des Patienten (NULL = noch keiner)
ALTER TABLE patients ADD COLUMN cal_token TEXT;
CREATE UNIQUE INDEX idx_patients_cal_token ON patients(cal_token) WHERE cal_token IS NOT NULL;
`},
	{7, `
-- Benutzer (vorher: ein einziger Login aus der .env – der wird beim ersten Start zum Admin)
CREATE TABLE users (
    id              INTEGER PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE COLLATE NOCASE,
    display_name    TEXT NOT NULL DEFAULT '',
    password_hash   TEXT NOT NULL DEFAULT '',          -- leer = nur Passkey
    role            TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    active          INTEGER NOT NULL DEFAULT 1,
    session_version INTEGER NOT NULL DEFAULT 1,        -- erhöhen = alle Sitzungen dieses Benutzers beenden
    webauthn_id     BLOB NOT NULL,                     -- zufällige Kennung für Passkeys (kein Name, keine ID)
    created_at      INTEGER NOT NULL DEFAULT (unixepoch()),
    last_login_at   INTEGER
);

-- Passkeys (WebAuthn): data = gespeicherte Credential-Struktur (öffentlicher Schlüssel, Zähler …)
CREATE TABLE passkeys (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BLOB NOT NULL UNIQUE,
    data          TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL DEFAULT (unixepoch()),
    last_used_at  INTEGER
);
CREATE INDEX idx_passkeys_user ON passkeys(user_id);

-- Einmal-Links zum Einrichten (Passkey/Passwort) – gespeichert wird nur der Hash
CREATE TABLE setup_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
`},
	{8, `
-- Fragebogen-/Schema-Attribut für Tests (JSON im Bereich befund.Questionnaire)
ALTER TABLE entries ADD COLUMN schema TEXT NOT NULL DEFAULT "";
`},
	{9, `
-- Tests, die einem Patienten zugeordnet sind (unabhaengig von einem Befund)
CREATE TABLE patient_tests (
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    entry_id   INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (patient_id, entry_id)
) WITHOUT ROWID;

-- Einzeldurchfuehrungen eines Tests (ohne Befund)
CREATE TABLE test_runs (
    id         INTEGER PRIMARY KEY,
    patient_id INTEGER NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    entry_id   INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    date       TEXT NOT NULL,
    data       TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_test_runs_patient ON test_runs(patient_id, entry_id, date);
`},
	{10, `
-- Erweiterte Stammdaten
ALTER TABLE patients ADD COLUMN address TEXT NOT NULL DEFAULT '';
ALTER TABLE patients ADD COLUMN job TEXT NOT NULL DEFAULT '';
ALTER TABLE patients ADD COLUMN doctor TEXT NOT NULL DEFAULT '';
ALTER TABLE patients ADD COLUMN doctors TEXT NOT NULL DEFAULT '';
ALTER TABLE patients ADD COLUMN insurance TEXT NOT NULL DEFAULT '';
ALTER TABLE patients ADD COLUMN payer_self INTEGER NOT NULL DEFAULT 0;
ALTER TABLE patients ADD COLUMN payer_private INTEGER NOT NULL DEFAULT 0;
ALTER TABLE patients ADD COLUMN payer_beihilfe INTEGER NOT NULL DEFAULT 0;
ALTER TABLE patients ADD COLUMN payer_bg INTEGER NOT NULL DEFAULT 0;
`},
	{11, `
-- Optionale Zwei-Faktor-Anmeldung (TOTP): Geheimnis bis zur Bestaetigung, dann aktiv
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0;

-- Wiederherstellungscodes: nur als Hash gespeichert, jeder genau einmal nutzbar
CREATE TABLE totp_recovery (
    id        INTEGER PRIMARY KEY,
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at   INTEGER
);
CREATE INDEX idx_totp_recovery_user ON totp_recovery(user_id);
`},
	{12, `
-- Behandler, der zu einem Benutzer gehoert ("nur meine Spalte" im Kalender).
ALTER TABLE users ADD COLUMN therapist_id INTEGER REFERENCES therapists(id) ON DELETE SET NULL;
CREATE INDEX idx_appointments_therapist ON appointments(therapist_id, starts_at);
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
	// Schutz gegen Rueckschritt: Eine Datenbank, die von einer neueren Version
	// geschrieben wurde, kennt Spalten, die dieses Binary nicht anlegt. Statt spaeter
	// mit kryptischen SQL-Fehlern zu scheitern, hier klar abbrechen.
	if head := len(migrations); current > head {
		return fmt.Errorf("die Datenbank hat Schemaversion %d, dieses Programm kennt nur %d – bitte die neuere PhysioBase-Version starten", current, head)
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
