package database

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupConsistentAndPrune(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer Close(db)
	if _, err := db.Exec(`INSERT INTO patients (name) VALUES ('Test')`); err != nil {
		t.Fatal(err)
	}
	bdir := filepath.Join(dir, "backup")
	name, err := Backup(context.Background(), db, bdir)
	if err != nil {
		t.Fatal(err)
	}
	// Backup muss ohne -wal vollständig lesbar sein
	bdb, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	bdb.QueryRow(`SELECT count(*) FROM patients`).Scan(&n)
	bdb.Close()
	if n != 1 {
		t.Fatalf("backup enthält %d Patienten", n)
	}
	for _, f := range []string{"physiobase-2020-01-01_000000.db", "physiobase-2020-01-02_000000.db"} {
		os.WriteFile(filepath.Join(bdir, f), nil, 0o600)
	}
	Prune(bdir, 2)
	files, _ := filepath.Glob(filepath.Join(bdir, "physiobase-*.db"))
	if len(files) != 2 || filepath.Base(files[1]) != filepath.Base(name) {
		t.Fatalf("prune: %v", files)
	}
	_ = slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestRestoreRoundtrip: Backup zurückholen (Datei an neuen Ort) und die
// wiederhergestellte Datenbank muss vollständig, integrabel und beschreibbar sein.
// Das ist der Weg, den eine Praxis im Notfall geht.
func TestRestoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "live.db")
	db, err := Open(live)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(db)

	// Etwas Realistisches anlegen: Patient, Befund, Eintrag.
	if _, err := db.Exec(`INSERT INTO patients (name, dob) VALUES ('Restore Test', '03.04.1970')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO entries (kind, title) VALUES ('test', 'Restore Test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO befunde (patient_id, date, title, data) VALUES (1, '2026-05-01', 'Erstbefund', '{}')`); err != nil {
		t.Fatal(err)
	}

	name, err := Backup(context.Background(), db, filepath.Join(dir, "backup"))
	if err != nil {
		t.Fatal(err)
	}

	// Wiederherstellen: Snapshot an den Platz der laufenden Datenbank kopieren.
	restored := filepath.Join(dir, "restored", "physiobase.db")
	if err := os.MkdirAll(filepath.Dir(restored), 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(restored, b, 0o600); err != nil {
		t.Fatal(err)
	}

	rdb, err := Open(restored)
	if err != nil {
		t.Fatalf("wiederhergestellte Datenbank öffnet nicht: %v", err)
	}
	defer Close(rdb)

	var integrity string
	if err := rdb.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q (%v)", integrity, err)
	}
	var patients, befunde, entries int
	if err := rdb.QueryRow(`SELECT count(*) FROM patients`).Scan(&patients); err != nil {
		t.Fatal(err)
	}
	if err := rdb.QueryRow(`SELECT count(*) FROM befunde`).Scan(&befunde); err != nil {
		t.Fatal(err)
	}
	if err := rdb.QueryRow(`SELECT count(*) FROM entries`).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if patients != 1 || befunde != 1 || entries != 1 {
		t.Fatalf("wiederhergestellt: %d Patienten, %d Befunde, %d Einträge", patients, befunde, entries)
	}
	// Schreibbar bleibt sie auch (WAL neu aufgebaut).
	if _, err := rdb.Exec(`INSERT INTO patients (name) VALUES ('Nach Restore')`); err != nil {
		t.Fatalf("Schreiben nach Restore: %v", err)
	}
}
