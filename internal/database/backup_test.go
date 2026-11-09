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
